# Younblock debug.24 — media-body starvation: measure first, then choose the fix

Owner: Ali. Audience: the autonomous coding agent.
Save as `docs/specs/debug24-media-starvation.md` in the repo and follow it end to end.
Input document: `ENGINEERING-CONSULTATION.md` (field report of debug.23, versionCode 22). Read it first.

---

## 0. START PROMPT (paste this one block into the agent)

```
You are the engineer for the Younblock repo (Go engine, Android VPN build via gomobile).
Read ENGINEERING-CONSULTATION.md and docs/specs/debug24-media-starvation.md completely, then execute EVERYTHING in the spec in the order of section 8, without asking me any questions.
Rules: if the spec conflicts with DECISIONS.md or an existing ADR, follow the repo decision, write a short ADR explaining the deviation, and continue. If something is ambiguous, pick the safest option, record it in the ADR, and continue. Never leave a manual step for me, except the optional file in work item W8.
Deliver: code, tests (all green), ADRs, release notes with the tester protocol, the built debug.24 APK (versionCode 23), and the final summary from section 10.
```

---

## 1. The problem in plain words

On the tester's phone the feed, API and thumbnails work, but video media (`*.googlevideo.com`) never transfers. Every media connection finishes the TCP dial and the TLS handshake, the app sends its request, and then nothing comes back. Over six rounds we fixed many other things and the failure did not change. We do not yet know WHERE the data is lost. This build's job is to find out, with evidence, and to make the system stop trusting "handshake succeeded" as proof that an address can carry video.

## 2. What the field report already tells us

1. **The identical 2.8 KB is the TLS handshake, not an answer.** The server's TLS 1.3 reply (ServerHello plus the certificate chain) is about 2.5–3 KB. Three different origins all delivering exactly 2.8 KB means each sent only its handshake, and the video request got no reply. The same flows show `up=3.1 KB`, which is the 1.8 KB ClientHello plus roughly 1.3 KB of Finished and request. So the request was sent by the engine. Whether the server ever received it is unknown. (HYPOTHESIS, strongly suggested by the sizes.)
2. **Two things changed together between the last good media report and now.** Report #8 (debug.16, 12:53 UTC = afternoon in Tehran) moved 14.6 MB with method `strong`. Since debug.22 the `auto` method resolves to `legacy` (300 fragments at 1 ms), and the failing report is from the evening (22:17 Tehran). The preset and the hour were never separated.
3. **Hello COUNT was measured, hello PACKET count was not.** Each `legacy` hello is 300 TCP packets. 81 hellos is about 24,000 tiny packets in 85 s. A censor that tracks packets per source could be reacting to that, not to the number of hellos.
4. **Three kinds of addresses exist:** blackholed (silent to any SNI), handshake-alive but body-dead (the main problem), and fine (the API and thumbnail fronts). Nothing in the engine can tell the second kind from a good one.
5. **The network, ISP and phone model are unrecorded.** Mobile networks and home lines filter differently.

## 3. Hard constraints

1. Do not change the default fragmentation presets or their definitions in `internal/fragment`. Experiments only switch which one is active.
2. Keep the `.ir` never-processed rule (D83), loop safety (ADR-0011, every outbound socket through `netx`/`protect()`), the IPv6 policy (ADR-0010), and desktop behavior unchanged.
3. Hostnames in logs only in the debug tester build, with the report line `DEBUG BUILD: hostnames included for diagnosis`. Release builds keep them off (test).
4. Both-ways tests for every behavior change (prove the test fails without the fix). Existing tests stay green; run race-detector tests on `internal/proxy`.
5. No new Android permissions. Use only what the app already has.

## 4. Work items

### W1 — TCP-level evidence per media flow (the decisive measurement)
Files: `internal/proxy/tunnel.go`, a new `internal/netx/tcpinfo_linux.go` (+ no-op for other platforms), `internal/stats`.

- For every budgeted googlevideo flow, read the kernel's TCP info for the origin socket (`golang.org/x/sys/unix` `GetsockoptTCPInfo` via `syscall.RawConn.Control`) at two moments: (A) when the first client bytes after the ClientHello are forwarded to the origin (the request), and (B) when the flow ends (done, stall kill, error).
- Log both snapshots on the `tunnel done` / `tunnel stall killed` line for googlevideo flows, in the message text: `tcp A[acked=… sent=… retrans=… unacked=… rtt=…ms mss=…] B[acked=… rcvd=… retrans=… unacked=… rtt=…ms lastRecv=…ms lastAck=…ms]`.
- Meaning to document in the ADR: if `acked` at B did not move past A and retransmits and unacked grew, the request never reached the server (the censor drops upstream data after the handshake). If `acked` moved but `rcvd` did not, the request arrived and the answer is being dropped or throttled (downstream). If both moved and the flow still stalled, the server or HTTP layer is the issue.
- Aggregate into the report: `tcp verdicts: upstream-lost=N downstream-lost=N both-ok=N unknown=N` (classify each stalled googlevideo flow with the rule above).
- If reading TCP info fails on the device, log `tcpinfo=unavailable reason=…` once and continue.

### W2 — TLS record timeline (first records only, no decryption)
- For googlevideo flows keep the first 12 server→client records and first 8 client→server records as `type:length@ms` (parse the 5-byte record headers of the relayed bytes only). Print it in the done line: `rec s2c=[22:122@600 20:1@600 23:2650@640 …] c2s=[…]`.
- Add `first-app-byte` : time from the client's request to the first server record AFTER the handshake flight. Handshake flight = the first server records until the cumulative server bytes pass the first 23-type records that end the flight; use the simple rule “the first server record that arrives after the client's first post-handshake record”.

### W3 — Winner class on every done line
- `win=orig|subst|race` on `tunnel done` lines for googlevideo flows (which candidate carried the flow).

### W4 — Experiment switches (so the tester can A/B in one session)
Add a diagnostics sub-screen with toggles and a button, all printed at the top of the report as `experiment: method=… substitution=… breaker=…`:
- **Method**: legacy / strong / balanced / light (this only selects the existing presets).
- **GoodIPSubstitution**: on / off (the knob exists).
- **Emission breaker**: on / off.
- Button **“Run network test”** (see W7).

### W5 — Breaker sensor fix (design defect A4)
- Googlevideo flows killed by the stall watchdog with `cause=pending` count toward the emission breaker the same way first-byte terminals do. Test: 3 pending kills within 30 s arm the breaker (`cooldowns>0`) and the door stays quiet for the cooldown.

### W6 — Substitution scope
- Default `GoodIPSubstitution` to **off for googlevideo** until an address has media-level evidence (W7 “answered”). Keep it on for non-googlevideo fronts. The pool for googlevideo may only be filled by W7 results, never by first-byte wins.

### W7 — Media-liveness probe and “Run network test”
Files: `internal/engine/engine.go`, `internal/proxy/server.go`, new `internal/probe/media.go`.

- Probe = open TCP to a candidate googlevideo IP, TLS handshake with SNI equal to a REAL learned name (`rrN---sn-<code>.googlevideo.com`, ALPN `http/1.1`), through the same fragmenter and the currently selected method; send `GET /videoplayback HTTP/1.1` with `Host`, `Connection: close`; wait up to 3 s for any HTTP status line. A bad request still gets a small answer (400/404) from a healthy server.
- Result classes: `answered` (HTTP status line seen), `silent-after-handshake`, `hello-silent`, `reset`, `error`.
- “Run network test” does, in one go, for up to 6 current googlevideo IPs:
  1. the probe under **each** of the four methods in turn (same minute, same IPs) → a table `ip × method → result`;
  2. the same probe with SNI `www.google.com`, and with no SNI;
  3. a control probe to a known-good front (`i.ytimg.com`, `GET /`) so we see the control class works;
  4. a packet counter per method: `hello-packets-emitted` for each probe.
- Print the whole table in the report under `network test:`. Only `answered` may certify an address as media-capable.
- The probe runs only when the tester presses the button or when the report is built once; never on a timer.

### W8 — Optional foreign-node experiment (Ali supplies a file)
- Add a loader for `internal/config/experiment_nodes.txt` (one googlevideo hostname per line, resolved on-device through the normal DoH path). Leave the file empty with a comment explaining it. If it has lines, “Run network test” also probes those addresses with the real SNI shown in the file line, and the table shows `foreign-cluster` results. Ali will fill the file with hostnames collected abroad; skip silently if it is empty.

### W9 — Packet accounting and network identity in the report
- Counters: `helloFragPackets` (total TCP segments the fragmenter wrote), `helloFragPackets per method` in the network test.
- Header lines: `device: <Build.MANUFACTURER> <Build.MODEL> android <SDK_INT>` and `net: type=wifi|cellular operator=<getNetworkOperatorName> mccmnc=<getNetworkOperator>` (no permission needed; if empty print `unknown`).

## 5. Order of work

1. Read the consultation file, `DECISIONS.md`, the ADRs for debug.18–23, and the code for flows, fragmentation, substitution and the breaker. Run the full test suite for a baseline.
2. W1, W2, W3, W9 (evidence).
3. W4 (switches), then W7 (probe and network test), then W8.
4. W5, W6.
5. Bump to **debug.24, versionCode 23**; update the version shown in the report.
6. ADR-0043 (evidence: W1–W3, W9), ADR-0044 (switches and probe: W4, W7, W8), ADR-0045 (breaker and substitution: W5, W6). Each: problem, evidence, decision, both-ways test proof, how the next report verifies it.
7. Full test suite green (including race detector on `internal/proxy` and `internal/probe`), build the APK, record SHA-256 and the commit.

## 6. Tester protocol (put in the release notes, plain language)

Do this in ONE sitting, with the phone in the same place:

1. Open Younblock → Diagnostics, note nothing yet. Press **Run network test** and wait for it to finish (about 30–60 s).
2. In the YouTube app open a video and let it try for 2 minutes. Press **I pressed play** before you start and **First frame** if it ever plays.
3. Switch the method to **strong**, press **Run network test** again, and play another video for 2 minutes.
4. Send the full report.
5. Repeat steps 1–4 once in the morning (before 9:00 Tehran time) and once in the evening, and say each time whether you used mobile data or Wi-Fi.
6. If a laptop with `backend.py` is available on the same network, play any video through it at the same time and say whether it plays fully.

## 7. How to read the next report (decision table; this decides the real fix)

| What the report shows | What it means | What to build next |
|---|---|---|
| Network test: `answered` for `strong` but not for `legacy` (or the reverse) | The preset or its 300 packets per hello triggers the censor | Make the working preset the default and stop `auto` choosing `legacy` |
| `tcp verdicts` mostly `upstream-lost` | The request packets are dropped after the handshake | Pick addresses by W7 evidence; test other clusters (W8); fragmentation tuning will not fix this |
| `tcp verdicts` mostly `downstream-lost`, handshake and probes fine | Response bytes are throttled or dropped | Same address selection work; time-of-day comparison decides whether it is load-based |
| Probe with SNI `www.google.com` or no SNI is `answered` on addresses where the real SNI is `silent-after-handshake` | Block is keyed on the media SNI | SNI-side work on the first records and a per-class strategy |
| Probe is `answered` on every address under every method but real flows still starve | The real app traffic differs from the probe (client fingerprint, HTTP/2, request size) | Compare record timelines (W2) and the real app request shape; probe with an HTTP/2 request next |
| All current-cluster addresses are body-dead and foreign nodes (W8) are `answered` | The cluster the geo-DNS gives is harmed | Build cross-cluster dialing using only media-verified addresses |
| Everything googlevideo is body-dead in the evening under every method and every cluster | No client-only fix can reach media from this network | Decision for Ali: accept evening limits, or add a small relay outside the network for media only |
| Control probe to `i.ytimg.com` fails | The test itself is broken; do not trust the table | Fix the test before drawing any conclusion |

## 8. Definition of done

- [ ] W1–W9 implemented; desktop behavior unchanged (a desktop test proves it).
- [ ] New tests are both-ways; full suite green; race-detector runs green.
- [ ] A sample-report test renders every new report section (`tcp verdicts`, `rec`, `network test`, `experiment`, `device`, `net`).
- [ ] Release build keeps hostnames off (test).
- [ ] ADR-0043 to ADR-0045 written; debug.24 APK built with SHA-256 and commit in the release notes.

## 9. Not in this build (decided on purpose)

- No QUIC passthrough yet: it is a large change and only worth doing if the table in section 7 points at it.
- No new fragmentation presets and no changes to hello volume discipline.
- No relay or server. That decision belongs to Ali after the next report.

## 10. Final summary the agent must output (short, plain language)

What changed per work item, any deviation from this spec and why, the APK path and hash, and the exact instruction Ali sends to the tester (section 6).
