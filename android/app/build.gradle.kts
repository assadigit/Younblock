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
        // versionCode 17: the M8-z build (debug.18 — the cold-start
        // round per docs/specs/debug18-cold-start.md): fragment EVERY
        // routed app's TLS on Android (FragmentAllRouted, settings v5,
        // Android-default-on — the debug.16 evidence's plain cohort:
        // the ad/attestation hosts the YouTube app needs at first
        // play, un-fragmented and hanging to the app's ~60 s
        // patience), the plain-relay 8 s safety net
        // (plainFirstByteTimeouts + class=plain kill lines — never a
        // dead-IP mark), the one-warm-connection hedged DoH
        // (http2.Transport + DialTLSContext over the fragmented
        // bootstrap dial, ReadIdleTimeout 15 s / PingTimeout 5 s,
        // WarmProbe at start + re-warm within 1 s of a
        // connection-shaped failure, the sticky server at t=0 and the
        // next at +300 ms with fast failures triggering the next
        // immediately, two no-SNI IP-literal endpoints appended to the
        // default list, the learned googlevideo sn-<code> codes
        // pre-resolved rr1..rr5 every 5 minutes and persisted to
        // googlevideo-codes.txt), the measurable QUIC fallback
        // (udp443 dropped + the next-TCP-flow elapsed line + v6Dropped;
        // the ICMP 3/3 reply stays gVisor's, pinned by the M8-j
        // guard test), and the ADR-0027 diagnostics surface (class=
        // on every tunnel line, rtt-class=near|far, the Top-10
        // slowest-or-failed hosts, the plain-relay summary, the DNS
        // detail with server/phase/latency, the one-shot dead-origin
        // SNI probe, the MARK timeline with the "I pressed play" /
        // "First frame" buttons, the DEBUG BUILD hostnames notice —
        // this lane's -ldflags sets config.DebugTesterBuild=true, a
        // release build never does). Installs over debug.17 (the same
        // pinned debug certificate — D85). The D84 rule: every debug
        // build bumps this. D103: the APK asset is named
        // v1.6.6-debug18.apk.
        versionCode = 17
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
