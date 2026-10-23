// pro-main.ts — the entry for the v3 main screen: the flex column, the
// lever button, the ring. The ONE screen's module, loaded by main.ts for
// every edition (the variant switch is retired — one UI everywhere).
//
// UI v3 (docs/ui-v3/UI-SPEC.md): the design tokens are IMPORTED from
// docs/ui-v3/younblock-tokens.css — the single source of truth; no value
// is copied by hand. The legacy tokens.css stays imported too: the menu
// sheet and the diagnostics view (both kept unchanged per spec §1) read
// their surfaces from it.
import "@fontsource-variable/sora";
import "../../docs/ui-v3/younblock-tokens.css";
import "./tokens.css";
import "./app.css";
import { mount } from "svelte";
import App from "./App.svelte";

const app = mount(App, {
  target: document.getElementById("app") as HTMLElement,
});

export default app;
