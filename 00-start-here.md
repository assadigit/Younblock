# 00 - Start Here (Younblock specs, v2)

**Audience:** the coding agent. Read this file first, then 01, 02 and 03. File 04 is for a later run.

## What is being built

**Younblock** is a small Windows app that helps YouTube load in Google Chrome on a filtered network. It runs a local relay: the browser sends traffic to it, it resolves names through secure DNS (DNS-over-HTTPS), and it sends the first TLS message to the site in small pieces so name-based filters do not recognise it. It is **not a VPN** and must never claim to be. An Android version (same engine, same UI) comes later.

It replaces an existing Python app (`legacy/backend.py`, previously branded SparoDNS, now Younblock) that works but is laggy as a 50-60 MB `.exe`, has a dated UI, and several reliability bugs.

## Owner and working style

Ali is a UI/UX designer and "vibe coder" who cannot read or patch code. He relies on you for complete, working, **verified** results. Write plain-English explanations for him; keep technical detail in the specs, `DECISIONS.md` and `PROGRESS.md`. Quality over speed: it is fine to take longer to be correct.

## Locked decisions (do not reopen without asking Ali)

| Topic | Decision |
|---|---|
| Audience | Ali and his friends; private use |
| Usage | Manual, ExpressVPN-style: open the app and press one big button. Opens with Windows (asked once), tray icon, closing hides to the tray |
| Control | Just the button; the app picks its method and repairs itself, and announces every automatic change |
| Browser | Chrome only (UI says "Works with Chrome") |
| Language | English only |
| Look | Dark charcoal base, red UNBLOCK logo kept for identity, calm neutral Off, clear green On, red only for logo and real errors (file 02) |
| Stack | Go engine; Windows app with Wails v2 and Svelte + TypeScript; later Android with the same engine and the same UI (file 04) |
| Repo | Private GitHub repo `Younblock` in Ali's account |

## File map

| File | Contents | When |
|---|---|---|
| `01-go-backend-rewrite-spec.md` | Engine, Windows adapters, tray, self-healing, UI-facing API, phases, acceptance criteria | now |
| `02-ui-design-spec.md` | Design tokens, screens, states, copy, tray icon design, frontend structure | now |
| `03-reliability-suggestions.md` | P0 bug fixes, P1 requirements, P2 later items, test targets | now (P0 and P1) |
| `04-android-spec.md` | Android version | later run, after desktop is accepted and Ali says go |

## Inputs the agent should look for in the repo (ask Ali if missing)

1. `legacy/backend.py` (required) and any other old app files, especially the part of the old app that made Chrome use the proxy. It tells you what the old app did that `backend.py` does not show.
2. `assets/logo.svg` or `assets/logo.png`: Ali's UNBLOCK logo. If missing, use the CSS wordmark in file 02 and tell Ali.
3. `docs/reference/`: screenshots of the old UI (optional reference only; the new design replaces them).

## Order of work

Follow the phases in file 01, section 11: (1) headless engine, (2) health and self-healing, (3) Windows adapters, (4) Wails app, tray, first run, UI from file 02, (5) remaining P1 items from file 03. After each phase: run the checks, commit, tag `phase-N`, update `PROGRESS.md`.

## Non-negotiables

- Complete files only: no `TODO`, no placeholders.
- Never invent packages, function names or flags; verify against installed versions.
- Never weaken a test to make it pass.
- Never claim the app unblocks YouTube on Ali's real network. Only Ali's real connection can prove that. Say what you proved and what you could not.
- Never write secrets into the repo. Never run the real Windows registry code on the development machine during development; use the fakes the spec describes. The real check is Ali's manual step.
- Nothing changes silently: every automatic change the app makes is shown to the user and logged.
- Answer with a short, plain-language report at the end of every phase.
