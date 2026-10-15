// main.ts — the variant dispatcher (D66): both screens live in the bundle,
// exactly one loads at startup.
//
//   * the baked default comes from VITE_UI_VARIANT at build time — the
//     "classic" desktop editions are built with it set, everything else
//     ships the redesigned screen;
//   * ?ui=classic or ?ui=pro on the URL overrides the default, which is
//     how the browser (single-executable) editions offer both.
async function boot(): Promise<void> {
  const fromUrl = new URLSearchParams(window.location.search).get("ui");
  const baked = import.meta.env.VITE_UI_VARIANT;
  const variant = (fromUrl ?? baked ?? "pro").toLowerCase();
  if (variant === "classic") {
    await import("./classic/main");
  } else {
    await import("./pro");
  }
}

void boot();
