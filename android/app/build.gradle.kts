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
        // versionCode 22: the M8-ae build (debug.23 -- the
        // wire-discipline round): the tester's post-debug.22 report
        // was "it doesn't even open video after a while. But the
        // feed refreshes. this is probably one of the worst
        // versions" plus two reference repositories
        // (gfw_resist_HTTPS_proxy, WebImmortalGuards) as "knowing
        // how this system works" -- and they told us exactly that:
        // the reference implementations emit ONE fragmented hello
        // per connection and ZERO background handshakes, while this
        // engine was emitting ~4 hellos per video connection (the
        // raced fan-out, each a legacy 299-cut hello) plus
        // 48-handshake background bursts at start, every 5 minutes,
        // and within 10 s of every failed flow. A censor that flags
        // noisy sources per service name trips on exactly that
        // volume: early session clean, then every googlevideo
        // connection reset forever while youtube.com (quieter,
        // different name) keeps serving -- the feed refreshes, video
        // never opens, our retries keep feeding the flag. The round
        // (ADR-0040/0041/0042): (1) a primary whose origin holds a
        // live verified mark skips the raced round -- ONE hello,
        // the reference steady-state shape; a raced round demotes
        // the answer set ONCE, not once per dead candidate. (2) The
        // probe pass is a trickle -- OriginProbePerPass 48->10
        // clamped inside ProbeOrigins, its own GoogleVideoProbeEvery
        // 15-minute cadence, the START-of-session pass and the
        // churn-kick lane DELETED (a fresh engine is silent on
        // googlevideo until the first flow speaks). (3) The EMISSION
        // BREAKER: 3 googlevideo terminals inside 30 s arm a ~45 s
        // cooldown (plus jitter) during which googlevideo flows fail
        // fast at BOTH doors with zero wire and the probe lane stays
        // paused -- the app's player rotates hostnames (the
        // reference recovery) and any per-source flag decays in the
        // quiet; non-gvideo hosts are untouched, the feed keeps
        // refreshing through the cooldown. (4) Diagnostics: the
        // gvideoHellos meter counts every googlevideo-SNI hello the
        // engine puts on the wire (flows + substitutes + probes)
        // and the report gains "gvideo discipline: hellos=N
        // cooldowns=M" -- read hellos against routedFlows; ~1 per
        // flow is the goal. Installs over debug.22 (the same pinned
        // debug certificate -- D85). The D84 rule: every debug
        // build bumps this. D103: the APK asset is named
        // v1.6.6-debug23.apk.
        versionCode = 22
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
