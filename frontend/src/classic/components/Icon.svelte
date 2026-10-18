<script lang="ts">
  // Inline Lucide icons (ISC license), stroke 1.75, round caps
  // (file 02, section 3). No icon font, no remote asset.
  interface Props {
    name: string;
    size?: number;
    strokeWidth?: number;
  }
  let { name, size = 20, strokeWidth = 1.75 }: Props = $props();

  const paths: Record<string, string> = {
    power: "M12 2v10 M18.4 6.6a9 9 0 1 1-12.77.04",
    menu: "M12 5a1 1 0 1 0 0.001 0 M12 12a1 1 0 1 0 0.001 0 M12 19a1 1 0 1 0 0.001 0",
    settings:
      "M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z M12 15a3 3 0 1 0 0.001 0",
    "life-buoy":
      "M12 22a10 10 0 1 0 0.001 0 M12 16a4 4 0 1 0 0.001 0 M4.93 4.93l4.24 4.24 M14.83 9.17l4.24-4.24 M14.83 14.83l4.24 4.24 M9.17 14.83l-4.24 4.24",
    info: "M12 22a10 10 0 1 0 0.001 0 M12 16v-4 M12 8h.01",
    "log-out": "M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4 M16 17l5-5-5-5 M21 12H9",
    "chevron-right": "m9 18 6-6-6-6",
    "chevron-left": "m15 18-6-6 6-6",
    check: "M20 6 9 17l-5-5",
    x: "M18 6 6 18 M6 6l12 12",
    copy: "M8 8h12a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2V10a2 2 0 0 1 2-2z M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2",
    "folder-open":
      "m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2",
    globe: "M12 22a10 10 0 1 0 0.001 0 M12 2a15 15 0 0 0 0 20 15 15 0 0 0 0-20 M2 12h20",
    // M8-p: the Diagnostics screen's glyphs (Lucide, same stroke).
    activity: "M22 12h-4l-3 9L9 3l-3 9H2",
    share: "M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8 M16 6l-4-4-4 4 M12 2v13",
    refresh: "M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8 M21 3v5h-5",
  };
  // Multi-segment paths split on " M" for correct SVG syntax.
  const segments = $derived(
    (paths[name] ?? "")
      .split(/ (?=M)/)
      .map((d) => d.trim())
      .filter(Boolean),
  );
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width={strokeWidth}
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
  focusable="false"
>
  {#each segments as d (d)}
    <path {d} />
  {/each}
</svg>
