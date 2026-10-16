package io.github.assadigit.younblock

import android.app.Activity
import android.os.Bundle
import android.util.Log
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import mobile.EventSink
import mobile.Mobile

/**
 * The M3 shell (docs/android-port 05 task 6, ADR-0005): one Activity, one
 * WebView, the engine in-process behind the gomobile bridge. The page is
 * served by the in-process loopback webui — the same server behind the
 * Linux web edition since v0.7.2 (D50) — so the existing Svelte bundle
 * runs unchanged and talks JSON+SSE to it. No Wails, no NDK at this layer.
 *
 * No VpnService yet (that is M4): Mobile.newEngine takes a null protector,
 * which the Go side treats exactly like the desktops — there is no TUN to
 * loop into, so dialing unprotected is correct. The bridge's threading
 * contract (doc 04 §4): gomobile calls run on the calling thread; engine
 * events arrive on the emitting goroutine — [BridgeEventSink] only logs,
 * so no marshaling is needed at this milestone.
 */
class MainActivity : Activity() {

    private lateinit var webView: WebView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val engine = EngineHolder.engine
            ?: Mobile.newEngine(filesDir.absolutePath, null, BridgeEventSink())
                .also { EngineHolder.engine = it }

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
    }

    override fun onDestroy() {
        // The page goes down with the activity; the engine instance stays
        // for the process lifetime. M5 owns the full lifecycle (service,
        // reconciliation); M3 keeps it simple and honest.
        EngineHolder.engine?.stopWebUI()
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
    }
}

/** One engine per process; recreated only if the process died. */
object EngineHolder {
    var engine: mobile.Engine? = null
}

/**
 * The Kotlin end of the bridge's EventSink. The WebView page receives
 * engine events through SSE; this sink is the shell's own channel —
 * M4/M5 turn it into the service notification and state reconciliation.
 */
class BridgeEventSink : EventSink {
    override fun onEvent(name: String, json: String) {
        Log.d("Younblock", "event $name $json")
    }
}
