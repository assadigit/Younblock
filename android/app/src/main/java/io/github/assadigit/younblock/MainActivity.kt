package io.github.assadigit.younblock

import android.app.Activity
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
 * M4 adds the connect flow (doc 04 §6): the user toggles Connect in the
 * page → the engine starts connecting → the state listener here checks
 * VPN consent ([VpnService.prepare]) → the system dialog on first
 * connect → [YounblockVpnService] starts (foreground) and binds the TUN
 * to the engine once it is serving. The full consent UX (first-run
 * explanation, denial handling, reconciliation) is M5's milestone; M4
 * wires the honest minimum: no consent, no routing.
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

    /**
     * The connect flow: consent check first (the system dialog on the
     * first connect; M5 adds the explanation step before it), then the
     * foreground service. If consent is denied the engine keeps its
     * honest phase and the user sees the page's state — nothing routes
     * without the system's explicit grant.
     */
    private fun ensureVpnService() {
        if (YounblockVpnService.running) return
        val prepare = VpnService.prepare(this)
        if (prepare != null) {
            runCatching { startActivityForResult(prepare, REQUEST_PREPARE) }
            return
        }
        startVpnService()
    }

    private fun startVpnService() {
        val intent = Intent(this, YounblockVpnService::class.java)
        startForegroundService(intent)
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == REQUEST_PREPARE) {
            if (resultCode == RESULT_OK) {
                startVpnService()
            } else {
                // Consent denied: no routing. The engine's own state
                // remains the honest truth the page shows; M5 adds the
                // explanation/denial copy on top.
                Log.i(TAG, "VPN consent denied by the user")
            }
        }
    }

    override fun onDestroy() {
        // The page goes down with the activity; the engine instance stays
        // for the process lifetime (the service keeps it if running).
        EngineEvents.removeListener(stateListener)
        if (!YounblockVpnService.running) {
            EngineHolder.engine?.stopWebUI()
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
