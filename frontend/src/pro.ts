// pro-main.ts — the entry for the redesigned connect screen (v0.7.0+):
// the stage, the drag pill, the halftone rings. Kept as its own module so
// the dispatcher (main.ts) can load exactly one variant per edition (D66).
import "@fontsource-variable/sora";
import "./tokens.css";
import "./app.css";
import { mount } from "svelte";
import App from "./App.svelte";

const app = mount(App, {
  target: document.getElementById("app") as HTMLElement,
});

export default app;
