import en from "./en.json";

// The copy deck (file 02, section 6): components never hard-code text.
// One language today; the file structure keeps future languages cheap.
export type Copy = typeof en;

export const copy: Copy = en as Copy;

/** Replace {placeholders} in a copy string. */
export function fill(template: string, vars: Record<string, string>): string {
  return template.replace(/\{(\w+)\}/g, (_, key) => vars[key] ?? "");
}

// D50: the copy deck is written for Windows. On the Linux edition the
// same deck is reused with the OS word swapped in place, so there is one
// source of copy (every change lands on every version). Called once by
// App.svelte after the backend answers and BEFORE the first render —
// mutating the imported object in place means every component sees it.
export function setOsWord(os: string): void {
  const walk = (node: unknown): void => {
    if (typeof node === "string") {
      return;
    }
    if (Array.isArray(node)) {
      for (let i = 0; i < node.length; i++) {
        if (typeof node[i] === "string") {
          (node as string[])[i] = node[i].replaceAll("Windows", os);
        } else {
          walk(node[i]);
        }
      }
      return;
    }
    if (node && typeof node === "object") {
      for (const key of Object.keys(node as Record<string, unknown>)) {
        const value = (node as Record<string, unknown>)[key];
        if (typeof value === "string") {
          (node as Record<string, string>)[key] = value.replaceAll("Windows", os);
        } else {
          walk(value);
        }
      }
    }
  };
  walk(copy);
}
