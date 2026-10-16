#!/usr/bin/env bash
# build-android.sh — the Younblock Android lane (docs/android-port 04 §3,
# doc 06; roadmap M3). One source tree, same as the desktop lanes; this
# script never touches build.sh / build.bat (ADR-0001).
#
# Prerequisites (documented, not hidden):
#   * Go 1.27+ on PATH (go.mod directive go 1.26.0)
#   * JDK 17+ with javac on PATH (JAVA_HOME set)
#   * Android SDK at ANDROID_HOME with:
#       - cmdline-tools, platform-tools
#       - platforms;android-26 and platforms;android-36
#       - build-tools;36.1.0
#       - an NDK under $ANDROID_HOME/ndk/ (set ANDROID_NDK_HOME)
#   * Node/npm for the frontend (same as every other lane)
#
# What it does, in order:
#   1. Frontend: npm ci -> vite build (the same bundle as every edition)
#   2. Copy the bundle into internal/webui/dist so go:embed carries it
#      (placeholder.txt stays; the copied files are git-ignored)
#   3. gomobile bind -> android/app/libs/younblock.aar
#      (arm64-v8a, armeabi-v7a, x86_64; min API 26 — doc 06 §1)
#      The bind runs with GOFLAGS=-mod=mod: gomobile must read the module
#      graph (`go list -m all`), which vendor mode cannot serve; the
#      module cache and proxy.golang.org cover it.
#   4. ./gradlew assembleDebug -> android/app/build/outputs/apk/debug/
#
# Exit codes are honest: any failing step stops the lane with its status.
set -euo pipefail
cd "$(dirname "$0")"

echo "=== Younblock Android build ==="

echo "[1/4] Frontend bundle..."
( cd frontend \
  && npm ci --no-audit --no-fund >/dev/null 2>&1 \
  || npm ci --no-audit --no-fund --legacy-peer-deps >/dev/null 2>&1 \
  || npm install --no-audit --no-fund --legacy-peer-deps >/dev/null 2>&1 )
( cd frontend && npm run build )
mkdir -p internal/webui/dist
cp -r frontend/dist/. internal/webui/dist/

echo "[2/4] gomobile bind -> android/app/libs/younblock.aar..."
mkdir -p android/app/libs
GOFLAGS=-mod=mod gomobile bind \
  -target=android/arm,android/arm64,android/amd64 \
  -androidapi=26 \
  -o android/app/libs/younblock.aar \
  ./mobile

echo "[3/4] Gradle assembleDebug..."
( cd android && ./gradlew assembleDebug )

APK="android/app/build/outputs/apk/debug/app-debug.apk"
echo "[4/4] Artifact record (doc 06 §5)..."
ls -la "$APK"
sha256sum "$APK"
echo "Done. Install with: adb install -r $APK"
