"""Small helpers to build the cassette: narration, timeline, SVG scene.

Used by build.py. The work files (WAVs) go to $WORK, never into the cassette.
"""
import hashlib
import json
import math
import os
import subprocess
import wave
from xml.sax.saxutils import escape

VIEW_W, VIEW_H = 1600, 900
LEAD = 0.6      # silence before the first sentence
TAIL = 2.2      # silence after the last sentence
RATE = 22050
PIPER = [os.path.expanduser("~/piper/bin/piper"), "--model",
         os.path.expanduser("~/piper/el_GR-rapunzelina-low.onnx"), "--length-scale", "1.1"]


def sh(cmd):
    subprocess.run(cmd, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


class Timing:
    """Start and end second of every sentence, and the narration's length."""

    def __init__(self, keys, starts, ends, total):
        self.keys = keys
        self.start = dict(zip(keys, starts))
        self.end = dict(zip(keys, ends))
        self.total = total

    def s(self, key, off=0.0):
        return round(self.start[key] + off, 3)

    def e(self, key, off=0.0):
        return round(self.end[key] + off, 3)


def synth(work, lesson_id, sentences, out_ogg):
    """Make the narration. sentences: [(key, text, pause_after)]. Returns Timing."""
    d = os.path.join(work, lesson_id)
    os.makedirs(d, exist_ok=True)
    pcm = bytearray()
    t = LEAD
    pcm += b"\x00\x00" * int(RATE * LEAD)
    starts, ends, keys = [], [], []
    for i, (key, text, pause) in enumerate(sentences):
        h = hashlib.sha1((" ".join(PIPER) + text).encode()).hexdigest()[:10]
        raw = os.path.join(d, f"{i:02d}-{h}-raw.wav")
        cvt = os.path.join(d, f"{i:02d}-{h}.wav")
        if not os.path.exists(cvt):
            subprocess.run(PIPER + ["--output_file", raw], input=text.encode(),
                           check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            sh(["ffmpeg", "-y", "-i", raw, "-ac", "1", "-ar", str(RATE), "-c:a", "pcm_s16le", cvt])
        with wave.open(cvt) as w:
            assert w.getframerate() == RATE and w.getnchannels() == 1
            frames = w.readframes(w.getnframes())
        n = len(frames) // 2
        starts.append(round(t, 3))
        t += n / RATE
        ends.append(round(t, 3))
        keys.append(key)
        pcm += frames
        gap = TAIL if i == len(sentences) - 1 else pause
        pcm += b"\x00\x00" * int(RATE * gap)
        t += gap
    joined = os.path.join(d, "joined.wav")
    with wave.open(joined, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(RATE)
        w.writeframes(bytes(pcm))
    sh(["ffmpeg", "-y", "-i", joined, "-ac", "1", "-ar", str(RATE), "-c:a", "libvorbis", "-q:a", "2", out_ogg])
    out = subprocess.run(
        ["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", out_ogg],
        check=True, capture_output=True, text=True).stdout.strip()
    total = float(out)
    return Timing(keys, starts, ends, total)


# ---------------------------------------------------------------- SVG helpers

def f(v):
    s = f"{v:.1f}"
    return s[:-2] if s.endswith(".0") else s


def text(x, y, s, size=56, fill="#263238", anchor="middle", weight="bold", extra=""):
    return (f'<text x="{f(x)}" y="{f(y)}" font-family="sans-serif" font-size="{size}" '
            f'font-weight="{weight}" fill="{fill}" text-anchor="{anchor}" {extra}>{escape(s)}</text>')


def text_box(x, y, s, size=56, anchor="middle", pad=14):
    """A reveal_box that surely covers the text (estimate: 0.66 * size per character)."""
    w = 0.66 * size * len(s) + 2 * pad
    if anchor == "middle":
        bx = x - w / 2
    elif anchor == "start":
        bx = x - pad
    else:
        bx = x - w + pad
    by = y - size * 1.0 - pad
    h = size * 1.4 + 2 * pad
    bx = max(0, bx)
    by = max(0, by)
    w = min(w, VIEW_W - bx)
    h = min(h, VIEW_H - by)
    return [round(bx), round(by), round(w), round(h)]


class Scene:
    def __init__(self):
        self.defs = []
        self.items = []     # (actor id or None, inner svg)
        self.origin = {}
        self.box = {}

    def define(self, svg):
        self.defs.append(svg)

    def static(self, svg):
        self.items.append((None, svg))

    def actor(self, aid, svg, origin=None, box=None):
        assert aid not in self.origin or True
        self.items.append((aid, svg))
        if origin:
            self.origin[aid] = list(origin)
        if box:
            self.box[aid] = list(box)

    def actor_text(self, aid, x, y, s, size=56, fill="#263238", anchor="middle", extra=""):
        self.actor(aid, text(x, y, s, size, fill, anchor, extra=extra),
                   origin=(x if anchor == "middle" else x + 0.33 * size * len(s), y - size * 0.3),
                   box=text_box(x, y, s, size, anchor))

    def svg(self):
        out = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {VIEW_W} {VIEW_H}">']
        if self.defs:
            out.append("  <defs>\n    " + "\n    ".join(self.defs) + "\n  </defs>")
        for aid, inner in self.items:
            if aid:
                out.append(f'  <g id="{aid}">{inner}</g>')
            else:
                out.append("  " + inner)
        out.append("</svg>\n")
        return "\n".join(out)

    def ids(self):
        return [a for a, _ in self.items if a]


class Lesson:
    def __init__(self, lid, title, summary, scene, T):
        self.id, self.title, self.summary, self.scene, self.T = lid, title, summary, scene, T
        self.tracks = {}   # actor -> {t: props}
        self.captions = []

    # ---- keys
    def k(self, actor, t, **props):
        assert actor in self.scene.ids(), f"{self.id}: unknown actor {actor}"
        t = round(t, 3)
        assert 0 <= t <= self.T.total, f"{self.id}: {actor} key at {t} beyond {self.T.total}"
        self.tracks.setdefault(actor, {}).setdefault(t, {}).update(props)

    def fade_in(self, a, t, d=0.6):
        self.k(a, t, opacity=0)
        self.k(a, t + d, opacity=1, ease="out")

    def fade_out(self, a, t, d=0.5):
        self.k(a, t, opacity=1)
        self.k(a, t + d, opacity=0, ease="in")

    def pop(self, a, t, d=0.5, s0=0.6):
        self.k(a, t, opacity=0, scale=s0)
        self.k(a, t + d, opacity=1, scale=1, ease="out")

    def rise(self, a, t, d=0.6, dy=50):
        self.k(a, t, opacity=0, y=dy)
        self.k(a, t + d, opacity=1, y=0, ease="out")

    def write(self, a, t, d=1.0, frm="left"):
        self.k(a, t, reveal=0)
        self.k(a, t + d, reveal=1)
        self.tracks[a]["_from"] = frm

    def hide_now(self, a, t):
        """Vanish at t (step)."""
        self.k(a, t - 0.001, opacity=1)
        self.k(a, t, opacity=0, ease="step")

    def spin(self, a, t0, t1, deg, ease="linear", frm=0):
        self.k(a, t0, rotate=frm)
        self.k(a, t1, rotate=deg, ease=ease)

    def move(self, a, t0, t1, frm=(0, 0), to=(0, 0), ease="in-out"):
        self.k(a, t0, x=frm[0], y=frm[1])
        self.k(a, t1, x=to[0], y=to[1], ease=ease)

    def bob(self, a, t0, n=2, amp=24, per=0.5):
        self.k(a, t0, y=0)
        for i in range(n):
            self.k(a, t0 + per * (i + 0.5), y=-amp, ease="out")
            self.k(a, t0 + per * (i + 1), y=0, ease="in")

    def pulse(self, a, t0, n=2, big=1.12, per=0.5):
        self.k(a, t0, scale=1)
        for i in range(n):
            self.k(a, t0 + per * (i + 0.5), scale=big, ease="out")
            self.k(a, t0 + per * (i + 1), scale=1, ease="in")

    def wiggle(self, a, t0, n=3, deg=6, per=0.3):
        self.k(a, t0, rotate=0)
        for i in range(n):
            self.k(a, t0 + per * (i + 0.25), rotate=deg, ease="in-out")
            self.k(a, t0 + per * (i + 0.75), rotate=-deg, ease="in-out")
        self.k(a, t0 + per * (n + 0.25), rotate=0, ease="in-out")

    # ---- output
    def lesson_json(self):
        tracks = []
        for actor in self.scene.ids():
            if actor not in self.tracks:
                continue
            ks = self.tracks[actor]
            frm = ks.pop("_from", None)
            keys = []
            need_origin = need_box = False
            for t in sorted(k for k in ks):
                props = ks[t]
                need_origin |= "scale" in props or "rotate" in props
                need_box |= "reveal" in props
                keys.append({"t": t, **props})
            tr = {"actor": actor}
            if need_origin:
                assert actor in self.scene.origin, f"{self.id}: {actor} needs an origin"
                tr["origin"] = [round(v, 1) for v in self.scene.origin[actor]]
            if need_box:
                assert actor in self.scene.box, f"{self.id}: {actor} needs a reveal box"
                tr["reveal_box"] = self.scene.box[actor]
                if frm and frm != "left":
                    tr["reveal_from"] = frm
            tr["keys"] = keys
            tracks.append(tr)
        caps = []
        for key, text_ in self.sentences:
            caps.append({"start": self.T.start[key], "end": self.T.end[key], "text": text_})
        return {"format": 1, "title": self.title, "summary": self.summary,
                "tracks": tracks, "captions": caps}


def dump_json(obj, indent=0):
    """JSON with one line per key / caption, readable and diff friendly."""
    def dumps(v):
        return json.dumps(v, ensure_ascii=False, separators=(",", ":"))
    pad = "  "
    out = ["{"]
    items = []
    for k, v in obj.items():
        if k == "tracks":
            tr = []
            for t in v:
                head = {kk: vv for kk, vv in t.items() if kk != "keys"}
                hs = ",\n".join(f'{pad*4}"{kk}": {dumps(vv)}' for kk, vv in head.items())
                ks = ",\n".join(f"{pad*5}{dumps(kk)}" for kk in t["keys"])
                tr.append(f"{pad*3}{{\n{hs},\n{pad*4}\"keys\": [\n{ks}\n{pad*4}]\n{pad*3}}}")
            items.append(f'{pad}"tracks": [\n' + ",\n".join(tr) + f"\n{pad}]")
        elif k == "captions":
            items.append(f'{pad}"captions": [\n' + ",\n".join(f"{pad*2}{dumps(c)}" for c in v) + f"\n{pad}]")
        else:
            items.append(f'{pad}"{k}": {dumps(v)}')
    out.append(",\n".join(items))
    out.append("}")
    return "\n".join(out) + "\n"


# ------------------------------------------------------------ preview of a frame

def _ease(name, u):
    if name == "step":
        return 1.0 if u >= 1 else 0.0
    if name == "in":
        return u ** 3
    if name == "out":
        return 1 - (1 - u) ** 3
    if name == "in-out":
        return 4 * u ** 3 if u < 0.5 else 1 - (-2 * u + 2) ** 3 / 2
    return u


def _value(keys, prop, t, default):
    pts = [(k["t"], k[prop], k.get("ease", "linear")) for k in keys if prop in k]
    if not pts:
        return default
    if t <= pts[0][0]:
        return pts[0][1]
    for (t0, v0, _), (t1, v1, e1) in zip(pts, pts[1:]):
        if t <= t1:
            u = (t - t0) / (t1 - t0)
            return v0 + (v1 - v0) * _ease(e1, u)
    return pts[-1][1]


def frame_svg(scene, lj, t):
    """The scene at second t, as a plain SVG (used only to check the work by eye)."""
    tr = {x["actor"]: x for x in lj["tracks"]}
    out = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {VIEW_W} {VIEW_H}">']
    if scene.defs:
        out.append("<defs>" + "".join(scene.defs) + "</defs>")
    n = 0
    for aid, inner in scene.items:
        if not aid:
            out.append(inner)
            continue
        x = tr.get(aid)
        if not x:
            out.append(f'<g>{inner}</g>')
            continue
        ks = x["keys"]
        op = _value(ks, "opacity", t, 1)
        dx, dy = _value(ks, "x", t, 0), _value(ks, "y", t, 0)
        sc_, rot = _value(ks, "scale", t, 1), _value(ks, "rotate", t, 1 and 0)
        rv = _value(ks, "reveal", t, 1)
        ox, oy = x.get("origin", [0, 0])
        tf = f"translate({dx} {dy}) translate({ox} {oy}) rotate({rot}) scale({sc_}) translate({-ox} {-oy})"
        clip = ""
        if rv < 1:
            bx, by, bw, bh = x["reveal_box"]
            frm = x.get("reveal_from", "left")
            n += 1
            if frm == "left":
                r = (bx, by, bw * rv, bh)
            elif frm == "right":
                r = (bx + bw * (1 - rv), by, bw * rv, bh)
            elif frm == "top":
                r = (bx, by, bw, bh * rv)
            else:
                r = (bx, by + bh * (1 - rv), bw, bh * rv)
            out.append(f'<clipPath id="rv{n}"><rect x="{r[0]}" y="{r[1]}" width="{r[2]}" height="{r[3]}"/></clipPath>')
            clip = f' clip-path="url(#rv{n})"'
        out.append(f'<g transform="{tf}" opacity="{op:.3f}"><g{clip}>{inner}</g></g>')
    out.append("</svg>")
    return "\n".join(out)
