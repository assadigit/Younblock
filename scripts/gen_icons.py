#!/usr/bin/env python3
"""Generate Younblock's tray and app icons (file 02, section 5).

Tray icons differ by SHAPE, not only color:
  off        = outlined ring with the power notch (gap at the top)
  on         = solid ring with a dark notch
  attention  = outlined ring with a solid dot
Each ships in a dark-taskbar and a light-taskbar variant, at 16/20/24/32 px,
bundled into .ico files with PNG entries (Windows supports PNG entries).

The app icon is Ali's logo on a dark rounded square.
"""
import os
import struct
import math

from PIL import Image, ImageDraw

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ASSETS = os.path.join(ROOT, "assets")

# Palette (file 02, section 2).
NEUTRAL_400 = (163, 166, 173, 255)
NEUTRAL_600 = (101, 105, 113, 255)
GREEN_500 = (52, 199, 123, 255)
GREEN_600 = (44, 170, 105, 255)
AMBER_400 = (242, 184, 75, 255)
AMBER_500 = (238, 161, 17, 255)
DARK = (20, 21, 22, 255)
BG_DARK = (30, 31, 33, 255)   # neutral-950 for the app icon


def attention_icon(size, *, ring_color, dot_color):
    ss = 4
    w = size * ss
    img = Image.new("RGBA", (w, w), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    cx = cy = w / 2
    r = w * 0.40
    width = max(2, int(w * 0.085))
    bbox = [cx - r, cy - r, cx + r, cy + r]
    d.ellipse(bbox, outline=ring_color, width=width)
    rd = w * 0.17
    d.ellipse([cx - rd, cy - rd, cx + rd, cy + rd], fill=dot_color)
    return img.resize((size, size), Image.LANCZOS)


def on_icon(size, *, ring_color, notch_color):
    ss = 4
    w = size * ss
    img = Image.new("RGBA", (w, w), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    cx = cy = w / 2
    r_outer = w * 0.46
    r_inner = w * 0.24
    width = int(r_outer - r_inner)
    gap_half = 34
    start = 270 + gap_half
    end = 270 - gap_half + 360
    bbox = [cx - r_outer, cy - r_outer, cx + r_outer, cy + r_outer]
    d.arc(bbox, start, end, fill=ring_color, width=width)
    # The dark notch: vertical bar inside the gap.
    bar_w = width
    y0 = cy - r_outer
    y1 = cy - r_inner + width * 0.2
    d.rounded_rectangle([cx - bar_w / 2, y0, cx + bar_w / 2, y1],
                        radius=bar_w / 2, fill=notch_color)
    return img.resize((size, size), Image.LANCZOS)


def off_icon(size, *, ring_color):
    ss = 4
    w = size * ss
    img = Image.new("RGBA", (w, w), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    cx = cy = w / 2
    r = w * 0.40
    width = max(2, int(w * 0.09))
    gap_half = 40
    start = 270 + gap_half
    end = 270 - gap_half + 360
    bbox = [cx - r, cy - r, cx + r, cy + r]
    d.arc(bbox, start, end, fill=ring_color, width=width)
    # Short vertical bar in the gap (the power stroke), outlined style.
    bar_w = width
    y0 = cy - r - width * 0.35
    y1 = cy - r * 0.25
    d.rounded_rectangle([cx - bar_w / 2, y0, cx + bar_w / 2, y1],
                        radius=bar_w / 2, fill=ring_color)
    return img.resize((size, size), Image.LANCZOS)


def write_ico_png_entries(path, images):
    """Bundle per-size RGBA images into an ICO with PNG entries."""
    with open(path, "wb") as f:
        count = len(images)
        f.write(struct.pack("<HHH", 0, 1, count))
        blobs = [_png_bytes(img) for img in images]
        offset = 6 + 16 * count
        for img, blob in zip(images, blobs):
            w = img.width % 256
            h = img.height % 256
            f.write(struct.pack("<BBBBHHII", w, h, 0, 0, 1, 32, len(blob), offset))
            offset += len(blob)
        for blob in blobs:
            f.write(blob)


def _png_bytes(img):
    import io
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return buf.getvalue()


def build_ico(path, render, sizes=(16, 20, 24, 32)):
    imgs = [render(s) for s in sizes]
    write_ico_png_entries(path, imgs)
    print("wrote", path)


def app_icon():
    """Ali's logo on a dark rounded square (1024 px + multi-size ico)."""
    logo = Image.open(os.path.join(ASSETS, "logo.png")).convert("RGBA")
    size = 1024
    img = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    d.rounded_rectangle([0, 0, size - 1, size - 1], radius=int(size * 0.22), fill=BG_DARK)
    # Logo at ~66% width, centered.
    target_w = int(size * 0.66)
    scale = target_w / logo.width
    target_h = int(logo.height * scale)
    scaled = logo.resize((target_w, target_h), Image.LANCZOS)
    img.alpha_composite(scaled, ((size - target_w) // 2, (size - target_h) // 2))
    img.save(os.path.join(ROOT, "build", "appicon.png"))
    # Windows icon sizes.
    imgs = []
    for s in (256, 128, 64, 48, 32, 16):
        imgs.append(img.resize((s, s), Image.LANCZOS))
    write_ico_png_entries(os.path.join(ROOT, "build", "windows", "icon.ico"), imgs)
    print("wrote appicon.png and build/windows/icon.ico")


def main():
    os.makedirs(ASSETS, exist_ok=True)
    # Dark taskbar variants.
    build_ico(os.path.join(ASSETS, "tray-off.ico"), lambda s: off_icon(s, ring_color=NEUTRAL_400))
    build_ico(os.path.join(ASSETS, "tray-on.ico"), lambda s: on_icon(s, ring_color=GREEN_500, notch_color=DARK))
    build_ico(os.path.join(ASSETS, "tray-attention.ico"),
              lambda s: attention_icon(s, ring_color=NEUTRAL_400, dot_color=AMBER_400))
    # Light taskbar variants: darker outlines for contrast.
    build_ico(os.path.join(ASSETS, "tray-off-light.ico"), lambda s: off_icon(s, ring_color=NEUTRAL_600))
    build_ico(os.path.join(ASSETS, "tray-on-light.ico"), lambda s: on_icon(s, ring_color=GREEN_600, notch_color=DARK))
    build_ico(os.path.join(ASSETS, "tray-attention-light.ico"),
              lambda s: attention_icon(s, ring_color=NEUTRAL_600, dot_color=AMBER_500))
    app_icon()


if __name__ == "__main__":
    main()
