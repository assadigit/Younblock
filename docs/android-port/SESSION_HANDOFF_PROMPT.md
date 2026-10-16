# Prompt — Wrap up the current session, then start from the Younblock Android specs

You are working on the Younblock repository:

<https://github.com/assadigit/Younblock>

The Android specification pack is in `docs/android-port/`. Treat these documents as the first mandatory reading, not as optional background:

- `docs/android-port/README.md`
- `docs/android-port/01-current-architecture-audit.md`
- `docs/android-port/02-requirements-and-feature-matrix.md`
- `docs/android-port/03-android-technical-feasibility.md`
- `docs/android-port/04-target-architecture.md`
- `docs/android-port/05-implementation-roadmap.md`
- `docs/android-port/06-build-and-test-plan.md`
- `docs/android-port/07-security-privacy-and-distribution.md`
- `docs/android-port/08-architecture-decisions.md`
- `docs/android-port/09-agent-handoff.md`

## Part A — Wrap up the current session safely

1. Stop expanding the current task and bring the working session to a safe checkpoint.
2. Inspect the current Git branch, latest commit, and `git status --short --branch`.
3. Preserve all pre-existing and uncommitted user changes. Do not discard, reset, or overwrite work merely to make the tree clean.
4. Review what has actually been completed in this session. Run relevant lightweight validation if appropriate, and record real results rather than assumptions.
5. If implementation or investigation notes exist only in the conversation, write a concise handoff into the appropriate spec or project note without overwriting unrelated content.
6. Do not claim the work is committed or pushed unless you have verified that operation succeeded.
7. Summarize the exact current state: branch/commit, changed files, tests run, unfinished work, and any blockers.

## Part B — Start the next work by reading the specs

Before making further source-code changes, read all ten documents listed above and then inspect the original repository files they identify. Do not start by generating a new plan, guessing the source architecture, or rewriting files from memory.

First determine which phase these documents represent:

- **If the documents are still the initial baseline** and contain `To verify`, unresolved architecture decisions, or an incomplete source audit, complete Phase 1 first. Inspect the actual code, verify Android routing and Go mobile feasibility, settle the architectural decisions, and update the documents. Do not implement the Android app or build an APK while those core decisions are unresolved.
- **If the documents contain a completed, evidence-backed audit and approved architecture**, proceed directly to Phase 2 implementation using `05-implementation-roadmap.md`. Start at the first incomplete milestone, validate assumptions against source, and avoid another broad discovery-only cycle.

## Guardrails

- Preserve existing Windows and Linux functionality.
- Do not assume Wails runs on Android or that Go cross-compilation proves mobile runtime compatibility.
- Do not assume Android permits ordinary apps to change the same global proxy settings as Windows/Linux.
- If `VpnService` is the selected solution, implement and test the actual packet/stream-to-relay path; a TUN interface alone is not enough.
- Keep “Connected” tied to genuine engine and connectivity readiness.
- Do not overstate VPN, privacy, encryption, IP-hiding, browser, or traffic coverage.
- Do not commit secrets, signing keys, credentials, or user-specific configuration.
- Do not make unrelated changes and do not force-push.
- If a required SDK, emulator, device, dependency, network access, GitHub permission, or signing key is missing, continue as far as practical and state the exact limitation.

## Required first response after reading

Report:

1. Whether the spec files are present and which of them are complete versus still provisional.
2. What the current repository state is, including existing uncommitted changes.
3. Whether you are continuing Phase 1 or Phase 2, with a short evidence-based explanation.
4. The exact next task you will execute.

Then carry out that task in the current session. Do not stop after merely repeating the instructions.
