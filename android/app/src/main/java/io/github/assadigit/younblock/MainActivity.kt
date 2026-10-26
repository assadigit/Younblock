package io.github.assadigit.younblock

import android.app.Activity
import android.app.AlertDialog
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.util.Log
import android.webkit.JavascriptInterface
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import mobile.Mobile
import kotlin.system.exitProcess

/**
 * The shell (docs/android-port 05 task 6, ADR-0005): one Activity, one
 * WebView, the engine in-process behind the gomobile bridge. The page is
 * served by the in-process loopback webui — the same server behind the
 * Linux web edition since v0.7.2 (D50) — so the existing Svelte bundle
 * runs unchanged and talks JSON+SSE to it. No Wails, no NDK at this layer.
 *
 * M5 owns the consent flow (doc 04 §6, doc 05 §M5) on top of M4's wiring:
 * the page's toggle starts the engine, the state listener here runs the
 * FIRST-RUN EXPLANATION (once) → [VpnService.prepare] → the system
 * dialog → the foreground service; a DENIAL stops the engine (which is
 * held at the platform start gate, D77, so it never reached "on") and
 * shows the honest denial copy. One consent request is in flight at a
 * time ([consentPending], process-wide so rotation cannot double-launch
 * it), and the service start is single-flight through its own running
 * flag.
 *
 * Reconciliation (doc 04 §5: "UI re-projects from GetState on next
 * resume, never a stale Connected"): [onResume] re-derives what should
 * be running — a connecting engine with no service gets the service
 * path restarted; a serving engine with no service (a state the gate
 * makes unreachable, kept as defense in depth) is stopped, because
 * without the service nothing routes and "Connected" would overstate.
 *
 * The bridge's threading contract (doc 04 §4): gomobile calls run on
 * the calling thread; engine events arrive on the emitting goroutine —
 * [EngineEvents] fans them out, and this Activity's listener marshals
 * to the main thread before touching UI or starting services.
 */
class MainActivity : Activity() {

    private lateinit var webView: WebView

    /** Reacts to engine state: connect flow on leaving "off". */
    private val stateListener = { name: String, json: String ->
        if (name == "state") {
            val phase = org.json.JSONObject(json).optString("phase", "")
            if (phase.isNotEmpty() && phase != "off" && phase != "stopping") {
                runOnUiThread { ensureVpnService() }
            }
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val engine = EngineHolder.ensure(this)

        webView = WebView(this)
        webView.settings.apply {
            javaScriptEnabled = true   // the Svelte app is the whole UI
            allowFileAccess = false    // doc 07 §6 hardening: local page only
            allowContentAccess = false
        }
        // The any-version floor (the 1.6.7-debug.2 round): Android 15+
        // draws targetSdk 35+ windows edge-to-edge — this theme's bar
        // colors are ignored there, and without this line the WebView
        // would extend UNDER the status and navigation bars (the
        // Svelte screen's top partially obscured on the newest phones —
        // the exact inverse of the old-phone gaps this round closes).
        // fitsSystemWindows keeps the content between the bars on 15/16
        // and is unchanged behavior on 8–14; the keyboard's adjustResize
        // rides the same classic path on every version.
        webView.fitsSystemWindows = true
        // M8-p (ADR-0015): the page's copy/share escape hatch. The
        // interface is deliberately minimal — two fire-and-forget actions
        // with CALLER-provided text, nothing that reads data back out —
        // and the only page that can ever reach it is the engine's own
        // loopback webui ([LoopbackOnlyClient] refuses every navigation
        // elsewhere).
        webView.addJavascriptInterface(PageBridge(this), "YounblockShell")
        webView.webViewClient = LoopbackOnlyClient
        setContentView(webView)

        // Port 0: the webui takes a free loopback port and hands back the
        // URL; the WebView loads it. Cleartext to 127.0.0.1 is permitted by
        // the network security config and nothing else.
        val url = engine.startWebUI(0)
        Log.i(TAG, "webui at $url")
        webView.loadUrl(url)

        EngineEvents.addListener(stateListener)
    }

    override fun onResume() {
        super.onResume()
        reconcile()
    }

    /**
     * State reconciliation on resume (doc 04 §5, M5): the truth is
     * re-derived from the engine + service, never trusted from a stale
     * UI or a missed event. The quit gate (D112) is defense in depth:
     * once a confirmed Quit is in flight, reconciliation must not
     * re-derive itself into a service start.
     */
    private fun reconcile() {
        if (YounblockVpnService.quitting) return
        val engine = EngineHolder.engine ?: return
        val phase = org.json.JSONObject(engine.state).optString("phase", "off")
        when {
            // The service owns the live cycle; nothing to fix.
            YounblockVpnService.running -> return
            // A connect attempt whose service is gone (system killed it):
            // re-run the consent→service path. The engine is held at the
            // start gate meanwhile — "connecting", never a false Connected.
            phase == "connecting" && !consentPending -> ensureVpnService()
            // Serving without the service cannot route: the gate makes
            // this unreachable, but if it ever happens the honest answer
            // is disconnect, not a stale "Connected".
            phase == "on" || phase == "degraded" -> {
                Log.w(TAG, "engine serving without the service; stopping (nothing routes)")
                Log.i(TAG, "engine stop: ${engine.stop()}")
            }
        }
    }

    /**
     * The connect flow: consent check first (the first-run explanation,
     * then the system dialog), then the foreground service. Consent is
     * single-flight: while one request is pending, nothing re-launches
     * it (engine state events fire repeatedly during connecting).
     */
    private fun ensureVpnService() {
        if (YounblockVpnService.quitting) return // D112: a quit in flight owns the process
        if (YounblockVpnService.running) return
        if (consentPending) return
        val prepare = VpnService.prepare(this)
        if (prepare != null) {
            if (!explainedOnce(this)) {
                showExplanation(prepare)
            } else {
                launchConsent(prepare)
            }
            return
        }
        startVpnService()
    }

    /** The first-run explanation (doc 04 §6: "first-run explanation once"). */
    private fun showExplanation(prepare: Intent) {
        markExplained(this)
        AlertDialog.Builder(this)
            .setTitle(R.string.consent_explanation_title)
            .setMessage(R.string.consent_explanation_text)
            .setPositiveButton(R.string.consent_continue) { _, _ -> launchConsent(prepare) }
            .setNegativeButton(R.string.consent_not_now) { _, _ -> onConsentAbandoned() }
            .setOnCancelListener { onConsentAbandoned() }
            .show()
    }

    /** Launches the system consent dialog; marks the request in flight. */
    private fun launchConsent(prepare: Intent) {
        consentPending = true
        if (!runCatching { startActivityForResult(prepare, REQUEST_PREPARE) }.isSuccess) {
            // The dialog could not even launch (activity finishing): treat
            // it as a denial — the engine must not sit held at the gate.
            consentPending = false
            onConsentAbandoned()
        }
    }

    private fun startVpnService() {
        if (YounblockVpnService.quitting) return // D112: never (re)start into a quitting process
        maybeRequestNotificationConsent()
        val intent = Intent(this, YounblockVpnService::class.java)
        // debug.2 hardening: Android 12+ can refuse a foreground start
        // whose caller is no longer eligible (the state listener fires
        // this from a "connecting" event while the user may already have
        // left the page — ForegroundServiceStartNotAllowedException).
        // A refusal is the honest stop, never a crash: the engine
        // leaves its gate-held "connecting" (no zombie — the same rule
        // as onDestroy), the process survives, and onResume's
        // reconciliation re-derives the state when the user returns.
        runCatching { startForegroundService(intent) }
            .onFailure {
                Log.w(TAG, "foreground service start refused: ${it.message}")
                EngineHolder.engine?.let { eng ->
                    Log.i(TAG, "engine stop (service start refused): ${eng.stop()}")
                }
            }
    }

    /**
     * The 13+ notification ask (the any-version round): Android 13 hides
     * every notification — including the VPN service's honest Active/
     * Connecting status — unless POST_NOTIFICATIONS is granted at runtime.
     * The tunnel itself never depends on it (the service runs either way,
     * 13+ shows it in the drawer's running-apps section), but a VPN whose
     * status nobody can see reads as broken. Asked once per install, at
     * the moment the user has just committed to connecting (after the VPN
     * consent — the dialogs never stack); a denial costs only the
     * notification, never the tunnel, so the result is deliberately not
     * inspected. Below 13 the permission does not exist and the ask is
     * skipped — the manifest declaration carries it there.
     */
    private fun maybeRequestNotificationConsent() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) return
        if (notifAskedOnce(this)) return
        markNotifAsked(this)
        runCatching {
            requestPermissions(
                arrayOf(android.Manifest.permission.POST_NOTIFICATIONS),
                REQUEST_POSTS,
            )
        }
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == REQUEST_PREPARE) {
            consentPending = false
            if (resultCode == RESULT_OK) {
                startVpnService()
            } else {
                // Consent denied (doc 05 M5): nothing routes, the engine
                // stops out of its gate-held "connecting", and the user
                // sees why in honest words.
                onConsentDenied()
            }
        }
    }

    /** The user refused the explanation dialog (or it was cancelled). */
    private fun onConsentAbandoned() {
        EngineHolder.engine?.let {
            Log.i(TAG, "engine stop (consent abandoned): ${it.stop()}")
        }
    }

    /** The system consent dialog was denied: stop + honest copy. */
    private fun onConsentDenied() {
        Log.i(TAG, "VPN consent denied by the user")
        EngineHolder.engine?.let {
            Log.i(TAG, "engine stop (consent denied): ${it.stop()}")
        }
        AlertDialog.Builder(this)
            .setTitle(R.string.consent_denied_title)
            .setMessage(R.string.consent_denied_text)
            .setPositiveButton(R.string.consent_retry) { _, _ -> ensureVpnService() }
            .setNegativeButton(R.string.consent_ok, null)
            .show()
    }

    /**
     * The back button's page half (the 1.6.7-debug.3 polish round): the
     * screen's modal surfaces — the menu sheet, the diagnostics view —
     * close on BACK before the system default (leaving the app) runs.
     * The shell probes the page's YounblockUI (lib/shellback.ts) with
     * evaluateJavascript; "true" means the page closed a modal and the
     * press is consumed. Every other result — no modal open, the page
     * still loading, a WebView that refused the probe — falls through
     * to the default back exactly as before this round: the behavior
     * is only ever additive, never broken. (Browser-initiated script is
     * not the page's own eval, so the page's CSP script-src 'self'
     * does not apply to the probe.)
     *
     * The manifest does not opt into OnBackInvokedCallback, so this
     * classic override fires on every version 8–16.
     */
    @Deprecated("Deprecated in Java")
    override fun onBackPressed() {
        webView.evaluateJavascript(BACK_PROBE) { consumed ->
            if (consumed != "true") defaultBack()
        }
    }

    @Suppress("DEPRECATION")
    private fun defaultBack() {
        super.onBackPressed()
    }

    /**
     * The "Quit Younblock" orchestration (D112) — the page's confirmed
     * Quit lands here (PageBridge.quitApp, marshaled to the main
     * thread), and the whole app ends, in order:
     *
     *  a-b-c-d. [YounblockVpnService.quit] runs the service's ordered
     *     teardown: engine + proxy stopped, TUN closed, VpnService
     *     stopped, foreground out, notification cancelled by ID, the
     *     start gate re-armed — and the [YounblockVpnService.quitting]
     *     flag it raises first is the restart-cancel half: the connect
     *     flow, resume reconciliation and any raced service start are
     *     all gated (this app has no boot receiver, jobs, alarms or
     *     Quick Settings tile to cancel, and the service was already
     *     START_NOT_STICKY — the manifest is the proof).
     *  e. the webui server stops (the page's own loopback server — the
     *     bridge call that triggered this has already returned) and the
     *     activity closes with [finishAndRemoveTask], taking the app out
     *     of Recents.
     *  f. the process ends: the Go runtime's threads and the WebView's
     *     render threads never all return, so after the teardown has
     *     settled (the service stop, the activity destroy, the engine's
     *     trailing state events — all main-thread messages queued ahead
     *     of this timer) the process is killed, with exitProcess(0) as
     *     the belt-and-braces fallback.
     *
     * Disconnect stays exactly what it was — the lever, the engine's
     * own stop path, the notification's life: nothing in this flow
     * touches them. The next manual launch is a fresh process: the
     * engine constructs, the page loads, the notification appears on
     * connect — the first-open behavior, unchanged.
     */
    private fun quitYounblock() {
        Log.i(TAG, "quit: the confirmed end-of-app sequence begins")
        // (a-d) the service-side ordered teardown (or the no-service
        // end-state: gate re-armed, notification cancelled by ID).
        YounblockVpnService.quit(this)
        // (e) the page's own server down, then the activity + Recents gone.
        EngineHolder.engine?.stopWebUI()
        finishAndRemoveTask()
        // (f) the process ends once the queued teardown settles.
        Handler(Looper.getMainLooper()).postDelayed({
            Log.i(TAG, "quit: teardown settled; ending the process")
            runCatching { android.os.Process.killProcess(android.os.Process.myPid()) }
            exitProcess(0)
        }, QUIT_KILL_DELAY_MS)
    }

    override fun onDestroy() {
        // The page goes down with the activity; the engine instance stays
        // for the process lifetime (the service keeps it if running).
        EngineEvents.removeListener(stateListener)
        if (!YounblockVpnService.running) {
            val engine = EngineHolder.engine
            engine?.stopWebUI()
            // M5: with no UI and no service, a gate-held "connecting" would
            // be an invisible zombie — the connect attempt does not
            // outlive the shells. (Rotation with the consent dialog up is
            // safe: the result is delivered to the new instance, and the
            // service restarts the engine itself.)
            engine?.let {
                val phase = org.json.JSONObject(it.state).optString("phase", "off")
                if (phase == "connecting") {
                    Log.i(TAG, "engine stop (activity gone, nothing to connect): ${it.stop()}")
                }
            }
        }
        super.onDestroy()
    }

    /**
     * Only the in-process loopback origin may render; anything else the
     * page could ever navigate to is refused (doc 07 §6: no remote pages).
     */
    private object LoopbackOnlyClient : WebViewClient() {
        override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
            val u = request.url
            return !(u.scheme == "http" && u.host == "127.0.0.1")
        }
    }

    /**
     * The page's escape hatch (M8-p, ADR-0015): the Svelte Diagnostics
     * screen reaches this object as window.YounblockShell. The Android
     * WebView is the one host where navigator.clipboard is unreliable
     * (missing or rejecting depending on the WebView build), so the
     * report's Copy button tries this NATIVE path first and the page's
     * clipboard ladder falls through gracefully when the bridge is
     * absent (the desktops and the Linux web edition never see it).
     *
     * Threading: [JavascriptInterface] methods arrive on a WebView binder
     * thread. ClipboardManager is a binder call (thread-safe); the share
     * intent marshals to the main thread before startActivity.
     *
     * Security surface (kept minimal by design): both methods are
     * write-only with caller-provided text — the shell never returns data
     * to the page, and only the engine's own loopback page can load here
     * at all (see [LoopbackOnlyClient]).
     */
    private class PageBridge(private val activity: MainActivity) {

        @JavascriptInterface
        fun copyToClipboard(text: String): Boolean {
            return try {
                val cm = activity.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
                cm.setPrimaryClip(ClipData.newPlainText("Younblock diagnostics", text))
                true
            } catch (e: RuntimeException) {
                Log.w(TAG, "clipboard write failed: ${e.message}")
                false
            }
        }

        @JavascriptInterface
        fun shareText(text: String): Boolean {
            return try {
                activity.runOnUiThread {
                    try {
                        val send = Intent(Intent.ACTION_SEND).apply {
                            type = "text/plain"
                            putExtra(Intent.EXTRA_TEXT, text)
                        }
                        activity.startActivity(
                            Intent.createChooser(send, activity.getString(R.string.share_diagnostics))
                        )
                    } catch (e: RuntimeException) {
                        Log.w(TAG, "share sheet failed: ${e.message}")
                    }
                }
                true // the sheet was dispatched; its own failure is logged above
            } catch (e: RuntimeException) {
                Log.w(TAG, "share dispatch failed: ${e.message}")
                false
            }
        }

        /**
         * The "Quit Younblock" door (D112): the page's confirmed Quit
         * hands the whole end-of-app sequence to the shell. Arrives on
         * the binder thread like every method here; the sequence itself
         * marshals to the main thread (where the service, the engine
         * calls and the activity lifecycle all live) and this returns
         * immediately — the page is torn down with the app moments
         * later, so there is nothing left to answer. Write-only like the
         * rest of this surface: it launches the teardown, reads nothing
         * back out.
         */
        @JavascriptInterface
        fun quitApp(): Boolean {
            return try {
                activity.runOnUiThread { activity.quitYounblock() }
                true // the teardown was dispatched; the app ends from here
            } catch (e: RuntimeException) {
                Log.w(TAG, "quit dispatch failed: ${e.message}")
                false
            }
        }

        /**
         * UI v3 (docs/ui-v3/UI-SPEC.md §6): the connected state's Open
         * YouTube card. Launches the YouTube app (the package's launch
         * intent with NEW_TASK — the same open the connect-time hand-off
         * uses); when the app is not installed, the mobile site opens in
         * the browser instead. Where no shell exists (the Linux web
         * edition, plain browsers) the page itself falls back to
         * window.open, so this bridge is an accelerator, not a dependency.
         *
         * Read-only like the rest of this surface: the method launches
         * and returns, never reading anything back out. Arrives on the
         * binder thread; the launch marshals to the main thread.
         */
        @JavascriptInterface
        fun openYouTube(): Boolean {
            return try {
                activity.runOnUiThread {
                    try {
                        val launch = activity.packageManager.getLaunchIntentForPackage(YOUTUBE_PACKAGE)
                        if (launch != null) {
                            launch.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                            activity.startActivity(launch)
                        } else {
                            Log.i(TAG, "openYouTube: $YOUTUBE_PACKAGE not installed; opening the browser")
                            val web = Intent(Intent.ACTION_VIEW, android.net.Uri.parse("https://m.youtube.com"))
                            web.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                            activity.startActivity(web)
                        }
                    } catch (e: RuntimeException) {
                        Log.w(TAG, "openYouTube failed: ${e.message}")
                    }
                }
                true // the launch was dispatched; its own failure is logged above
            } catch (e: RuntimeException) {
                Log.w(TAG, "openYouTube dispatch failed: ${e.message}")
                false
            }
        }
    }

    companion object {
        private const val TAG = "Younblock"
        private const val REQUEST_PREPARE = 100
        private const val REQUEST_POSTS = 101
        /** The quit's process-kill grace (D112): long enough for the
         *  service stop, the activity destroy and the engine's trailing
         *  state events (all main-thread messages queued ahead of the
         *  timer) to settle; short enough that "the app is gone" is
         *  what the user sees. */
        private const val QUIT_KILL_DELAY_MS = 600L
        /** The back probe: returns true only when the page closed a
         *  modal (window.YounblockUI is exposed the moment the bundle
         *  boots — see frontend/src/lib/shellback.ts). */
        private const val BACK_PROBE = "window.YounblockUI&&window.YounblockUI.back()"
        /** The Open YouTube card's target (UI v3 spec §6) — the same
         *  package the connect-time hand-off opens. */
        private const val YOUTUBE_PACKAGE = "com.google.android.youtube"
        private const val PREFS = "m5_lifecycle"
        private const val KEY_EXPLAINED = "consent_explained"
        private const val KEY_NOTIF_ASKED = "posts_notifications_asked"

        private fun notifAskedOnce(context: Context): Boolean =
            context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
                .getBoolean(KEY_NOTIF_ASKED, false)

        private fun markNotifAsked(context: Context) {
            context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
                .edit().putBoolean(KEY_NOTIF_ASKED, true).apply()
        }

        /**
         * One consent request in flight, process-wide (survives rotation;
         * process death resets it together with the engine itself).
         */
        @Volatile
        private var consentPending: Boolean = false

        private fun explainedOnce(context: Context): Boolean =
            context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
                .getBoolean(KEY_EXPLAINED, false)

        private fun markExplained(context: Context) {
            context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
                .edit().putBoolean(KEY_EXPLAINED, true).apply()
        }
    }
}

/**
 * One engine per process, created by whichever component needs it first
 * (Activity or service), always with the process-wide bridge doors
 * ([VpnProtector], [EngineEvents]) — see Bridge.kt. Recreated only if
 * the process died.
 */
object EngineHolder {
    @Volatile
    var engine: mobile.Engine? = null
        private set

    /**
     * The radio's alphabetic operator name (spec W9): a permission-free
     * TelephonyManager getter — empty off-radio or on devices that will
     * not share it, and the Go side renders unknown then.
     */
    private fun operatorName(context: Context): String =
        runCatching {
            (context.getSystemService(Context.TELEPHONY_SERVICE) as? android.telephony.TelephonyManager)
                ?.networkOperatorName.orEmpty()
        }.getOrDefault("")

    /**
     * The radio's numeric operator identity, MCC+MNC (spec W9): the same
     * permission-free getter family as [operatorName].
     */
    private fun operatorCode(context: Context): String =
        runCatching {
            (context.getSystemService(Context.TELEPHONY_SERVICE) as? android.telephony.TelephonyManager)
                ?.networkOperator.orEmpty()
        }.getOrDefault("")

    /**
     * The wifi-vs-cellular guess (spec W9, ADR-0043): ConnectivityManager
     * needs ACCESS_NETWORK_STATE, which this app deliberately does not
     * hold (the manifest's inventory says why), so the guess reads what
     * is free — the telephony data state says cellular-active, a wlan
     * interface with addresses says wifi, anything else is unknown. The
     * tester's own report words stay authoritative; this line is the
     * cross-check, and its heuristic nature is recorded in the ADR.
     */
    private fun networkTypeGuess(context: Context): String {
        val cellular = runCatching {
            (context.getSystemService(Context.TELEPHONY_SERVICE) as? android.telephony.TelephonyManager)
                ?.dataState == android.telephony.TelephonyManager.DATA_CONNECTED
        }.getOrDefault(false)
        if (cellular) return "cellular"
        val wifi = runCatching {
            java.net.NetworkInterface.getNetworkInterfaces().asSequence()
                .any { ni ->
                    ni.name.startsWith("wlan") && !ni.isLoopback && runCatching {
                        ni.interfaceAddresses.any { it.address != null }
                    }.getOrDefault(false)
                }
        }.getOrDefault(false)
        if (wifi) return "wifi"
        return "unknown"
    }

    fun ensure(context: Context): mobile.Engine {
        val existing = engine
        if (existing != null) return existing
        synchronized(this) {
            val again = engine
            if (again != null) return again
            val created = Mobile.newEngine(
                context.filesDir.absolutePath,
                VpnProtector,
                EngineEvents,
            )
            // M8-p (ADR-0015): name the exact build in every diagnostic
            // report — versionName alone stays 1.6.6 across debug rounds,
            // versionCode is what maps a pasted report to its release.
            // Best-effort: a PackageManager refusal must not block the
            // engine (the report then simply carries no shell line).
            runCatching {
                @Suppress("DEPRECATION") // the flags overload needs API 33; minSdk is 26
                val info = context.packageManager.getPackageInfo(context.packageName, 0)
                val code = if (Build.VERSION.SDK_INT >= 28) {
                    info.longVersionCode
                } else {
                    @Suppress("DEPRECATION")
                    info.versionCode.toLong()
                }
                created.setShellVersion("${info.versionName} (versionCode $code)")
            }.onFailure { Log.w("Younblock", "shell version not reported: ${it.message}") }
            // M8-af (debug.24, spec W9): the environment the consultation's
            // §1.4 called unrecorded — phone model and network identity, read
            // with NO new permissions: Build fields are free;
            // TelephonyManager's getNetworkOperatorName/getNetworkOperator
            // getters need none (they read empty when the device will not
            // share them, and the report prints unknown then); the wifi vs
            // cellular guess is a shell-owned heuristic (data state connected
            // -> cellular, else a wlan interface with addresses -> wifi, else
            // unknown) recorded as such in ADR-0043 — the tester's own words
            // in the report stay authoritative.
            runCatching {
                created.setDeviceInfo(Build.MANUFACTURER, Build.MODEL, Build.VERSION.SDK_INT.toLong().toInt())
            }.onFailure { Log.w("Younblock", "device info not reported: ${it.message}") }
            runCatching {
                created.setNetworkInfo(networkTypeGuess(context), operatorName(context), operatorCode(context))
            }.onFailure { Log.w("Younblock", "network info not reported: ${it.message}") }
            engine = created
            return created
        }
    }
}
