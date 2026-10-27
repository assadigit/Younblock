#!/usr/bin/env python3
"""render_lite_preview.py — a faithful PIL mockup of the lite desktop
edition's v3 screen (cmd/younblock-lite, the D115 round): the same
design tokens as the Go painter (screen_windows.go — every value ported
from docs/ui-v3/younblock-tokens.css and the Svelte components), the
same layout math (window_windows.go relayout), the same copy (view.go),
drawn at 2x for clarity. Five panels: idle, connecting, connected,
failed, and the menu sheet. This verifies the DESIGN before any Windows
machine runs it (no Windows display needed) and produces the design
document at docs/ui-v3/screenshots/lite-desktop-preview.png.

The D116 lesson lives here: PIL's Image.paste REPLACES pixels where the
mask is set — it never alpha-blends. The sheen and the bottom shade are
mostly-transparent layers, so pasting them through the cap mask wiped
the satin-red face to transparent black and the preview's button read
dark instead of red (the owner's verdict: "your button color is off. it
must be red" — the exe was always right; only this mockup was wrong).
The rule the file now follows: clip with Image.composite, blend with
alpha_composite, and only ever paste an OPAQUE source.

Usage: python3 scripts/render_lite_preview.py [output.png]
(needs Pillow; the fonts are the sandbox's DejaVu — close enough to
Segoe UI for a design check)
"""
import os
import sys

from PIL import Image, ImageDraw, ImageFilter, ImageFont

S = 2  # supersample factor
W, H = 360 * S, 640 * S

# ---- tokens (screen_windows.go, ported) ----
BG_STOPS = {
    "idle":       [(0x3b, 0x3a, 0x4a, 0), (0x2d, 0x2d, 0x37, .35), (0x26, 0x26, 0x2e, .70), (0x26, 0x26, 0x2e, 1)],
    "connecting": [(0x3b, 0x3a, 0x4a, 0), (0x2d, 0x2d, 0x37, .35), (0x26, 0x26, 0x2e, .70), (0x26, 0x26, 0x2e, 1)],
    "connected":  [(0x54, 0x75, 0x6a, 0), (0x46, 0x66, 0x5b, .38), (0x2d, 0x44, 0x3c, .78), (0x25, 0x3a, 0x33, 1)],
    "failed":     [(0x7b, 0x5a, 0x3f, 0), (0x66, 0x4a, 0x33, .38), (0x3a, 0x2b, 0x20, .78), (0x2e, 0x22, 0x19, 1)],
}
LED = {"idle": (0x6b, 0x6b, 0x78), "connecting": (0xff, 0xff, 0xff),
       "connected": (0x39, 0xb9, 0x4c), "failed": (0xff, 0xa7, 0x26)}
SUB = {"idle": (0xc9, 0xc8, 0xd4), "connecting": (0xc9, 0xc8, 0xd4),
       "connected": (0xf0, 0xf7, 0xf3), "failed": (0xff, 0xd9, 0xa0)}
AURA = {"idle": None, "connecting": (255, 90, 70, 140),
        "connected": (57, 185, 76, 140), "failed": (255, 167, 38, 140)}
PILLGLOW = {"idle": (255, 40, 30, 100), "connecting": (255, 40, 30, 120),
            "connected": (255, 40, 30, 70), "failed": (255, 40, 30, 100)}
TEXT = (0xF4, 0xF0, 0xEE)
HINT = (0xd9, 0xd8, 0xe2)
EDGE = (0x8f, 0x13, 0x0e)
CAP_FACE = [(0xec, 0x3d, 0x34, 0), (0xd6, 0x2a, 0x20, .55), (0xc8, 0x23, 0x1d, 1)]
SHEET_BG = (0x1d, 0x1c, 0x26)
SHEET_RAISED = (0x26, 0x25, 0x32)
RED400 = (0xe0, 0x6b, 0x6b)

TRACK = (116 * S, 248 * S, 244 * S, 520 * S)      # 128x272 centered at (180, 384)
CAP_W, CAP_H, TRAVEL = 112 * S, 124 * S, 122 * S

try:
    def F(px, weight=None):
        name = {600: "-Bold", 700: "-Bold"}.get(weight)
        path = "/usr/share/fonts/truetype/dejavu/DejaVuSans%s.ttf" % (name or "")
        return ImageFont.truetype(path, int(px * S))
except Exception:
    F = lambda px, weight=None: ImageFont.load_default()


def vgrad(w, h, stops):
    """Multi-stop vertical gradient as an RGB image."""
    img = Image.new("RGB", (w, h))
    d = ImageDraw.Draw(img)
    for y in range(h):
        t = y / max(1, h - 1)
        for i in range(len(stops) - 1):
            a, b = stops[i], stops[i + 1]
            if a[3] <= t <= b[3]:
                f = 0 if b[3] == a[3] else (t - a[3]) / (b[3] - a[3])
                col = tuple(int(a[j] + (b[j] - a[j]) * f) for j in range(3))
                d.line([(0, y), (w, y)], fill=col)
                break
    return img


def rr(d, box, r, **kw):
    d.rounded_rectangle(box, radius=r, **kw)


def glow(base, box, r, color, blur):
    """Soft outward halo: the shape drawn on a layer, blurred, composited."""
    layer = Image.new("RGBA", base.size, (0, 0, 0, 0))
    ld = ImageDraw.Draw(layer)
    rr(ld, box, r, fill=color)
    layer = layer.filter(ImageFilter.GaussianBlur(blur))
    base.alpha_composite(layer)


def capsule(d, p1, p2, width, fill):
    """Round-capped stroke (the painter's capsule primitive)."""
    import math
    r = width // 2
    d.line([p1, p2], fill=fill, width=width)
    for p in (p1, p2):
        d.ellipse([p[0] - r, p[1] - r, p[0] + r, p[1] + r], fill=fill)


def chevron_up(d, cx, cy, col, sw):
    x0, y0 = cx - 14 * S, cy - 8 * S
    pts = [(x0 + 4 * S, y0 + 13 * S), (x0 + 14 * S, y0 + 4 * S), (x0 + 24 * S, y0 + 13 * S)]
    capsule(d, pts[0], pts[1], sw, col)
    capsule(d, pts[1], pts[2], sw, col)


def power_icon(d, cx, cy, col, sw):
    s = 24 * S / 24.0
    # stem (12,3)-(12,11)
    capsule(d, (cx + 12 * S * s / 2 - 6 * S, cy - 8 * S), (cx + 0, cy + 3 * S), sw, col)
    # arc: 269deg around center (12,12.41), gap at top
    import math
    ccx, ccy, r = cx, cy, 8 * S
    import math
    pts = []
    for i in range(37):
        ang = math.radians(224.6 - 269.2 * i / 36)
        pts.append((ccx + r * math.cos(ang), ccy + (r) * math.sin(ang) - 0.41 * S))
    for i in range(len(pts) - 1):
        capsule(d, pts[i], pts[i + 1], sw, col)


def draw_screen(state, sub_line, timer=None, stats=None, card=False, sheet=False, tall=True):
    img = vgrad(W, H, BG_STOPS[state]).convert("RGBA")

    # halftone dots (7px grid, alpha ramping in below 35% height)
    dots = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    dd = ImageDraw.Draw(dots)
    step, r = 7 * S, 1 * S
    for y in range(0, H, step):
        frac = y / H
        m = max(0.0, min(1.0, (frac - 0.35) / 0.65))
        a = int(0.075 * 255 * m)
        if a <= 0:
            continue
        for x in range(0, W, step):
            dd.ellipse([x - r, y - r, x + r, y + r], fill=(255, 255, 255, a))
    img.alpha_composite(dots)

    # 5% grain
    import random
    rnd = random.Random(0x9E3779B9)
    grain = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    gd = ImageDraw.Draw(grain)
    gs = 160 * S
    tile = Image.new("RGBA", (gs, gs), (0, 0, 0, 0))
    td = ImageDraw.Draw(tile)
    for gy in range(gs):
        for gx in range(gs):
            v = rnd.randrange(256)
            td.point((gx, gy), fill=(v, v, v, 13))
    for ty in range(0, H, gs):
        for tx in range(0, W, gs):
            grain.alpha_composite(tile, (tx, ty))
    img.alpha_composite(grain)

    d = ImageDraw.Draw(img)

    # ---- the lever ----
    # aura (behind everything)
    if AURA[state]:
        glow(img, [TRACK[0] - 32 * S, TRACK[1] - 32 * S, TRACK[2] + 32 * S, TRACK[3] + 32 * S], 66 * S, AURA[state], 22 * S)
    # the track
    d.rounded_rectangle(TRACK, radius=34 * S, fill=(20, 20, 24, 160))
    inset = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    idd = ImageDraw.Draw(inset)
    rr(idd, [TRACK[0] + S, TRACK[1] + S, TRACK[2] - S, TRACK[1] + 26 * S], 33 * S, fill=(0, 0, 0, 140))
    inset = inset.filter(ImageFilter.GaussianBlur(6 * S))
    img.alpha_composite(inset)
    d = ImageDraw.Draw(img)
    rr(d, [TRACK[0] + 1 * S, TRACK[1] + 1 * S, TRACK[2] - 1 * S, TRACK[3] - 1 * S], 33 * S, outline=(0, 0, 0, 70), width=S)

    # cue chevrons (idle/failed)
    if state in ("idle", "failed"):
        for i in range(3):
            a = int(0.45 * 255)
            col = (255, 255, 255, a)
            y = TRACK[1] + 30 * S + i * 18 * S + 8 * S
            chevron_up(d, 180 * S, y, col, max(2, int(2.6 * S)))

    # connected down-chevron
    if state == "connected":
        cx, cy = 180 * S, TRACK[3] - 28 * S
        col = (255, 255, 255, 217)
        s20 = 20 * S / 20.0
        for dy in (4, 10):
            p1 = (cx - 5 * s20, TRACK[3] - 28 * S + dy * s20 - 10 * S)
            p2 = (cx, TRACK[3] - 28 * S + (dy + 5) * s20 - 10 * S)
            p3 = (cx + 5 * s20, TRACK[3] - 28 * S + dy * s20 - 10 * S)
            capsule(d, p1, p2, max(2, int(2 * S)), col)
            capsule(d, p2, p3, max(2, int(2 * S)), col)

    # ---- the cap ----
    cap_x = TRACK[0] + 8 * S
    cap_y = TRACK[1] + 8 * S + (0 if state in ("connected", "connecting") else TRAVEL)
    cap_box = [cap_x, cap_y, cap_x + CAP_W, cap_y + CAP_H]
    # cap glow + cast shadow
    glow(img, [cap_box[0] - 26 * S, cap_box[1] - 26 * S, cap_box[2] + 26 * S, cap_box[3] + 26 * S], 52 * S, PILLGLOW[state], 18 * S)
    glow(img, [cap_box[0] - 8 * S, cap_box[1] + 6 * S, cap_box[2] + 8 * S, cap_box[3] + 26 * S], 36 * S, (0, 0, 0, 90), 9 * S)
    d = ImageDraw.Draw(img)
    # wall (9px) + seam
    rr(d, [cap_x, cap_y, cap_x + CAP_W, cap_y + CAP_H + 9 * S], 26 * S, fill=EDGE)
    rr(d, [cap_x, cap_y + CAP_H + 7 * S, cap_x + CAP_W, cap_y + CAP_H + 9 * S], 2 * S, fill=(0, 0, 0, 97))
    # face gradient
    face = vgrad(CAP_W, CAP_H, CAP_FACE).convert("RGBA")
    mask = Image.new("L", (CAP_W, CAP_H), 0)
    md = ImageDraw.Draw(mask)
    md.rounded_rectangle([0, 0, CAP_W - 1, CAP_H - 1], radius=26 * S, fill=255)
    img.paste(face, (cap_x, cap_y), mask)
    # sheen (top-center radial-ish glow), CLIPPED to the cap and
    # alpha-composited over the face. (The D116 fix: paste() REPLACES
    # pixels — the earlier paste of a mostly-transparent layer through
    # the cap mask wiped the satin-red face to transparent black, and
    # the preview's button read dark instead of red.)
    sheen = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    sd = ImageDraw.Draw(sheen)
    sd.ellipse([cap_x + 6 * S, cap_y - 10 * S, cap_x + CAP_W - 6 * S, cap_y + 48 * S], fill=(255, 255, 255, 50))
    sheen = sheen.filter(ImageFilter.GaussianBlur(10 * S))
    shmask = Image.new("L", (W, H), 0)
    sm = ImageDraw.Draw(shmask)
    sm.rounded_rectangle(cap_box, radius=26 * S, fill=255)
    img.alpha_composite(Image.composite(sheen, Image.new("RGBA", (W, H), (0, 0, 0, 0)), shmask))
    d = ImageDraw.Draw(img)
    # top bevel + bottom shade
    rr(d, [cap_x + 2 * S, cap_y + 1 * S, cap_x + CAP_W - 2 * S, cap_y + 3 * S], 2 * S, fill=(255, 255, 255, 115))
    shade = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    sd2 = ImageDraw.Draw(shade)
    rr(sd2, [cap_x + 2 * S, cap_y + CAP_H - 8 * S, cap_x + CAP_W - 2 * S, cap_y + CAP_H - 2 * S], 8 * S, fill=(80, 0, 0, 77))
    shade = shade.filter(ImageFilter.GaussianBlur(2 * S))
    # same D116 fix: composite, never paste, over the face
    img.alpha_composite(Image.composite(shade, Image.new("RGBA", (W, H), (0, 0, 0, 0)), shmask))
    d = ImageDraw.Draw(img)

    # LED (24x4, r2) + glow
    lw, lh = 24 * S, 4 * S
    lx, ly = cap_x + (CAP_W - lw) // 2, cap_y + 13 * S
    glow(img, [lx - 6 * S, ly - 6 * S, lx + lw + 6 * S, ly + lh + 6 * S], 8 * S,
         (*LED[state], 180), 4 * S)
    d = ImageDraw.Draw(img)
    rr(d, [lx, ly, lx + lw, ly + lh], lh // 2, fill=LED[state])
    # grip (3 ridges at y=30)
    gx = cap_x + (CAP_W - 34 * S) // 2
    for i in range(3):
        gy = cap_y + 30 * S + i * 6 * S
        rr(d, [gx, gy, gx + 34 * S, gy + 2 * S], 1 * S, fill=(70, 0, 0, 102))
        rr(d, [gx, gy + 2 * S, gx + 34 * S, gy + 3 * S], 1 * S, fill=(255, 255, 255, 46))
    # label (18/700, +2.2px letterspacing) at y=53
    label = {"idle": "START", "connecting": "WAIT", "connected": "STOP", "failed": "RETRY"}[state]
    fcap = F(18, 700)
    sp = 2.2 * S
    widths = []
    for ch in label:
        bb = d.textbbox((0, 0), ch, font=fcap)
        widths.append(bb[2] - bb[0])
    total = sum(widths) + sp * (len(label) - 1)
    x = cap_x + (CAP_W - total) / 2
    for ch, wch in zip(label, widths):
        bb = d.textbbox((0, 0), ch, font=fcap)
        th = bb[3] - bb[1]
        d.text((x - bb[0], cap_y + 53 * S + (22 * S - th) / 2 + S), ch, font=fcap, fill=(0x8f, 0x1a, 0x16))
        d.text((x - bb[0], cap_y + 53 * S + (22 * S - th) / 2), ch, font=fcap, fill=TEXT)
        x += wch + sp
    # power icon at center y=100
    power_icon(d, cap_x + CAP_W // 2, cap_y + 100 * S, TEXT, max(2, int(1.9 * S)))

    # ---- header ----
    mk = [16 * S, 24 * S, 50 * S, 48 * S]
    glow(img, [mk[0] - 7 * S, mk[1] - 7 * S, mk[2] + 7 * S, mk[3] + 7 * S], 14 * S, (255, 40, 30, 140), 6 * S)
    d = ImageDraw.Draw(img)
    tile = vgrad(34 * S, 24 * S, [(0xf0, 0x39, 0x2c, 0), (0xc3, 0x18, 0x0f, 1)]).convert("RGBA")
    tmask = Image.new("L", (34 * S, 24 * S), 0)
    tm = ImageDraw.Draw(tmask)
    tm.rounded_rectangle([0, 0, 34 * S - 1, 24 * S - 1], radius=7 * S, fill=255)
    img.paste(tile, (mk[0], mk[1]), tmask)
    d = ImageDraw.Draw(img)
    rr(d, [mk[0] + S, mk[1] + S, mk[2] - S, mk[1] + 3 * S], 2 * S, fill=(255, 255, 255, 89))
    d.polygon([(mk[0] + 13 * S, mk[1] + 7 * S), (mk[0] + 13 * S, mk[1] + 17 * S), (mk[0] + 22 * S, mk[1] + 12 * S)], fill=(255, 255, 255))
    fbrand = F(17, 600)
    bb = d.textbbox((0, 0), "Yg", font=fbrand)
    d.text((60 * S, 36 * S - (bb[3] - bb[1]) / 2 - bb[1]), "Younblock", font=fbrand, fill=TEXT)
    # the "..." glass circle
    mb = [296 * S, 12 * S, 344 * S, 60 * S]
    glow(img, [mb[0] - 4 * S, mb[1] + 2 * S, mb[2] + 4 * S, mb[3] + 8 * S], 30 * S, (0, 0, 0, 89), 5 * S)
    circ = vgrad(48 * S, 48 * S, [(255, 255, 255, 0), (255, 255, 255, 1)]).convert("RGBA")
    cmask = Image.new("L", (48 * S, 48 * S), 0)
    cm = ImageDraw.Draw(cmask)
    cm.ellipse([0, 0, 48 * S - 1, 48 * S - 1], fill=255)
    img.paste(circ, (mb[0], mb[1]), cmask)
    d = ImageDraw.Draw(img)
    for dx in (-9, 0, 9):
        d.ellipse([180 * S + dx * S - 2 * S, 36 * S - 2 * S, 180 * S + dx * S + 2 * S, 36 * S + 2 * S], fill=(255, 255, 255))

    # ---- the status column ----
    show_ghost = state != "connected"
    show_timer = state == "connected"
    show_stats = state == "connected"
    subcol = SUB[state]

    stack = 0
    gh = 66 * S if show_ghost else 0
    stack += gh + (12 * S if show_ghost else 0)
    stack += 21 * S + 4 * S + 38 * S + 25 * S
    if show_timer:
        stack += 6 * S + 21 * S
    if show_stats:
        stack += 6 * S + 22 * S
    top = 72 * S + ((TRACK[1] - 4 * S) - 72 * S - stack) / 2
    y = top

    if show_ghost:
        gx0 = (W - 92 * S) / 2
        rr(d, [gx0, y, gx0 + 92 * S, y + 66 * S], 20 * S, fill=(255, 255, 255, 18))
        rr(d, [gx0 + 2 * S, y + S, gx0 + 90 * S, y + 3 * S], 2 * S, fill=(255, 255, 255, 26))
        tcol = (255, 190, 90, 77) if state == "failed" else (255, 255, 255, 31)
        d.polygon([(gx0 + 37 * S, y + 19 * S), (gx0 + 37 * S, y + 47 * S), (gx0 + 61 * S, y + 33 * S)], fill=tcol)
        y += gh + 12 * S

    feyeb = F(15)
    bb = d.textbbox((0, 0), "YouTube", font=feyeb)
    d.text(((W - (bb[2] - bb[0])) / 2, y + (21 * S - (bb[3] - bb[1])) / 2 - bb[1]), "YouTube", font=feyeb,
           fill=tuple(int(c * 0.85) for c in subcol))
    y += 21 * S + 4 * S

    headline = {"idle": "Not connected", "connecting": "Connecting...", "connected": "Connected", "failed": "Couldn't connect"}[state]
    fhead = F(32, 700)
    bb = d.textbbox((0, 0), headline, font=fhead)
    d.text(((W - (bb[2] - bb[0])) / 2, y + (38 * S - (bb[3] - bb[1])) / 2 - bb[1]), headline, font=fhead, fill=TEXT)
    y += 38 * S

    if sub_line:
        fsub = F(15)
        amber = state == "failed"
        iconw = (18 * S + 6 * S) if amber else 0
        bb = d.textbbox((0, 0), sub_line, font=fsub)
        tw = bb[2] - bb[0]
        left = (W - tw - iconw) / 2
        if amber:
            # the warning icon (triangle outline + bang)
            icx, icy = left + 9 * S, y + 12 * S
            s18 = 18 * S / 24.0
            pts = [(icx - 12 * s18 + 10.29 * s18, icy - 12 * s18 + 3.86 * s18),
                   (icx - 12 * s18 + 1.82 * s18, icy - 12 * s18 + 18 * s18),
                   (icx - 12 * s18 + 3.53 * s18, icy - 12 * s18 + 20.73 * s18),
                   (icx - 12 * s18 + 20.47 * s18, icy - 12 * s18 + 20.73 * s18),
                   (icx - 12 * s18 + 22.18 * s18, icy - 12 * s18 + 18 * s18),
                   (icx - 12 * s18 + 13.71 * s18, icy - 12 * s18 + 3.86 * s18)]
            col = subcol
            swd = max(2, int(2 * S))
            for i in range(len(pts) - 1):
                capsule(d, pts[i], pts[i + 1], swd, col)
            capsule(d, pts[-1], pts[0], swd, col)
            capsule(d, (icx, icy - 12 * s18 + 9 * s18), (icx, icy - 12 * s18 + 13 * s18), swd, col)
            d.ellipse([icx - swd / 2, icy - 12 * s18 + 16.5 * s18 - swd / 2, icx + swd / 2, icy - 12 * s18 + 16.5 * s18 + swd / 2], fill=col)
        d.text((left + iconw, y + (25 * S - (bb[3] - bb[1])) / 2 - bb[1]), sub_line, font=fsub, fill=subcol)
    y += 25 * S

    if show_timer and timer:
        ftimer = F(16)
        digitw, colonw = 13 * S, 6 * S
        total = sum(digitw if c != ":" else colonw for c in timer)
        x = (W - total) / 2
        for c in timer:
            bw = colonw if c == ":" else digitw
            col = tuple(int(0.72 * c2) for c2 in TEXT) if c == ":" else TEXT
            bb = d.textbbox((0, 0), c, font=ftimer)
            d.text((x + (bw - (bb[2] - bb[0])) / 2 - bb[0], y + (21 * S - (bb[3] - bb[1])) / 2 - bb[1]), c, font=ftimer, fill=col)
            x += bw
        y += 6 * S + 21 * S

    if show_stats and stats:
        fstats = F(15)
        down, up = stats
        bd = d.textbbox((0, 0), down, font=fstats)
        bu = d.textbbox((0, 0), up, font=fstats)
        dw, uw = bd[2] - bd[0], bu[2] - bu[0]
        arroww, gap, ggap = 14 * S, 6 * S, 18 * S
        total = (arroww + gap) * 2 + dw + uw + ggap
        x = (W - total) / 2
        cy = y + 11 * S
        swd = max(2, int(2 * S))
        s14 = 14 * S / 24.0
        # down arrow
        capsule(d, (x + 7 * S, cy - 7 * S + 5 * s14), (x + 7 * S, cy - 7 * S + 19 * s14), swd, subcol)
        capsule(d, (x + 7 * S - 7 * s14, cy - 7 * S + 12 * s14), (x + 7 * S, cy - 7 * S + 19 * s14), swd, subcol)
        capsule(d, (x + 7 * S, cy - 7 * S + 19 * s14), (x + 7 * S + 7 * s14, cy - 7 * S + 12 * s14), swd, subcol)
        x += arroww + gap
        d.text((x, y + (22 * S - (bd[3] - bd[1])) / 2 - bd[1]), down, font=fstats, fill=subcol)
        x += dw + ggap
        # up arrow
        capsule(d, (x + 7 * S, cy - 7 * S + 19 * s14), (x + 7 * S, cy - 7 * S + 5 * s14), swd, subcol)
        capsule(d, (x + 7 * S - 7 * s14, cy - 7 * S + 12 * s14), (x + 7 * S, cy - 7 * S + 5 * s14), swd, subcol)
        capsule(d, (x + 7 * S, cy - 7 * S + 5 * s14), (x + 7 * S + 7 * s14, cy - 7 * S + 12 * s14), swd, subcol)
        x += arroww + gap
        d.text((x, y + (22 * S - (bu[3] - bu[1])) / 2 - bu[1]), up, font=fstats, fill=subcol)

    # ---- hint ----
    fhint = F(14)
    hint = {"idle": "Tap or swipe up to connect", "connecting": "Almost there",
            "connected": "Tap or swipe down to disconnect", "failed": "Tap or swipe up to retry"}[state]
    bb = d.textbbox((0, 0), hint, font=fhint)
    d.text(((W - (bb[2] - bb[0])) / 2, 530 * S + (20 * S - (bb[3] - bb[1])) / 2 - bb[1]), hint, font=fhint,
           fill=tuple(int(0.9 * c) for c in HINT))

    # ---- the card ----
    if card:
        cb = [16 * S, 560 * S, 344 * S, 624 * S]
        glow(img, [cb[0] - 5 * S, cb[1] + 2 * S, cb[2] + 5 * S, cb[3] + 10 * S], 27 * S, (0, 0, 0, 77), 6 * S)
        cardg = vgrad(W, 64 * S, [(255, 255, 255, 0.12), (255, 255, 255, 0.05)]).convert("RGBA")
        cm2 = Image.new("L", (W, 64 * S), 0)
        cm2d = ImageDraw.Draw(cm2)
        cm2d.rounded_rectangle([16 * S, 0, 344 * S, 64 * S - 1], radius=22 * S, fill=255)
        img.paste(cardg, (0, 560 * S), cm2)
        d = ImageDraw.Draw(img)
        rr(d, [16 * S + 2 * S, 560 * S + S, 344 * S - 2 * S, 560 * S + 3 * S], 2 * S, fill=(255, 255, 255, 56))
        # red play tile 48x40 r11
        ib = [30 * S, 576 * S, 78 * S, 616 * S]
        itile = vgrad(48 * S, 40 * S, [(0xf0, 0x39, 0x2c, 0), (0xc3, 0x18, 0x0f, 1)]).convert("RGBA")
        imask = Image.new("L", (48 * S, 40 * S), 0)
        im = ImageDraw.Draw(imask)
        im.rounded_rectangle([0, 0, 48 * S - 1, 40 * S - 1], radius=11 * S, fill=255)
        img.paste(itile, (ib[0], ib[1]), imask)
        d = ImageDraw.Draw(img)
        rr(d, [ib[0] + S, ib[1] + S, ib[2] - S, ib[1] + 3 * S], 2 * S, fill=(255, 255, 255, 89))
        d.polygon([(ib[0] + 19 * S, ib[1] + 11 * S), (ib[0] + 19 * S, ib[1] + 29 * S), (ib[0] + 32 * S, ib[1] + 20 * S)], fill=(255, 255, 255))
        ft = F(17, 600)
        bb = d.textbbox((0, 0), "Open YouTube", font=ft)
        d.text((92 * S, 578 * S), "Open YouTube", font=ft, fill=TEXT)
        fs2 = F(14)
        d.text((92 * S, 598 * S), "Launch the YouTube app", font=fs2, fill=SUB[state])

    # ---- the sheet ----
    if sheet:
        ov = Image.new("RGBA", (W, H), (0, 0, 0, 0))
        od = ImageDraw.Draw(ov)
        od.rectangle([0, 0, W, H], fill=(0, 0, 0, 128))
        img.alpha_composite(ov)
        d = ImageDraw.Draw(img)
        sh = [0, H - 368 * S, W, H]
        glow(img, [8 * S, sh[1] - 8 * S, W - 8 * S, sh[1] + 8 * S], 24 * S, (0, 0, 0, 90), 7 * S)
        d = ImageDraw.Draw(img)
        rr(d, [sh[0], sh[1], sh[2], H + 30 * S], 24 * S, fill=SHEET_BG)
        rr(d, [2 * S, sh[1] + S, W - 2 * S, sh[1] + 3 * S], 2 * S, fill=(255, 255, 255, 18))
        rr(d, [(W - 40 * S) / 2, sh[1] + 10 * S, (W + 40 * S) / 2, sh[1] + 14 * S], 2 * S, fill=(255, 255, 255, 46))
        # head
        fst = F(15, 600)
        d.text((22 * S, sh[1] + 24 * S + 13 * S), "Younblock", font=fst, fill=TEXT)
        xb = [W - 66 * S, sh[1] + 24 * S, W - 22 * S, sh[1] + 24 * S + 44 * S]
        d.ellipse(xb, fill=(255, 255, 255, 20), outline=(255, 255, 255, 36), width=S)
        xcx, xcy = (xb[0] + xb[2]) / 2, (xb[1] + xb[3]) / 2
        swd = max(2, int(2 * S))
        capsule(d, (xcx - 5 * S, xcy - 5 * S), (xcx + 5 * S, xcy + 5 * S), swd, (255, 255, 255, 199))
        capsule(d, (xcx + 5 * S, xcy - 5 * S), (xcx - 5 * S, xcy + 5 * S), swd, (255, 255, 255, 199))
        # rows
        ry = sh[1] + 74 * S
        rows = [
            ("action", "Open YouTube", "Open the site in your browser"),
            ("toggle", "Connect on launch", "Start connecting when this window opens", False),
            ("toggle", "Open YouTube when connected", "Open the browser as soon as the tunnel is up", True),
            ("exit", "Exit Younblock", None),
        ]
        frt, frh = F(13, 600), F(12)
        for kind, title, hintx in [(r[0], r[1], r[2] if len(r) > 2 else None) for r in rows]:
            on = rows[[r[0] for r in rows].index(kind)][2] if kind == "toggle" and len(rows[[r[0] for r in rows].index(kind)]) > 2 else None
            on = None
            if kind == "toggle":
                idx = [r[1] for r in rows].index(title)
                on = rows[idx][3] if len(rows[idx]) > 3 else False
            if kind in ("action", "exit"):
                rh = 56 * S if kind == "action" else 44 * S
                if kind == "action":
                    rr(d, [22 * S, ry, W - 22 * S, ry + rh], 12 * S, fill=SHEET_RAISED, outline=(255, 255, 255, 36), width=S)
                else:
                    rr(d, [22 * S, ry, W - 22 * S, ry + rh], 12 * S, fill=(255, 255, 255, 15))
                ic = 36 * S
                icx, icy = 22 * S + 14 * S + ic / 2, ry + (rh - ic) / 2 + ic / 2
                if kind == "action":
                    rr(d, [icx - ic / 2, icy - ic / 2, icx + ic / 2, icy + ic / 2], 10 * S, fill=(255, 0, 0, 36), outline=(255, 0, 0, 64), width=S)
                    d.polygon([(icx - 5 * S, icy - 7 * S), (icx - 5 * S, icy + 7 * S), (icx + 8 * S, icy)], fill=RED400)
                else:
                    rr(d, [icx - ic / 2, icy - ic / 2, icx + ic / 2, icy + ic / 2], 10 * S, fill=(255, 255, 255, 20))
                    power_icon(d, icx, icy, (255, 255, 255, 158), max(2, int(1.9 * S)))
                bb = d.textbbox((0, 0), title, font=frt)
                d.text((22 * S + 14 * S + ic + 12 * S, ry + (rh - (bb[3] - bb[1])) / 2 - bb[1] if kind == "exit" else ry + 14 * S), title, font=frt,
                       fill=TEXT if kind == "action" else (255, 255, 255, 158))
                if hintx:
                    d.text((22 * S + 14 * S + ic + 12 * S, ry + 32 * S), hintx, font=frh, fill=(255, 255, 255, 158))
                if kind == "action":
                    ccx, ccy = W - 18 * S - 10 * S, ry + rh / 2
                    swd = max(2, int(2 * S))
                    capsule(d, (ccx - 3 * S, ccy - 6 * S), (ccx + 3 * S, ccy), swd, (255, 255, 255, 98))
                    capsule(d, (ccx + 3 * S, ccy - 6 * S), (ccx - 3 * S, ccy), swd, (255, 255, 255, 98))
                    capsule(d, (ccx - 3 * S, ccy), (ccx + 3 * S, ccy + 6 * S), swd, (255, 255, 255, 98))
                    capsule(d, (ccx + 3 * S, ccy), (ccx - 3 * S, ccy + 6 * S), swd, (255, 255, 255, 98))
            else:
                rh = 52 * S
                bb = d.textbbox((0, 0), title, font=frt)
                d.text((26 * S, ry + 8 * S), title, font=frt, fill=(255, 255, 255, 199))
                d.text((26 * S, ry + 28 * S), hintx, font=frh, fill=(255, 255, 255, 158))
                tw_, th_ = 44 * S, 26 * S
                tx, ty = W - 26 * S - tw_, ry + (rh - th_) / 2
                rr(d, [tx, ty, tx + tw_, ty + th_], th_ / 2, fill=(255, 0, 0, 255) if on else (255, 255, 255, 46))
                kw = 20 * S
                kx = tx + 3 * S + (18 * S if on else 0)
                ky = ty + (th_ - kw) / 2
                d.ellipse([kx, ky, kx + kw, ky + kw], fill=(0xF4, 0xF1, 0xEE))
            ry += rh + 8 * S
        # footer
        d.text((22 * S, ry + 14 * S), "Younblock Lite  ·  1.6.12", font=frh, fill=(255, 255, 255, 199))
        d.text((22 * S, ry + 32 * S), "Created by ChitaCodes", font=frh, fill=(255, 255, 255, 158))

    return img


def main():
    out = sys.argv[1] if len(sys.argv) > 1 else os.path.join(
        os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
        "docs", "ui-v3", "screenshots", "lite-desktop-preview.png")
    panels = [
        draw_screen("idle", "Last session: 22 min, 15.8 MB"),
        draw_screen("connecting", "Setting things up"),
        draw_screen("connected", "YouTube unblocked.", timer="00:21:13", stats=("15.7 MB", "828 KB"), card=True),
        draw_screen("failed", "Could not start. Try again. (NO_TARGET_APPS)"),
        draw_screen("idle", "", sheet=True),
    ]
    gap = 24
    total_w = W * len(panels) + gap * (len(panels) - 1)
    sheet = Image.new("RGB", (total_w, H + 40), (12, 12, 16))
    for i, p in enumerate(panels):
        sheet.paste(p.convert("RGB"), (i * (W + gap), 40))
    d = ImageDraw.Draw(sheet)
    fl = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 20)
    labels = ["idle", "connecting", "connected", "failed", "menu sheet"]
    for i, lab in enumerate(labels):
        d.text((i * (W + gap) + 12, 8), "younblock-lite-desktop — the v3 screen, 360x640 — " + lab, font=fl, fill=(214, 210, 206))
    sheet.save(out)
    print("wrote", out, sheet.size)


if __name__ == "__main__":
    main()
