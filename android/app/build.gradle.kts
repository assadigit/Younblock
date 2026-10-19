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
        // versionCode 15: the M8-x build (the seventh pasted report —
        // still shell line versionCode 13, the debug.14 build: the
        // user's own words named the M8-v stall watch's regression:
        // "videos used to have lag for starting to play, but after
        // that you could easily go next instantly; now even going next
        // and back in the same video lags a lot"). The report's log is
        // the smoking gun: thirteen "tunnel stall killed
        // idle=15xxx ms" lines in ~70 s, every one a connection that
        // had ALREADY carried real data — 2.1 MB, 1.6 MB, 723.6 KB,
        // 633.7 KB, 162.6 KB down — murdered at exactly the lean 15 s
        // budget mid-pacing: the player's pooled video connections
        // (buffer, quiet, next range on the SAME connection) were
        // being closed out from under the app, so every next/seek
        // re-dialed from scratch through a ~50 % dirty dial path.
        // ADR-0022 tiers the watch: a flow that proved itself —
        // StallRichBytes=32 KB delivered down, or one idle gap
        // survived and closed by fresh activity (the pacing proof,
        // gap >= StallTimeout/5) — escalates to StallRichTimeout=90 s
        // and dies QUIETLY when it finally fires: no answer-set
        // demotion (the origin just carried megabytes), no error
        // count (hygiene, not a dead-origin signal), the stall line
        // carrying "down=N tier=rich" for the next report. The LEAN
        // tier keeps the exact M8-v semantics for the debug.13 dead
        // cohort: first byte plus headers, never woke, silent to the
        // 15 s budget — closed LOUD (demotion + error + tier=lean).
        // This build also carries the M8-w first-dial race +
        // ClearNegative DNS probe (versionCode 14, v1.6.6-debug15.apk)
        // that the report never exercised — the shell line proves the
        // device was still on the debug.14 build. D102 stays Ali's.
        // Installs over debug.15 (same pinned debug certificate — D85).
        // The D84 rule: every debug build bumps this. D103: the APK
        // asset is named v1.6.6-debug16.apk.
        versionCode = 15
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
