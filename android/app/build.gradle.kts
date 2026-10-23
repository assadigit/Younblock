// The Younblock Android app module (docs/android-port 04 §3, doc 06 §1).
// The engine is the gomobile AAR at libs/younblock.aar (build-android.sh
// regenerates it; it is git-ignored). Version numbers track the desktop
// app (internal/config AppVersion is the one source of truth for what the
// About page reports; the packaging version below is what Android shows
// in system UI and must be bumped in lockstep).
//
// Kotlin support is built into AGP 9 — no separate kotlin-android plugin
// (verified this session: applying it fails the build with instructions
// to remove it).
plugins {
    id("com.android.application")
}

android {
    namespace = "io.github.assadigit.younblock"
    compileSdk = 36

    defaultConfig {
        applicationId = "io.github.assadigit.younblock"
        minSdk = 26
        targetSdk = 36
        // versionCode 34: the one-UI round (1.6.7-debug.1 -- the
        // unified-release round). Every edition -- this APK, the
        // Windows apps, the Linux window and web editions -- now
        // ships the SAME UI v3 main screen: the classic editions
        // and their variant switch are retired (one source tree,
        // one screen, so a UI change lands on every version at
        // once), and every version rides the SAME GitHub release
        // from here on (release-unified.sh; D110/D111). The base
        // version moves with the unified line (1.6.6 -> 1.6.7),
        // so the debug counter restarts at .1 under the new base
        // -- the versionCode keeps climbing monotonically (34 >
        // 33), so this still installs over debug.34 in place (the
        // same pinned debug certificate -- D85). The Go engine is
        // untouched; the desktops additionally gain the real
        // slow-mode/warming signals the WebView already had
        // (ui_bridge's HealthSource). D103: the APK asset is
        // named v1.6.7-debug1.apk.
        // versionCode 33 was: the UI v3 build (debug.34 -- the
        // main-screen rework round, docs/ui-v3). The new main
        // screen per docs/ui-v3/UI-SPEC.md, its design tokens
        // IMPORTED (never copied) from docs/ui-v3/
        // younblock-tokens.css: the charcoal+red palette (no green
        // anywhere), the state carried by the cap's end, the LED,
        // the recolored ring, the glow and the words -- never color
        // alone; the power icon is gone (the cap says
        // START/CANCEL/RETRY/STOP); the Open YouTube card appears
        // only while connected, through the new
        // PageBridge.openYouTube() bridge (app first, the browser
        // as fallback); haptics ride navigator.vibrate behind the
        // one new permission (VIBRATE, listed in docs/ui-v3/
        // REPORT.md); slow mode is the engine's real degraded
        // signal, and the VPN-badge teardown was audited (the
        // finding is in the report). The Go engine is untouched --
        // no proxy, dialer, pool, tier or setting change rides in
        // this build. Installs over debug.33 (the same pinned debug
        // certificate -- D85).
        // versionCode 32 was: the M8-ap build (debug.33 -- the
        // refresh-half removal round, ADR-0055, D109). The
        // debug.32 field report: "This build, still is not
        // good, it took a while (much more than v29), then
        // next videos were okay. so far v29 was our best."
        // The report's own instrument found the first video's
        // critical googlevideo connection burning 6920 ms in
        // the app's QUIC job timeout inside the TUN's UDP
        // drops (the M8-j ICMP refusal is correct; the app's
        // QUIC stack ignores it) -- and the debug.30-.32
        // builds are the only ones that GUARANTEE that burn on
        // every first video, because the M8-am "refresh" half
        // (killBackgroundProcesses) cold-started YouTube at
        // every connect: a fresh process has no in-memory
        // "QUIC is dead here" knowledge, no warm feed, and a
        // cold terminal ladder. debug.29 -- the tester's best
        // -- never killed anything. ADR-0055 drops the kill
        // and the KILL_BACKGROUND_PROCESSES permission; the
        // launch half (once per run, warm + TUN-bound, the
        // setting default unchanged) now reproduces the
        // debug.29 warm-app flow, automated. The warm gate
        // (ADR-0052) and the persisted dead-cluster marks
        // (ADR-0053) ride unchanged. Installs over debug.32
        // (the same pinned debug certificate -- D85).
        // versionCode 31 was: the M8-ao build (debug.32 -- the
        // logo-icon round, ADR-0054, D108). The owner uploaded
        // younblock-logo-v1.png (a 449px finished icon design: the
        // 3D padlock with the play button, the silver shackle, the
        // red sparks, the younblock wordmark, on his own dark
        // gradient squircle) and ruled it the app's icon. His
        // design is a full-bleed tile -- the wordmark sits in the
        // bottom quarter, where every Android launcher mask would
        // crop it away -- so scripts/gen_android_icons.py (rewritten
        // for the raster source; the M8-am icon.ico trace retires)
        // recomposes it: the tile's own gradient as the full-bleed
        // adaptive background, the art soft-matted out of it as the
        // foreground with every element inside the 66dp safe circle
        // (z=0.647, the content's extent centered; the render gates
        // measured IoU 0.97-0.98 against the matte on every density,
        // and the composed icon VLM-verified complete and faithful),
        // the same silhouette as the Android 13+ monochrome layer,
        // and the legacy tiles carry his squircle exactly as drawn.
        // Engine and playback behavior are untouched -- no proxy,
        // dialer, pool, tier or setting change rides in this build.
        // Installs over debug.31 (the same pinned debug
        // certificate -- D85).
        // versionCode 30 was: the M8-an build (debug.31 -- the
        // wait-until-ready round, ADR-0052/0053, D106/D107).
        // The debug.30 field report: "the app reverted to our
        // old problem... 1-2 minutes... I think the app
        // connection time is too short, so when it
        // automatically switches to youtube app it's not ready
        // yet. the better job is to make the user wait and see
        // connecting, only you're sure that even initial video
        // is smooth and fast." The hand-off had fired at second
        // zero of the tunnel's life into an empty per-session
        // cluster memory -- the app's ladder re-paid the
        // discovery burn and the breaker's arm gate summed it.
        // The answers: (1) the warm gate -- the hand-off now
        // waits for the engine's readiness signal (the serving
        // run held 3s + one light re-check, a 15s never-trap
        // cap), and the connect screen holds "Getting the
        // first video ready..." until it lifts; (2) the
        // dead-cluster marks persist across sessions
        // (googlevideo-dead-clusters.txt, a 5min seed TTL, the
        // re-probe/rehab safety nets) so the tunnel the app
        // wakes into is marked and fed from second zero.
        // Installs over debug.30 (the same pinned debug
        // certificate -- D85).
        // versionCode 29 was: the M8-am build (debug.30 -- the
        // release-candidate polish round, ADR-0051). The
        // debug.29 verdict round closed the residual question
        // (doc 11 §5.1: the engine's share of the felt 5-6s
        // load is ~500ms; the app's own dead-host ladder plus
        // buffering own the rest -- the release state), and the
        // owner added two asks before the official release
        // (D104/D105): (1) the real launcher icon -- his
        // icon.ico mark (the padlock with the play button, the
        // gold bolt, the impact dashes), traced to vector
        // geometry by scripts/gen_android_icons.py (per-layer
        // IoU-verified against the source, VLM-checked
        // faithful) as the adaptive mipmap set with legacy
        // tiles; the pill glyph stays the notification mark;
        // (2) the connect-time hand-off -- when the engine
        // turns on with the TUN bound, the shell refreshes
        // (killBackgroundProcesses) and opens the YouTube app
        // so the first video cold-starts through the live
        // tunnel instead of resuming the pre-connect ladder;
        // a new settings row (autoOpenYouTube, schema v8)
        // carries it, checked by default per the owner's
        // ruling, read fresh at the moment "on" arrives. The
        // engine's playback behavior is untouched: no proxy,
        // dialer, pool or tier change rides in this build.
        // Installs over debug.29 (the same pinned debug
        // certificate -- D85). The D84 rule: every debug build
        // bumps this. D103: the APK asset is named
        // v1.6.6-debug30.apk.
        versionCode = 34
        versionName = "1.6.7"
    }

    signingConfigs {
        // D85: the pinned test-circle debug key — the exact certificate
        // that signed the published v1.6.6-android-debug.1 (SHA-256
        // digest c76bb7c6…8551d). Overriding AGP's default debug
        // signingConfig here makes every assembleDebug use the committed
        // android/app/debug.keystore instead of the machine-local
        // ~/.android/debug.keystore, which is regenerated per sandbox
        // wipe and would break the documented promise that successive
        // debug builds install over debug.1 as updates. A *debug*
        // keystore is not a secret (the well-known "android" store
        // password); the no-committed-keys rule (doc 09) covers release
        // keystores, which stay out of the repo per D84/doc 07 §8.
        getByName("debug") {
            storeFile = rootProject.file("app/debug.keystore")
            storePassword = "android"
            keyAlias = "androiddebugkey"
            keyPassword = "android"
        }
    }

    buildTypes {
        debug {
            signingConfig = signingConfigs.getByName("debug")
        }
        release {
            // The M3 shell ships debug-signed sideload builds (ADR-0012);
            // minification decisions wait for M7 hardening. No signed
            // variant exists in the test circle (D84); release signing
            // returns only for the public release (doc 07 §8).
            isMinifyEnabled = false
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

dependencies {
    implementation(files("libs/younblock.aar"))
}
