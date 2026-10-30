"""Simple flat icons, drawn around (0, 0). Each returns SVG markup; use g() to place one."""
import math
from lib import f

INK = "#37474f"
C_CHEM, C_ELEC, C_LIGHT, C_HEAT = "#43a047", "#1e88e5", "#f9a825", "#e53935"
C_NUC, C_KIN, C_POT = "#8e24aa", "#fb8c00", "#00897b"


def g(inner, x=0, y=0, s=1.0, rot=0):
    t = f"translate({f(x)} {f(y)})"
    if rot:
        t += f" rotate({f(rot)})"
    if s != 1:
        t += f" scale({s})"
    return f'<g transform="{t}">{inner}</g>'


def sun(r=60, rays=12, col="#fdd835"):
    s = ""
    for k in range(rays):
        a = 2 * math.pi * k / rays
        c, n = math.cos(a), math.sin(a)
        s += (f'<line x1="{f((r+16)*c)}" y1="{f((r+16)*n)}" x2="{f((r+46)*c)}" y2="{f((r+46)*n)}" '
              f'stroke="{col}" stroke-width="12" stroke-linecap="round"/>')
    s += f'<circle r="{r}" fill="{col}"/><circle r="{r-14}" fill="#fff176"/>'
    return s


def glow(r=110):
    return f'<circle r="{r}" fill="#fff59d" opacity="0.55"/><circle r="{r*0.7}" fill="#fff176" opacity="0.6"/>'


def bulb(on=True):
    s = ""
    fill = "#fff59d" if on else "#eceff1"
    s += f'<path d="M-30 40 C-70 10 -62 -62 0 -66 C62 -62 70 10 30 40 Z" fill="{fill}" stroke="{INK}" stroke-width="6"/>'
    s += f'<path d="M-14 40 L-14 6 L-6 -10 L6 -10 L14 6 L14 40" fill="none" stroke="{"#ef6c00" if on else "#90a4ae"}" stroke-width="5" stroke-linejoin="round"/>'
    s += f'<rect x="-30" y="40" width="60" height="16" rx="4" fill="#90a4ae" stroke="{INK}" stroke-width="5"/>'
    s += f'<rect x="-26" y="58" width="52" height="14" rx="4" fill="#78909c" stroke="{INK}" stroke-width="5"/>'
    s += f'<rect x="-14" y="74" width="28" height="12" rx="5" fill="{INK}"/>'
    return s


def battery(w=170, h=90):
    x = -w / 2
    s = f'<rect x="{f(x+w)}" y="{f(-h*0.22)}" width="18" height="{f(h*0.44)}" rx="4" fill="#90a4ae"/>'
    s += f'<rect x="{f(x)}" y="{f(-h/2)}" width="{w}" height="{h}" rx="14" fill="#455a64"/>'
    s += f'<rect x="{f(x+10)}" y="{f(-h/2+10)}" width="{f(w*0.36)}" height="{h-20}" rx="8" fill="#43a047"/>'
    s += f'<rect x="{f(x+w*0.36+14)}" y="{f(-h/2+10)}" width="{f(w*0.26)}" height="{h-20}" rx="8" fill="#66bb6a"/>'
    s += f'<text x="{f(x+w*0.82)}" y="{f(h*0.2)}" font-family="sans-serif" font-size="{f(h*0.6)}" font-weight="bold" fill="#fff" text-anchor="middle">+</text>'
    return s


def spring(length=220, coils=7, amp=34, horizontal=True, col="#607d8b", sw=9):
    pts = []
    n = coils * 2
    for i in range(n + 1):
        p = -length / 2 + length * i / n
        a = 0 if i in (0, n) else (amp if i % 2 else -amp)
        pts.append((p, a) if horizontal else (a, p))
    d = "M" + " L".join(f"{f(x)} {f(y)}" for x, y in pts)
    return f'<path d="{d}" fill="none" stroke="{col}" stroke-width="{sw}" stroke-linejoin="round" stroke-linecap="round"/>'


def ball(r=40, col="#fb8c00"):
    return (f'<circle r="{r}" fill="{col}"/><path d="M{-r*0.7} {-r*0.3} Q0 {-r*0.9} {r*0.7} {-r*0.3}" '
            f'fill="none" stroke="#fff" stroke-width="5" opacity="0.7"/>')


def paper_ball(r=34):
    return (f'<circle r="{r}" fill="#f5f5f5" stroke="#9e9e9e" stroke-width="5"/>'
            f'<path d="M-18 -10 L-4 4 L-16 14 M6 -22 L12 -6 L24 -2 M2 18 L14 24" fill="none" stroke="#bdbdbd" stroke-width="4"/>')


def speed_lines(n=3, length=80, col="#ffcc80", gap=26):
    return "".join(f'<line x1="{-length}" y1="{(i-(n-1)/2)*gap}" x2="{-length*0.35}" y2="{(i-(n-1)/2)*gap}" '
                   f'stroke="{col}" stroke-width="9" stroke-linecap="round"/>' for i in range(n))


def flame():
    return ('<path d="M0 -90 C30 -50 60 -30 55 10 C50 50 20 60 0 60 C-20 60 -52 50 -55 10 C-58 -20 -30 -30 -20 -70 '
            'C-10 -55 -5 -70 0 -90 Z" fill="#ef6c00"/>'
            '<path d="M0 -40 C18 -15 34 -5 30 22 C27 44 12 50 0 50 C-12 50 -29 44 -30 22 C-32 2 -12 -10 0 -40 Z" fill="#fdd835"/>')


def logs():
    s = ""
    for (x, y) in [(-50, 20), (50, 20), (0, -34)]:
        s += (f'<rect x="{x-80}" y="{y-26}" width="160" height="52" rx="26" fill="#8d6e63"/>'
              f'<circle cx="{x+80-26}" cy="{y}" r="22" fill="#d7b899"/><circle cx="{x+80-26}" cy="{y}" r="10" fill="#a1887f"/>')
    return s


def barrel(col="#455a64"):
    s = f'<rect x="-60" y="-80" width="120" height="160" rx="16" fill="{col}"/>'
    s += '<rect x="-60" y="-50" width="120" height="14" fill="#263238"/><rect x="-60" y="36" width="120" height="14" fill="#263238"/>'
    s += '<ellipse cx="0" cy="-80" rx="60" ry="14" fill="#607d8b"/>'
    s += '<path d="M-14 -10 C-14 -30 14 -30 14 -10 C14 6 -14 6 -14 -10 Z" fill="#fdd835" transform="translate(0 10)"/>'
    return s


def coal():
    return ('<path d="M-110 70 L-70 -10 L-30 -50 L20 -20 L50 -60 L100 30 L120 70 Z" fill="#37474f"/>'
            '<path d="M-60 40 L-30 0 L0 30 Z M30 40 L60 0 L90 40 Z" fill="#546e7a"/>')


def pylon():
    return ('<path d="M0 -150 L-50 100 M0 -150 L50 100 M-30 -40 L30 -40 M-40 10 L40 10 M-20 -90 L20 -90 '
            'M-30 -40 L40 10 M30 -40 L-40 10 M-100 -100 L100 -100 M-84 -100 L-84 -70 M84 -100 L84 -70" '
            f'fill="none" stroke="{INK}" stroke-width="8" stroke-linecap="round"/>')


def bolt(col="#fdd835"):
    return f'<path d="M20 -90 L-40 10 L-4 10 L-20 90 L44 -22 L6 -22 Z" fill="{col}" stroke="#f9a825" stroke-width="6" stroke-linejoin="round"/>'


def atom(col=C_NUC):
    s = ""
    for r in (0, 60, 120):
        s += f'<ellipse rx="95" ry="34" fill="none" stroke="{col}" stroke-width="7" transform="rotate({r})"/>'
    s += f'<circle r="20" fill="{col}"/><circle cx="-84" cy="-18" r="9" fill="#ce93d8"/><circle cx="48" cy="-80" r="9" fill="#ce93d8"/><circle cx="44" cy="76" r="9" fill="#ce93d8"/>'
    return s


def apple():
    return ('<circle cx="-18" cy="6" r="42" fill="#e53935"/><circle cx="18" cy="6" r="42" fill="#e53935"/>'
            '<path d="M0 -30 Q4 -52 22 -58" fill="none" stroke="#5d4037" stroke-width="7" stroke-linecap="round"/>'
            '<path d="M6 -42 Q30 -66 50 -46 Q30 -28 6 -42 Z" fill="#43a047"/>')


def bread():
    return ('<path d="M-90 20 C-110 -40 -50 -70 0 -70 C50 -70 110 -40 90 20 L90 50 L-90 50 Z" fill="#d7993b"/>'
            '<path d="M-50 -48 L-34 -10 M-8 -62 L8 -22 M34 -50 L50 -12" stroke="#a8691a" stroke-width="9" stroke-linecap="round"/>')


def cheese():
    return ('<path d="M-100 40 L100 40 L100 -10 L-100 -70 Z" fill="#fdd835"/><path d="M-100 40 L100 40 L100 60 L-100 60 Z" fill="#f9a825"/>'
            '<circle cx="-40" cy="10" r="14" fill="#f9a825"/><circle cx="30" cy="0" r="10" fill="#f9a825"/><circle cx="62" cy="26" r="8" fill="#f9a825"/>')


def oil_bottle():
    return ('<path d="M-26 -90 L26 -90 L26 -54 C60 -30 60 20 56 60 L-56 60 C-60 20 -60 -30 -26 -54 Z" fill="#fdd835" opacity="0.95"/>'
            '<rect x="-20" y="-110" width="40" height="26" rx="6" fill="#8d6e63"/><rect x="-36" y="-10" width="72" height="44" rx="6" fill="#fff"/>')


def chocolate():
    s = '<rect x="-80" y="-60" width="160" height="120" rx="10" fill="#5d4037"/>'
    for i in range(3):
        for j in range(2):
            s += f'<rect x="{-72+i*50}" y="{-52+j*58}" width="44" height="50" rx="6" fill="#795548"/>'
    return s


def salad():
    return ('<path d="M-90 0 L90 0 C80 60 40 80 0 80 C-40 80 -80 60 -90 0 Z" fill="#eceff1" stroke="#90a4ae" stroke-width="6"/>'
            '<circle cx="-40" cy="-6" r="36" fill="#66bb6a"/><circle cx="10" cy="-20" r="40" fill="#43a047"/><circle cx="52" cy="-4" r="32" fill="#81c784"/>'
            '<circle cx="-4" cy="-6" r="12" fill="#e53935"/>')


def rice():
    return ('<path d="M-90 -6 L90 -6 C80 54 40 76 0 76 C-40 76 -80 54 -90 -6 Z" fill="#eceff1" stroke="#90a4ae" stroke-width="6"/>'
            '<path d="M-84 -6 C-60 -60 60 -60 84 -6 Z" fill="#fafafa" stroke="#cfd8dc" stroke-width="4"/>')


def milk():
    return ('<path d="M-34 -80 L34 -80 L50 -50 L50 70 L-50 70 L-50 -50 Z" fill="#fff" stroke="#90a4ae" stroke-width="6"/>'
            '<rect x="-50" y="-20" width="100" height="50" fill="#90caf9"/>')


def car(col="#e53935"):
    return (f'<path d="M-130 30 L-130 -10 L-90 -20 L-50 -70 L50 -70 L90 -20 L130 -10 L130 30 Z" fill="{col}"/>'
            '<path d="M-42 -60 L-72 -22 L-6 -22 L-6 -60 Z M6 -60 L6 -22 L74 -22 L44 -60 Z" fill="#bbdefb"/>'
            '<circle cx="-70" cy="34" r="30" fill="#263238"/><circle cx="70" cy="34" r="30" fill="#263238"/>'
            '<circle cx="-70" cy="34" r="12" fill="#b0bec5"/><circle cx="70" cy="34" r="12" fill="#b0bec5"/>')


def truck():
    return ('<rect x="-160" y="-100" width="200" height="120" rx="8" fill="#1e88e5"/>'
            '<path d="M50 -60 L110 -60 L150 -10 L150 20 L50 20 Z" fill="#fb8c00"/><path d="M62 -50 L102 -50 L130 -12 L62 -12 Z" fill="#bbdefb"/>'
            '<circle cx="-100" cy="32" r="30" fill="#263238"/><circle cx="-30" cy="32" r="30" fill="#263238"/><circle cx="100" cy="32" r="30" fill="#263238"/>'
            '<circle cx="-100" cy="32" r="12" fill="#b0bec5"/><circle cx="-30" cy="32" r="12" fill="#b0bec5"/><circle cx="100" cy="32" r="12" fill="#b0bec5"/>')


def fan_blades(col="#29b6f6"):
    s = ""
    for r in (0, 120, 240):
        s += f'<ellipse cx="0" cy="-52" rx="26" ry="54" fill="{col}" transform="rotate({r})"/>'
    s += '<circle r="16" fill="#455a64"/>'
    return s


def fan_stand():
    return ('<rect x="-12" y="40" width="24" height="90" fill="#78909c"/><rect x="-70" y="124" width="140" height="22" rx="10" fill="#546e7a"/>'
            '<circle r="96" fill="none" stroke="#90a4ae" stroke-width="7"/>')


def person(shirt="#1e88e5", skin="#ffcc80", pose="stand", tired=False):
    """A child, about 260 tall, feet at y=130, head near y=-130."""
    s = ""
    if pose == "run":
        s += f'<path d="M-6 20 L-60 70 L-86 128" fill="none" stroke="#37474f" stroke-width="22" stroke-linecap="round" stroke-linejoin="round"/>'
        s += f'<path d="M6 20 L46 78 L80 90" fill="none" stroke="#37474f" stroke-width="22" stroke-linecap="round" stroke-linejoin="round"/>'
        s += f'<path d="M0 -50 L-56 -10 M0 -50 L54 -30" stroke="{skin}" stroke-width="18" stroke-linecap="round"/>'
        s += f'<rect x="-30" y="-70" width="60" height="100" rx="24" fill="{shirt}" transform="rotate(12)"/>'
        s += f'<circle cx="14" cy="-100" r="36" fill="{skin}"/><path d="M-18 -112 Q14 -150 48 -112 Q14 -128 -18 -112 Z" fill="#5d4037"/>'
        return s
    s += '<rect x="-26" y="30" width="22" height="98" rx="10" fill="#37474f"/><rect x="4" y="30" width="22" height="98" rx="10" fill="#37474f"/>'
    s += f'<rect x="-34" y="-70" width="68" height="108" rx="26" fill="{shirt}"/>'
    if tired:
        s += f'<path d="M-30 -40 L-62 40 M30 -40 L62 40" stroke="{skin}" stroke-width="18" stroke-linecap="round"/>'
        s += f'<circle cx="0" cy="-96" r="38" fill="{skin}"/><path d="M-36 -104 Q0 -150 36 -104 Q0 -118 -36 -104 Z" fill="#5d4037"/>'
        s += '<path d="M-20 -88 L-8 -88 M8 -88 L20 -88" stroke="#37474f" stroke-width="5"/><path d="M-12 -70 Q0 -78 12 -70" fill="none" stroke="#37474f" stroke-width="5"/>'
    else:
        s += f'<path d="M-30 -40 L-58 10 M30 -40 L58 10" stroke="{skin}" stroke-width="18" stroke-linecap="round"/>'
        s += f'<circle cx="0" cy="-96" r="38" fill="{skin}"/><path d="M-36 -104 Q0 -150 36 -104 Q0 -118 -36 -104 Z" fill="#5d4037"/>'
        s += '<circle cx="-12" cy="-96" r="5" fill="#37474f"/><circle cx="12" cy="-96" r="5" fill="#37474f"/><path d="M-14 -78 Q0 -66 14 -78" fill="none" stroke="#37474f" stroke-width="5" stroke-linecap="round"/>'
    return s


def house():
    return ('<rect x="-110" y="-30" width="220" height="150" fill="#ffe0b2"/><path d="M-130 -26 L0 -120 L130 -26 Z" fill="#c62828"/>'
            '<rect x="-30" y="40" width="60" height="80" fill="#8d6e63"/><rect x="50" y="10" width="44" height="44" fill="#bbdefb" stroke="#78909c" stroke-width="5"/>'
            '<rect x="-94" y="10" width="44" height="44" fill="#bbdefb" stroke="#78909c" stroke-width="5"/>')


def panel():
    s = '<path d="M-110 40 L-70 -50 L110 -50 L70 40 Z" fill="#283593" stroke="#90a4ae" stroke-width="7"/>'
    s += '<path d="M-90 -5 L90 -5 M-45 -50 L-83 40 M0 -50 L-14 40 M45 -50 L55 40 M-60 20 L82 20 L58 40" fill="none" stroke="#7986cb" stroke-width="4" stroke-linejoin="round"/>'
    s += '<rect x="-6" y="40" width="12" height="60" fill="#78909c"/><rect x="-50" y="96" width="100" height="14" rx="6" fill="#546e7a"/>'
    return s


def cloud(col="#cfd8dc"):
    return (f'<circle cx="-50" cy="10" r="44" fill="{col}"/><circle cx="0" cy="-20" r="56" fill="{col}"/>'
            f'<circle cx="55" cy="6" r="46" fill="{col}"/><rect x="-92" y="10" width="190" height="40" rx="20" fill="{col}"/>')


def rain():
    return "".join(f'<line x1="{x}" y1="{y}" x2="{x-12}" y2="{y+34}" stroke="#42a5f5" stroke-width="8" stroke-linecap="round"/>'
                   for x, y in [(-60, 0), (-20, 30), (20, 0), (60, 30), (100, 0)])


def plant():
    return ('<path d="M0 100 L0 -20" stroke="#2e7d32" stroke-width="12" stroke-linecap="round"/>'
            '<path d="M0 40 Q-60 40 -70 -10 Q-10 -10 0 40 Z" fill="#66bb6a"/><path d="M0 10 Q60 10 70 -40 Q10 -40 0 10 Z" fill="#43a047"/>'
            '<circle cx="0" cy="-34" r="22" fill="#f06292"/><circle cx="0" cy="-34" r="8" fill="#fdd835"/>')


def leaf(col="#ef6c00"):
    return f'<path d="M-40 0 Q0 -50 40 0 Q0 50 -40 0 Z" fill="{col}"/><path d="M-40 0 L40 0" stroke="#bf360c" stroke-width="5"/>'


def windmill_tower():
    return ('<path d="M-30 150 L-18 -20 L18 -20 L30 150 Z" fill="#eceff1" stroke="#90a4ae" stroke-width="6"/>'
            '<rect x="-24" y="-40" width="48" height="30" rx="8" fill="#b0bec5"/>')


def windmill_blades():
    s = ""
    for r in (0, 90, 180, 270):
        s += f'<path d="M0 0 L-16 -110 L16 -110 Z" fill="#ef5350" transform="rotate({r})"/><rect x="-5" y="-110" width="10" height="110" fill="#c62828" opacity="0.4" transform="rotate({r})"/>'
    s += '<circle r="14" fill="#546e7a"/>'
    return s


def wheel(r=90, spokes=8, col="#455a64"):
    s = f'<circle r="{r}" fill="none" stroke="{col}" stroke-width="12"/><circle r="{r-10}" fill="none" stroke="#b0bec5" stroke-width="3"/>'
    for i in range(spokes):
        a = math.pi * i / spokes
        s += f'<line x1="{f(-(r-8)*math.cos(a))}" y1="{f(-(r-8)*math.sin(a))}" x2="{f((r-8)*math.cos(a))}" y2="{f((r-8)*math.sin(a))}" stroke="#90a4ae" stroke-width="4"/>'
    s += '<circle r="10" fill="#37474f"/>'
    return s


def water_wheel():
    s = '<circle r="90" fill="#8d6e63"/><circle r="64" fill="#fff8e1"/><circle r="12" fill="#5d4037"/>'
    for r in range(0, 360, 45):
        s += f'<rect x="-9" y="-62" width="18" height="124" fill="#6d4c41" transform="rotate({r})"/>'
        s += f'<rect x="-22" y="-100" width="44" height="22" rx="4" fill="#a1887f" transform="rotate({r})"/>'
    return s


def waterfall():
    return ('<path d="M-110 -80 L-40 -80 L-40 -60 L-110 -60 Z" fill="#8d6e63"/><rect x="-120" y="-90" width="100" height="160" fill="#78909c"/>'
            '<rect x="-30" y="-80" width="60" height="150" fill="#4fc3f7"/><path d="M-30 -60 L30 -60 M-30 -20 L30 -20 M-30 20 L30 20" stroke="#e1f5fe" stroke-width="7"/>'
            '<ellipse cx="0" cy="76" rx="70" ry="18" fill="#29b6f6"/><rect x="20" y="-90" width="100" height="160" fill="#78909c"/>')


def jerrycan():
    return ('<path d="M-60 -70 L20 -70 L60 -30 L60 80 L-60 80 Z" fill="#c62828"/><rect x="-60" y="-30" width="120" height="14" fill="#8e0000" opacity="0.5"/>'
            '<path d="M-40 -70 L-40 -96 L10 -96 L10 -70" fill="none" stroke="#8e0000" stroke-width="12"/><circle cx="36" cy="-48" r="10" fill="#fdd835"/>')


def camp_stove():
    return ('<rect x="-50" y="-10" width="100" height="100" rx="12" fill="#1e88e5"/><rect x="-50" y="14" width="100" height="14" fill="#bbdefb"/>'
            '<rect x="-30" y="-34" width="60" height="26" rx="6" fill="#90a4ae"/><path d="M-50 -34 L50 -34" stroke="#546e7a" stroke-width="10"/>'
            '<path d="M0 -92 C16 -70 28 -62 24 -48 C20 -36 8 -34 0 -34 C-8 -34 -20 -36 -24 -48 C-28 -62 -10 -70 0 -92 Z" fill="#ef6c00"/>')


def toy_car(col="#8e24aa"):
    return (f'<path d="M-80 20 L-80 -10 L-50 -16 L-30 -50 L30 -50 L52 -16 L80 -10 L80 20 Z" fill="{col}"/>'
            '<path d="M-24 -42 L-42 -16 L-4 -16 L-4 -42 Z M4 -42 L4 -16 L44 -16 L26 -42 Z" fill="#e1bee7"/>'
            '<circle cx="-44" cy="24" r="20" fill="#263238"/><circle cx="44" cy="24" r="20" fill="#263238"/>'
            '<circle cx="-44" cy="24" r="8" fill="#b0bec5"/><circle cx="44" cy="24" r="8" fill="#b0bec5"/>')


def key_wind():
    return ('<path d="M0 0 L0 -50" stroke="#fdd835" stroke-width="10"/><ellipse cx="0" cy="-70" rx="30" ry="18" fill="none" stroke="#fdd835" stroke-width="10"/>')


def cell():
    return ('<ellipse rx="52" ry="40" fill="#f8bbd0" stroke="#ec407a" stroke-width="6"/><circle cx="8" cy="0" r="16" fill="#ad1457"/>')


def brain():
    return ('<path d="M-60 10 C-90 -20 -70 -70 -30 -66 C-20 -90 30 -90 40 -64 C80 -66 90 -20 62 6 C70 40 30 56 10 40 C-20 60 -66 44 -60 10 Z" fill="#f48fb1" stroke="#ad1457" stroke-width="6"/>'
            '<path d="M-10 -60 C-14 -30 10 -20 0 40 M-40 -20 C-20 -10 -20 10 -44 20 M30 -20 C14 -4 22 14 44 20" fill="none" stroke="#ad1457" stroke-width="5"/>')


def heat_waves(col=C_HEAT, n=3):
    s = ""
    for i in range(n):
        x = (i - (n - 1) / 2) * 44
        s += f'<path d="M{x} 50 C{x-16} 20 {x+16} 0 {x} -30 C{x-14} -56 {x+14} -70 {x} -96" fill="none" stroke="{col}" stroke-width="10" stroke-linecap="round"/>'
    return s


def arrow(x0, y0, x1, y1, col="#455a64", w=14, head=34):
    ang = math.atan2(y1 - y0, x1 - x0)
    bx, by = x1 - head * math.cos(ang), y1 - head * math.sin(ang)
    px, py = -math.sin(ang), math.cos(ang)
    return (f'<line x1="{f(x0)}" y1="{f(y0)}" x2="{f(bx)}" y2="{f(by)}" stroke="{col}" stroke-width="{w}" stroke-linecap="round"/>'
            f'<path d="M{f(x1)} {f(y1)} L{f(bx+px*head*0.65)} {f(by+py*head*0.65)} L{f(bx-px*head*0.65)} {f(by-py*head*0.65)} Z" fill="{col}"/>')


def bike_frame():
    return ('<g fill="none" stroke="#c62828" stroke-width="12" stroke-linecap="round" stroke-linejoin="round">'
            '<path d="M-110 60 L-30 -30 L60 -30 L110 60 M-30 -30 L10 60 L60 -30 M10 60 L-110 60 M-30 -30 L-44 -60 M60 -30 L50 -70 L90 -70"/></g>'
            '<rect x="-60" y="-70" width="40" height="14" rx="6" fill="#263238"/>')


def tiny_dynamo():
    return '<rect x="-14" y="-24" width="28" height="48" rx="8" fill="#90a4ae" stroke="#455a64" stroke-width="5"/><circle cx="0" cy="26" r="8" fill="#455a64"/>'
