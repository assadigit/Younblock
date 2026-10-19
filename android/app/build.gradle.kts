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
        // versionCode 13: the M8-v build (the debug.13 report round —
        // the first report from the salvage build itself, shell line
        // versionCode 12). It proved the salvage on-device (retry lines
        // absorbing dead attempts, "tunnel first-byte salvaged dials=N"
        // saving flows at 0.7-1.4 s) and named the two remaining
        // start-lag sources this build attacks (ADR-0020): (1) the
        // serial salvage chain — 21 terminal "reason=timeout" dones at
        // 8014-8227 ms = three 2.5 s budgets walked one after another —
        // so the remaining re-dials now RACE in parallel (distinct live
        // origins via the rotated answer set, first speaker wins, a
        // silent-host terminal costs ~2x the budget, never 3x); (2) the
        // mid-stream stall — five video-start flows lived 57-64 s
        // (first byte + ~7 KB down, then total silence) until the APP's
        // own patience killed them — so the relay now runs under the
        // stall watch: both directions silent for StallTimeout (15 s)
        // closes the flow engine-side, the answer set demoted, the app
        // retries onto a fresh origin in seconds instead of a minute.
        // New report line: "tunnel stall killed idle=Nms". D102 (the
        // next-preset retry after exhausted salvages) stays Ali's
        // ruling, never code.
        // Installs over debug.13 (same pinned debug certificate — D85).
        // The D84 rule: every debug build bumps this. D103: the APK
        // asset is named v1.6.6-debug14.apk.
        versionCode = 13
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
