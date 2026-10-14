#!/usr/bin/env python3
"""Redraw the Younblock app icon at high resolution (D53).

Ali's icon exists only as a 32x32 .ico (assets/icon.ico, installed as
build/appicon.png in phase 6), which is far too small for the hicolor icon
set that .deb, AppImage and Flatpak installs need (up to 512 px). This
script redraws the same design natively at 4096 px and downscales:

  - red rounded-square padlock body (bottom, full width)
  - charcoal shackle (an inverted-U band at the top left)
  - white play triangle on the body (base left, apex right)
  - gold lightning bolt striking the top-right corner: a stroke that
    widens down-left, a horizontal bar, and a tapering tail — with a
    light-gold edge highlight and a charcoal shadow where it passes
    over the body

The palette and every coordinate below were measured from the 32 px
original pixel-by-pixel (same layout at a glance; `--check` prints a
side-by-side ASCII comparison of the original and the redraw at 32 px).

Output: packaging/icons/hicolor/<size>/apps/younblock.png and the same
images under the flatpak name (io.github.assadigit.Younblock.png) for
sizes 16..512, plus a 512 px younblock.png for the AppImage top level.
"""
import argparse
import os

from PIL import Image, ImageDraw, ImageFilter

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT_DEFAULT = os.path.join(ROOT, "packaging", "icons")

SIZES = (16, 24, 32, 48, 64, 96, 128, 256, 512)

# Palette measured from the 32 px original.
RED = (206, 32, 31, 255)
GOLD = (250, 196, 50, 255)
GOLD_LIGHT = (250, 218, 124, 255)
CHARCOAL = (63, 66, 69, 255)
WHITE = (251, 252, 252, 255)

# Everything below is expressed on the 32 px design grid and scaled.
GRID = 32
SUPERSAMPLE = 128  # draw at 4096 px for smooth edges


def s(v):
    """Scale one design-grid coordinate to the supersampled canvas."""
    return v * SUPERSAMPLE


def pts(*xy):
    return [(s(x), s(y)) for x, y in xy]


def render(size=GRID, supersample=SUPERSAMPLE):
    """Draw the logo; `size` is the design grid, `supersample` the multiplier."""
    W = size * supersample
    img = Image.new("RGBA", (W, W), (0, 0, 0, 0))

    # --- padlock body: rounded square across the bottom two thirds ---
    body_box = [s(0.2), s(13.2), s(26.4), s(31.7)]
    body = Image.new("RGBA", (W, W), (0, 0, 0, 0))
    ImageDraw.Draw(body).rounded_rectangle(body_box, radius=s(3.4), fill=RED)
    img.alpha_composite(body)

    # --- charcoal shadow where the bolt's tail passes over the body ---
    # the same right triangle as the tail, offset down-right and softened
    tail = pts((21.5, 14.0), (30.0, 14.0), (21.5, 21.5))
    off = (s(2.5), s(1.8))
    tail_off = [(x + off[0], y + off[1]) for x, y in tail]
    shadow = Image.new("RGBA", (W, W), (0, 0, 0, 0))
    ImageDraw.Draw(shadow).polygon(tail_off, fill=CHARCOAL[:3] + (185,))
    shadow = shadow.filter(ImageFilter.GaussianBlur(s(0.45)))
    img.alpha_composite(shadow)

    # --- shackle: inverted-U band, drawn as an annulus cropped at the body top ---
    shackle = Image.new("RGBA", (W, W), (0, 0, 0, 0))
    ImageDraw.Draw(shackle).ellipse([s(3.0), s(1.2), s(18.6), s(24.6)], fill=CHARCOAL)
    hole = Image.new("L", (W, W), 0)
    ImageDraw.Draw(hole).ellipse([s(7.3), s(4.6), s(14.4), s(24.6)], fill=255)
    alpha = shackle.split()[3]
    alpha.paste(0, (0, 0), hole)  # punch the hole
    # keep only the part above the body top; the legs just touch its edge
    below = Image.new("L", (W, W), 0)
    ImageDraw.Draw(below).rectangle([0, s(13.3), W, W], fill=255)
    alpha.paste(0, (0, 0), below)
    shackle.putalpha(alpha)
    img.alpha_composite(shackle)

    # --- white play triangle on the body ---
    dimg = ImageDraw.Draw(img)
    dimg.polygon(pts((10.0, 18.2), (10.0, 25.9), (18.2, 22.05)), fill=WHITE)

    # --- gold lightning bolt at the top-right corner ---
    # upper stroke: widens as it descends to the left
    dimg.polygon(pts((26.9, 0.2), (30.7, 0.2), (27.7, 9.6), (19.6, 9.6)), fill=GOLD)
    # horizontal bar
    dimg.polygon(pts((17.6, 9.8), (30.8, 9.8), (30.8, 14.2), (17.6, 14.2)), fill=GOLD)
    # tapering right triangle hanging from the bar's left end
    dimg.polygon(pts((21.5, 14.0), (30.0, 14.0), (21.5, 21.5)), fill=GOLD)
    # light-gold highlight along the bolt's working edges
    hl = int(1.05 * supersample)
    dimg.line(pts((26.6, 0.5), (19.9, 9.4)), fill=GOLD_LIGHT, width=hl)
    dimg.line(pts((22.2, 14.4), (22.2, 21.0)), fill=GOLD_LIGHT, width=hl)
    dimg.line(pts((28.8, 14.6), (23.0, 20.6)), fill=GOLD_LIGHT, width=int(hl * 0.7))

    if supersample > 1:
        img = img.resize((size, size), Image.LANCZOS)
    return img


def ascii_map(img, size=32):
    """Render a color-coded ASCII map for eyeball comparison (32 px grid)."""

    def near(c, t, tol):
        return all(abs(c[i] - t[i]) <= tol for i in range(3))

    small = img.resize((size, size), Image.LANCZOS)
    px = small.load()
    lines = []
    for y in range(size):
        row = []
        for x in range(size):
            c = px[x, y]
            if c[3] < 40:
                row.append(".")
            elif near(c, RED, 55):
                row.append("R")
            elif near(c, GOLD, 70):
                row.append("G")
            elif near(c, CHARCOAL, 30):
                row.append("K")
            elif near(c, WHITE, 30):
                row.append("W")
            else:
                row.append("?")
        lines.append("".join(row))
    return "\n".join(lines)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default=OUT_DEFAULT, help="output directory")
    ap.add_argument("--check", action="store_true",
                    help="print a 32 px ASCII comparison against the original icon")
    args = ap.parse_args()

    master = render()

    if args.check:
        original = Image.open(os.path.join(ROOT, "build", "appicon.png")).convert("RGBA")
        print("original (32 px):")
        print(ascii_map(original))
        print()
        print("redraw (downscaled to 32 px):")
        print(ascii_map(master))
        return

    hicolor = os.path.join(args.out, "hicolor")
    names = ("younblock", "io.github.assadigit.Younblock")
    for n in SIZES:
        img = master.resize((n, n), Image.LANCZOS)
        for name in names:
            path = os.path.join(hicolor, str(n), "apps", f"{name}.png")
            os.makedirs(os.path.dirname(path), exist_ok=True)
            img.save(path)
    # AppImage top-level icon (the desktop file's Icon= name).
    os.makedirs(args.out, exist_ok=True)
    master.resize((512, 512), Image.LANCZOS).save(os.path.join(args.out, "younblock.png"))
    print(f"wrote {len(SIZES)} sizes x {len(names)} names + younblock.png to {args.out}")


if __name__ == "__main__":
    main()
