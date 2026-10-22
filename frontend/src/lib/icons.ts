// icons.ts — every icon the screen uses, through this one file (round-2
// fixes #4/#6/#7: real Lucide icons instead of hand-drawn paths; the logo
// and the faint play mark stay hand-drawn by design). Keeping them in one
// place means a package move (lucide-svelte → @lucide/svelte) or an icon
// swap touches exactly one file.
//
// Sizing: Lucide's `size` prop is pixels, but this screen scales with the
// root font (app.css), so every icon is sized in rem through CSS instead —
// the components render plain <svg> elements the sheet's styles can reach.
export {
  Power,
  Ellipsis,
  ChevronsUp,
  ChevronsDown,
  ArrowDown,
  ArrowUp,
  X,
  ChevronRight,
  ChevronLeft,
  Activity,
  Copy,
  Share2,
  RefreshCw,
  Check,
  // UI v3 (docs/ui-v3): the Open YouTube card's launch glyph (right side,
  // external-link only — no chevron) and the amber lines' warning icons
  // (never amber alone; the icon always sits beside the words).
  ExternalLink,
  AlertTriangle,
} from "lucide-svelte";
