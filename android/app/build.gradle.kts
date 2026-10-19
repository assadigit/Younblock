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
        // versionCode 14: the M8-w build (the debug.14 report round —
        // the first report from the salvage-race + stall-watch build
        // itself, shell line versionCode 13). It proved ADR-0020
        // on-device: the retry lines batch at the same millisecond (the
        // re-dials race), the stall watch kills at 15 s (idle=15xxx ms
        // lines), saves at 738-848 ms — and it named the two remaining
        // costs this build attacks (ADR-0021): (1) the SERIAL FRONT —
        // attempt 0 still walked its 2.5 s budget ALONE before the race
        // launched (six timeout terminals at 5320-5576 ms = the
        // two-budget chain) while CONCURRENT flows to the same hosts
        // answered at 72-162 ms in the same seconds (the dirt is
        // per-origin, not per-moment) — so the whole dial budget is now
        // spent racing from the flow's start: FirstByteRaceDials=2
        // extra candidates dial distinct origins in parallel with the
        // primary, the client's first record replayed byte-exact
        // through each, first speaker wins, an all-dead flow terminals
        // at ~1 budget instead of 2 (new report line: "tunnel
        // first-byte raced dials=N" — a raced save; a clean primary
        // stays quiet); (2) the DNS degraded state — dns fail 16 with
        // DNS_UNREACHABLE while the network flowed: a dirty-window DoH
        // blip negative-cached the probe name and the periodic check
        // read the 15 s memory instead of asking any server — so the
        // health probe now clears exactly the negative entry first
        // (ClearNegative; a good cached answer stays) and asks what
        // the servers say NOW. D102 (the next-preset retry) stays
        // Ali's ruling, never code.
        // Installs over debug.14 (same pinned debug certificate — D85).
        // The D84 rule: every debug build bumps this. D103: the APK
        // asset is named v1.6.6-debug15.apk.
        versionCode = 14
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
