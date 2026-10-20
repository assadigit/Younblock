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
        // versionCode 21: the M8-ad build (debug.22 -- the
        // long-lived-connection round): the tester's post-debug.21
        // report brought the round's whole brief in two sentences --
        // "the problem is not solved yet (video lags to begin)" and
        // "after a while playing and seeing a video like minute 5 of
        // a 10min video, it goes loading, and as soon as you refresh
        // it fixes" (more on Linux, less on Windows), plus a pointer
        // back at the legacy Python backend: "read the backend script
        // again, maybe it gives you ideas." The ideas it gave: (1)
        // ADR-0037 rich-flow liveness -- the minute-5 wedge was three
        // death shapes the 90 s hygiene budget sat on: a PENDING
        // death (the player's next range request riding the dead
        // pooled conn) now closes at StallRichPendingTimeout=10 s
        // with the demote landing; a blackholed conn now surfaces in
        // ~30 s via tuned TCP keepalive (15 s idle + 3x5 s probes,
        // every origin dial including the DoH lane's); and the
        // per-connection THROTTLE (bytes keep arriving under the
        // floor -- the both-silent clock never even started, and the
        // debug.20 report's 5x-less-down was its fingerprint) now
        // dies to the trickle kill: a 12 s rolling window that keeps
        // landing above zero but under 128 KB on a 1 MB+ flow closes
        // both ends QUIETLY so the player's retry lands on a fresh,
        // unthrottled conn -- engine-side what the tester's REFRESH
        // did by hand. (2) ADR-0038 -- stall kills never invalidate
        // the answer set anymore (the debug.20 churn spiral's feed:
        // dns=579-846 ms late-session, 59 DoH dials; the Python
        // cached DNS forever). (3) ADR-0039 -- auto STARTS at legacy,
        // the Python's own 299-cut 1 ms hello (the preset existed
        // since the rewrite but auto started at balanced and the
        // sweep tried legacy LAST; under a censor that now RSTs
        // recognizably-shaped hellos -- 35 of 77 probe verdicts --
        // weak splitting was never in the fight), and the heal sweep
        // walks DOWN the strength ladder from there. The report's
        // stall line gains the rich cause split: rich=N
        // (pending=Q trickle=T) -- the numbers that say whether this
        // round caught what the tester saw. Installs over debug.21
        // (the same pinned debug certificate -- D85). The D84 rule:
        // every debug build bumps this. D103: the APK asset is named
        // v1.6.6-debug22.apk.
        versionCode = 21
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
