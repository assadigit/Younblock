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
        // versionCode 19: the M8-ab build (debug.20 — the escape-the-pool
        // round): the debug.19 report proved the dead-origin pool works
        // but only ever inside ONE hostname's rotating answer set
        // (auto-timeline elapsed=44999ms pairs=5 — five retry waves
        // gambling fresh unknown qxau5 IPs while the good table held 24
        // verified origins no dial list ever saw, the video starting
        // only when the app itself fell back to a foreign cache), so:
        // (1) the cross-origin substitution (ADR-0031) — a
        // *.googlevideo.com flow's raced round gains ONE substitute
        // candidate dialing the good pool's verified-live origins
        // (not in the current answer set, not boxed, freshest verdict
        // first; the identical fragmented ClientHello replays with the
        // SNI unchanged — Google's video edge serves any
        // *.googlevideo.com name, the Iranian hosts-file IP-pin
        // practice rides exactly this; flat dial count — the
        // substitute replaces one own-set rotation), flow first-byte
        // SUCCESSES now feed the pool too (the session's own
        // foreign-cache win arms every later flow), and a first-byte
        // death on a pool origin drops its good mark and boxes it (the
        // ADR-0028 demote actually implemented — the debug.19 code's
        // latent leak that resurfaced dead IPs at the FRONT of dial
        // lists once the box lapsed); (2) the probe coverage and
        // cadence (ADR-0032) — the pass's name list walks the learned
        // code store ROUND-ROBIN (one rr name per code, 12 codes a
        // pass, 48 IPs the cap: every remembered code, foreign
        // clusters included, is verified within a few passes instead
        // of the freshest-2 local cluster forever), a fast-start
        // cadence (45 s passes while the engine is young and the
        // gvideo pool is thin — the debug.19 session ran ONE pass in
        // its whole 45 s cold start), and the churn kick (a
        // googlevideo terminal — all-dead raced round + dead salvage
        // — bumps gvideoTerminals and the gvideo loop's 1 s watch
        // kicks an immediate pass, 10 s debounce); (3) the persisted
        // verified-live pool (ADR-0033) — googlevideo-good-ips.txt
        // seeded back at start with the shorter 5-minute TTL so the
        // FIRST video of session N+1 rides the origins session N
        // verified (a stale seed self-corrects through the demote);
        // and the substitution line in the report
        // (attempts/wins/pool — the one line that says whether the
        // cross-cache bet worked in the field). Android-gated lanes
        // stay android-gated (ADR-0028 decision 5: desktops
        // byte-for-byte). Installs over debug.19 (the same pinned
        // debug certificate — D85). The D84 rule: every debug build
        // bumps this. D103: the APK asset is named
        // v1.6.6-debug20.apk.
        versionCode = 19
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
