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
        // versionCode 27: the M8-aj build (debug.28 -- the
        // lane retires round, ADR-0049). The debug.27 field
        // report: "this one did not even open a single video,
        // only feed refreshed" -- and it was the cleanest
        // experiment result yet: substitution attempts=40
        // wins=37 big=0 small=37, the fed pool kept the verdict
        // lane engaged on every dead-cluster attempt, every
        // answer the ~5KB verdict shape, and the player NEVER
        // left the dead URL set (one host: 20 flows, one every
        // ~4s; no live cluster ever dialed; 1.2MB total vs
        // debug.26's 23.1MB). Worse: dead=0 -- substituted flows
        // never terminal, so the cluster was never marked dead;
        // in debug.26 the same regime recovered in ~40s exactly
        // because the (accidentally drained) lane let the flows
        // terminal and the door's fast-fail RESETS let the
        // player's ladder escalate to a fresh player-API fetch.
        // The RST is the ladder's escalation signal; the HTTP
        // verdict is its sedative. The fix: the four-round
        // experiment concludes OFF (zero media ever carried
        // cross-cluster) -- schema v7 retires
        // experimentSubstitution by default and migrates every
        // existing file off (a v7 file's explicit on is the
        // owner's word); and the round found the switch's off
        // has been a NO-OP at engine start since debug.24 (the
        // engine knob resolved off to 0, which the proxy reads
        // as "use the default") -- it now returns -1, the real
        // disabled shape, so OFF means OFF. The door's
        // dead-cluster shape returns to debug.26's probe ->
        // fast-fail. Installs over debug.27 (the same pinned
        // debug certificate -- D85). The D84 rule: every debug
        // build bumps this. D103: the APK asset is named
        // v1.6.6-debug28.apk.
        versionCode = 27
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
