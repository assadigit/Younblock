package io.github.assadigit.younblock

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.net.VpnService
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.ParcelFileDescriptor
import android.util.Log
import org.json.JSONArray
import org.json.JSONObject

/**
 * The Android network service (docs/android-port 04 §2/§6, ADR-0009/
 * 0010/0011; roadmaps M4 + M5). Foreground — `specialUse` type on
 * Android 14+, `connectedDevice` on 29–33 (the debug.2 fix): builds
 * the TUN with the per-app allow-list, hands the fd to the engine
 * facade once the engine is serving ([mobile.Engine.startTun]), and
 * unwinds honestly on every path to "off" — user stop, engine error,
 * [onRevoke] (equal to disconnect plus re-consent on next connect).
 *
 * M5 adds the lifecycle contract (doc 05 §M5, DECISIONS.md D77/D78):
 *
 *   - The start gate: the engine's connect run is HELD at "connecting"
 *     (platform start gate) until this service establishes the TUN and
 *     calls [mobile.Engine.setVpnReady] with true. No consent, no TUN,
 *     no "on" — the page can never show a Connected that does not
 *     route. Every unwind path re-arms the gate closed.
 *   - Notification honesty: the status text is derived from the
 *     facade's [mobile.Engine.vpnState] report — "active" only when it
 *     actually reports routing, "degraded" when it routes degraded,
 *     "connecting" otherwise. The same JSON the M5 host tests verify.
 *   - Service-death reconciliation (doc 04 §5): [onDestroy] stops the
 *     engine whenever it is serving or connecting — without the service
 *     nothing routes, and a stale "Connected" must not survive it. The
 *     MainActivity re-derives the whole state on resume; the service
 *     restarts the engine itself on its next onStartCommand when the
 *     connect intent survives (e.g. a consent result after rotation).
 *
 * TUN shape (ADR-0009/0010):
 *   - IPv4 default route into the TUN for the ALLOWED apps only — the
 *     `apps` settings list (schema v2, M6), read through the facade's
 *     [mobile.Engine.routedApps] at establish time: the same normalized
 *     list the WebView's Apps-to-unblock row edits and settings.json
 *     persists. D86 put the YouTube app on the list (Ali's ruling after
 *     the debug.1 device observation; the v3 settings migration carried
 *     debug.1's born-empty files to it); D87 — his second ruling —
 *     extends the default to the Chrome, Firefox, Opera and Brave
 *     browsers (the v4 migration carries debug.2's [youtube]-only files
 *     to the five-app default). Entries whose apps are NOT installed
 *     are skipped per-package below (NameNotFoundException is caught),
 *     so the five-app default establishes fine on devices that only
 *     have some of them. A change saved while the TUN
 *     is live applies on the NEXT connect — the list is baked into the
 *     VpnService builder at establish, and rebuilding an established
 *     TUN under live traffic is riskier than the delay (the UI copy
 *     says so; no silent re-route).
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
 * product scope. If the list is empty (the user removed every entry —
 * the D87 default is five apps, so empty is a CHOICE; the v3/v4
 * migrations fill only the never-configured files of their eras),
 * cannot be read, or none of its packages is installed, the service
 * refuses to establish through the M7 typed door (D81): the engine
 * fails the connect run with NO_TARGET_APPS, the page shows the honest
 * reason, and this service stops with the state reconciled.
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

    /** True once a startForeground call succeeded — a later refusal is
     *  a notification refresh (cosmetic), never the promotion itself
     *  (debug.2 hardening). */
    private var foregroundUp = false

    /** True once startTun succeeded (the fd is Go's from then on). */
    private var tunBound = false

    /**
     * The M8-am connect-time hand-off's once-per-run latch (ADR-0051,
     * D105; the fire condition itself is M8-an's ADR-0052): true once
     * the YouTube open fired for THIS connect run.
     * Reset when the run ends (phase off/error); a degraded→on
     * recovery flap must not re-launch the app over the user, and a
     * service restart cycle (no fresh "on" event — the engine kept
     * serving through establishTun's already-serving path) is the
     * SAME run, so it never re-fires either.
     */
    private var autoOpenFired = false

    private val main = Handler(Looper.getMainLooper())

    /** The engine's state events drive the TUN lifecycle (doc 04 §6). */
    private val stateListener = { name: String, json: String ->
        if (name == "state") {
            val state = JSONObject(json)
            val phase = state.optString("phase", "")
            // M8-an (ADR-0052): the warm flag rides the same state event
            // — the hand-off's readiness half.
            val warm = state.optBoolean("warm", false)
            main.post { onEngineState(phase, warm) }
        }
    }

    override fun onCreate() {
        super.onCreate()
        instance = this
        running = true
        VpnProtector.attach(this)
        EngineEvents.addListener(stateListener)
        createChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        // The "Quit Younblock" round (D112): a start that raced the quit —
        // e.g. a state event had already queued the connect flow when the
        // user confirmed Quit on the same main thread — must not resurrect
        // anything. Unwind honestly: start no engine, promote no
        // foreground, remove any notification the quit already cancelled,
        // and stop promptly (the startForegroundService contract's own
        // mitigation, the same shape the debug.2 hardening ships: the app
        // is in the foreground, a prompt stop is accepted).
        if (quitting) {
            Log.i(TAG, "service start raced the quit; unwinding without starting")
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
            return START_NOT_STICKY
        }
        // The service exists to carry traffic; if it was somehow started
        // without the engine, start it (idempotent-or-rejecting on the Go
        // side — the engine's phase guards decide, not this shell). The
        // connect run then holds at the start gate (D77) until
        // establishTun below opens it.
        val engine = EngineHolder.ensure(this)
        // debug.2 hardening: the promotion must succeed or the run dies
        // HERE — an uncaught startForeground refusal was exactly the
        // crash the v0.8.0 field reported (the Android 14 type-policy
        // SecurityException). stopSelfHonest unwinds the gate-held
        // engine, re-arms the gate, and satisfies the
        // startForegroundService contract with a prompt stopSelf; the
        // page shows Not Connected, never a tombstone.
        if (!startForegroundHonest()) {
            stopSelfHonest()
            return START_NOT_STICKY
        }

        if (JSONObject(engine.getState()).optString("phase", "off") == "off") {
            val res = engine.start()
            Log.i(TAG, "engine start: $res")
        }

        if (tunPfd == null && !tunBound) {
            establishTun()
        }
        return START_NOT_STICKY // a sticky restart would route traffic
        // without a user decision behind it; M5's reconciliation is
        // pull-based (MainActivity.onResume re-derives the state).
    }

    /** Builds the TUN (per-app allow-list, virtual DNS, v4+v6 routes). */
    private fun establishTun() {
        val engine = EngineHolder.ensure(this)
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

        // M6: the allow-list comes from the engine's normalized `apps`
        // settings — the same truth the WebView edits (D79). Reading it
        // fresh at establish time is what makes "changes apply on the
        // next connect" true without any extra plumbing.
        val apps = readRoutedApps(engine)
        if (apps.isEmpty()) {
            // Fail closed toward refusal — an empty list would route
            // EVERYTHING. D87: the fresh list carries the YouTube app
            // and the four browsers (the v4 migration carries debug.2's
            // [youtube]-only files to the five-app default), so empty
            // arriving HERE means the user removed every entry or the
            // bridge read failed — either way the refusal is the
            // typed NO_TARGET_APPS door (D81): the engine fails the
            // gate-held run and the page shows the honest "add an app"
            // reason, never a silent unwind.
            Log.w(TAG, "allow-list empty (every entry removed, or unreadable); refusing to route")
            val res = engine.failStart("NO_TARGET_APPS")
            if (!res.contains("\"error\":null")) {
                // NOT_HELD (the engine was not held at the gate — e.g. a
                // restart cycle) or BAD_CODE: the honest-stop path below
                // owns the engine either way; this is only visible.
                Log.w(TAG, "failStart: $res")
            }
            stopSelfHonest()
            return
        }
        var allowed = 0
        for (pkg in apps) {
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
            // D81: refuse the gate-held connect run with the typed code —
            // the engine lands in phase "error" (NO_TARGET_APPS) and the
            // page shows "Can't Connect" with the honest reason, not a
            // silent return to "Not Connected". stopSelfHonest's phase
            // check then skips the engine stop (the run already failed)
            // and still re-arms the gate + tears the service down.
            val res = engine.failStart("NO_TARGET_APPS")
            if (!res.contains("\"error\":null")) {
                // NOT_HELD (the engine was not held at the gate — e.g. a
                // restart cycle) or BAD_CODE: the honest-stop path below
                // owns the engine either way; this is only visible.
                Log.w(TAG, "failStart: $res")
            }
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

        // M5 (D77): consent is granted AND the TUN exists — open the
        // engine's start gate. A connect run held at "connecting" now
        // proceeds; an already-serving engine just keeps serving.
        EngineHolder.ensure(this).setVpnReady(true)

        // The engine may already be serving (a restart cycle); bind now.
        // Otherwise the state listener binds when "on" arrives.
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
            startForegroundHonest()
        } else {
            tunBound = false
            Log.e(TAG, "startTun refused: ${parsed.optString("error")}")
            // D76's loud refusal made visible: unwind whatever stack the
            // facade still holds (idempotent) so the next connect cannot
            // inherit it, then fail closed — the engine must not keep a
            // non-routing "on".
            engine.stopTun()
            stopSelfHonest()
        }
    }

    /** Engine state events (marshaled to the main thread). */
    private fun onEngineState(phase: String, warm: Boolean) {
        when (phase) {
            "on", "degraded" -> {
                bindTun()
                // The notification follows the honest combined state at
                // every transition (degraded on; recovered back).
                startForegroundHonest()
                // The M8-an hand-off gate (ADR-0052, D105): only when the
                // TUN is actually bound AND the engine reports its video
                // lane warm — the debug.30 field report showed the
                // second-zero open cold-starting YouTube into a chain
                // still paying its own per-flow establishment (the felt
                // result: the old 1–2 minute first-video open), and the
                // engine now holds "warm" closed until the serving phase
                // held through a settle window and one light re-check
                // proved the chain (a bounded cap never traps the user).
                // The conjunction is checked on every state event, so
                // whichever half arrives first — the bind or the warm
                // flag — the hand-off fires when both hold, once per run.
                if (tunBound && warm) maybeAutoOpenYouTube()
            }
            "off", "error" -> {
                // doc 04 §8: every path to off/error ends with the TUN
                // down and no routes leaked; the UI state comes from the
                // engine itself.
                if (tunBound || tunPfd != null) {
                    unbindTun()
                }
                autoOpenFired = false // the run ended; the next connect may hand off again
                stopSelfHonest()
            }
            // "connecting"/"stopping": nothing to do — the TUN binds
            // when serving arrives and unwinds on the terminal phase.
        }
    }

    /**
     * The connect-time hand-off (M8-am, ADR-0051, D105; the fire
     * moment is M8-an's ADR-0052; the refresh half is M8-ap's
     * ADR-0055 removal): once per connect run, when the engine
     * reports serving AND warm AND the TUN is bound, open the
     * YouTube app — so the first video starts through a
     * tunnel that has already held and re-proved its chain (and, via
     * ADR-0053's persisted dead-cluster marks, already knows the
     * blackholed clusters) instead of the app resuming its
     * pre-connect dead-host ladder into a second-zero tunnel (doc 11
     * §5.1 measured the ladder owning most of the felt 5-6 s
     * first-open load; §5.3 measured the second-zero variant at the
     * old 1–2 minutes).
     *
     * Gated by the `autoOpenYouTube` setting (schema v8, default ON —
     * the owner's checked-by-default ruling), read fresh at this moment
     * through the facade so the settings file, the UI row and this
     * door can never disagree. A change saved mid-connect applies on
     * the NEXT connect, exactly like the allow-list.
     *
     * The refresh half is GONE (M8-ap, ADR-0055): M8-am read
     * "refresh it" as killBackgroundProcesses before the launch, so
     * every connect cold-started YouTube — and the debug.30-.32
     * field reports measured what that cold start costs the first
     * video: a fresh process has no in-memory "QUIC is dead on this
     * network" knowledge, so the first media connection burned its
     * QUIC job timeout into the TUN's UDP drops (6920 ms on the
     * debug.32 report — the ICMP refusal the stack has sent since
     * M8-j is correct and the app's QUIC stack ignores it), on top
     * of a cold feed and a cold terminal ladder. The debug.29 state
     * the tester called the best never killed anything: a warm app
     * already has QUIC blacklisted in memory and its feed live, and
     * YouTube refreshes its own feed on resume. The open: the
     * package's launch intent with NEW_TASK. Every refusal path is
     * logged honestly and costs nothing — a missed open is
     * recoverable (the user taps YouTube), an unwanted one is not.
     */
    private fun maybeAutoOpenYouTube() {
        if (autoOpenFired) return
        autoOpenFired = true // once per run: success, refusal, or off — no re-fire on flaps
        val engine = EngineHolder.engine ?: return
        val enabled = runCatching {
            JSONObject(engine.autoOpenYouTube()).optBoolean("enabled", false)
        }.getOrDefault(false)
        if (!enabled) {
            Log.i(TAG, "auto-open: off (the setting); nothing opened")
            return
        }
        // M8-ap (ADR-0055): no kill. The warm relaunch IS the
        // refresh — the app's own on-resume feed reload plus its
        // in-memory QUIC blacklist are exactly the warm state the
        // debug.29 opens rode (the debug.32 report measured the cold
        // variant's QUIC burn at 6920 ms on the first video's
        // critical connection).
        // The open half. The app is in the foreground at connect time
        // (the user just tapped Connect here), so the background-start
        // restriction does not apply; a refusal is still caught and
        // logged, never fatal.
        val launch = packageManager.getLaunchIntentForPackage(YOUTUBE_PACKAGE)
        if (launch == null) {
            Log.i(TAG, "auto-open: $YOUTUBE_PACKAGE not installed; nothing to open")
            return
        }
        launch.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        runCatching { startActivity(launch) }
            .onFailure { Log.w(TAG, "auto-open: launch refused: ${it.message}") }
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
        if (instance === this) instance = null
        EngineEvents.removeListener(stateListener)
        VpnProtector.detach(this)
        unbindTun()
        // M5 service-death reconciliation (doc 04 §5, D78): without this
        // service nothing routes. A serving engine would be a lying
        // "Connected"; a gate-held "connecting" would be an invisible
        // zombie (MainActivity.onDestroy stops it when no service runs,
        // and the connect intent — e.g. a pending consent result —
        // restarts it through this service's own engine.start()).
        EngineHolder.engine?.let { eng ->
            val phase = JSONObject(eng.getState()).optString("phase", "off")
            if (phase != "off" && phase != "error" && phase != "stopping") {
                Log.i(TAG, "engine stop (service death, phase $phase): ${eng.stop()}")
            }
            eng.setVpnReady(false)
        }
        running = false
        super.onDestroy()
    }

    // ------------------------------------------------------------ plumbing

    /**
     * The honest foreground notification (M5): the text comes from the
     * facade's vpnState report — the same JSON the host tests verify.
     * "Active" only when it reports routing; "degraded" when it routes
     * degraded; "connecting" while the gate/start sequence is pending.
     *
     * The type branch (debug.2, the Android 14 connect-crash fix): API
     * 34+ enforces a per-type permission policy at startForeground
     * time — connectedDevice demands one of its runtime prerequisites
     * (CHANGE_NETWORK_STATE, BLUETOOTH_CONNECT, …) which this app's
     * honest inventory deliberately does not hold, and the resulting
     * SecurityException crashed the process on the connect tap of
     * every Android 14+ phone (the owner's <= 13 device never enforced
     * it — which is why it looked like "only my phone works").
     * specialUse is Google's designated escape hatch: no runtime
     * prerequisites, declared in the manifest union with the required
     * PROPERTY_SPECIAL_USE_FGS_SUBTYPE and started with the matching
     * type flag here. 29–33 keep the shipped connectedDevice call
     * (unpoliced there; the manifest union satisfies the API 29+
     * subset rule for both branches), and < 29 takes the untyped
     * call the platform predates.
     *
     * Returns false when the platform refuses the call instead of
     * throwing through the caller (debug.2 hardening): a refused
     * PROMOTION kills the fresh connect run honestly through
     * [stopSelfHonest] (onStartCommand), while a refused REFRESH — the
     * tunnel is already foreground and serving — is logged and
     * survived: a stale status text must never tear down a tunnel
     * that routes.
     */
    private fun startForegroundHonest(): Boolean {
        val text = notificationText()
        val notification = buildNotification(text)
        val refreshing = foregroundUp
        return try {
            when {
                Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE ->
                    startForeground(
                        NOTIFICATION_ID, notification,
                        ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE,
                    )
                Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q ->
                    startForeground(
                        NOTIFICATION_ID, notification,
                        ServiceInfo.FOREGROUND_SERVICE_TYPE_CONNECTED_DEVICE,
                    )
                else ->
                    startForeground(NOTIFICATION_ID, notification)
            }
            foregroundUp = true
            true
        } catch (e: Exception) {
            if (refreshing) {
                // A refresh of an already-foreground notification: the
                // tunnel routes and keeps routing — the stale text is
                // the honest cost of never killing a live tunnel over
                // cosmetics.
                Log.w(TAG, "notification refresh refused (tunnel keeps serving): $e")
            } else {
                // The promotion itself: nothing can follow on a
                // service the system refuses to foreground —
                // onStartCommand unwinds the whole run honestly.
                Log.e(TAG, "foreground promotion refused: $e")
            }
            false
        }
    }

    /** Derives the status text from the facade's honest report. */
    private fun notificationText(): String {
        val vpn = EngineHolder.engine?.let { runCatching { JSONObject(it.vpnState()) }.getOrNull() }
            ?: return getString(R.string.vpn_notification_connecting)
        val routing = vpn.optBoolean("routing", false)
        val phase = vpn.optString("phase", "off")
        return when {
            routing && phase == "degraded" ->
                getString(R.string.vpn_notification_degraded)
            routing ->
                getString(R.string.vpn_notification_active)
            else ->
                getString(R.string.vpn_notification_connecting)
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
            // M8-am: the pill glyph (the launcher icon is now the
            // owner's mark in the mipmap set; the status bar tints
            // small icons monochrome, where the simple mark reads).
            .setSmallIcon(R.drawable.ic_notification)
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

    /**
     * The "Quit Younblock" teardown (D112) — the one deliberate,
     * confirmed end-of-app path, deliberately separate from every
     * disconnect semantics above (the lever tap, the notification's
     * life, [onRevoke] — all unchanged). Order, per the product's ask:
     *
     *  a. the engine run stops cleanly — the same synchronous [stop]
     *     the honest paths use (proxy listener down, watchdog and
     *     self-repair loops down, platform detach — a no-op on Android);
     *  b. the TUN closes ([unbindTun]: the Go stack stops, the fd it
     *     owns closes, the routes go with it) and the service stops
     *     ([stopSelf]);
     *  c. foreground mode ends with STOP_FOREGROUND_REMOVE AND the
     *     notification is then cancelled by its ID — the belt-and-braces
     *     pair, so the status bar is empty even if a promotion raced or
     *     the system kept the post around;
     *  d. the start gate re-arms closed. Nothing else can resurrect the
     *     service: this service was already START_NOT_STICKY (no sticky
     *     restart), and this app has no boot receiver, no jobs, no
     *     alarms, no Quick Settings tile (the manifest's inventory is
     *     the proof) — the one real re-starter, the Activity's connect
     *     flow, is gated by the [quitting] flag this quit sets.
     *
     * Runs on the main thread (the caller marshals); the engine-stop
     * events it triggers are posted behind it and land on paths already
     * unwound — every step below is idempotent under their arrival.
     */
    private fun performQuit() {
        // (a) the engine run stops cleanly
        EngineHolder.engine?.let { eng ->
            val phase = JSONObject(eng.getState()).optString("phase", "off")
            if (phase != "off" && phase != "error" && phase != "stopping") {
                Log.i(TAG, "engine stop (quit, phase $phase): ${eng.stop()}")
            }
        }
        // (b) the TUN closes
        unbindTun()
        // (c) foreground out, then the notification gone by its ID
        stopForeground(STOP_FOREGROUND_REMOVE)
        getSystemService(NotificationManager::class.java).cancel(NOTIFICATION_ID)
        // (d) the gate re-arms; nothing else may restart the service
        EngineHolder.engine?.setVpnReady(false)
        stopSelf()
    }

    /**
     * The one honest stop: foreground out, service out, and the engine
     * stopped whenever it was doing anything — without this service
     * nothing routes, so no state may claim otherwise (D78). The gate
     * is re-armed closed either way: the next connect re-runs the
     * whole consent → TUN sequence before anything can serve.
     */
    private fun stopSelfHonest() {
        EngineHolder.engine?.let { eng ->
            val phase = JSONObject(eng.getState()).optString("phase", "off")
            if (phase != "off" && phase != "error" && phase != "stopping") {
                Log.i(TAG, "engine stop (service unwinding, phase $phase): ${eng.stop()}")
            }
            eng.setVpnReady(false)
        }
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    companion object {
        private const val TAG = "Younblock"
        private const val CHANNEL_ID = "vpn"

        /** The hand-off's target (M8-am, ADR-0051, D105): the official
         *  YouTube app — the D86 default allow-list's first entry and
         *  the app this product exists to unblock. */
        private const val YOUTUBE_PACKAGE = "com.google.android.youtube"
        private const val NOTIFICATION_ID = 1
        private const val MTU = 1500
        private const val TUN_ADDRESS = "10.111.0.1"
        private const val VIRTUAL_DNS = "10.111.0.53"

        /** The live service instance, main-thread-confined (onCreate →
         *  onDestroy), so the quit door can reach the teardown of a
         *  running service without inventing a second lifecycle. */
        @Volatile
        private var instance: YounblockVpnService? = null

        /**
         * True while a confirmed "Quit Younblock" is in flight (D112).
         * Once set it never clears for this process's lifetime: nothing —
         * the Activity's connect flow, a raced service start, a resume
         * reconciliation — may (re)start the service or the engine, and
         * [onStartCommand]'s quit guard unwinds any start that slips
         * through anyway. The flag dies with the process (the quit's
         * last step), so the NEXT manual launch is a fresh first-open,
         * exactly as before this feature.
         */
        @Volatile
        var quitting: Boolean = false
            private set

        /**
         * The quit door (D112): marks the process as quitting, then runs
         * the ordered teardown on the live service — or, with no service
         * running (the app sat disconnected), still lands the honest
         * end-state directly: the gate re-armed and the notification
         * cancelled by its ID (a stale post from any earlier run cannot
         * survive the quit). Main-thread only (the Activity marshals).
         */
        fun quit(context: Context) {
            quitting = true
            val live = instance
            if (live != null) {
                live.performQuit()
            } else {
                Log.i(TAG, "quit with no live service; clearing the end-state")
                EngineHolder.engine?.setVpnReady(false)
                context.getSystemService(NotificationManager::class.java)
                    ?.cancel(NOTIFICATION_ID)
            }
        }

        /** Reads the configured per-app allow-list from the facade
         *  (M6, D79; D86): {"apps":[...]}. The list starts with the
         *  YouTube app (the engine's normalized settings — the v3
         *  migration carries debug.1's born-empty v2 files to it), so
         *  an empty result means the user emptied the list or the
         *  bridge read failed; both take the typed refusal above
         *  (never a silent route-everything fallback). */
        private fun readRoutedApps(engine: mobile.Engine): List<String> =
            runCatching {
                val arr = JSONObject(engine.routedApps()).optJSONArray("apps") ?: JSONArray()
                List(arr.length()) { arr.optString(it) }.filter { it.isNotBlank() }
            }.getOrDefault(emptyList())

        /** Observable liveness for the Activity's connect flow. */
        @Volatile
        var running: Boolean = false
            private set
    }
}
