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
        // versionCode 16: the M8-y build (the EIGHTH pasted report —
        // the first from the right build, shell line versionCode 15:
        // both debug.16 signatures present, the raced line and the
        // tier= stall line, and Ali's own words confirmed ADR-0022 —
        // "the problem of easily going next and back in same video
        // instantly is fixed" — while "the original problem (videos
        // taking time to play) persist"). That report names the start
        // lag's two remaining sources, both measured: (1) the DNS
        // negative-cache lockout — dns ok 142 / fail 50 with
        // doorFailures=50, every resolver failure a dead door dial,
        // the failure lines pairing at the same millisecond (four
        // pairs inside 22 ms = instant negative hits, not server
        // walks): one DoH blip negative-cached a batch of fresh
        // per-video hostnames and the 15 s TTL killed every retry
        // wave at the door; ADR-0023's negative re-ask makes a
        // DNS_UNREACHABLE negative re-ask the servers (singleflight-
        // collapsed, rate-limited to one walk per second per host —
        // the window starts at the failure's own walk), so the next
        // retry after the blip recovers in one walk instead of 15 s
        // of dead flows. (2) The dead-origin re-gamble — ~17
        // reason=timeout terminals at 2892-4304 ms plus ~8 resets,
        // all down=0 B through the raced round, and the SECOND
        // video's start paying the full discovery cost again (a new
        // rr hostname, the per-flow dead-IP map dead with its flow,
        // the authoritative answer returning the same blackholed
        // cache-node IPs): ADR-0023's origin-IP penalty box marks
        // every classified first-byte death's IP for 60 s and every
        // dial list skips the marked IPs (a preference, never a
        // verdict — an all-penalized list dials in full), so the
        // knowledge crosses flows AND hostnames. The socks5 door's
        // DNS line now bakes reason= (the 50 bare lines could not say
        // why). Installs over debug.16 (same pinned debug certificate
        // — D85). The D84 rule: every debug build bumps this. D103:
        // the APK asset is named v1.6.6-debug17.apk.
        versionCode = 16
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
