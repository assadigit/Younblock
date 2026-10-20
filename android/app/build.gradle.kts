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
        // versionCode 24: the M8-ag build (debug.25 -- the
        // breaker-stops-starving round, ADR-0046). The debug.24
        // field report was the W5 sensor working exactly as
        // designed -- and killing playback with it: armed on the
        // play-burst's dead sn-qxau5 cache cluster (flows=3
        // failures=3, worstFirstByte=0ms), ONE 45s+jitter cooldown
        // blanket-covered the session: 58 door-dial refusals with
        // zero wire, the player's retry budget dead at +46s, video
        // never opened, feed perfect -- while the same report proved
        // the wire healthy twice (network test answered 5/5 under
        // the real SNI, media-verifying pool=6/gvideo 5; the one
        // flow that slipped through pre-arm delivered 406KB
        // win=orig). Four fixes, all inside the breaker's scope:
        // (1) the pool escape -- a media-verified address is
        // answered-under-the-real-SNI evidence, the strongest
        // wire-alive proof the engine owns, so a flow the pool can
        // serve passes the door and rides the substitute lane
        // (attempts=0 in the report: the door-block short-circuited
        // before the raced round where substitutes engage);
        // (2) the scout -- each cooldown's FIRST googlevideo flow
        // goes to the wire (the app's own request, zero synthetic
        // emission) so an armed breaker is never blind; (3) the
        // lift -- any googlevideo first byte ends an active cooldown
        // on the spot (a speaking wire falsifies the arm's premise;
        // the breaker becomes self-correcting both ways);
        // (4) the budget -- cooldown 45s -> 15s: the block can
        // never outlive the player's measured retry patience, while
        // 15s still spans the app's 2-7s retry cadence for hostname
        // rotation. The discipline line gains lifts= and scouts= so
        // the next report tells the worlds apart. Switches keep
        // their meaning: breaker-off = the debug.22 shape;
        // breaker-on+substitution-off = the M8-ae door + the scout.
        // Installs over debug.24 (the same pinned debug
        // certificate -- D85). The D84 rule: every debug build
        // bumps this. D103: the APK asset is named
        // v1.6.6-debug25.apk.
        versionCode = 24
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
