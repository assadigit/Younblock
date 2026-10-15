# Agent instructions: build the new Younblock connect screen

Usage: paste this whole file to your coding agent, together with the folder `younblock-ui/`. Stack: Go + Wails v2 + Svelte + TypeScript, Windows first.
Read `design.md` for the rules and `preview/` for the target look. Do not redesign; copy.

## 0. Ground rules
- Do NOT replace your existing Go proxy logic. Only wrap it (step 3).
- Keep the template's existing `main.ts` mount code (it already matches your Svelte version). Only add the three imports in step 2.
- The Svelte code uses classic syntax (`export let`, `$:`, `on:click`), tested on Svelte 4.2 and valid for Svelte 3 and 5 (non-runes).

## 1. Install the font (inside `frontend/`)
`npm install @fontsource-variable/sora`

## 2. Frontend files (copy, then wire)
Copy from `younblock-ui/frontend/src/` into the project's `frontend/src/`:
`App.svelte` (replace), `app.css`, `tokens.css`, `lib/` (all 7 files).
In the existing `main.ts`, add at the very top:
```ts
import '@fontsource-variable/sora'
import './tokens.css'
import './app.css'
```
Remove the template's `style.css` import and logo assets if they style `html`, `body` or `#app` (they would fight `app.css`).
In `frontend/index.html` make sure the viewport meta is `width=device-width, initial-scale=1, viewport-fit=cover`.

## 3. Go side
1. Copy `go/ui_bridge.go` (and `ui_bridge_test.go`) next to `main.go`. Run `go test -race ./...`: it must pass.
2. Write an `Engine` wrapper around the existing proxy (3 methods: `Start(ctx) error`, `Stop() error`, `Stats() (downBps, upBps float64, pingMs int)`). `Start` must return only when the proxy is really ready, and must return quickly when `ctx` is cancelled. `Stop` must be safe to call twice. Use `-1` for unknown ping.
3. Bind it in your `App` struct (adjust names to your code):
```go
type App struct {
	ctx    context.Context
	bridge *Bridge
}

func NewApp() *App {
	a := &App{}
	a.bridge = NewBridge(newYounblockEngine(), func(s Snapshot) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "younblock:status", s)
		}
	})
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go a.bridge.RunStats(ctx)
}
func (a *App) Connect() error        { return a.bridge.Connect() }
func (a *App) Disconnect() error     { return a.bridge.Disconnect() }
func (a *App) GetSnapshot() Snapshot { return a.bridge.GetSnapshot() }
func (a *App) shutdown(ctx context.Context) { _ = a.bridge.Disconnect() }
```
(`runtime` is `github.com/wailsapp/wails/v2/pkg/runtime`; `newYounblockEngine()` returns your Engine.)
4. In `main.go` options: `OnStartup: app.startup`, `OnShutdown: app.shutdown`, `Bind: []interface{}{app}`, `Width: 360`, `Height: 760`, `DisableResize: true`, `BackgroundColour: &options.RGBA{R: 0x3c, G: 0x3b, B: 0x4b, A: 255}`. Open devtools once and check `window.innerWidth/innerHeight`; raise `Height` until the inner height is 760 (the title bar takes some). If it is slightly off, the screen still scales correctly.
5. If your struct or method names differ, change only the `CONFIG` block at the top of `frontend/src/lib/backend.ts`.

## 4. Check it (acceptance list)
Run `npm run build` in `frontend/` (must have no errors), then `wails dev`:
1. Opens in the charcoal "Not Connected" look; red thumb at the top, red ring, "START".
2. Click the thumb → "Connecting…" with spinner → green screen, timer counts, speeds and ping update, thumb at the bottom showing "STOP" with a green LED.
3. Drag the thumb down past halfway to connect; a short drag snaps back. Drag up past halfway to disconnect.
4. Click while "Connecting…" cancels.
5. ArrowDown/ArrowUp and Enter work on the focused thumb.
6. Make `Start` fail on purpose: "Can't Connect" + "Could not connect. Try again." and the person can retry.
7. Menu button opens About / How it works / Help; Escape closes it. Edit the three texts in `MenuSheet.svelte` (CONFIG block).
8. Nothing flag, premium or location related appears anywhere.

## 5. Optional
- Your logo: put `logo.png` in `frontend/public/` and set `LOGO_URL = '/logo.png'` in the `App.svelte` CONFIG block.
- Frameless window: set `Frameless: true`; the header already has the drag region, but you must add your own close/minimize buttons first.
- Mobile later: the screen scales by rem, uses pointer events and safe-area insets, so it can be reused in a mobile web shell. Only `backend.ts` needs a new adapter.
