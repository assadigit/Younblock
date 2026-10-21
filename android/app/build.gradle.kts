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
        // versionCode 28: the M8-ak build (debug.29 -- the
        // method-documentation round, ADR-0050). The debug.28
        // field report: "by far our best version yet -- 90% of
        // the initial delay of each video is gone, and it's
        // that much good that it can be the first official
        // release." The numbers behind it: dead=1 fastFails=90
        // (the cluster memory + instant resets owning the
        // recovery), cooldowns=1 lifted in 730ms, substitution
        // attempts=0 (the lane's tombstone), auto-timeline
        // 1000ms pairs=5, a live cluster delivering 2.1MB. Per
        // the tester's asks: (1) the method documented in
        // docs/android-port/11-the-video-opening-war.md (the
        // scoreboard, the loop method, the arc, the semantic
        // discoveries, the residual decomposition, the
        // lessons); (2) the auto-timeline re-scoped to
        // googlevideo (ADR-0050) so the remaining 5-10%
        // residual measures play -> video data: a play is a
        // googlevideo flow after googlevideo quiet, first-data
        // is a googlevideo first byte -- the old any-down-burst
        // close let 1.4MB feed bursts close pairs that carried
        // no video. Diagnostics-only: playback behavior is
        // byte-identical to debug.28. Installs over debug.28
        // (the same pinned debug certificate -- D85). The D84
        // rule: every debug build bumps this. D103: the APK
        // asset is named v1.6.6-debug29.apk.
        versionCode = 28
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
