package io.github.assadigit.younblock

import android.util.Log
import mobile.EventSink
import mobile.SocketProtector

/**
 * The process-wide bridge objects (docs/android-port 04 §4/§6, roadmap
 * M4). The engine is constructed once per process — by whichever of
 * MainActivity / YounblockVpnService needs it first — with these two
 * singletons as its two doors; every component that cares about engine
 * state or socket protection attaches to them instead of holding its
 * own reference into the bridge.
 *
 * Threading (the bridge contract, doc 04 §4): events arrive on the Go
 * goroutine's attached thread; [EngineEvents] fans them out synchronously
 * and must therefore never block — listeners marshal to their own
 * threads. A throwing listener is isolated (runCatching): one bad
 * listener must not starve the others, mirroring the Go side's
 * protectedSink recover.
 */
object EngineEvents : EventSink {

    private const val TAG = "Younblock"

    private val listeners = mutableListOf<(name: String, json: String) -> Unit>()

    fun addListener(listener: (name: String, json: String) -> Unit) {
        synchronized(listeners) { listeners.add(listener) }
    }

    fun removeListener(listener: (name: String, json: String) -> Unit) {
        synchronized(listeners) { listeners.remove(listener) }
    }

    override fun onEvent(name: String, json: String) {
        Log.d(TAG, "event $name $json")
        val snapshot = synchronized(listeners) { listeners.toList() }
        for (l in snapshot) {
            runCatching { l(name, json) }
        }
    }
}

/**
 * ADR-0011's protect hook, process-wide. While the VpnService is alive
 * every engine-upstream dial goes through VpnService.protect(fd) so it
 * bypasses the TUN (no routing loop). Before the service exists — or
 * after it is gone — there IS no TUN to loop into, so passing the dial
 * through unprotected is exactly the desktop behavior and the honest
 * answer.
 *
 * The Android system additionally never routes the VPN app's own UID
 * through its TUN unless the app allows itself explicitly (it does
 * not); protect() here is the second, invariant layer.
 */
object VpnProtector : SocketProtector {

    @Volatile
    private var service: YounblockVpnService? = null

    fun attach(service: YounblockVpnService) {
        this.service = service
    }

    fun detach(service: YounblockVpnService) {
        if (this.service === service) this.service = null
    }

    override fun protect(fd: Int): Boolean {
        val s = service ?: return true // no TUN: nothing to exempt from
        return s.protect(fd)
    }
}
