# 02 - UI Design Spec (v2): Younblock

**Audience:** a coding agent (frontend and Windows shell) and Ali (design owner). Backend contract: `01-go-backend-rewrite-spec.md`, section 10. Reliability rules touching the UI: `03-reliability-suggestions.md`. The same frontend will later run inside an Android shell (`04-android-spec.md`), so follow the fluid-layout and adapter rules below now.

## 0. Plain-language summary (for Ali)

One calm dark window, built around the same idea as your current app: a big power button, one state word, a timer. Changes: the harsh white disc is gone, "Off" is a quiet neutral instead of alarm red, green means working, red is kept only for your logo and real errors. Everything technical is hidden. A small menu holds Settings, Help and About. When you close the window the app keeps running in the tray, and the tray icon tells you the state. If something stops working, the app tries to repair itself first and only then asks you to look.

## 1. Locked decisions (do not reopen without asking Ali)

| Topic | Decision |
|---|---|
| Audience | Ali and his friends; not a public release |
| Usage | Manual: open the app and press the button. Auto-start with Windows (asked once on first run) and a tray icon. Auto-turn-on is an option, off by default |
| Control | Just the button. The app picks its own method and repairs itself. Technical settings live only in a collapsed Advanced section |
| Browser | Google Chrome. The UI says "Works with Chrome" |
| Language | English only. No Farsi, no right-to-left work |
| Look | Direction B: charcoal background from the current brand, the red UNBLOCK logo kept, calm neutral when off, clear green when on, red only for the logo and real errors |
| Platforms | Windows now (Wails v2). Android later with the same Svelte UI |
| Name | Younblock. Optional reference screenshots of the old app may be in `docs/reference/` |

## 2. Design plan

**Reviewed against the generic VPN-app template, and changed:** no world map or server list (there are no servers), no speed graph, no gradient hero, no tracked all-caps labels, no row of identical cards, no colored state text. Boldness is spent in one place only: the power button. The old white disc, pink inner disc, blue gradient and "Ping" line are retired. Ping showed `-- ms` even while connected, so it looked broken.

### Color

Brand colors were sampled from the old app: charcoal `#2E2E2E`, logo red `#CE201F`. Category convention (network utility) is a deep neutral base with one clear accent. Harmony: a complementary pair (red logo, green On) inside a dominant neutral, roughly 60% charcoal, 30% surface and text, 10% green plus the logo. Red-for-off is removed on purpose: the red/green pair is the hardest for color-blind users, and "off" is a normal state that should not look like an alarm. State is also carried by shape and by the word, never by color alone. Classification: **durable** (neutral dark, one green, a brand red), not trend-driven. No logged taste data exists for Ali yet, so this is a fresh judgment; the old neon green `#17FF38` was replaced by a calmer emerald.

| Role | Hex | Token | Use |
|---|---|---|---|
| Background | `#1E1F21` | `--neutral-950` | window background |
| Surface | `#2A2B2E` | `--neutral-850` | button disc, menu, rows |
| Border | `#35373B` | `--neutral-800` | hairlines, off ring |
| Text | `#F2F2F2` | `--color-text` | primary text |
| Muted text | `#A3A6AD` | `--neutral-400` | secondary text |
| On / primary | `#34C77B` | `--green-500` | On state, primary actions |
| Connecting / warning | `#F2B84B` | `--amber-400` | connecting, check needed |
| Error | `#FF6A5F` | `--danger` | error ring and text |
| Brand red | `#CE201F` | `--brand-red` | **logo only** (and the tray attention dot never uses it) |
| Focus ring | `#96E4BB` | `--green-300` | keyboard focus |

Contrast (computed, WCAG): text on background 14.73:1; muted on background 6.77:1; muted on surface 5.81:1; green on background 7.54:1; dark glyph `#141516` on green 8.36:1; amber on background 9.22:1; error on background 5.87:1; white text on the brand-red logo box 4.87:1. All text pairs pass AA. **Brand red on the background is 3.02:1 and fails for text; use it only for the logo graphic.** Do not use `--neutral-500` or darker for text.

```css
:root {
  /* scales */
  --neutral-50:#F7F7F8; --neutral-100:#ECEDEE; --neutral-200:#D7D8DB; --neutral-300:#B9BBC1; --neutral-400:#A3A6AD;
  --neutral-500:#787D87; --neutral-600:#656971; --neutral-700:#4D5056; --neutral-800:#35373B;
  --neutral-850:#2A2B2E; --neutral-900:#222325; --neutral-950:#1E1F21;
  --green-50:#F4FBF7; --green-100:#E4F6ED; --green-200:#C2EFD8; --green-300:#96E4BB; --green-400:#65D79C;
  --green-500:#34C77B; --green-600:#2CAA69; --green-700:#228150; --green-800:#175937; --green-900:#123523;
  --amber-300:#F6CE83; --amber-400:#F2B84B; --amber-500:#EEA111; --amber-700:#98670B;
  --red-400:#E65757; --brand-red:#CE201F; --red-600:#BA1D1C; --red-700:#8E1615;
  --danger:#FF6A5F;

  /* roles */
  --color-bg: var(--neutral-950);
  --color-surface: var(--neutral-850);
  --color-border: var(--neutral-800);
  --color-text: #F2F2F2;
  --color-text-muted: var(--neutral-400);
  --color-primary: var(--green-500);
  --color-on-primary: #141516;
  --color-warn: var(--amber-400);
  --color-danger: var(--danger);
  --color-focus: var(--green-300);
  --glow-on: 0 0 0 10px rgba(52,199,123,.12), 0 0 44px rgba(52,199,123,.26);

  /* type */
  --font: "Inter", "Segoe UI", system-ui, sans-serif;
  --text-xs:12px; --text-sm:14px; --text-md:16px; --text-lg:20px; --text-xl:28px; --text-timer:28px;

  /* space (4 px base) and radii (a hierarchy, not one radius for everything) */
  --space-1:4px; --space-2:8px; --space-3:12px; --space-4:16px; --space-6:24px; --space-8:32px; --space-12:48px;
  --radius-sm:8px; --radius-md:12px; --radius-lg:20px; --radius-full:999px;
}
```
Put this in `frontend/src/styles/tokens.css`.

### Type

One family: **Inter** (🔵 durable, neutral; the logo supplies the personality). Bundle static `.woff2` files for weights 400, 500, 600, **subset to Basic Latin + Latin-1 + common punctuation**, total fonts 120 KB or less, with the OFL license. No CDN. Sentence case everywhere, no all-caps labels. Scale: caption 12/16, body 14/20, body-large 16/24, title 20/28, state label 28/34 (weight 600, letter-spacing -0.01em), timer 28/32 with `font-feature-settings: "tnum"`. Lines under 60 characters in Help and About.

### Logo

If Ali has put his logo in `assets/` (`logo.svg` or `logo.png`), use it, at about 28 px tall in the header. If not, build a CSS wordmark: "YO" in `--neutral-600`, then "UNBLOCK" in white on a `--brand-red` box (radius 4 px), Inter weight 800, letter-spacing -0.02em, and tell Ali in your report that his original logo file should replace it. **Drop the tagline "unmatched speed"**; it is a claim the app cannot prove.

## 3. Window and platform

- **Windows (Wails v2, WebView2):** logical window **360 x 560**, fixed (`DisableResize: true`), standard dark title bar (dark theme option; custom title-bar colors matching `--neutral-950` if the installed Wails version supports it; verify, do not guess). Window background `--neutral-950` so there is no white flash. Launched with `--minimized`: start hidden in the tray.
- **Fluid layout (required now):** the page must work from **320 to 480 CSS px wide and 480 to 900 tall**, not only at 360 x 560, because Android will reuse it. Use flex layout, `100dvh`, `env(safe-area-inset-*)` padding, and no fixed pixel positions for the hero.
- **Capabilities:** read `GetAppInfo().capabilities` and show only what the platform supports (for example the "Open when Windows starts" row only if `startWithOS`).
- **Frontend:** Svelte + TypeScript (Wails template `svelte-ts`), plain CSS with the tokens above. No component kits, no Tailwind runtime, no CDN assets. Bundle excluding fonts: 250 KB or less.
- **Icons:** inline SVG from Lucide (ISC license): `power`, `menu`, `settings`, `life-buoy`, `info`, `log-out`, `chevron-right`, `chevron-left`, `check`, `x`, `copy`, `folder-open`, `globe`. Stroke 1.75, round caps. List the license in About.

## 4. Screens

Navigation: a **header menu button** (no tab bar). Header: logo at the start, menu button at the end. The menu is a small panel with: Settings, Help, About, a divider, Quit Younblock. Settings, Help and About open as full-screen views with a back chevron. Esc closes menus and views.

### 4.1 Connect (home)

```
+--------------------------------+
| [YO|UNBLOCK]              ( = ) |   header 52 px
|                                |
|          .----------.          |
|        .'  .------.  '.        |
|       |   |   (|)   |   |      |   power button (hit area 208 px)
|        '.  '------'  .'        |
|          '----------'          |
|                                |
|              On                |   state label 28/34, white
|      Unblocking YouTube        |   subtitle 14/20, muted
|           00:42:18             |   timer 28, tabular
|         12.4 MB used           |   quiet line, muted
|                                |
|     (globe) Works with Chrome  |   footer note 12/16, muted
+--------------------------------+
```

**Power button** (the one bold element):
- Layers: halo ring 184 px (1 px, 30% of the state color), ring 156 px (2 px), disc 132 px, glyph 48 px (power icon, stroke 3.5, round caps). Hit area 208 x 208. Pressed: scale 0.97 over 80 ms.
- A real `<button role="switch" aria-checked>` named "Unblock YouTube". Click, Space and Enter toggle. While `connecting`, click cancels. While `stopping`, clicks are ignored. Debounce so a double click cannot start two operations.
- Readable without color: Off is an **outlined** glyph on a dark disc; On is a **solid green disc** with a dark glyph; check-needed has a **dashed** ring; error has a **solid error-colored** ring. The state word is always shown, in `--color-text` (never colored).

| Phase | Disc | Glyph | Ring | State label | Subtitle (14/20) | Extra |
|---|---|---|---|---|---|---|
| `off` | `--neutral-850` + 1 px `--neutral-800` | `--neutral-400` outline | `--neutral-800` | Off | Turn on to unblock YouTube | none |
| `connecting` | `--neutral-850` | `--amber-400` | amber arc, 90 degree sweep, rotating (1.2 s linear loop) | Starting… | from `detail`: starting = "Getting ready…", checking = "Checking YouTube…", trying:* = "Trying another method…" | click cancels |
| `on` | `--green-500` solid | `#141516` | `--green-500` | On | Unblocking YouTube | `--glow-on`, static; timer runs |
| `degraded` | `--neutral-850` | `--amber-400` | amber, dashed | Check needed | On, but YouTube isn't responding | primary button "Fix connection" below the quiet line |
| `error` | `--neutral-850` | `--danger` | `--danger` solid | Couldn't turn on | plain sentence from the error code (4.6), in `--danger` | link "Open help" |
| `stopping` | `--neutral-850` | `--neutral-400` | `--neutral-800` | Turning off… | | inert |

- **Notice line:** when `state.notice` is present (for example `METHOD_SWITCHED`), show one line under the subtitle for 10 s, in `--color-text-muted`: "Switched to a stronger method" or "Fixed". Text from the copy deck.
- **Timer and quiet line** keep their height in every state (no layout jump). The timer shows only in `on` and `degraded`. The quiet line shows "`<n>` used" (bytesDown plus bytesUp; 1024-based; B, KB, MB, GB; one decimal above KB) only in `on` and `degraded`.
- The footer note "Works with Chrome" is plain text with a globe icon, not a button.
- **Fix connection** calls `AutoFix`, shows the connecting presentation while it runs, then the result.

### 4.2 First run (shown once, when `firstRunDone` is false)

Full-screen view over Connect: logo; title "Welcome" (20/28); text "Younblock helps YouTube load in Chrome. Turn it on when you need it. Closing this window keeps it running in the tray." (14/20, muted); one checkbox row **"Open Younblock when Windows starts"**, checked by default; button **"Get started"**. Saving sets `firstRunDone` and `startWithWindows` from the checkbox. Nothing else on this screen.

### 4.3 Settings

Start-aligned list with 1 px dividers (not cards). Rows: **Open Younblock when Windows starts** (switch), **Turn on automatically when the app opens** (switch, off by default). Below, a collapsed row **Advanced** (label "For troubleshooting") containing: Unblock mode (YouTube only, All websites, Manual; each with a one-line explanation; Manual shows `127.0.0.1:<port>` with a Copy button), Connection method (Automatic, Fast, Balanced, Strong, Original), port, secure-DNS servers (one per line), YouTube domain list (one per line), "Keep website names in the log" (off), Reset to defaults. Each change saves immediately through `SaveSettings` and shows the normalized value returned.

### 4.4 Help

One scroll, start-aligned, text and dividers, no cards.

**Connection test.** Button "Run test". Four steps with a check or cross, a short result and a hint when one fails: *This app is running*, *Secure lookup works*, *YouTube answers*, *Video servers answer*. Progress arrives via `test:progress`. If step 3 or 4 fails, show "Fix connection" (calls `AutoFix`, then re-runs the test). If nothing works: "None of the methods worked on this connection. Your network may be blocking the secure lookup. Open Settings, Advanced, and try another secure-DNS server."

**Common problems** (accordion, one open at a time):
1. *YouTube still doesn't load.* Run the connection test. If a step fails, tap Fix connection. Then fully close Chrome and open it again.
2. *Where did the window go?* Younblock keeps running in the tray, near the clock. Click its icon to open the window. Choose Quit to stop it completely.
3. *Does it work in other browsers or apps?* It is made for Chrome. Other apps that follow Windows' proxy setting may work, but they are not tested.
4. *My internet stopped after the app closed.* Tap "Restore internet settings" below. It removes the setting the app added. It also happens automatically the next time the app starts.
5. *Windows or my antivirus shows a warning.* The app is not code-signed yet, and Windows warns about unsigned apps that change proxy settings. Only run it from a source you trust.
6. *Is this a VPN?* No. It does not hide your IP address and adds no encryption; YouTube's own HTTPS still protects the content. It only changes how the first message to the site is sent.
7. *YouTube loads but videos buffer.* Tap Fix connection once. If it still buffers, the limit is probably your connection speed, not this app.

**Still stuck?** Three buttons: "Copy diagnostic report" (calls `GetDiagnosticReport`, copies it, toast "Copied"), "Open logs folder", "Restore internet settings" (calls `RestoreWindowsProxy`, toast "Internet settings restored").

### 4.5 About

Order: wordmark and version; **How it works** (a real sequence, so numbering is appropriate) with a simple inline-SVG diagram of three nodes (*Chrome*, *Younblock on your PC*, *YouTube*) drawn with the tokens; **What it doesn't do**; **Privacy**; licenses.

How it works:
1. Chrome sends its request to Younblock, which runs only on your PC.
2. Younblock looks up YouTube's address with a secure DNS service, so the lookup can't be tampered with on the way.
3. Younblock sends the first "hello" message to YouTube in small pieces. Filters that scan that message for the website's name usually don't recognise the pieces.
4. After that, everything flows normally, protected by YouTube's own HTTPS.

What it doesn't do: it is not a VPN. It does not hide your IP address. It does not add encryption beyond what HTTPS already provides. Your internet provider can still see which servers you connect to. It may stop working if the filtering changes.

Privacy: no account, no analytics, no updates in the background. The only network requests are secure DNS lookups to the provider in use and the YouTube checks the app runs. Website names are not written to the log unless you turn that on.

Licenses: an expandable list (Inter, Lucide, Svelte, Wails, tray library, Go libraries).

### 4.6 Error code to plain words

| Code | Message |
|---|---|
| `PORT_BUSY` | Another program is using this app's port. Close it, or open Settings, Advanced, and change the port. |
| `DNS_UNREACHABLE` | The secure lookup service can't be reached. Check your internet, or try another one in Settings, Advanced. |
| `DNS_BLOCKED_ANSWER` | The lookup returned an address that can't be right. Try another secure-DNS server in Settings, Advanced. |
| `PROXY_SETTING_FAILED` | Windows wouldn't accept the setting. Open Help and tap Restore internet settings. |
| `SELFTEST_FAILED` | YouTube still isn't reachable. Open Help and run the connection test. |
| `UNKNOWN` | Something unexpected happened. Open Help and copy the diagnostic report. |

Notice codes: `METHOD_SWITCHED` = "Switched to a stronger method"; `RECOVERED` = "Fixed".

## 5. Tray icon and notifications (Windows)

- Three icons at 16, 20, 24 and 32 px, designed to read at 16 px, differing by **shape**: **off** = outlined ring with the power notch in `--neutral-400`; **on** = solid `--green-500` ring with a dark notch; **attention** = ring with a solid `--amber-400` dot. They must remain distinguishable in grayscale. Light-taskbar variants with darker outlines are required (Windows can use a light taskbar).
- Tooltip: `Younblock: Off`, `On`, `Starting…`, `Check needed`.
- First time the window is closed (`closeNoticeShown` false): a small in-app message before it hides, "Still running. Find Younblock near the clock. Choose Quit to stop it." with an "OK" button; then set `closeNoticeShown`.
- Windows notifications only for failure and recovery (rules in file 01, section 9). Plain text, no sound beyond the Windows default.

## 6. Copy deck (English)

All strings live in `frontend/src/i18n/en.json` (keep the file even though there is one language, so Android and future languages are cheap). **No hard-coded text in components.**

State labels: Off, Starting…, On, Check needed, Couldn't turn on, Turning off…. Buttons: Fix connection, Run test, Get started, Copy diagnostic report, Open logs folder, Restore internet settings, Open help, OK. Menu: Settings, Help, About, Quit Younblock. Stats: "`<n>` used". Footer: "Works with Chrome". Toasts: "Copied", "Internet settings restored". Everything else is written out in section 4. Voice: plain verbs, sentence case, errors say what happened and what to do, no apologies, no jargon on the main screen (never "fragment", "DoH", "SNI", "proxy" there).

## 7. Motion (restraint)

Allowed, and nothing else:
1. Color and fill transitions between states: 280 ms ease-out.
2. The connecting arc rotation (a functional loading indicator).
3. **One** orchestrated moment: when the phase becomes `on`, a single ring ripple expands from the button over 600 ms and fades. No looping pulse, no entrance animations, no hover animations on rows.
4. View changes: 160 ms fade.

`prefers-reduced-motion: reduce` removes 2, 3 and 4; state changes become instant.

## 8. Accessibility

WCAG AA (verified numbers in section 2). Visible keyboard focus (3 px `--color-focus` ring, 3 px offset) on every control; logical tab order (power button first). `aria-live="polite"` on the state label, subtitle and notice line; `aria-busy` while connecting. Click targets at least 44 x 44 px. Esc closes menus and views. Never rely on color alone (section 4.1). Toasts are announced and last 3 s.

## 9. Frontend structure and backend adapters

```
frontend/
  index.html
  public/fonts/                 Inter woff2 subset + license
  src/
    main.ts  App.svelte
    styles/ tokens.css base.css
    components/ PowerButton.svelte Menu.svelte StatusBlock.svelte Accordion.svelte
                TestSteps.svelte Toast.svelte Icon.svelte Wordmark.svelte
    screens/ Connect.svelte FirstRun.svelte Settings.svelte Help.svelte About.svelte
    i18n/ en.json index.ts
    lib/ backend.ts adapters/{wails.ts, android.ts, mock.ts} format.ts
```

`lib/backend.ts` defines one TypeScript `Backend` interface that mirrors the API in file 01, section 10 (same method names, same JSON shapes) and picks an adapter at startup: `wails.ts` (Wails bindings and events), `mock.ts` (used in a normal browser; controlled by the URL, for example `?mock=off|connecting|on|degraded|error|firstrun`), and `android.ts` (a stub that throws "not built yet" for now; file 04 fills it in). **Components never call Wails directly.** Subscribe to events (`state`, `stats`, `test:progress`) through the interface.

## 10. Acceptance checklist

1. With the mock backend, take screenshots at 360 x 560 of: Connect in all six phases; Connect with a notice line; menu open; first-run; Settings (Advanced closed and open); Help (before test, mid-test, failed, passed); About. Also Connect `on` at 320 x 568 and at 480 x 900. Put them in `docs/screenshots/` and compare each with sections 2 and 4 before reporting; fix differences first.
2. The power button is the largest and most contrasting element in every phase; Off and On are distinguishable at arm's length and in grayscale.
3. Keyboard only: Tab reaches every control; Space toggles the button; focus ring always visible; Esc closes the menu.
4. Reduced-motion setting removes the ripple and spinner motion.
5. No network requests except to the local backend (check the dev-tools network panel); fonts and icons are local.
6. No hard-coded strings; no `left`/`right` positioning that would break if a language were added (use logical CSS properties).
7. Real Windows (Ali tests): On reaches `on` or `degraded` within 35 s; Off restores settings; closing hides to the tray with the one-time message; tray icon states match the window; no white flash at startup.

## 11. Rules for you, the agent

Complete files only, no placeholders. Verify every Wails option and Svelte API against the installed versions. Run `npm run build` and `wails build`, report sizes, and list anything you could not verify. If a requirement cannot be done with the installed Wails version, say so and propose the closest option instead of silently dropping it.
