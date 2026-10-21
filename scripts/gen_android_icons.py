#!/usr/bin/env python3
"""Generate Younblock's Android launcher icon from the owner's mark
(M8-am, ADR-0051, D104).

The source is assets/icon.ico — the 32x32 RGBA mark the owner uploaded
to the repo root as icon.ico (a padlock with the play button in its
body, a gold lightning bolt breaking through with a light outline, and
two impact dashes; byte-identical files, asserted below). 32px is far
too small to upscale for a launcher, so this script re-derives the mark
as VECTOR geometry and emits the density set from it:

  * res/drawable/ic_launcher_foreground.xml — the traced mark as one
    VectorDrawable, five flat-color paths, scaled into the adaptive
    safe zone (52 of the 108dp canvas, centered).
  * res/mipmap-anydpi-v26/ic_launcher.xml — the adaptive icon: the
    brand dark as the background layer, the mark as the foreground,
    and the same foreground as the Android 13+ monochrome layer.
  * res/mipmap-{m, h, x, xx, xxx}dpi/ic_launcher.png — the legacy
    raster set for pre-adaptive contexts: the mark on the same dark
    rounded square the Windows app icon uses (gen_icons.py's tile,
    radius 22%, mark at 62% of the tile).
  * res/values/colors.xml — the launcher_background entry (idempotent
    append).

How the vector is derived (fidelity first — the owner's art, not a
redraw):

  1. Upscale the 32px source 16x with LANCZOS (smooth region edges for
     the tracer; no new colors are invented — see 3).
  2. Snap every pixel to the mark's five flat colors (measured from the
     source's opaque interiors): the padlock red, the bolt gold, the
     shackle charcoal, the play white, and the bolt's light outline
     cream. Blend pixels at region edges (anti-aliasing) are resolved
     by pair-mix snapping: a pixel is first explained as a mix of two
     palette colors and snapped to the nearer ENDPOINT, so a red/white
     blend never lands on the gold (the naive nearest-color trap).
     Alpha < 128 is transparent.
  3. Per color: connected components; specks under 400px (at 512) die.
     The white layer keeps only the play triangle — the small white
     sliver where the bolt crosses the shackle folds into the cream
     outline (one continuous light stroke, which is what the mark
     reads as at a glance).
  4. Each color layer is traced to spline paths on its own binary mask
     (vtracer), then all five layers assemble into one SVG with the
     exact palette and the source's z-order (shackle under body, body
     under outline, outline under bolt, triangle on top).
  5. The same traced geometry renders the legacy raster tiles.

Every run prints a per-layer IoU against the quantized source (the
geometry proof) and refuses to write anything below 0.90.

Dependencies (pip): pillow, numpy, scipy, vtracer, cairosvg. The
generated files are committed, so rerunning is only needed when the
source mark changes.
"""
import io
import os
import re
import sys

import numpy as np
from PIL import Image
from scipy import ndimage

try:
    import vtracer
    import cairosvg
except ImportError:
    print("pip install vtracer cairosvg first (see the docstring)", file=sys.stderr)
    sys.exit(1)

# The tracer is pinned: 0.6.12 produced the committed, VLM-verified
# geometry; a newer vtracer changed its spline fitting mid-session
# (0.6.15 exploded the same inputs into off-canvas overshoots), so a
# version mismatch is loud instead of silently different icons.
try:
    import importlib.metadata
    _vt = importlib.metadata.version("vtracer")
    if _vt != "0.6.12":
        print(f"note: vtracer {_vt} (the committed geometry was traced with "
              "0.6.12); the per-layer IoU gate below still verifies fidelity")
except importlib.metadata.PackageNotFoundError:
    pass

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC_ICO = os.path.join(ROOT, "assets", "icon.ico")
ROOT_ICO = os.path.join(ROOT, "icon.ico")
RES = os.path.join(ROOT, "android", "app", "src", "main", "res")

# The mark's five flat colors, measured from the 32px source's opaque
# region interiors (the owner's art, not a redesign).
RED = (206, 32, 31)      # the padlock body #CE201F
GOLD = (250, 195, 50)    # the lightning bolt #FAC332
DARK = (64, 68, 71)      # the shackle + dashes #404447
WHITE = (245, 242, 240)  # the play triangle #F5F2F0
CREAM = (250, 222, 145)  # the bolt's light outline #FADE91
PALETTE = [RED, GOLD, DARK, WHITE, CREAM]

# The brand dark behind the mark (gen_icons.py's app-icon tile, the
# same value the Windows app icon uses).
TILE = (30, 31, 33)  # #1E1F21

# The adaptive canvas: 108dp viewport, the mark fitted into 52 units
# centered — inside the 66dp safe circle's square, and the mark's own
# corners are rounded enough that every standard mask (the 72dp circle
# worst case) leaves them whole.
CANVAS = 108
MARK = 52

UPSCALE = 512  # the trace space (16x the 32px source)
SPECKLE = 400  # px at 512 (1.5px at 32) below which a component dies
MIN_IOU = 0.90


def quantize():
    """Upscale and snap to the five flat colors. Returns (array, masks)."""
    src = Image.open(SRC_ICO).convert("RGBA")
    if src.size != (32, 32):
        sys.exit(f"source mark changed shape: {src.size}, expected 32x32")
    big = src.resize((UPSCALE, UPSCALE), Image.LANCZOS)
    arr = np.array(big).astype(int)

    alpha = arr[:, :, 3] >= 128
    rgb = arr[:, :, :3]

    # Direct palette hits first.
    direct = np.zeros(rgb.shape[:2], dtype=int) - 1
    best = np.full(rgb.shape[:2], 1 << 30)
    for idx, c in enumerate(PALETTE):
        d = ((rgb - np.array(c)) ** 2).sum(axis=2)
        hit = d < best
        best[hit] = d[hit]
        direct[hit] = idx
    snapped = np.where((best < 900) & alpha, direct, -1)

    # Blend pixels: explained as a mix of two palette colors, snapped
    # to the nearer endpoint (never a third color).
    pair_best_res = np.full(rgb.shape[:2], 1 << 30)
    pair_best_idx = np.full(rgb.shape[:2], -1)
    for i in range(len(PALETTE)):
        for j in range(i + 1, len(PALETTE)):
            a, b = np.array(PALETTE[i]), np.array(PALETTE[j])
            ab = a - b
            denom = float((ab ** 2).sum())
            # t = clamp(<rgb-b, ab> / |ab|^2)
            t = ((rgb - b) * ab).sum(axis=2) / denom
            t = np.clip(t, 0.0, 1.0)
            mix = b[None, None, :] + t[:, :, None] * ab[None, None, :]
            res = ((rgb - mix) ** 2).sum(axis=2)
            better = res < pair_best_res
            pair_best_res[better] = res[better]
            pair_best_idx[better] = np.where(t[better] >= 0.5, i, j)
    blend = (pair_best_res < 2500) & (snapped == -1) & alpha
    snapped[blend] = pair_best_idx[blend]

    masks = {name: snapped == idx for idx, name in
             enumerate(["red", "gold", "dark", "white", "cream"])}
    return masks


def clean_layers(masks):
    """Drop specks; keep only the triangle in white, fold the rest
    (the bolt-over-shackle outline sliver) into cream."""
    out = {}
    for name, m in masks.items():
        lab, n = ndimage.label(m)
        if n == 0:
            out[name] = m
            continue
        sizes = ndimage.sum(m, lab, range(1, n + 1))
        keep = np.zeros_like(m)
        for i, s in enumerate(sizes):
            if s >= SPECKLE:
                keep |= lab == (i + 1)
        out[name] = keep

    # White: keep the largest component only (the play triangle); the
    # smaller white pieces are the outline's sliver at the shackle
    # crossing — they belong to the cream stroke.
    lab, n = ndimage.label(out["white"])
    if n > 1:
        sizes = ndimage.sum(out["white"], lab, range(1, n + 1))
        main = int(np.argmax(sizes)) + 1
        sliver = out["white"] & (lab != main)
        out["white"] = lab == main
        out["cream"] = out["cream"] | sliver
    return out


def trace_layer(mask, speckle):
    """Binary-trace one layer's black-on-white mask; returns the SVG
    path d-strings with vtracer's translate baked into the coordinates
    (vtracer traces in a local frame and positions the path with
    transform="translate(x,y)" — dropping that attribute, as a naive
    d-only reassembly does, lands every shape near the origin)."""
    img = np.full((UPSCALE, UPSCALE, 3), 255, dtype=np.uint8)
    img[mask] = (0, 0, 0)
    tmp_png = os.path.join(os.path.dirname(os.path.abspath(__file__)), ".tmp-mask.png")
    tmp_svg = tmp_png.replace(".png", ".svg")
    Image.fromarray(img).save(tmp_png)
    vtracer.convert_image_to_svg_py(
        tmp_png, tmp_svg,
        colormode="binary", hierarchical="none", mode="spline",
        filter_speckle=speckle, color_precision=8, layer_difference=0,
        corner_threshold=60, length_threshold=3.0, splice_threshold=45,
        path_precision=1,
    )
    svg = open(tmp_svg).read()
    os.remove(tmp_png)
    os.remove(tmp_svg)
    out = []
    for tag in re.findall(r"<path[^>]*/?>", svg):
        dm = re.search(r'd="([^"]+)"', tag)
        if not dm:
            continue
        d = dm.group(1)
        tm = re.search(r'transform="translate\(([-\d.eE]+)[, ]+([-\d.eE]+)\)"', tag)
        if tm:
            d = bake(d, 1.0, float(tm.group(1)), 1.0, float(tm.group(2)))
        else:
            tm2 = re.search(r'transform="([^"]+)"', tag)
            if tm2:
                sys.exit(f"vtracer emitted an unsupported transform: {tm2.group(1)}")
        out.append(d)
    return out


def layer_iou(d_list, mask):
    svg = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">'
           + "".join(f'<path fill="#000" d="{d}"/>' for d in d_list) + "</svg>")
    png = io.BytesIO()
    cairosvg.svg2png(bytestring=svg.encode(), write_to=png,
                     output_width=UPSCALE, output_height=UPSCALE,
                     background_color="white")
    r = np.array(Image.open(io.BytesIO(png.getvalue())).convert("RGB"))
    traced = r[:, :, 0] < 128
    return (traced & mask).sum() / max(1, (traced | mask).sum())


def main():
    # The two icon.ico files must stay the same bytes — the owner's
    # upload is the source of truth this script traces.
    if os.path.exists(ROOT_ICO):
        a = open(SRC_ICO, "rb").read()
        b = open(ROOT_ICO, "rb").read()
        if a != b:
            sys.exit("assets/icon.ico and the repo-root icon.ico differ; "
                     "which one is the mark?")

    masks = clean_layers(quantize())

    # Trace + verify each layer. The speckle filter is 20 everywhere:
    # 100 ate the mark's two impact dashes (836px and 420px at 512 —
    # small marks vtracer's own units treat as specks), and 20 keeps
    # every designed element while still killing true AA crumbs.
    speckles = {"red": 20, "gold": 20, "dark": 20, "white": 20, "cream": 20}
    layers = {}
    for name in ["dark", "red", "cream", "gold", "white"]:  # z-order
        d = trace_layer(masks[name], speckles[name])
        if not d:
            sys.exit(f"layer {name} traced to nothing")
        iou = layer_iou(d, masks[name])
        print(f"  {name:5s}: {len(d)} path(s), IoU {iou:.4f}")
        if iou < MIN_IOU:
            sys.exit(f"layer {name} IoU {iou:.4f} < {MIN_IOU}: the trace "
                     "does not match the mark; nothing written")
        layers[name] = d

    # ---- the mark's opaque bbox (fit THAT into the safe zone) ----
    union = np.zeros((UPSCALE, UPSCALE), dtype=bool)
    for m in masks.values():
        union |= m
    ys, xs = np.where(union)
    x0, x1, y0, y1 = xs.min(), xs.max(), ys.min(), ys.max()
    bw, bh = x1 - x0 + 1, y1 - y0 + 1

    hexes = {"dark": "#404447", "red": "#CE201F", "cream": "#FADE91",
             "gold": "#FAC332", "white": "#F5F2F0"}
    mark_svg_paths = []
    for name in ["dark", "red", "cream", "gold", "white"]:
        for d in layers[name]:
            mark_svg_paths.append((hexes[name], d))
    mark_svg = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">'
                + "".join(f'<path fill="{c}" d="{d}"/>' for c, d in mark_svg_paths)
                + "</svg>")

    # ---- res/drawable/ic_launcher_foreground.xml ----
    # The mark's bbox maps into MARK x MARK units, centered on the
    # 108dp canvas. The traced paths live in the 512-space; a group
    # transform would work, but baking the transform into the
    # coordinates keeps the drawable dumb (and aapt is happy either
    # way). Slight vertical offset keeps the bbox CENTERED, not
    # bottom-anchored (the mark is 30x29-ish, near-square).
    scale = MARK / max(bw, bh)
    ox = (CANVAS - bw * scale) / 2 - x0 * scale
    oy = (CANVAS - bh * scale) / 2 - y0 * scale
    fg = os.path.join(RES, "drawable", "ic_launcher_foreground.xml")
    os.makedirs(os.path.dirname(fg), exist_ok=True)
    with open(fg, "w") as f:
        f.write('''<?xml version="1.0" encoding="utf-8"?>
<!-- The Younblock launcher mark, traced from assets/icon.ico (the
     owner's icon — a padlock with the play button, a gold bolt with a
     light outline breaking through, two impact dashes) by
     scripts/gen_android_icons.py. Five flat colors, the source's
     z-order, the mark fitted into the adaptive safe zone. Generated:
     do not hand-edit; rerun the script instead. -->
<vector xmlns:android="http://schemas.android.com/apk/res/android"
    android:width="108dp"
    android:height="108dp"
    android:viewportWidth="108"
    android:viewportHeight="108">
''')
        for c, d in mark_svg_paths:
            f.write(f'    <path\n        android:fillColor="{c}"\n'
                    f'        android:pathData="{bake(d, scale, ox, scale, oy)}" />\n')
        f.write('</vector>\n')
    print(f"wrote {fg}")

    # ---- res/mipmap-anydpi-v26/ic_launcher.xml ----
    ad = os.path.join(RES, "mipmap-anydpi-v26", "ic_launcher.xml")
    os.makedirs(os.path.dirname(ad), exist_ok=True)
    with open(ad, "w") as f:
        f.write('''<?xml version="1.0" encoding="utf-8"?>
<!-- The adaptive launcher icon (M8-am, ADR-0051, D104): the brand dark
     as the background (whatever the launcher's mask shape, the mark
     sits on Younblock's dark), the traced mark as the foreground, and
     the same foreground as the Android 13+ monochrome layer (the
     silhouette reads cleanly tinted). Generated by
     scripts/gen_android_icons.py. -->
<adaptive-icon xmlns:android="http://schemas.android.com/apk/res/android">
    <background android:drawable="@color/launcher_background" />
    <foreground android:drawable="@drawable/ic_launcher_foreground" />
    <monochrome android:drawable="@drawable/ic_launcher_foreground" />
</adaptive-icon>
''')
    print(f"wrote {ad}")

    # ---- res/values/colors.xml: the launcher background ----
    colors = os.path.join(RES, "values", "colors.xml")
    cur = open(colors).read()
    if "launcher_background" not in cur:
        cur = cur.replace("</resources>",
                          '    <!-- The launcher icon\'s background layer (M8-am, ADR-0051):\n'
                          '         the brand dark gen_icons.py tiles the Windows app icon\n'
                          '         with — the same dark, every edition. -->\n'
                          '    <color name="launcher_background">#1E1F21</color>\n'
                          "</resources>")
        open(colors, "w").write(cur)
        print(f"wrote {colors} (launcher_background appended)")
    else:
        print(f"{colors} already carries launcher_background")

    # ---- the legacy raster set: mark on the dark rounded tile ----
    mark_hi = io.BytesIO()
    cairosvg.svg2png(bytestring=mark_svg.encode(), write_to=mark_hi,
                     output_width=1024, output_height=1024)
    mark_img = Image.open(io.BytesIO(mark_hi.getvalue())).convert("RGBA")
    from PIL import ImageDraw
    for dpi, size in [("mdpi", 48), ("hdpi", 72), ("xhdpi", 96),
                      ("xxhdpi", 144), ("xxxhdpi", 192)]:
        tile = Image.new("RGBA", (size, size), (0, 0, 0, 0))
        d = ImageDraw.Draw(tile)
        d.rounded_rectangle([0, 0, size - 1, size - 1],
                            radius=int(size * 0.22), fill=TILE + (255,))
        target = int(size * 0.62)
        scaled = mark_img.resize((target, target), Image.LANCZOS)
        tile.alpha_composite(scaled, ((size - target) // 2, (size - target) // 2))
        out = os.path.join(RES, f"mipmap-{dpi}", "ic_launcher.png")
        os.makedirs(os.path.dirname(out), exist_ok=True)
        tile.save(out)
        print(f"wrote {out} ({size}x{size})")


def bake(d, sx, ox, sy, oy):
    """Per-axis scale+offset bake for an absolute-command path."""
    out = []
    for tok in re.split(r"([MCLZmlcz])", d):
        if not tok or tok in "MCLZmlcz":
            out.append(tok)
            continue
        nums = [float(x) for x in re.findall(r"-?\d+(?:\.\d+)?", tok)]
        for k in range(0, len(nums) - 1, 2):
            nums[k] = ox + nums[k] * sx
            nums[k + 1] = oy + nums[k + 1] * sy
        out.append("".join(f"{round(v, 2):g} " for v in nums))
    return "".join(out).replace("  ", " ").strip()


if __name__ == "__main__":
    main()
