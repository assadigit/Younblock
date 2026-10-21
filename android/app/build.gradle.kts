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
        // versionCode 29: the M8-am build (debug.30 -- the
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
        versionCode = 29
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
