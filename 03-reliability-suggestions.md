# 03 - Reliability Suggestions (v2)

**Audience:** the coding agent (and Ali). **P0** items are weaknesses in the current `backend.py` that the rewrite in `01-go-backend-rewrite-spec.md` must fix. **P1** items are required for a one-click app people can trust. **P2** items come after P0 and P1 are accepted. Each item says what is wrong or missing, what to do, and how to check it.

## 0. Plain-language summary (for Ali)

The old script works, but it leaves connections open, hides every error, tries only one address per website, and quietly lets other devices on your Wi-Fi use it. Part of the fix is the Go rewrite. The rest is a safety net: a built-in test, an automatic repair that tells you what it did, protection so a crash can never leave you without internet, a tray icon that shows the state, and logs that never record where you browse.

## 1. P0 - Fix these in the rewrite (bugs and risks in the Python)

| # | Problem in `backend.py` | Fix | How to check |
|---|---|---|---|
| R1 | Listens on all network interfaces (`''`), so anyone on the same Wi-Fi or LAN can use the PC as a proxy | Bind only to `127.0.0.1` | Unit test asserts the listener address; the port is unreachable from another device |
| R2 | Sockets are not closed on failure paths (failed DNS, failed connect, early return); handles and threads leak | One `closeBoth` helper and `defer` on every path; `recover` per connection | Soak test: 5,000 tunnels including failing ones; goroutine and handle counts return to baseline |
| R3 | `except: pass` hides every error | Typed errors with codes, logged once, mapped to plain UI messages | No empty error handling anywhere; each error code has a test |
| R4 | Uses only the first IP from DNS and never retries | Try every returned address in order; remember the last that worked | Test with a fake server whose first IP refuses connections |
| R5 | DNS cache never expires or shrinks; DoH lookups go through the app's own proxy port | TTL cache with clamp, size limit, stale-on-failure; DoH dials directly (through `netx` and the fragment wrapper) with `Proxy: nil` | Cache tests; DoH works while the proxy listener is stopped |
| R6 | 21-second socket timeout kills quiet but valid connections and makes failures slow | Idle timeout 300 s refreshed on activity; dial timeout 5 s per IP | Behavior tests for both |
| R7 | Traffic stats stored per remote IP in dictionaries that grow forever | Fixed atomic counters only | Memory flat in the soak test |
| R8 | One new thread per connection, no cap | Goroutines with a cap of 1024; the next connection gets `503` | The 1025th connection is refused cleanly |
| R9 | `time.sleep(0.001)` x up to 299 pieces; on Windows each sleep can last about 15 ms, so a page may wait seconds before the first byte | Fewer, larger pieces by default (`balanced`); `legacy` kept as an option; measure real sleep length at startup and log it | Planner tests; startup log shows measured sleep; first-byte delay for `balanced` under 100 ms on a normal PC |
| R10 | No `TCP_NODELAY`, so the OS may merge the small pieces and quietly defeat the trick | Go sets it by default; never disable it | Code review rule; `SetNoDelay(false)` appears nowhere |
| R11 | HTTP parsed by splitting the `repr()` of bytes; IPv6 targets and edge cases break | Real parsing with `http.ReadRequest` and `net.SplitHostPort`; fuzz test | `go test -fuzz` for 60 s, no panic |

## 2. P1 - Required for a trustworthy one-click app

**R12 - Built-in self-test.** Four checks (app listening, secure lookup, YouTube answers, video servers answer) with plain-language results. Also `younblock-core.exe --selftest`, so Ali can run it and paste the output when something fails. Check: tests with fake networks; each failing step gives a different, correct hint.

**R13 - Self-healing that announces itself.** When the quick check fails on Start, or the watchdog fails three times, the engine runs one automatic sweep over the methods (`balanced`, `strong`, `light`, `legacy`), keeps the first that passes, and reports it through the `notice` field ("Switched to a stronger method") and the log. Limits: one sweep per Start press and per watchdog failure, 10-minute cooldown between automatic sweeps, 35 s maximum, never touches YouTube more than the checks require. **Rule: nothing changes silently.** Every automatic change is visible in the UI line, the log and, if the window is hidden, a single notification. Check: fake networks that accept only one preset; assert the notice, the time cap and the cooldown.

**R14 - Watchdog and "Check needed".** While On, a light check every 120 s (one lookup plus one request). Three failures in a row trigger R13; if that does not fix it, the phase becomes `degraded` (amber) with a "Fix connection" button and one notification. One later success returns to `on`. Check: fake-network tests for every transition.

**R15 - Several secure-DNS providers.** Primary RethinkDNS, then Cloudflare, then Google (all editable). Move to the next after 3 s; remember the last good one. Optional: if the primary has not answered in 1.5 s, also ask the next and use whichever answers first. Reject answers in private or loopback ranges. Check: fake DoH servers that time out, return garbage, or return `127.0.0.1`.

**R16 - Windows proxy safety.** The biggest "lose internet" risk of any proxy app.
- Use a PAC script returning `PROXY 127.0.0.1:<port>; DIRECT`: if the app is not running, the browser goes direct.
- Back up the original values before changing them; restore on Off and on exit; restore only if the current value is still ours.
- Crash recovery at every start, plus `--restore-proxy` and the Help button "Restore internet settings".
- Test by killing the process while On: internet must still work, and the next start must clean up (manual check for Ali on Windows).

**R17 - Lifecycle hygiene.** Single instance (a second launch shows the first window). Port busy: pick another port automatically and report it. Graceful stop within 3 s. `recover()` in every connection goroutine. Check: tests for double start, double stop, stop with open tunnels.

**R18 - Tray and autostart safety.** Closing the window hides to the tray; **Quit** always stops and restores Windows settings; Quit and a system shutdown never leave the proxy setting behind (crash recovery from R16 covers the rest). The autostart entry is created only when the user agrees on first run or in Settings, is removed when turned off, is never duplicated, and is rewritten if the exe moved. A launch with `--minimized` stays hidden and in state `off` unless "Turn on automatically" is enabled. Check: unit tests for the Run-key logic with a fake registry; manual restart test for Ali.

**R19 - Network changes and sleep.** After the PC wakes or the network changes (poll interface addresses every 5 s while On), idle connections are dead and DNS may differ. Flush the DNS cache, reset the DoH client, and run the watchdog check immediately. Check: unit test with a fake interface list.

**R20 - Settings file robustness.** Versioned schema, validation and clamping (ports, sizes, timeouts), atomic write, corrupt file moved to `settings.json.bad` with defaults restored and a one-line notice in Help. Check: tests with truncated, wrong-type and future-version files.

**R21 - Privacy by default (important on filtered networks).** No telemetry, no crash upload, no background update checks. Logs never contain URLs, paths, headers or payloads; website names are written only if "Keep website names in the log" is on (default off), and the in-memory ring is cleared on Stop. The diagnostic report contains only: app version, Windows version, settings (no custom lists unless asked), self-test results, counters, the last 100 log lines (names removed) and the measured sleep time. The frontend loads nothing remote and ships a strict content security policy (`default-src 'self'`, no inline script, no `eval`). Check: automated scan of report and log samples for hostnames and IPs; the network panel shows only local traffic.

**R22 - Android-ready core (so file 04 stays cheap).** The core packages stay free of Windows, Wails, tray and toast imports; every outgoing socket goes through `netx.Dialer` with the optional control hook; the UI API is one JSON contract. Check: the platform-neutral script and the `GOOS=linux GOARCH=arm64` build from file 01, section 3, run in every phase and must pass.

## 3. P2 - After P0 and P1 are accepted

**R23 - Remember what worked.** Persist the last working method and DNS server so the next start does not rediscover them (the heal sweep then only runs when something breaks).

**R24 - Editable rules without a new build.** Import and export one JSON file with the DoH list, YouTube domain list and presets, validated like settings (R20). When filtering changes, Ali can fix his friends by sending a small file instead of a new `.exe`. No automatic downloading; a manual "check for updates" link that opens a web page is enough.

**R25 - Smarter splitting (`targeted` preset).** Cut the first piece in the middle of the server name. Often works with fewer pieces, so pages start faster. Fall back to `balanced` if parsing fails. Ship only after the self-test can show it helps on a real connection.

**R26 - Fragment only what needs it.** In "All websites" mode, optionally fragment only domains in the YouTube list and pass other sites through. Less delay for everything else (`fragmentScope = listed | all`).

**R27 - Release hygiene.** Build with `-trimpath -ldflags "-s -w"`, inject the version with `-X`, no UPX. Add Windows version information and an icon. Publish a SHA-256 next to each download. Code-sign the exe if Ali can afford it: unsigned proxy tools often get SmartScreen and antivirus warnings; otherwise explain in the README how to verify the checksum. Optional: a GitHub Actions workflow on `windows-latest` that runs the tests and builds on every change.

## 4. Test and monitoring plan (what "reliable" means, in numbers)

| Measure | Target |
|---|---|
| App launch to listening | 300 ms or less |
| Press the button to a final state (`on` or `degraded`) | 35 s or less; typical under 3 s when the first method works |
| Added delay to the first byte (`balanced`) | 100 ms or less on a normal PC |
| Soak test (5,000 tunnels, 10 minutes) | goroutines and handles back to baseline, memory flat |
| Races | `go test -race ./...` clean |
| Parser fuzzing | 60 s, no panic |
| Kill while On | internet keeps working; next start restores settings |
| Offline start | the app opens, shows a clear error, never hangs |
| Automatic changes | every one visible in the notice line and the log |

## 5. Honest limits (the short version goes in About)

- Whether the splitting works depends on the filtering system on the user's network, which can change at any time. The self-test, the automatic repair and the editable rules (R13, R24) are the answer; nothing in code can promise it.
- Desktop handles TCP only. With a proxy configured, Chrome should use TCP and TLS for proxied sites, which is what this app handles. If Chrome still bypasses the proxy for YouTube, add disabling QUIC to the Help page after testing it on a real network.
- Some networks also block secure-DNS providers, so several providers and a custom one are supported.
- It is not a VPN: no IP hiding, no extra encryption. The interface and About text must never suggest otherwise.

## 6. Rules for you, the agent

Implement P0 and P1 fully; propose P2 only after they are accepted. For every item, state in your report: done or not done, the check you ran and its real output, and anything you could not verify (especially anything needing a real Windows machine or the real filtering network). Do not weaken a check to make it pass. Complete files, no placeholders.
