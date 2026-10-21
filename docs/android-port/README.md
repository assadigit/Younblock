# Younblock Android Port — Specification Pack

**Repository:** <https://github.com/assadigit/Younblock>  
**Documentation location in the repository:** `docs/android-port/`  
**Status:** Phase 1 (audit, feasibility, architecture) **COMPLETE** — 2026-10-16. The architecture is accepted (doc 08) and Phase 2 (implementation, roadmap M3+) may begin. The pack originated as a planning baseline prepared without repository access; every document has since been completed with verified evidence from the repository at commit `36273fb` (v0.7.6 / app 1.6.6).

## Purpose

This document set defines how to investigate, design, implement, test, and package an Android edition of Younblock while protecting the existing Windows and Linux applications.

The pack is based on the project description supplied for this task. The repository source could not be independently inspected while preparing this initial pack, so repository-specific facts, exact package names, API signatures, dependencies, and supported behaviors must be verified by the coding agent. These documents intentionally distinguish known product context from recommendations and open questions. They must not be treated as proof that any implementation has already been validated.

## Intended deliverable

An installable Android APK that preserves Younblock's core purpose: help YouTube work on networks that apply certain forms of hostname-based filtering, using the existing networking logic where feasible. The Android edition must be honest about traffic coverage, DNS behavior, privacy, and limitations. It must not describe itself as a remote VPN service if it only processes traffic locally and does not tunnel it to a remote VPN gateway.

## Current known context

The provided project description says Younblock is a Windows/Linux Go application with a Svelte/TypeScript UI. It describes a local relay, configurable secure DNS, TLS ClientHello splitting, platform-specific proxy configuration, connection checks, automatic recovery, settings, and diagnostics. It also explicitly says the product is not a conventional VPN, does not hide the user's public IP, and adds no encryption of its own. **All of these starting assumptions were verified against the repository during Phase 1** (doc 01) and hold.

## Reading order

1. [`01-current-architecture-audit.md`](01-current-architecture-audit.md) — establish what exists today.
2. [`02-requirements-and-feature-matrix.md`](02-requirements-and-feature-matrix.md) — define the target behavior.
3. [`03-android-technical-feasibility.md`](03-android-technical-feasibility.md) — test the risky platform and networking assumptions.
4. [`04-target-architecture.md`](04-target-architecture.md) — settle the system design after the feasibility gate.
5. [`05-implementation-roadmap.md`](05-implementation-roadmap.md) — implement in validated milestones.
6. [`06-build-and-test-plan.md`](06-build-and-test-plan.md) — define repeatable builds and evidence.
7. [`07-security-privacy-and-distribution.md`](07-security-privacy-and-distribution.md) — handle trust, permissions, and publication.
8. [`08-architecture-decisions.md`](08-architecture-decisions.md) — record decisions and changes.
9. [`09-agent-handoff.md`](09-agent-handoff.md) — session handoff and first actions.
10. [`10-release-notes.md`](10-release-notes.md) — the v1 release-notes draft (M7; M8 finalizes against the artifact).
11. [`11-the-video-opening-war.md`](11-the-video-opening-war.md) — the M8 method and record: how the 1–2 minute video opens became ~1 second (written at the debug.28 field verdict, at the tester's request).

## Non-negotiable project rules

- Do not assume that Wails itself can run on Android. Separate portable Go functionality from desktop framework and operating-system integration.
- Do not assume Android exposes a general system-proxy control equivalent to Windows or Linux.
- Do not equate opening a `VpnService` TUN interface with successfully forwarding TCP, UDP, DNS, IPv4, and IPv6 traffic. The packet-processing architecture must be explicit and tested.
- Do not state that traffic is protected, encrypted, hidden, or routed unless the implementation proves that claim.
- Do not regress desktop builds. Android-specific code should be isolated with explicit package boundaries and appropriate build constraints.
- Do not invent source paths, functions, test results, SDK versions, device support, or successful build outcomes. Verify and cite actual evidence in the documents.
- Do not let a UI display “Connected” unless the relevant service has successfully initialized and a real connectivity check passes.
- Preserve the user's existing local changes and inspect Git status before modifying the repository.

## Required investigation

The source audit and feasibility work required of the coding agent are **done**: every document distinguishes **Verified** (with file/symbol/test evidence) from **Recommendation** / gated selection, and the major architectural decisions are recorded as ADRs in doc 08. If new evidence changes a recommendation, update this pack before continuing (decision-change procedure at the end of doc 08).

## Phases

### Phase 1 — Audit and finalize specifications — COMPLETE (2026-10-16)

The repository was audited with evidence (doc 01), options compared and scored (doc 03), the architecture settled and recorded (docs 04, 08), requirements reconciled and traced (doc 02), and the plan/toolchain tables grounded (docs 05, 06, 07). No application code was changed in this phase.

### Phase 2 — Implement and build — CURRENT

Read the completed specs and handoff (especially docs 08, 05, 09), implement the selected architecture in milestones M3–M8, run tests, attempt the real APK build, verify the produced artifact, and update documentation with actual results. If a critical assumption fails, stop pretending it works: document the blocker and use the documented fallback or revise the ADR with evidence.

## Official reference starting points

Verify the current versions and relevant requirements when implementing; these links are entry points rather than proof of compatibility:

- Android VPN: <https://developer.android.com/develop/connectivity/vpn>
- Android foreground services: <https://developer.android.com/develop/background-work/services/fgs>
- Android app architecture: <https://developer.android.com/topic/architecture>
- Android app signing: <https://developer.android.com/studio/publish/app-signing>
- Go mobile tooling: <https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile>
- Go build constraints: <https://pkg.go.dev/cmd/go#hdr-Build_constraints>
- Wails project and supported platforms: <https://github.com/wailsapp/wails>
