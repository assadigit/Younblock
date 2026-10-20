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
        // versionCode 23: the M8-af build (debug.24 -- the
        // prove-before-change round): the external engineer's review
        // of the debug.23 consultation said the cause is still
        // UNPROVEN -- the 2.8 KB corpses are probably
        // handshake-only replies, but sizes suggest, they don't
        // prove -- and this round measures instead of guessing
        // (ADR-0043/0044/0045, younblock-debug24-spec.md): (1) TCP
        // kernel-counter evidence per media flow (TCP_INFO
        // snapshots A at the request write and B at flow end; the
        // stalled-flow verdicts upstream-lost / downstream-lost /
        // both-ok / unknown answer WHERE the data is lost -- no
        // packet capture). (2) The TLS record timeline (first 12
        // server / 8 client records as type:length@ms, plus
        // first-app-byte) -- the corpse question measured, not
        // inferred. (3) win=orig|subst|race on every done line.
        // (4) The experiment switches (method / substitution /
        // breaker) in a diagnostics sub-screen, all printed on the
        // report's experiment line. (5) "Run network test": the
        // current addresses probed under all four methods in the
        // same minute, with the real SNI, www.google.com and no SNI,
        // an i.ytimg.com control and per-method packet counts -- the
        // same-minute preset-vs-hour comparison no round has ever
        // had; only its "answered" verdicts (an HTTP status line
        // from a deliberately bad GET /videoplayback) certify
        // media capability. (6) The breaker sensor fix (W5, the one
        // PROVEN defect: pending stall kills feed the emission
        // breaker now -- debug.23 died 13 media flows with
        // cooldowns=0 because the breaker watched the wrong death
        // class) and the substitution scope fix (W6: the pool eats
        // media-verified verdicts only, never first-byte wins --
        // debug.23's 18 substitution "wins" delivered <=12.1 KB
        // each). (7) device: and net: header lines (phone model,
        // operator -- the consultation's unrecorded-environment
        // gaps). (8) The optional foreign-cluster file (W8,
        // experiment_nodes.txt). Installs over debug.23 (the same
        // pinned debug certificate -- D85). The D84 rule: every
        // debug build bumps this. D103: the APK asset is named
        // v1.6.6-debug24.apk.
        versionCode = 23
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
