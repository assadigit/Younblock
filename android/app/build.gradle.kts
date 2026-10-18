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
        // versionCode 12: the M8-t build (the debug.12 report round). The
        // third pasted diagnostic report proved the M8-s budget on-device
        // ("tunnel first-byte failed reason=timeout" dones at ~8.3 s —
        // the tuning evidence ADR-0018 asked for) and named the
        // remaining per-video start lag: waves where roughly half the
        // flows die at first byte (origin resets at ~0.5 s, the rest
        // silent to the budget) while the app re-opens wave after wave.
        // This build retunes FirstByteTimeout 8 s -> 2.5 s (every
        // budget-killed flow was 100% silent; healthy routed origins
        // answer in 420-508 ms, one 2399 ms outlier) and adds the
        // salvage re-dial (ADR-0019): a dead first-byte origin is
        // re-dialed INSIDE the same flow — same host, same preset, the
        // client's buffered bytes replayed byte-exact — up to 2 times,
        // so the app never sees the failure and its serial retry chain
        // collapses into bounded in-engine attempts (per-flow preset
        // variation stays D102, Ali's ruling, never code). New report
        // lines: "tunnel first-byte retry reason=… attempt=N" and
        // "tunnel first-byte salvaged dials=N".
        // Installs over debug.12 (same pinned debug certificate — D85).
        // The D84 rule: every debug build bumps this. D99: the APK asset
        // carries the version name (v1.6.6-debug.apk).
        versionCode = 12
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
