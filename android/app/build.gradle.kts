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
        // versionCode 25: the M8-ah build (debug.26 -- the
        // dead-cluster memory round, ADR-0047). The debug.25
        // field report: "every video takes initially 1-2
        // minutes to open, after opening it's smooth" -- and
        // the report itself held the whole story: the entire
        // sn-qxau5 cluster blackholed (21 flows, every one
        // worstFirstByte=0ms: the dial accepts, no TLS byte
        // ever returns; the dead-origin probe confirmed
        // IP-range blackholing -- silent under every SNI),
        // each attempt burning its 3s first-byte budget,
        // interleaved with 7 breaker cooldowns (7 x ~15s ~=
        // the whole open latency), while sn-aj4g55/u14g55/
        // 4g5ednkd answered instantly and carried 1.8MB/559KB;
        // the pool sat empty (gvideo 0, attempts 0) because
        // only the operator-pressed network test could feed
        // it. Four fixes: (1) dead-cluster memory -- 3
        // distinct hosts of one sn-cluster silent-terminal
        // inside 10min mark the CLUSTER dead: its hosts fail
        // INSTANTLY at the door (zero wire, no 3s burn) unless
        // the media-verified pool serves the flow or the
        // per-cluster probe (1 per 2min) is due; an answered
        // own-dial rehabilitates. (2) The payload-fed pool --
        // a gvideo flow that delivered >=64KB certifies its
        // origin media-verified automatically (5x above the
        // debug.23 false-positive class, 6x below the smallest
        // real flow): the pool feeds itself from the app's own
        // traffic. (3) The multi-cluster arm gate -- the
        // breaker needs terminals from TWO distinct clusters
        // before arming: a single-cluster burst is the
        // blackhole signature; the debug.25 session would have
        // armed zero times. (4) The pool file's versioned
        // format (ADR-0045's flip condition) -- persisted
        // entries carry a per-line "media" marker and the next
        // session seeds them back into the POOL: after one
        // good session, every session's FIRST video rides the
        // verified origin. New report line: "gvideo clusters:
        // dead=... fastFails=... probes=... rehabilitated=...
        // payloadVerified=...". Installs over debug.25 (the
        // same pinned debug certificate -- D85). The D84 rule:
        // every debug build bumps this. D103: the APK asset is
        // named v1.6.6-debug26.apk.
        versionCode = 25
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
