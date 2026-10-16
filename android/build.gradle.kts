// Root build file: pins the toolchain versions for the whole module
// (values verified against maven.google.com / repo1.maven.org this
// session; see docs/android-port/06 for the reproducibility table).
// Kotlin support is built into AGP 9 — no separate kotlin-android plugin.
plugins {
    id("com.android.application") version "9.2.0" apply false
}
