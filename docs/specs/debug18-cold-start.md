# Younblock debug.18 — Android cold-start fix: complete instruction and spec

Owner: Ali. Audience: the autonomous coding agent.
Save this file in the repo as `docs/specs/debug18-cold-start.md` and follow it end to end.

---

## 0. START PROMPT (paste this one block into the agent, nothing else is needed)

```
You are the engineer for the Younblock repo (Go engine, Android VPN build via gomobile, desktop via Wails).
Read docs/specs/debug18-cold-start.md completely, then execute EVERYTHING in it, in the order given in section 9, without asking me any questions.
Rules: if the spec conflicts with DECISIONS.md or an existing ADR, follow the repo decision, write a short ADR explaining the deviation, and continue. If something is ambiguous, pick the safest option, record it in the ADR, and continue. Never leave a manual step for me.
Deliver: code, tests (all green), ADRs, release notes, the built debug.18 APK (versionCode 17), and the final summary described in section 11.
```

---

## 1. Goal (plain words)

On the tester's Android phone, the first video after opening the app takes 1–2 minutes to start. After that, going next/back is instant, and the feed loads fast. Windows and Linux are fast. Make a cold video start take a few seconds at most, and make the next diagnostics report able to prove why.

Target numbers for the tester report after this build:

- Time from "I pressed play" to "First frame" (new in this build): **under 5 s** on a fresh start, ideally 1–3 s.
- DNS failure rate: **under 5 %** (was 26 %, 50 of 192 lookups).
- No flow of class=plain may sit silent for more than ~8 s before its first byte (they are closed and counted instead).

---

## 2. What is already known (evidence)

Source: tester diagnostics report #8 (build debug.16, versionCode 15, method=strong, mode=youtube) and a read of the repo code. Items are labeled **VERIFIED IN CODE** or **HYPOTHESIS**.

### 2.1 Architecture facts (VERIFIED IN CODE)

1. The phone's DNS queries go into the TUN and are answered instantly by `internal/tunstack/dns.go`: A queries get a fake address from 198.18.0.0/15 (TTL 30 s); AAAA and all other types get an empty NOERROR. The app therefore never waits for real DNS.
2. The TCP flow to the fake address is accepted at once by gVisor (`internal/tunstack/tunstack.go`, `handleTCP`), then `pumpFlow` opens a SOCKS5 connection to the door (`internal/tunstack/socks5.go`) with the DOMAIN (ATYP 0x03). The real resolution (DoH) and origin dial happen inside the door (`internal/proxy/socks5.go`: `s.resolve(host)`, then `DialHost`). So a DNS failure reaches the app as "connected, then closed", not as a DNS error.
3. DoH IS fragmented: `internal/doh/resolver.go` `dialBootstrap` wraps the connection in `fragment.NewConn`. (An earlier theory that it was not fragmented was wrong.)
4. `internal/doh/resolver.go`: servers are asked **serially** (`askServers`), 3 s each; `http.Transport` with `MaxIdleConnsPerHost: 2`, `ForceAttemptHTTP2: true`, no keep-alive pings. Concurrent queries that arrive before the first connection is ready each dial their own new fragmented TLS connection.
5. UDP other than DNS: `handleUDP` returns `false` and relies on gVisor to send ICMP port-unreachable. Whether it really does is not proven. IPv6 packets die silently at the NIC (ADR-0010).
6. **Only 7 domains are "routed" (fragmented, budgeted)** in youtube mode: `youtube.com`, `youtu.be`, `googlevideo.com`, `ytimg.com`, `ggpht.com`, `youtube-nocookie.com`, `youtubei.googleapis.com` (`internal/config/defaults.go`, `defaultYouTubeDomains`).
7. Every other host (class=plain) goes through `runBidirectionalCopy` → `copyPlain` in `internal/proxy/tunnel.go` with: no fragmentation, no first-byte budget, no stall kill, only the 300 s idle timeout. The decision is `routedHost()` in `internal/proxy/server.go`, used at `internal/proxy/socks5.go` (`fragmentTLS := s.routedHost(host)`) and gating the budgeted path at `tunnel.go` (`if fragmentTLS && s.cfg.FirstByteTimeout > 0`).
8. `.ir` hosts are never processed (D83, `internal/config/direct.go`). Keep that.

### 2.2 Measurements from report #8

| item | value |
|---|---|
| conns total / errors | 130 / 80 (≈61 %) |
| dns ok / fail | 142 / 50 (26 %); doorFailures = 50 (1:1) |
| udpDropped | 99 |
| good path | dns 0–230 ms warm, dial 45–150 ms, first byte 133–900 ms |
| slow DoH lookups | 1055–1806 ms (cold) vs ~130 ms (warm) |
| dead origins | ~17 first-byte timeouts (2.9–4.3 s cycles) + ~8 resets (~0.6 s) |
| dial times | two groups: 45–150 ms and 500–1000 ms |
| warm path | healthy (tiered stall watch fixed next/back) |

### 2.3 Main hypotheses (to be confirmed by the new diagnostics)

- **H1 (strongest):** the 1–2 minute delay is caused by plain-relay hosts (ads, attestation, Google APIs, etc.) that the YouTube app needs at first play, that are blocked by SNI inspection, and that hang silently because they are not fragmented and have no timeout. The app gives up after about 60 s.
- **H2:** DoH blips come from bursts of fresh names each opening a new fragmented TLS connection, plus serial 3 s server walking.
- **H3:** QUIC fallback may be slower than needed if ICMP port-unreachable is not actually sent.
- **H4:** the 500–1000 ms dial group is far or lossy cache nodes; resolver choice may decide which nodes are returned.

---

## 3. Hard constraints (do not break)

1. Do not change fragmentation presets, the auto-heal sweep order, or the `Plan` logic in `internal/fragment`.
2. Keep the `.ir` never-processed rule (D83) in every door.
3. Keep loop safety (ADR-0011): every outbound socket goes through `netx` and the `protect()` hook; the only socket the stack opens is loopback to the door.
4. Keep IPv6 policy (ADR-0010): never enabled in the stack.
5. Desktop (Windows/Linux) behavior must not change. New behavior is Android-only or behind a setting that defaults to the old behavior on desktop.
6. Privacy: hostnames stay out of logs unless `LogHostnames` is on. This build turns it on **for the debug tester build only** (section 7.1), with a visible notice in the report. Do not persist hostnames beyond section 6.4's rule.
7. Never fail open silently: any "never a refusal" rules (e.g. the dead-IP penalty) stay.
8. Existing tests must stay green; new behavior gets both-ways tests (prove it fails without the fix and passes with it), as in ADR-0019 to ADR-0023.

---

## 4. Work item A — Fragment every TLS flow of routed apps on Android (fixes H1)

**Files:** `internal/config/settings.go`, `internal/config/defaults.go`, `internal/proxy/server.go`, `internal/proxy/socks5.go`, `internal/proxy/tunnel.go`, `internal/engine/*` (wiring), `mobile/mobile.go` (Android default).

Requirements:

1. Add a boolean setting `FragmentAllRouted`. Default **true on Android**, **false on desktop**. Persist and migrate it like other settings (follow the existing settings version/migration pattern in `settings.go`).
2. Split the single decision into two:
   - `fragmentHost(host)`: true if not under a direct TLD AND (`FragmentAllRouted` OR the host matches the routed domain list, as today).
   - `budgetedHost(host)`: true only if not under a direct TLD AND the host matches the YouTube domain list (exactly today's `routedHost` result).
3. In `internal/proxy/socks5.go` pass both values down. In `internal/proxy/tunnel.go` `runBidirectionalCopy`:
   - fragment the first TLS record when `fragmentHost`;
   - run the budgeted shape (`copyFirstByteBudgeted`: first-byte budget, salvage, race, dead-IP penalty, tiered stall kill) **only** when `budgetedHost && FirstByteTimeout > 0`.
   - Plain-but-fragmented flows use `copyPlain` with the fragment wrapper (this path already exists for `isTLS && fragmentTLS`).
4. The HTTP door (`internal/proxy/server.go`) must use the same two functions so the logic stays shared.
5. Tests: (a) FragmentAllRouted on → a non-listed host's first record is fragmented, and it does NOT use the budgeted path; (b) off → old behavior byte for byte; (c) `.ir` host is never fragmented in either setting; (d) a listed host is budgeted in both settings.

## 5. Work item B — Plain-relay safety net (fixes H1 fallout)

**File:** `internal/proxy/tunnel.go` (`copyPlain` and a small helper).

1. For TLS flows that are NOT budgeted: if no byte arrives from the origin within **8 s** after the client's first record was sent, close both sides and count it (`plainFirstByteTimeouts` in `internal/stats`). A TLS server always answers a ClientHello quickly, so silence means dead.
2. Do not mark IPs in the dead-IP penalty box for these; do not retry or salvage them (keep it simple).
3. Non-TLS or flows that already got a first byte keep the existing 300 s idle rule.
4. Log one line per kill: `tunnel plain first-byte failed class=plain reason=timeout` (hostname only if `LogHostnames`).
5. Tests: a fake origin that accepts and stays silent is closed at ~8 s (use an injected clock/shortened constant in the test); a fake origin that answers in 200 ms is untouched.

## 6. Work item C — DoH: one warm shared connection, hedged servers, pre-resolve (fixes H2, helps H4)

**Files:** `internal/doh/resolver.go`, `internal/doh/cache.go`, `internal/config/defaults.go`, `internal/engine/engine.go` (start/stop wiring).

### 6.1 One shared HTTP/2 connection
- Replace the `http.Transport` with `golang.org/x/net/http2.Transport` using `DialTLSContext`. The dial function must: call the existing `dialBootstrap` (so the ClientHello stays fragmented), then run `tls.Client` with `ServerName` set to the server's URL host and ALPN `h2`, then return the TLS conn. Honor `cfg.TLS` for tests.
- `http2.Transport` dials once per address and shares the connection across concurrent requests; a burst of fresh names must produce **one** TLS handshake, not many.
- Set `ReadIdleTimeout` = 15 s and `PingTimeout` = 5 s so dead connections are noticed and replaced.
- Keep the request shape (`?dns=` GET, `type=A&ct=application/dns-message`, `accept` header), the cache, singleflight, serve-stale (6 h), negative cache (15 s) and the M8-y negative re-ask exactly as they are.

### 6.2 Warm at start and keep warm
- When the engine starts, open the DoH connection immediately (do not wait for the first query) and keep it alive via the ping above. If it drops, reopen it in the background within 1 s.
- Keep `PrewarmNames` behavior.

### 6.3 Hedged server walk (replace serial `askServers`)
- Ask the sticky/first server at t=0, the next at +300 ms, the third at +600 ms (constants in `config`: `DoHHedgeDelay = 300ms`). The first valid answer wins; cancel the others. A server that fails fast triggers the next one immediately.
- Keep the per-server timeout (3 s), the sticky server update, and the existing log lines `dns server failed server=#N reason=...` and `dns server failed; using the next one ...`.
- Add two extra server entries at the END of the default list: Cloudflare `https://1.1.1.1/dns-query?dns={dns}` and Google `https://8.8.8.8/dns-query?dns={dns}` dialed by IP (bootstrap = the IP). These send a ClientHello with no SNI (Go omits SNI for IP literals), so there is no name for inspection to match. Their certificates carry IP SANs; make sure verification works, and add a test.

### 6.4 Pre-resolve learned googlevideo nodes
- When a name `rr<N>---sn-<code>.googlevideo.com` or `r<N>---sn-<code>.googlevideo.com` resolves, remember the `sn-<code>` part only (cap 64 codes, in memory).
- At engine start (after the DoH connection is warm) and every 5 minutes, resolve `rr1`…`rr5` of each remembered code in the background, at most 3 lookups in flight, so the next fresh hostname is already cached.
- Persisting the codes to a small file in the app data directory is allowed only if it does not conflict with DECISIONS.md privacy rules (check D-entries and file 03 R21). If it conflicts, keep memory-only and note it in the ADR.
- Tests: bursts of 10 concurrent distinct names create exactly 1 dial (count dials in the test); hedge returns the second server's answer when the first stalls; prewarm only touches remembered codes.

## 7. Work item D — UDP/QUIC fast fallback (fixes H3)

**File:** `internal/tunstack/tunstack.go` (`handleUDP`), plus a host test.

1. First write a host test that sends a UDP datagram to a non-DNS port through the stack and records whether an ICMP port-unreachable comes back on the wire.
2. If gVisor already sends it for these spoofed/promiscuous flows, keep the code and keep the test as a guard. If it does not, add an explicit ICMPv4 destination-unreachable (type 3, code 3) reply for every dropped UDP datagram, built from the received packet header + first 8 bytes of payload, written to the wire.
3. IPv6: leave as is (policy), but add a counter `v6Dropped`.
4. Add a timeline log line when a UDP/443 drop happens: `udp443 dropped` (no names), so the report can be compared with the first TCP flow that follows it.

## 8. Work item E — Diagnostics (so the next report proves the cause)

**Files:** `internal/logx/*`, `internal/engine/health.go`, `internal/engine/jsonapi.go`, `internal/stats/stats.go`, `internal/proxy/*.go` (log lines), `mobile/mobile.go` (facade), the Android shell/UI source (find it in the repo: the diagnostics screen and report builder).

### 8.1 Hostnames on, clearly labeled
- In the **debug tester build only**, `LogHostnames` is on by default. The report must start with the line: `DEBUG BUILD: hostnames included for diagnosis`. A release build must keep hostnames off (test this).

### 8.2 Class tag on every tunnel line
- Every `tunnel up`, `tunnel first-byte`, `tunnel first-byte failed`, `tunnel done`, `tunnel stall killed`, and the new plain kill line carries `class=routed` (budgeted list host) or `class=plain` in the message text (the ring drops attributes, so it must be in the message).

### 8.3 New report sections
1. **Top 10 slowest or failed hosts**: per host (count, failures, worst time to first byte, class). Computed from a small in-memory table (cap 256 hosts, evict least recent).
2. **Plain-relay summary**: number of plain flows, number killed by the 8 s rule, worst first-byte time.
3. **DNS detail**: per DNS failure, the server number, the phase that failed (`tcp`, `tls`, `http`, `status`, `parse`), and latency. Also counters: DoH dials opened, DoH connection reuses, hedge wins by server number.
4. **Dial vs first-byte split** on every `tunnel up`/`first-byte` pair is already there; add `rtt-class=near` if dial < 200 ms else `far`, and a counter of each.
5. **Dead-origin probe (one-shot, at most once per report)**: for up to 2 IPs currently in the dead-IP penalty box, open a TCP connection and send a ClientHello with SNI `www.google.com`, then another with no SNI; log whether each gets a byte within 2 s: `probe dead-origin sni=alt result=answered|silent|reset`. This answers whether the block is by SNI or by IP. Run it only when the app is On, never on a timer.
6. **Timeline marks**: the Go facade exposes `Mark(label string)`, which writes a line `MARK <label>` with a timestamp. The Android diagnostics screen gets two buttons, **"I pressed play"** and **"First frame"**, that call it. The report prints the elapsed time between the last pair and the number of failed/slow flows inside that window.

### 8.4 Counters
- Add to the report header: `plainFirstByteTimeouts`, `v6Dropped`, `dohDials`, `dohReuse`, `hedgeWins`.

---

## 9. Order of work

1. Read the repo: `DECISIONS.md`, ADR-0008 to ADR-0023, the files named above. Run the full test suite once and record the baseline.
2. Work item A, then B (the likely cause of the minute delay).
3. Work item E (diagnostics) — so the build proves itself even if A–B are not the whole story.
4. Work item C (DoH).
5. Work item D (UDP).
6. Bump the Android version to **debug.18, versionCode 17**; update the version shown in the report header.
7. Write ADR-0024 (A+B), ADR-0025 (C), ADR-0026 (D), ADR-0027 (E). Each ADR: problem, evidence, decision, both-ways test proof, how the next report verifies it.
8. Run the full suite (all packages green, including a race-detector run on `internal/doh` and `internal/proxy`), build the Android APK, record its SHA-256.
9. Write release notes (section 10) and the final summary (section 11).

## 10. Release notes and tester checklist (put in the release notes file)

Release notes for debug.18 must contain, in plain language: what changed, what to look for, and this checklist for the tester:

1. Install debug.18 and open the app. Turn it **On**.
2. Fully close the YouTube app, then open it fresh.
3. Tap **"I pressed play"** in the Younblock diagnostics screen, then play a **new** video in YouTube.
4. When the first frame appears, tap **"First frame"**.
5. Play two more new videos (do not tap the buttons), then open **Diagnostics** and send the whole report.

## 11. Definition of done and the final summary you must output

Done means all of these are true:

- [ ] Work items A–E implemented; desktop behavior unchanged (a desktop test proves `FragmentAllRouted=false` gives the old results).
- [ ] New tests are both-ways and the full suite is green, including race-detector runs.
- [ ] Release build keeps hostnames off (test).
- [ ] ADR-0024 to ADR-0027 written.
- [ ] debug.18 APK built; SHA-256 and commit SHA recorded in the release notes.
- [ ] The report builder prints every section in 8.3 and 8.4 (a test renders a sample report).

Final summary to output (short, plain language): what changed per work item, any deviation from this spec with the reason, the APK path and hash, and what Ali should send back (the tester's diagnostics report).

---

## 12. How the next report will be read (so you log the right things)

- If the "Top 10 slowest or failed hosts" list shows class=plain hosts with long first-byte times or kills, H1 is confirmed and the fix in A–B should have removed them in this build.
- If DoH failures remain, the phase column says whether it is `tcp`, `tls` (path interference) or `status`/`parse` (server problem).
- If the dead-origin probe answers on `sni=alt` but not on the real name, the block is by SNI and fragmentation tuning matters; if both are silent, the IP is blackholed and the penalty box is right.
- The elapsed time between "I pressed play" and "First frame" is the single number that says whether the work succeeded.

## 13. Reference answers to the six consult questions (context for decisions)

- **Q1 DoH over direct:** keep DoH, already fragmented; fix it with one shared warm connection, hedged servers, and no-SNI IP endpoints (section 6). A plaintext control query is not possible on Android under D74 (no ACCESS_NETWORK_STATE), so skip it.
- **Q2 fresh hostnames:** the `sn-<code>` part is stable per location, so remember codes and pre-resolve `rr1…rr5` (section 6.4). No published known-good strategy was found; this is inferred from how the names work.
- **Q3 dead origins:** use the SNI probe in 8.3 item 5 before trusting the penalty box long term. The current 60 s box with "never refuse" is safe.
- **Q4 QUIC:** make the ICMP reply explicit and measurable (section 7). Do not build a QUIC relay.
- **Q5 race width:** already staggered at 750 ms in `internal/netx/dialer.go`; keep it and do not widen it before the new report arrives.
- **Q6 instrumentation:** sections 8.1–8.4, with the host table and the timeline marks as the top priority.
- **Realistic target:** DNS ~130 ms warm + dial 45–150 ms + first byte 130–600 ms means a 0.3–0.7 s cold start is achievable on a good node. The minute-scale delay is not a network floor.
