#!/usr/bin/env python3
"""Generate Younblock's Android launcher icon from the owner's logo
(M8-ao, ADR-0054, D108 — supersedes the icon.ico trace of M8-am/D104).

The source is younblock-logo-v1.png at the repo root — the owner's
449x449 RGBA upload (commit 35dfcc1; a padlock with the play button
in its red body, a silver open shackle, three red impact sparks, the
"younblock" wordmark at the bottom — "youn" white, "block" red — on
a dark blue-grey gradient inside his own squircle tile with a ~10%
corner radius and a 1px light rim). The M8-am round traced the old
32px icon.ico into a VectorDrawable because 32px could not be
upscaled; this source is the opposite problem: a finished full-bleed
icon design that must be RECOMPOSED for Android's adaptive masks.

The recomposition (fidelity first — his pixels, never a redraw):

  * The tile's content spans 79% of its width and its wordmark sits
    in the bottom quarter, so a full-bleed adaptive rendering would
    place the wordmark ENTIRELY outside the 72dp visible window
    (every launcher would crop it away). The art is therefore split:
  * BACKGROUND — the tile's own dark gradient, reconstructed as a
    smooth 2D field (masked 32px average-pool of the tile's
    background samples, nearest-cell fill, bicubic upsample) and
    rendered FULL-BLEED on the 108dp canvas. The launcher's mask
    supplies the shape his squircle owns on other platforms.
  * FOREGROUND — the tile soft-matted against that field: a pixel
    keeps his exact color with alpha ramping on its local
    difference from the field (content solid, his drop shadows
    semi-transparent, plain background fully transparent — so the
    field shows through and there is no tile boundary at all). The
    2px rim band and the top glow band (both tile-edge artifacts of
    his squircle, not art) are forced transparent. The matte is
    scaled so every element lands inside the 66dp safe circle (the
    strictest mask guarantee — no corner-rounding asterisks), with
    the content's extent optionally rigid-shifted a few px to
    center its reach (his arrangement is untouched; the background
    is position-independent, so the shift is invisible).
  * MONOCHROME — the same matte sharpened to a silhouette, white
    with alpha, at the identical geometry (the Android 13+ themed
    layer; the letter counters survive because the matte follows
    his pixels).
  * LEGACY mipmap PNGs — his tile EXACTLY as drawn (squircle, rim,
    glow and all), one per density, for the pre-adaptive contexts.

Verification (the M8-am IoU gate's heirs — every gate is evaluated
on in-memory renders BEFORE any file is written; a failure writes
nothing and exits non-zero):

  * source shape gate (449x449 RGBA, sha256 recorded);
  * the geometry proof — the rendered foreground's alpha>=128
    region must sit inside S*(33/108)+0.75px of canvas center on
    every density, and its IoU against the source matte resampled
    to the same geometry must be >= 0.92;
  * the field's fidelity — mean |tile - field| over the tile's
    background pixels (the matte's cleanliness depends on it);
  * content-presence gates on the composed 432px preview (red
    padlock pixels, white wordmark pixels, the monochrome's
    silhouette);
  * structure gates (background fully opaque, everything centered,
    the legacy tile carries his colors at its center).

Dependencies (pip): pillow, numpy, scipy. (The vtracer/cairosvg of
the M8-am pipeline are retired with the trace.) The generated files
are committed, so rerunning is only needed when the source changes.
"""
import hashlib
import io
import os
import sys

import numpy as np
from PIL import Image
from scipy import ndimage

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC = os.path.join(ROOT, "younblock-logo-v1.png")
RES = os.path.join(ROOT, "android", "app", "src", "main", "res")

# ---- the adaptive canvas (dp) -----------------------------------------
CANVAS_DP = 108.0
SAFE_RADIUS_DP = 33.0   # the 66dp safe circle: the strictest mask spec
DENSITIES = [("mdpi", 108), ("hdpi", 162), ("xhdpi", 216),
             ("xxhdpi", 324), ("xxxhdpi", 432)]
LEGACY = [("mdpi", 48), ("hdpi", 72), ("xhdpi", 96),
          ("xxhdpi", 144), ("xxxhdpi", 192)]

# ---- the source's structure (measured 2026-10-21, M8-ao) --------------
SRC_W, SRC_H = 449, 449
EXPECTED_TILE = (16, 15, 434, 433)   # the squircle's alpha bbox, +-2px

# ---- the matting knobs -------------------------------------------------
CONTENT_BRIGHT = 300     # r+g+b above which a pixel is unambiguous art
CONTENT_RED = 40         # r-g above which a pixel is red art however
                         # dark its 3D shading (the padlock body's
                         # shadow side drops to bright~73 — brightness
                         # alone cannot split it from the bg, chroma can:
                         # the tile's bg is neutral-blue, r-g ~ -1)
RIM_KILL = 2             # px of tile edge forced transparent (the rim)
MATTE_FLOOR = 4.0        # field-difference units below which alpha is 0
MATTE_SCALE = 26.0       # units over which the matte ramps to full
MATTE_SIGMA = 0.6        # the matte's edge softness (px, in 449 space)
GLOW_BAND = 32           # tile rows above this are the top glow, not art
FIELD_CELL = 32          # the field's pooling cell (px)
FIELD_MIN_SAMPLES = 30   # pool cells below this borrow their nearest

# ---- the gates ---------------------------------------------------------
MIN_IOU = 0.92           # rendered foreground vs the source matte
FIELD_MEAN_MAX = 6.0     # mean |tile - field| over the tile's bg
SHIFT_MIN_GAIN = 0.02    # apply the centering shift only above 2% gain


def fail(msg):
    sys.exit(f"gen_android_icons: GATE FAILED — {msg} (nothing written)")


def load_source():
    if not os.path.exists(SRC):
        sys.exit(f"gen_android_icons: source missing: {SRC}")
    raw = open(SRC, "rb").read()
    print(f"source: {SRC}")
    print(f"  sha256 {hashlib.sha256(raw).hexdigest()}")
    im = Image.open(io.BytesIO(raw)).convert("RGBA")
    if im.size != (SRC_W, SRC_H):
        fail(f"source shape {im.size}, expected {(SRC_W, SRC_H)}")
    arr = np.array(im).astype(np.float64)
    alpha = arr[:, :, 3] > 128
    if not alpha.any():
        fail("source has no opaque pixels")
    ys, xs = np.where(alpha)
    bbox = (int(xs.min()), int(ys.min()), int(xs.max()), int(ys.max()))
    if (abs(bbox[0] - EXPECTED_TILE[0]) > 2 or abs(bbox[1] - EXPECTED_TILE[1]) > 2
            or abs(bbox[2] - EXPECTED_TILE[2]) > 2 or abs(bbox[3] - EXPECTED_TILE[3]) > 2):
        fail(f"the tile's alpha bbox moved: {bbox}, expected ~{EXPECTED_TILE} "
             "(the source changed — re-measure EXPECTED_TILE before trusting it)")
    print(f"  tile bbox {bbox} (the squircle), {alpha.sum()} opaque px")
    return arr, alpha


def build_masks(arr, alpha):
    """The art-detection masks everything else is derived from."""
    bright = arr[:, :, :3].sum(axis=2)
    redness = arr[:, :, 0] - arr[:, :, 1]
    core = ndimage.binary_erosion(alpha, iterations=RIM_KILL + 1)
    content = core & ((bright > CONTENT_BRIGHT) | (redness > CONTENT_RED))
    if content.sum() < 20000:
        fail(f"only {content.sum()} content pixels detected (expected the "
             "padlock + wordmark + sparks — do CONTENT_BRIGHT/CONTENT_RED "
             "fit this source?)")
    # The background samples: everything opaque, away from the art and
    # its AA/shadow halo. No brightness filter — the top glow band is a
    # background feature and belongs in the field.
    bgmask = core & ~ndimage.binary_dilation(content, iterations=6)
    if bgmask.sum() < 20000:
        fail(f"only {bgmask.sum()} background samples for the field")
    # The art must not reach into the glow band this pipeline kills.
    ys, _ = np.where(content)
    if ys.min() < GLOW_BAND:
        fail(f"content starts at y={ys.min()}, inside the glow band "
             f"(y<{GLOW_BAND}) — the band-kill rule would eat art; "
             "re-measure GLOW_BAND for this source")
    print(f"  content {content.sum()} px (bright>{CONTENT_BRIGHT} or "
          f"r-g>{CONTENT_RED}), bg samples {bgmask.sum()} px, "
          f"content top y={ys.min()}")
    return bright, core, content, bgmask


def build_field(arr, bgmask):
    """The tile's background as a smooth 2D field (masked pooling)."""
    h, w = arr.shape[:2]
    gh, gw = (h + FIELD_CELL - 1) // FIELD_CELL, (w + FIELD_CELL - 1) // FIELD_CELL
    cnt = np.zeros((gh, gw))
    sums = np.zeros((gh, gw, 3))
    ys, xs = np.where(bgmask)
    rgb = arr[ys, xs, :3]
    gy, gx = ys // FIELD_CELL, xs // FIELD_CELL
    np.add.at(cnt, (gy, gx), 1.0)
    for k in range(3):
        np.add.at(sums[:, :, k], (gy, gx), rgb[:, k])
    valid = cnt >= FIELD_MIN_SAMPLES
    field = np.zeros((gh, gw, 3))
    field[valid] = sums[valid] / cnt[valid][:, None]
    if not valid.all():
        _, (ny, nx) = ndimage.distance_transform_edt(
            ~valid, return_indices=True)
        field = field[ny, nx]
    img = Image.fromarray(np.clip(field, 0, 255).astype(np.uint8))
    field_big = np.array(img.resize((w, h), Image.BICUBIC)).astype(np.float64)

    # Gate: the field must agree with the tile's own background.
    diff = np.abs(arr[:, :, :3] - field_big).max(axis=2)
    m = diff[bgmask]
    print(f"  field fidelity over bg px: mean {m.mean():.2f}, p95 "
          f"{np.percentile(m, 95):.1f} (gate mean<={FIELD_MEAN_MAX})")
    if m.mean() > FIELD_MEAN_MAX:
        fail(f"field reconstruction mean error {m.mean():.2f} > {FIELD_MEAN_MAX}")
    return field_big


def build_matte(arr, alpha, field, content):
    """The soft difference matte: his pixels, ramped by how far they
    sit from the reconstructed background."""
    diff = np.abs(arr[:, :, :3] - field).max(axis=2)
    matte = np.clip((diff - MATTE_FLOOR) / MATTE_SCALE, 0.0, 1.0)
    interior = ndimage.binary_erosion(alpha, iterations=RIM_KILL)
    matte[~interior] = 0.0          # the rim and the corner AA die here
    matte[:GLOW_BAND, :] = 0.0      # the top glow band is a tile-edge
    #                                   feature, not art
    matte = ndimage.gaussian_filter(matte, MATTE_SIGMA)
    matte[~interior] = 0.0          # the blur cannot leak past the rim
    solid = int((matte >= 0.95).sum())
    soft = int(((matte > 0.05) & (matte < 0.95)).sum())
    if solid < 8000:
        fail(f"matte has only {solid} solid px (expected the art at full "
             "alpha) — the matting knobs do not fit this source")
    print(f"  matte: {solid} solid px, {soft} soft px (the AA + shadows)")
    return matte


def optimal_shift(matte):
    """Rigidly center the art's REACH (not its arrangement): the tile's
    own placement is his, but the canvas seat is ours. Search small
    integer shifts, keep the best only on a real gain."""
    m = matte >= 0.5
    if not m.any():
        fail("the matte is empty at alpha>=0.5")
    ys, xs = np.where(m)
    h, w = matte.shape
    cy, cx = (h - 1) / 2.0, (w - 1) / 2.0
    base = np.sqrt((xs - cx) ** 2 + (ys - cy) ** 2).max()

    def reach(sy, sx):
        return np.sqrt((xs + sx - cx) ** 2 + (ys + sy - cy) ** 2).max()

    best = (0, 0, base)
    for sy in range(-40, 41):
        for sx in range(-20, 21):
            r = reach(sy, sx)
            if r < best[2]:
                best = (sy, sx, r)
    sy, sx, r = best
    gain = (base - r) / base
    if gain < SHIFT_MIN_GAIN:
        print(f"  extent centering: no shift "
              f"(best ({sy},{sx}) gains {gain:.1%} < {SHIFT_MIN_GAIN:.0%})")
        return 0, 0, base
    print(f"  extent centering: shift ({sy},{sx}) px, reach {base:.1f}"
          f"->{r:.1f} px ({gain:.1%} larger art)")
    return sy, sx, r


def resize_rgba_pm(img, size):
    """LANCZOS resize with premultiplication (a naive RGBA resize
    blends RGB with fully-transparent neighbors and fringes dark)."""
    arr = np.array(img).astype(np.float64)
    a = arr[:, :, 3:4] / 255.0
    arr[:, :, :3] *= a
    small = Image.fromarray(np.clip(arr, 0, 255).astype(np.uint8))
    small = small.resize((size, size), Image.LANCZOS)
    out = np.array(small).astype(np.float64)
    a2 = out[:, :, 3:4]
    with np.errstate(invalid="ignore", divide="ignore"):
        out[:, :, :3] = np.where(a2 > 0, out[:, :, :3] / (a2 / 255.0), 0)
    return Image.fromarray(np.clip(out, 0, 255).astype(np.uint8))


def render(canvas_px, tile_px, offs, fg_src, field_img, mono_src):
    """One density's three adaptive layers + the geometry."""
    S = canvas_px
    # background: the field, full-bleed, opaque.
    bg = field_img.resize((S, S), Image.BICUBIC).convert("RGBA")
    # foreground: the matted tile at tile_px, centered + the shift.
    fg_tile = resize_rgba_pm(fg_src, tile_px)
    fg = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    ox = (S - tile_px) // 2 + round(offs[1] * tile_px / SRC_W)
    oy = (S - tile_px) // 2 + round(offs[0] * tile_px / SRC_H)
    fg.paste(fg_tile, (ox, oy))
    # monochrome: the silhouette at the same geometry.
    mono_tile = resize_rgba_pm(mono_src, tile_px)
    mono = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    mono.paste(mono_tile, (ox, oy))
    return bg, fg, mono, (ox, oy)


def main():
    arr, alpha = load_source()
    bright, core, content, bgmask = build_masks(arr, alpha)
    field = build_field(arr, bgmask)
    matte = build_matte(arr, alpha, field, content)

    # The geometry: z fits the matte's reach into the safe circle.
    sy, sx, reach = optimal_shift(matte)
    z = min(1.0, (SAFE_RADIUS_DP / CANVAS_DP) * SRC_W / reach)
    print(f"  safe-zone zoom z={z:.4f} "
          f"(the tile renders {z * CANVAS_DP:.1f}dp of the {CANVAS_DP:.0f}dp canvas)")

    # The monochrome: the matte sharpened (faint shadows drop out,
    # the art and its AA edges survive).
    mono_a = np.clip((matte - 0.30) / 0.25, 0, 1)
    mono_rgb = np.zeros((SRC_H, SRC_W, 4))
    mono_rgb[:, :, 0:3] = 255.0
    mono_rgb[:, :, 3] = mono_a * 255.0
    mono_src = Image.fromarray(mono_rgb.astype(np.uint8))
    if (mono_a >= 0.5).sum() < 8000:
        fail("the monochrome silhouette is too small to read")

    # the foreground source: his pixels, the matte as their alpha.
    fg_arr = arr.copy()
    fg_arr[:, :, 3] = matte * 255.0
    fg_src = Image.fromarray(np.clip(fg_arr, 0, 255).astype(np.uint8))
    src_rgba = Image.fromarray(arr.astype(np.uint8))
    field_img = Image.fromarray(
        np.clip(field, 0, 255).astype(np.uint8))

    # ---- render every density in memory, gate, and only then write ----
    outputs = {}
    gate_log = []

    def content_reach(fg_img):
        a = np.array(fg_img)[:, :, 3] >= 128
        if not a.any():
            return None, 0
        er = ndimage.binary_erosion(a, iterations=1)
        ys, xs = np.where(er)
        S = fg_img.size[0]
        cy = (S - 1) / 2.0
        d = np.sqrt((xs - cy) ** 2 + (ys - cy) ** 2)
        return d.max(), int(a.sum())

    for dpi, S in DENSITIES:
        tile_px = max(1, round(z * S))
        bg, fg, mono, (ox, oy) = render(
            S, tile_px, (sy, sx), fg_src, field_img, mono_src)
        limit = S * (SAFE_RADIUS_DP / CANVAS_DP) + 0.75
        reach_px, npx = content_reach(fg)
        if reach_px is None or reach_px > limit:
            fail(f"{dpi}: foreground reach {reach_px}px > {limit:.1f}px "
                 "(the safe-circle geometry broke)")
        # the geometry proof: IoU against the source matte resampled.
        expect = np.zeros((S, S), dtype=bool)
        mm = np.array(Image.fromarray(
            (matte >= 0.5).astype(np.uint8) * 255
        ).resize((tile_px, tile_px), Image.NEAREST)) >= 128
        expect[max(0, oy):max(0, oy) + tile_px, max(0, ox):max(0, ox) + tile_px] |= mm
        got = np.array(fg)[:, :, 3] >= 128
        iou = (got & expect).sum() / max(1, (got | expect).sum())
        if iou < MIN_IOU:
            fail(f"{dpi}: foreground/matte IoU {iou:.4f} < {MIN_IOU}")
        # background: opaque, and its corners are the dark family.
        bga = np.array(bg)
        if not (bga[:, :, 3] == 255).all():
            fail(f"{dpi}: background is not fully opaque")
        corners = np.concatenate([bga[:8, :8], bga[:8, -8:], bga[-8:, :8],
                                  bga[-8:, -8:]]).reshape(-1, 4)[:, :3]
        if corners.min() < 20 or corners.max() > 130:
            fail(f"{dpi}: background corners {corners.min()}..{corners.max()} "
                 "left the tile's dark family")
        # monochrome: present, and aligned with the foreground (the
        # centroid is NOT the check — his art's mass is legitimately
        # asymmetric; the layers sharing one geometry is the invariant).
        ma = np.array(mono)[:, :, 3]
        mys, mxs = np.where(ma >= 128)
        if len(mys) < 400:
            fail(f"{dpi}: monochrome silhouette too small")
        fys, fxs = np.where(got)
        mbc = ((mys.mean() + mys.min() + mys.max()) / 3,
               (mxs.mean() + mxs.min() + mxs.max()) / 3)
        fbc = ((fys.mean() + fys.min() + fys.max()) / 3,
               (fxs.mean() + fxs.min() + fxs.max()) / 3)
        off = ((mbc[0] - fbc[0]) ** 2 + (mbc[1] - fbc[1]) ** 2) ** 0.5
        if off > 1.5:
            fail(f"{dpi}: monochrome misaligned from the foreground by "
                 f"{off:.1f}px (they must share one geometry)")
        gate_log.append((dpi, S, tile_px, reach_px, limit, iou, int(ma.any() and (ma >= 128).sum())))
        outputs[dpi] = (bg, fg, mono)

    # the legacy set: his tile exactly as drawn.
    for dpi, S in LEGACY:
        legacy = resize_rgba_pm(src_rgba, S)
        px = np.array(legacy)
        # the bg sample: the corner-interior patch (the tile's center
        # is the padlock body, not background — measured, not assumed).
        c = int(S * 0.13)
        patch = px[c:c + 3, c:c + 3, :3].reshape(-1, 3)
        med = np.median(patch, axis=0)
        reds = int(((px[:, :, 0] > 150) & (px[:, :, 0] - px[:, :, 1] > 60)
                    & (px[:, :, 3] > 128)).sum())
        whites = int(((px[:, :, :3].min(axis=2) > 200) & (px[:, :, 3] > 128)).sum())
        if not (20 <= med.min() and med.max() <= 130 and med[0] - med[1] < 30
                and reds > 50 and whites > 50):
            fail(f"legacy {dpi}: lost the tile (corner {tuple(med.astype(int))}, "
                 f"{reds} red px, {whites} white px)")
        outputs[f"legacy-{dpi}"] = legacy

    # the composed 432px preview gates: his art, present and whole.
    bg, fg, mono, _ = render(432, round(z * 432), (sy, sx),
                             fg_src, field_img, mono_src)
    comp = np.array(Image.alpha_composite(bg, fg))
    reds = int(((comp[:, :, 0] > 150) & (comp[:, :, 0] - comp[:, :, 1] > 60)
                & (comp[:, :, 3] > 200)).sum())
    whites = int(((comp[:, :, :3].min(axis=2) > 200)
                  & (comp[:, :, 3] > 200)).sum())
    if reds < 2000 or whites < 2000:
        fail(f"the composed preview lost the art ({reds} red, {whites} white)")
    print(f"  composed 432px preview: {reds} red px, {whites} white px")

    print("\n  density canvas tile  reach  limit    IoU  mono-px")
    for dpi, S, tile_px, reach_px, limit, iou, mn in gate_log:
        print(f"  {dpi:7s} {S:5d} {tile_px:5d} {reach_px:5.0f} {limit:6.1f} "
              f"{iou:.4f}  {mn}")

    # ---- everything passed: write --------------------------------------
    for dpi, S in DENSITIES:
        bg, fg, mono = outputs[dpi]
        for name, img in [("ic_launcher_background", bg),
                          ("ic_launcher_foreground", fg),
                          ("ic_launcher_monochrome", mono)]:
            out = os.path.join(RES, f"mipmap-{dpi}", name + ".png")
            os.makedirs(os.path.dirname(out), exist_ok=True)
            img.save(out, optimize=True)
            print(f"wrote {out} ({img.size[0]}x{img.size[1]}, "
                  f"{os.path.getsize(out)} B)")
    for dpi, S in LEGACY:
        out = os.path.join(RES, f"mipmap-{dpi}", "ic_launcher.png")
        outputs[f"legacy-{dpi}"].save(out, optimize=True)
        print(f"wrote {out} ({S}x{S}, {os.path.getsize(out)} B)")

    # the adaptive wiring: bitmaps, not the traced vector.
    ad = os.path.join(RES, "mipmap-anydpi-v26", "ic_launcher.xml")
    os.makedirs(os.path.dirname(ad), exist_ok=True)
    with open(ad, "w") as f:
        f.write('''<?xml version="1.0" encoding="utf-8"?>
<!-- The adaptive launcher icon (M8-ao, ADR-0054, D108): the owner's
     younblock-logo-v1.png â the 449px squircle tile with the 3D
     padlock, the play button, the silver shackle, the red sparks and
     the younblock wordmark â recomposed for Android's masks. The
     tile's own dark gradient is the full-bleed background; the art is
     soft-matted out of it as the foreground, every element inside the
     66dp safe circle; the same silhouette is the Android 13+
     monochrome layer. The launcher's mask supplies the shape his
     squircle owns on other platforms. Generated by
     scripts/gen_android_icons.py (do not hand-edit; rerun it). -->
<adaptive-icon xmlns:android="http://schemas.android.com/apk/res/android">
    <background android:drawable="@mipmap/ic_launcher_background" />
    <foreground android:drawable="@mipmap/ic_launcher_foreground" />
    <monochrome android:drawable="@mipmap/ic_launcher_monochrome" />
</adaptive-icon>
''')
    print(f"wrote {ad}")

    # retire the M8-am artifacts this round supersedes.
    old = os.path.join(RES, "drawable", "ic_launcher_foreground.xml")
    if os.path.exists(old):
        os.remove(old)
        print(f"removed {old} (the icon.ico trace, M8-am — superseded)")
    colors = os.path.join(RES, "values", "colors.xml")
    cur = open(colors).read()
    if "launcher_background" in cur:
        # strip the entry and its comment block, idempotently
        out = []
        skip_until_close = False
        for line in cur.splitlines(keepends=True):
            if "The launcher icon's background layer" in line:
                skip_until_close = True
                continue
            if skip_until_close:
                if "<color" in line:
                    skip_until_close = False
                continue
            if 'name="launcher_background"' in line:
                continue
            out.append(line)
        open(colors, "w").write("".join(out))
        print(f"stripped launcher_background from {colors} (the background "
              "is the owner's gradient raster now)")

    # ---- the VLM preview set (out of the repo; the round's visual proof)
    prev = Image.alpha_composite(bg, fg)
    for name, mk in [
        ("circle", lambda im: _mask(im, "circle")),
        ("squircle", lambda im: _mask(im, "squircle")),
    ]:
        p = f"/tmp/younblock-icon-preview-{name}.png"
        mk(prev).save(p)
        print(f"preview {p}")
    outputs["legacy-xxxhdpi"].resize((192, 192), Image.LANCZOS).save(
        "/tmp/younblock-icon-preview-legacy.png")
    print("preview /tmp/younblock-icon-preview-legacy.png")


def _mask(im, shape):
    """Crop a 432px composite to the 72dp window (288px) as a launcher
    would: the circle worst case, and a One-UI-ish squircle."""
    S = im.size[0]
    win = round(S * 72 / 108)
    x0 = (S - win) // 2
    crop = im.crop((x0, x0, x0 + win, x0 + win))
    m = Image.new("L", (win, win), 0)
    from PIL import ImageDraw
    d = ImageDraw.Draw(m)
    if shape == "circle":
        d.ellipse([0, 0, win - 1, win - 1], fill=255)
    else:
        d.rounded_rectangle([0, 0, win - 1, win - 1],
                            radius=int(win * 0.34), fill=255)
    out = Image.new("RGBA", (win, win), (0, 0, 0, 0))
    out.paste(crop, (0, 0), m)
    return out


if __name__ == "__main__":
    main()
