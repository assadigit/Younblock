import {defineConfig} from 'vite'
import {svelte} from '@sveltejs/vite-plugin-svelte'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [svelte()],
  build: {
    // The any-version parse floor (the 1.6.7-debug.2 round): the Android
    // edition's minSdk is 26 (Android 8.0, 2017), and an 8.0/8.1 phone
    // whose WebView was never updated ships Chrome ~58-60 — plenty of
    // devices where Play cannot deliver WebView updates. Vite 8's
    // default target ('baseline-widely-available', ~Chrome 107) emits
    // syntax those WebViews cannot PARSE (optional chaining, class
    // static blocks...): a page that never boots — the same
    // "only my phone works" failure class the round is closing, one
    // layer down. es2017 keeps async/await native (Chrome 55+) and
    // downlevels every newer SYNTAX through esbuild; runtime APIs are
    // guarded at their call sites (LeverButton's ResizeObserver is
    // feature-detected for exactly this reason). The desktop editions
    // lose nothing — the transpiled bundle behaves identically.
    target: 'es2017',
  },
})
