package io.github.assadigit.younblock

import android.app.Activity
import android.app.AlertDialog
import android.content.Context
import android.content.Intent
import android.net.VpnService
import android.os.Bundle
import android.util.Log
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import mobile.Mobile

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
     * UI or a missed event.
     */
    private fun reconcile() {
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
        val intent = Intent(this, YounblockVpnService::class.java)
        startForegroundService(intent)
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

    companion object {
        private const val TAG = "Younblock"
        private const val REQUEST_PREPARE = 100
        private const val PREFS = "m5_lifecycle"
        private const val KEY_EXPLAINED = "consent_explained"

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
            engine = created
            return created
        }
    }
}
