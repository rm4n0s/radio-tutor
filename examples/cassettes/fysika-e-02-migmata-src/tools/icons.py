"""Simple flat icons. Every function returns SVG markup centered on (cx, cy)."""
import math
from lib import rect, circle, path, line, text, INK


def drop(cx, cy, r=40, fill="#42a5f5", stroke="#1565c0"):
    d = ("M%s %s C%s %s %s %s %s %s A%s %s 0 0 1 %s %s C%s %s %s %s %s %s Z" % (
        cx, cy - 1.7 * r, cx + 0.5 * r, cy - 0.7 * r, cx + r, cy - 0.1 * r, cx + r, cy + 0.5 * r,
        r, r, cx - r, cy + 0.5 * r, cx - r, cy - 0.1 * r, cx - 0.5 * r, cy - 0.7 * r, cx, cy - 1.7 * r))
    return path(d, fill, stroke, 5) + path("M%s %s Q%s %s %s %s" % (cx - 0.55 * r, cy + 0.35 * r, cx - 0.55 * r, cy - 0.1 * r, cx - 0.2 * r, cy - 0.35 * r), "none", "#ffffff", 5, 'opacity="0.7"')


def window(cx, cy, s=1.0):
    w, h = 220 * s, 260 * s
    x, y = cx - w / 2, cy - h / 2
    pw, ph = (w - 48 * s) / 2, (h - 48 * s) / 2
    out = rect(x, y, w, h, "#cfd8dc", 10 * s, "#78909c", 8 * s)
    for i in range(2):
        for j in range(2):
            out += rect(x + 16 * s + i * (pw + 16 * s), y + 16 * s + j * (ph + 16 * s), pw, ph, "#bbdefb", 4, "#90a4ae", 4)
            out += line(x + 26 * s + i * (pw + 16 * s), y + 28 * s + j * (ph + 16 * s), x + 50 * s + i * (pw + 16 * s), y + 28 * s + j * (ph + 16 * s) + 26, "#ffffff", 5)
    return out


def coil(cx, cy, s=1.0):
    out = line(cx - 120 * s, cy + 40 * s, cx - 90 * s, cy + 40 * s, "#bf6a2c", 10)
    for i in range(6):
        x = cx - 90 * s + i * 34 * s
        out += '<ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="none" stroke="#d2782f" stroke-width="12"/>' % (x, cy, 26 * s, 60 * s)
    out += line(cx + 84 * s, cy + 40 * s, cx + 120 * s, cy + 40 * s, "#bf6a2c", 10)
    return out


def thermometer(cx, cy, s=1.0):
    return (rect(cx - 20 * s, cy - 130 * s, 40 * s, 230 * s, "#ffffff", 20 * s, "#78909c", 6) +
            circle(cx, cy + 100 * s, 36 * s, "#ffffff", "#78909c", 6) +
            circle(cx, cy + 100 * s, 26 * s, "#9e9e9e") +
            rect(cx - 8 * s, cy - 70 * s, 16 * s, 170 * s, "#9e9e9e", 8 * s) +
            "".join(line(cx + 30 * s, cy - 100 * s + k * 28 * s, cx + 52 * s, cy - 100 * s + k * 28 * s, "#78909c", 4) for k in range(6)))


def bowl(cx, cy, w=360, h=150, fill="#fb8c00", inner="#ffe0b2"):
    x0, x1 = cx - w / 2, cx + w / 2
    return ('<ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="%s"/>' % (cx, cy, w / 2, h * 0.22, inner) +
            path("M%s %s L%s %s A%s %s 0 0 1 %s %s Z" % (x0, cy, x1, cy, w / 2, h, x0, cy), fill) +
            '<ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="none" stroke="%s" stroke-width="8"/>' % (cx, cy, w / 2, h * 0.22, "#e65100"))


def bowl_back(cx, cy, w=360, h=150, inner="#ffe0b2"):
    return '<ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="%s"/>' % (cx, cy, w / 2, h * 0.22, inner)


def bowl_front(cx, cy, w=360, h=150, fill="#fb8c00"):
    x0, x1 = cx - w / 2, cx + w / 2
    return (path("M%s %s A%s %s 0 0 0 %s %s Z" % (x0, cy, w / 2, h * 0.22, x1, cy), "none") +
            path("M%s %s L%s %s A%s %s 0 0 1 %s %s Z" % (x0, cy, x1, cy, w / 2, h, x0, cy), fill, "#e65100", 6))


def cube(cx, cy, s=44, fill="#ffffff", stroke="#90a4ae"):
    d = s / 2
    return (rect(cx - d, cy - d, s, s, fill, 6, stroke, 5) +
            path("M%s %s L%s %s" % (cx - d + 8, cy - d + 10, cx - d + 20, cy - d + 10), "none", "#cfd8dc", 4))


def salt_shaker(cx, cy, s=1.0):
    return (path("M%s %s L%s %s L%s %s L%s %s Z" % (cx - 36 * s, cy + 80 * s, cx - 46 * s, cy - 20 * s, cx + 46 * s, cy - 20 * s, cx + 36 * s, cy + 80 * s), "#e1f5fe", "#4fc3f7", 6) +
            path("M%s %s Q%s %s %s %s Z" % (cx - 50 * s, cy - 20 * s, cx, cy - 100 * s, cx + 50 * s, cy - 20 * s), "#b0bec5", "#78909c", 6) +
            "".join(circle(cx - 14 * s + i * 14 * s, cy - 40 * s, 4, "#37474f") for i in range(3)) +
            rect(cx - 30 * s, cy + 20 * s, 60 * s, 30 * s, "#b3e5fc", 4))


def sugar_pile(cx, cy, s=1.0):
    return (path("M%s %s L%s %s L%s %s Z" % (cx - 70 * s, cy + 40 * s, cx, cy - 40 * s, cx + 70 * s, cy + 40 * s), "#fff8e1", "#bcaaa4", 5) +
            "".join(circle(cx + dx * s, cy + dy * s, 5, "#d7ccc8") for dx, dy in ((-10, 10), (20, 20), (-30, 28), (5, -10), (40, 30))))


def cup(cx, cy, fill="#6d4c41", s=1.0, steam=False, rim="#eceff1"):
    out = ""
    out += path("M%s %s a%s %s 0 0 1 0 %s" % (cx + 80 * s, cy - 20 * s, 40 * s, 40 * s, 80 * s), "none", rim, 14 * s)
    out += path("M%s %s L%s %s Q%s %s %s %s L%s %s Z" % (cx - 90 * s, cy - 50 * s, cx - 70 * s, cy + 50 * s, cx, cy + 90 * s, cx + 70 * s, cy + 50 * s, cx + 90 * s, cy - 50 * s), "#ffffff", "#b0bec5", 6)
    out += '<ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="%s"/>' % (cx, cy - 50 * s, 90 * s, 18 * s, fill)
    return out


def glass_cup(cx, base, w=200, h=260, liquid="#64b5f6", level=0.75, opacity=0.9):
    """Glass tapering to the bottom. Returns (back_and_liquid, front_outline)."""
    x0, x1 = cx - w / 2, cx + w / 2
    top = base - h
    ly = base - h * level
    k = 20.0

    def edge(y):
        f = (y - top) / h
        return x0 + k * f, x1 - k * f
    ax, bx = edge(ly)
    liq = "M%s %s L%s %s L%s %s L%s %s Z" % (ax, ly, x0 + k, base, x1 - k, base, bx, ly)
    out = path(liq, liquid, extra='opacity="%s"' % opacity)
    outline = path("M%s %s L%s %s L%s %s L%s %s" % (x0, top, x0 + k, base, x1 - k, base, x1, top), "none", "#78909c", 8)
    return out + outline


def glass_only(cx, base, w=200, h=260, liquid="#64b5f6", level=0.75, opacity=0.9):
    return glass_cup(cx, base, w, h, liquid, level, opacity)


def soda_bottle(cx, base, liquid="#ff9800"):
    t = base - 500
    d = ("M%s %s L%s %s C%s %s %s %s %s %s L%s %s Q%s %s %s %s L%s %s Q%s %s %s %s L%s %s "
         "C%s %s %s %s %s %s L%s %s Z" % (
             cx - 32, t, cx - 32, base - 430, cx - 32, base - 390, cx - 90, base - 400, cx - 90, base - 340,
             cx - 90, base - 24, cx - 90, base, cx - 66, base, cx + 66, base, cx + 90, base, cx + 90, base - 24,
             cx + 90, base - 340, cx + 90, base - 400, cx + 32, base - 390, cx + 32, base - 430, cx + 32, t))
    return (path(d, liquid, "#e65100", 6) + rect(cx - 36, t - 34, 72, 38, "#c62828", 8) +
            rect(cx - 90, base - 290, 180, 130, "#ffffff", 8, "#e0e0e0", 3) +
            circle(cx, base - 225, 32, "#fb8c00") + circle(cx, base - 225, 16, "#ffffff"))


def orange_slice(cx, cy, r=40):
    out = circle(cx, cy, r, "#fb8c00", "#e65100", 5) + circle(cx, cy, r - 8, "#ffcc80")
    for k in range(8):
        a = k * math.pi / 4
        out += line(cx, cy, cx + (r - 10) * math.cos(a), cy + (r - 10) * math.sin(a), "#fb8c00", 4)
    return out


def magnifier(cx, cy, r=170, fill="#e3f2fd"):
    """Lens ring and handle; returns (lens_fill, ring_and_handle)."""
    a = math.pi / 4
    hx, hy = cx + (r + 10) * math.cos(a), cy + (r + 10) * math.sin(a)
    back = circle(cx, cy, r, fill)
    front = (line(hx, hy, hx + 150 * math.cos(a), hy + 150 * math.sin(a), "#5d4037", 28) +
             circle(cx, cy, r, "none", "#455a64", 16))
    return back, front


def pebble(cx, cy, r=22, fill="#9e9e9e"):
    return '<ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="%s" stroke="#616161" stroke-width="3"/>' % (cx, cy, r, r * 0.8, fill)


def bean(cx, cy, s=1.0, fill="#8d6e63", rot=0):
    return '<ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="%s" transform="rotate(%s %s %s)"/>' % (cx, cy, 20 * s, 12 * s, fill, rot, cx, cy)


def fish(cx, cy, s=1.0, fill="#ff7043"):
    return (path("M%s %s Q%s %s %s %s Q%s %s %s %s Z" % (cx - 60 * s, cy, cx - 10 * s, cy - 50 * s, cx + 50 * s, cy, cx - 10 * s, cy + 50 * s, cx - 60 * s, cy), fill, "#bf360c", 5) +
            path("M%s %s L%s %s L%s %s Z" % (cx + 45 * s, cy, cx + 90 * s, cy - 34 * s, cx + 90 * s, cy + 34 * s), fill, "#bf360c", 5) +
            circle(cx - 30 * s, cy - 8 * s, 6 * s, "#ffffff") + circle(cx - 30 * s, cy - 8 * s, 3 * s, "#000000"))
