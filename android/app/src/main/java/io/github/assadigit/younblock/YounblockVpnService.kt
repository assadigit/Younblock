package io.github.assadigit.younblock

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.net.VpnService
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.ParcelFileDescriptor
import android.util.Log
import org.json.JSONObject

/**
 * The Android network service (docs/android-port 04 §2/§6, ADR-0009/
 * 0010/0011; roadmap M4). Foreground, `connectedDevice` type: builds
 * the TUN with the per-app allow-list, hands the fd to the engine
 * facade once the engine is serving ([mobile.Engine.startTun]), and
 * unwinds honestly on every path to "off" — user stop, engine error,
 * [onRevoke] (equal to disconnect plus re-consent on next connect).
 *
 * TUN shape (ADR-0009/0010):
 *   - IPv4 default route into the TUN for the ALLOWED apps only
 *     (default list: Chrome + YouTube; M6 makes it configurable via
 *     the `apps` setting).
 *   - Virtual DNS 10.111.0.53: answered INSIDE the stack (any UDP:53
 *     destination is — the tunstack policy), real resolution happens
 *     at the SOCKS5 door through the engine's DoH resolver.
 *   - An IPv6 route into the TUN as well, so v6 cannot bypass the
 *     tunnel (the stack blackholes it — fail-safe, visible breakage
 *     for that flow, never a leak).
 *   - MTU 1500, matching the userspace stack's default.
 *
 * Fail-closed invariant: a VpnService.Builder with ZERO allowed
 * applications routes EVERY app's traffic — the exact opposite of the
 * product scope. If none of the allow-list packages is installed the
 * service refuses to establish and stops (M6 turns this into the
 * engine's NO_TARGET_APPS error surface).
 *
 * fd ownership (the facade's single-owner rule): after
 * engine.startTun(fd, mtu) returns — success OR refusal — the Go side
 * has consumed the fd. This service therefore never closes a detached
 * fd; the only Kotlin-side close is the PFD close on the paths where
 * startTun was never called (the engine died before serving).
 */
class YounblockVpnService : VpnService() {

    /** The established TUN, held until the engine is serving. */
    private var tunPfd: ParcelFileDescriptor? = null

    /** True once startTun succeeded (the fd is Go's from then on). */
    private var tunBound = false

    private val main = Handler(Looper.getMainLooper())

    /** The engine's state events drive the TUN lifecycle (doc 04 §6). */
    private val stateListener = { name: String, json: String ->
        if (name == "state") {
            val phase = JSONObject(json).optString("phase", "")
            main.post { onEnginePhase(phase) }
        }
    }

    override fun onCreate() {
        super.onCreate()
        running = true
        VpnProtector.attach(this)
        EngineEvents.addListener(stateListener)
        createChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        // The service exists to carry traffic; if it was somehow started
        // without the engine, start it (idempotent-or-rejecting on the Go
        // side — the engine's phase guards decide, not this shell).
        val engine = EngineHolder.ensure(this)
        startForegroundHonest()

        if (JSONObject(engine.getState()).optString("phase", "off") == "off") {
            val res = engine.start()
            Log.i(TAG, "engine start: $res")
        }

        if (tunPfd == null && !tunBound) {
            establishTun()
        }
        return START_NOT_STICKY // a sticky restart would route traffic
        // without a user decision behind it — M4 stays honest; M5 owns
        // the reconciliation policy.
    }

    /** Builds the TUN (per-app allow-list, virtual DNS, v4+v6 routes). */
    private fun establishTun() {
        val builder = Builder()
            .setSession("Younblock")
            .setMtu(MTU)
            .addAddress(TUN_ADDRESS, 32)
            .addDnsServer(VIRTUAL_DNS)
            .addRoute("0.0.0.0", 0)

        // ADR-0010: route v6 into the TUN too so it cannot bypass; the
        // stack blackholes v6 (the wire drops it before any NIC sees it).
        // If a platform refuses a v6 route without a v6 address, the
        // omission is logged honestly — the on-device gate (doc 06 M4)
        // verifies the real behavior instead of a guess.
        try {
            builder.addRoute("::", 0)
        } catch (e: IllegalArgumentException) {
            Log.w(TAG, "v6 blackhole route refused: ${e.message}")
        }

        var allowed = 0
        for (pkg in DEFAULT_APPS) {
            try {
                builder.addAllowedApplication(pkg)
                allowed++
            } catch (e: PackageManager.NameNotFoundException) {
                Log.i(TAG, "allow-list target not installed: $pkg")
            }
        }
        if (allowed == 0) {
            // Fail closed: zero allowed apps would route EVERYTHING.
            Log.e(TAG, "no allow-list target installed; refusing to route")
            stopSelfHonest()
            return
        }

        val pfd = try {
            builder.establish()
        } catch (e: Exception) {
            null
        }
        if (pfd == null) {
            Log.e(TAG, "TUN establish failed (permission revoked or rejected)")
            stopSelfHonest()
            return
        }
        tunPfd = pfd
        Log.i(TAG, "TUN established (mtu $MTU, $allowed app(s) routed)")

        // The engine may already be serving (the user connected first);
        // bind now. Otherwise the state listener binds on "on".
        val phase = JSONObject(EngineHolder.ensure(this).getState())
            .optString("phase", "off")
        if (phase == "on" || phase == "degraded") {
            bindTun()
        }
    }

    /** Hands the fd to the facade (which consumes it on every path). */
    private fun bindTun() {
        val pfd = tunPfd ?: return
        if (tunBound) return
        val engine = EngineHolder.ensure(this)

        val fd = pfd.detachFd()
        val result = engine.startTun(fd, MTU)
        val parsed = JSONObject(result)
        tunPfd = null // detached: Go owns the fd now, success or refusal
        if (parsed.isNull("error")) {
            tunBound = true
            Log.i(TAG, "stack bound to door ${parsed.optString("door")}")
            startForegroundHonest(activeText = true)
        } else {
            tunBound = false
            Log.e(TAG, "startTun refused: ${parsed.optString("error")}")
            stopSelfHonest()
        }
    }

    /** Engine phase transitions (marshaled to the main thread). */
    private fun onEnginePhase(phase: String) {
        when (phase) {
            "on", "degraded" -> bindTun()
            "off", "error" -> {
                // doc 04 §8: every path to off/error ends with the TUN
                // down and no routes leaked; the UI state comes from the
                // engine itself.
                if (tunBound || tunPfd != null) {
                    unbindTun()
                }
                stopSelfHonest()
            }
            // "connecting"/"stopping": nothing to do — the TUN binds
            // when serving arrives and unwinds on the terminal phase.
        }
    }

    /** Stops the stack if bound; closes the held PFD if never bound. */
    private fun unbindTun() {
        if (tunBound) {
            EngineHolder.engine?.stopTun()
            tunBound = false
        }
        tunPfd?.let { runCatching { it.close() } } // startTun never saw it
        tunPfd = null
    }

    /** The system revoked the VPN permission mid-run. */
    override fun onRevoke() {
        Log.w(TAG, "VPN permission revoked")
        main.post {
            EngineHolder.engine?.let {
                Log.i(TAG, "engine stop: ${it.stop()}")
            }
            unbindTun()
            stopSelfHonest()
        }
    }

    override fun onDestroy() {
        EngineEvents.removeListener(stateListener)
        VpnProtector.detach(this)
        unbindTun()
        running = false
        super.onDestroy()
    }

    // ------------------------------------------------------------ plumbing

    private fun startForegroundHonest(activeText: Boolean = false) {
        val text = if (activeText) {
            getString(R.string.vpn_notification_active)
        } else {
            getString(R.string.vpn_notification_connecting)
        }
        val notification = buildNotification(text)
        if (Build.VERSION.SDK_INT >= 29) {
            startForeground(
                NOTIFICATION_ID, notification,
                ServiceInfo.FOREGROUND_SERVICE_TYPE_CONNECTED_DEVICE,
            )
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
    }

    private fun buildNotification(text: String): Notification {
        val tap = PendingIntent.getActivity(
            this, 0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE,
        )
        // minSdk 26: the channel constructor always exists.
        return Notification.Builder(this, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_launcher)
            .setContentTitle(getString(R.string.app_name))
            .setContentText(text)
            .setContentIntent(tap)
            .setOngoing(true)
            .build()
    }

    private fun createChannel() {
        if (Build.VERSION.SDK_INT < 26) return // minSdk 26: unreachable, kept for the honest check
        val channel = NotificationChannel(
            CHANNEL_ID,
            getString(R.string.vpn_notification_channel),
            NotificationManager.IMPORTANCE_LOW,
        )
        getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
    }

    private fun stopSelfHonest() {
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    companion object {
        private const val TAG = "Younblock"
        private const val CHANNEL_ID = "vpn"
        private const val NOTIFICATION_ID = 1
        private const val MTU = 1500
        private const val TUN_ADDRESS = "10.111.0.1"
        private const val VIRTUAL_DNS = "10.111.0.53"

        /** The default per-app allow-list (M6: the `apps` setting). */
        private val DEFAULT_APPS = listOf(
            "com.android.chrome",
            "com.google.android.youtube",
        )

        /** Observable liveness for the Activity's connect flow. */
        @Volatile
        var running: Boolean = false
            private set
    }
}
