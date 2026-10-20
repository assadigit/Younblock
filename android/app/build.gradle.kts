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
        // versionCode 18: the M8-aa build (debug.19 — the dead-origin-pool
        // round): the verified prewarm (the 5-minute googlevideo loop
        // now PROBES the answer sets with the exact wire shape a real
        // flow sends — a fragmented ClientHello carrying the prewarmed
        // name as SNI — through the one netx dialer, at engine start
        // and on every cycle: "answered" marks the origin IP
        // verified-live for 15 minutes and every dial list starts
        // there, "silent"/"reset" marks it dead for 10 minutes with
        // strike escalation 60s->5m->30m ending the debug.18 report's
        // re-gamble — the top failing host ran 13 flows for 8
        // failures, every wave re-rolling the same mixed live-and-
        // blackholed answer set the 60 s penalty kept re-serving; the
        // stable API fronts probe first, the freshest sn-codes'
        // rr1..rr5 right behind), the raced-path salvage round (an
        // all-dead race now runs ONE more raced round over the answer
        // set's untried unboxed origins before the terminal — the
        // app's retry wave collapses into an in-engine budget, gated
        // so a hopeless set keeps the fast ~2.6 s terminal), the
        // Android lean stall budget 15s->8s (ADR-0029: the report's
        // three mid-response corpses each held the app's retry for
        // the whole 15 s desktop budget; the platform gate lives in
        // the engine's listener build, desktop keeps 15 s
        // byte-for-byte), and the ADR-0030 diagnostics (the
        // auto-timeline heuristic — play-suspected after routed quiet,
        // first-data on the down-burst, the buttons stay authoritative
        // — plus the origin-probe counters, the box's live/strikes
        // counts, the good count, the stall tier split with the
        // budget that fired, racedSaves/salvagedSaves/routedFlows).
        // Installs over debug.18 (the same pinned debug certificate —
        // D85). The D84 rule: every debug build bumps this. D103: the
        // APK asset is named v1.6.6-debug19.apk.
        versionCode = 18
        versionName = "1.6.6"
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
