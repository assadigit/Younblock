import en from "./en.json";

// The copy deck (file 02, section 6): components never hard-code text.
// One language today; the file structure keeps future languages cheap.
export type Copy = typeof en;

export const copy: Copy = en as Copy;

/** Replace {placeholders} in a copy string. */
export function fill(template: string, vars: Record<string, string>): string {
  return template.replace(/\{(\w+)\}/g, (_, key) => vars[key] ?? "");
}
