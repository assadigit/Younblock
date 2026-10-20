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
        // versionCode 20: the M8-ac build (debug.21 -- the hostile-network
        // round): the tester's FIRST debug.20 report came from 6:04 PM
        // Tehran (evening peak; the debug.19 test was 4:17 AM) and
        // measured a REGIME CHANGE, not just a regression: the
        // dead-origin probe answered on both SNI variants (the IP
        // blackholing is GONE), 35 of 77 probe verdicts came back
        // reset, and the DoH lane's connections churned (dials 59,
        // was 6 thirteen hours earlier) -- the censor moved from
        // blackholing cache-node IPs to killing connections in
        // flight. The debug.20 machinery itself worked (substitution
        // attempts=7 wins=5, first bytes 70-524ms everywhere,
        // auto-timeline play->first-data elapsed=2500ms pairs=2 --
        // the 45 s cold-start was SOLVED early in the session) but
        // five of its choices became self-harm under the new regime:
        // the box held 12 penalized IPs that all ANSWERED (deaths are
        // temporal now, not per-IP), the flow-fed good pool drained
        // to gvideo=1 (every terminal's deaths demoting the very
        // marks its wins had written), liveIPs pinned every flow onto
        // the same verified origin (www.youtube.com 17/17), every
        // raced death invalidated DNS through the dying DoH lane, and
        // the 45 s fast-start cadence sent scan-like bursts exactly
        // while the network was hostile. The round: (1) the
        // evidence-weighted marking (ADR-0034) -- reset-class deaths
        // box 20 s WITHOUT demoting (a temporal signal), timeout
        // deaths keep 60 s + demote, PENDING stall kills (the client
        // spoke after the origin's last byte: a request left
        // unanswered) demote at BOTH tiers, IDLE kills never demote,
        // probe resets box 60 s (was 10 m), strike escalation capped
        // at [60s, 2m, 5m], and the probe pass REHABILITATES boxed
        // IPs (answered = cleared + marked good + counted); (2) the
        // anti-concentration rotation (ADR-0034) -- liveIPs and
        // goodSubstitutes rotate their fronts per call so consecutive
        // flows spread across verified origins; (3) the cadence
        // rebalance + DoH brownout (ADR-0035) -- the 45 s fast-start
        // lane REMOVED (the persisted seed and flow-fed marks own its
        // job), the churn kick stays debounced, and >=3 DoH dials in
        // 60 s pauses every background lane until the resolver
        // recovers (counted); (4) the cause-split diagnostics
        // (ADR-0036) -- stall kills split pending/idle (THE number
        // that says whether the engine is killing dead origins or
        // the app's pooled connections), rehabilitated/goodDrops/
        // brownouts on the report. Installs over debug.20 (the same
        // pinned debug certificate -- D85). The D84 rule: every
        // debug build bumps this. D103: the APK asset is named
        // v1.6.6-debug21.apk.
        versionCode = 20
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
