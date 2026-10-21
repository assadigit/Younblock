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
        // versionCode 26: the M8-ai build (debug.27 -- the
        // pool keeps its food round, ADR-0048). The debug.26
        // field report: "first video still buffers (less than
        // before), later videos mostly open nearly quickly" --
        // and the report showed every M8-ah piece working
        // (dead=1, payloadVerified=6, cooldowns=1, attempts=14
        // wins=12) beside fastFails=150: the player ground its
        // hostname rotation against instant resets for ~40s per
        // unlucky open because the verdict lane (the door's
        // pool pass) STARVED mid-storm -- the pool was an
        // OUTFLOW machine. Three drains: a lost substitute race
        // demoted its pool origin; a pending stall kill demoted
        // the origin a substituted flow rode; and -- the
        // biggest, caught by this round's own test before the
        // field did -- every first-byte WIN replaced the
        // winner's good-table entry with a handshake mark,
        // un-flagging media-certified origins one flow at a
        // time (six certifications fed, two held at session
        // end). Four fixes, one property -- a media
        // certification (>=64KB delivered payload or an
        // answered real-SNI probe) is revocable ONLY by its
        // 15-minute TTL: (1) the no-downgrade rule (a handshake
        // mark keeps the flag); (2) the demote guard (deaths
        // box but never un-certify; the strike-escalated box
        // stays the rotation guard); (3) the feed clears the
        // box (a fresh delivery outranks the stale death);
        // (4) the verdict lane made visible -- the door's
        // dead-cluster pool pass counts and logs (verdicts= on
        // the clusters line), substituted flows classify by
        // size (big=/small= on the substitution line), and the
        // doors log their refusal REASON (cluster-dead vs
        // breaker-cooldown). Two readings retired hypotheses:
        // the substituted wins all delivered ~5KB answers (the
        // cross-cluster substitute is the fast-verdict lane,
        // never a media carrier -- every payload verification
        // rode an own-dial win), and the udp443 "fallback
        // elapsed" samples are heuristic pairings, not QUIC
        // timeouts (gVisor's ICMP port-unreachable is already
        // on the wire). Installs over debug.26 (the same
        // pinned debug certificate -- D85). The D84 rule:
        // every debug build bumps this. D103: the APK asset is
        // named v1.6.6-debug27.apk.
        versionCode = 26
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
