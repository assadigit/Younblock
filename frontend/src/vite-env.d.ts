/// <reference types="svelte" />
/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Which screen this edition ships: "pro" (default) or "classic". */
  readonly VITE_UI_VARIANT?: string;
}
