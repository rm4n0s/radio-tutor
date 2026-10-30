"""Helpers to build the cassette: narration (piper), timelines, SVG scenes."""
import hashlib
import json
import os
import subprocess
import wave

PIPER = os.path.expanduser("~/piper/bin/piper")
MODEL = os.path.expanduser("~/piper/el_GR-rapunzelina-low.onnx")
LENGTH_SCALE = "1.15"  # a little slower than the default, for children
RATE = 22050


def run(*args):
    subprocess.run(args, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def synth(text, work):
    """Text to a mono 22050 Hz WAV (cached by text). Returns the path."""
    h = hashlib.sha1((text + LENGTH_SCALE).encode()).hexdigest()[:16]
    raw = os.path.join(work, "raw-%s.wav" % h)
    out = os.path.join(work, "s-%s.wav" % h)
    if not os.path.exists(out):
        subprocess.run([PIPER, "--model", MODEL, "--length_scale", LENGTH_SCALE,
                        "--output_file", raw], input=text.encode(), check=True,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        run("ffmpeg", "-y", "-i", raw, "-ac", "1", "-ar", str(RATE), "-c:a", "pcm_s16le", out)
    return out


class Narration:
    """Joins sentences with pauses; knows each sentence's start and end."""

    def __init__(self, work, sentences, lead=0.5, tail=2.0, gap=0.45):
        # sentences: list of (key, text) or (key, text, pause_after)
        self.work = work
        self.items = []
        pcm = bytearray()
        t = lead
        pcm += b"\0\0" * int(RATE * lead)
        self.keys = {}
        for i, s in enumerate(sentences):
            key, text = s[0], s[1]
            pause = s[2] if len(s) > 2 else gap
            with wave.open(synth(text, work)) as w:
                assert w.getframerate() == RATE and w.getnchannels() == 1
                frames = w.readframes(w.getnframes())
                dur = w.getnframes() / RATE
            pcm += frames
            if dur > 0.16 * len(text) + 0.5:  # the voice sometimes garbles a very short sentence
                print("WARNING: sentence %s is suspiciously long (%.1f s): %s" % (key, dur, text))
            self.items.append({"key": key, "text": text, "start": t, "end": t + dur})
            assert key not in self.keys, key
            self.keys[key] = self.items[-1]
            t += dur
            last = i == len(sentences) - 1
            p = tail if last else pause
            pcm += b"\0\0" * int(RATE * p)
            t += p
        self.pcm = bytes(pcm)
        self.length = len(self.pcm) / 2 / RATE

    def write_ogg(self, path):
        wav = os.path.join(self.work, "joined.wav")
        with wave.open(wav, "wb") as w:
            w.setnchannels(1)
            w.setsampwidth(2)
            w.setframerate(RATE)
            w.writeframes(self.pcm)
        run("ffmpeg", "-y", "-i", wav, "-ac", "1", "-ar", str(RATE), "-c:a", "libvorbis",
            "-q:a", "2", path)
        out = subprocess.run(["ffprobe", "-v", "error", "-show_entries", "format=duration",
                              "-of", "csv=p=0", path], capture_output=True, text=True, check=True)
        self.ogg_length = float(out.stdout)
        return self.ogg_length

    def s(self, key):
        return self.keys[key]["start"]

    def e(self, key):
        return self.keys[key]["end"]

    def cue(self, key, word, after=0.0):
        """Approximate time when `word` is spoken in sentence `key`."""
        it = self.keys[key]
        i = it["text"].find(word)
        assert i >= 0, (key, word)
        return it["start"] + (it["end"] - it["start"]) * i / len(it["text"]) + after

    def captions(self):
        res = []
        prev_end = 0
        for it in self.items:
            start = round(it["start"], 2)
            end = round(it["end"] + 0.15, 2)
            assert start >= prev_end
            res.append({"start": start, "end": end, "text": it["text"]})
            prev_end = end
        return res


DEFAULTS = {"opacity": 1, "x": 0, "y": 0, "scale": 1, "rotate": 0, "reveal": 1}


class Track:
    def __init__(self, actor, origin=None, reveal_box=None, reveal_from=None):
        self.actor = actor
        self.origin = origin
        self.reveal_box = reveal_box
        self.reveal_from = reveal_from
        self.props = {}

    def cur(self, prop):
        l = self.props.get(prop)
        return l[-1][1] if l else DEFAULTS[prop]

    def last_t(self, prop):
        l = self.props.get(prop)
        return l[-1][0] if l else -1

    def set(self, prop, t, val, ease="linear"):
        t = round(t, 3)
        l = self.props.setdefault(prop, [])
        if l and t <= l[-1][0] + 1e-9:
            if abs(t - l[-1][0]) < 0.0015:
                t = round(l[-1][0] + 0.002, 3)
            else:
                raise ValueError("%s.%s: key at %s not after %s" % (self.actor, prop, t, l[-1][0]))
        # one ease per key: a key at the same time with another ease is nudged a little
        while any(abs(tt - t) < 1e-9 and ee != ease for p2, l2 in self.props.items() if p2 != prop for (tt, _, ee) in l2):
            t = round(t + 0.004, 3)
        l.append((t, val, ease))
        return self

    def hold(self, prop, t):
        """Key that keeps the current value up to time t."""
        if self.last_t(prop) < round(t, 3) - 0.0015:
            self.set(prop, t, self.cur(prop), "linear")

    def show(self, t, dur=0.5, ease="out"):
        if not self.props.get("opacity"):
            self.set("opacity", 0, 0) if t > 0.01 else None
            if t > 0.01:
                self.hold("opacity", t)
        else:
            self.hold("opacity", t)
        self.set("opacity", t + dur, 1, ease)
        return self

    def hide(self, t, dur=0.4, ease="in"):
        self.hold("opacity", t)
        self.set("opacity", t + dur, 0, ease)
        return self

    def hidden_until(self, t, dur=0.5, ease="out"):
        """Invisible from the start, fades in at t."""
        self.set("opacity", 0, 0)
        return self.show(t, dur, ease)

    def move(self, t0, t1, x=None, y=None, ease="in-out"):
        for p, v in (("x", x), ("y", y)):
            if v is not None:
                self.hold(p, t0)
                self.set(p, t1, v, ease)
        return self

    def scale(self, t0, t1, v, ease="in-out"):
        self.hold("scale", t0)
        self.set("scale", t1, v, ease)
        return self

    def rotate(self, t0, t1, v, ease="in-out"):
        self.hold("rotate", t0)
        self.set("rotate", t1, v, ease)
        return self

    def reveal(self, t0, t1, ease="linear"):
        self.set("reveal", t0, 0)
        self.set("reveal", t1, 1, ease)
        return self

    def wiggle(self, t0, period, n, amp, prop="y", base=0.0, ease="in-out"):
        """Move prop to base+amp and back, n times, starting at t0."""
        t = t0
        self.hold(prop, t)
        for i in range(n):
            self.set(prop, t + period / 2, base + amp, ease)
            self.set(prop, t + period, base, ease)
            t += period
        return self

    def compile(self, max_t):
        times = {}
        for prop, l in self.props.items():
            for (t, v, e) in l:
                times.setdefault(t, {})[prop] = (v, e)
        keys = []
        for t in sorted(times):
            d = times[t]
            eases = {e for (_, e) in d.values()}
            if len(eases) > 1:
                raise ValueError("%s: conflicting eases at %s: %s" % (self.actor, t, d))
            k = {"t": min(t, max_t)}
            for prop in ("opacity", "x", "y", "scale", "rotate", "reveal"):
                if prop in d:
                    v = d[prop][0]
                    k[prop] = round(v, 3) if isinstance(v, float) else v
            e = eases.pop()
            if e != "linear":
                k["ease"] = e
            keys.append(k)
        for a, b in zip(keys, keys[1:]):
            assert b["t"] > a["t"], (self.actor, a, b)
        res = {"actor": self.actor}
        if self.origin:
            res["origin"] = self.origin
        if self.reveal_box:
            res["reveal_box"] = self.reveal_box
            if self.reveal_from:
                res["reveal_from"] = self.reveal_from
        res["keys"] = keys
        return res


class Scene:
    def __init__(self, bg_top="#e3f2fd", bg_bottom="#fff8e1"):
        self.defs = []
        self.parts = []  # in order: ("static", svg) or ("actor", id)
        self.actors = {}
        self.tracks = {}
        self.bg = (bg_top, bg_bottom)

    def define(self, svg):
        self.defs.append(svg)

    def static(self, svg):
        self.parts.append(("static", svg))

    def actor(self, id_, inner, origin=None, reveal_box=None, reveal_from=None):
        assert id_ not in self.actors, id_
        self.actors[id_] = inner
        self.parts.append(("actor", id_))
        tr = Track(id_, origin, reveal_box, reveal_from)
        self.tracks[id_] = tr
        return tr

    def svg(self):
        out = ['<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1600 900">', "<defs>",
               '<linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">'
               '<stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient>' % self.bg]
        out += self.defs
        out.append("</defs>")
        out.append('<rect width="1600" height="900" fill="url(#bg)"/>')
        for kind, v in self.parts:
            if kind == "static":
                out.append(v)
            else:
                out.append('<g id="%s">%s</g>' % (v, self.actors[v]))
        out.append("</svg>")
        return "\n".join(out) + "\n"

    def lesson_json(self, title, summary, narration):
        tracks = [t.compile(narration.length) for t in self.tracks.values() if t.props]
        return {"format": 1, "title": title, "summary": summary, "tracks": tracks,
                "captions": narration.captions()}


# ---------- SVG helpers ----------
INK = "#263238"
BLUE = "#1e88e5"
NAVY = "#283593"


def esc(s):
    return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def text(x, y, s, size=48, fill=INK, anchor="middle", bold=True, extra=""):
    return ('<text x="%s" y="%s" font-family="sans-serif" font-size="%s" %s fill="%s" '
            'text-anchor="%s" %s>%s</text>' % (x, y, size, 'font-weight="bold"' if bold else "",
                                               fill, anchor, extra, esc(s)))


def text_w(s, size, bold=True):
    return len(s) * size * (0.58 if bold else 0.52)


def box_for_text(x, y, s, size=48, anchor="middle", pad=12):
    w = text_w(s, size) + 2 * pad
    x0 = x - w / 2 if anchor == "middle" else (x - pad if anchor == "start" else x - w + pad)
    return [round(max(0, x0)), round(y - size - pad / 2), round(min(w, 1600 - max(0, x0))), round(size * 1.35 + pad)]


def rect(x, y, w, h, fill, rx=0, stroke=None, sw=0, extra=""):
    st = ' stroke="%s" stroke-width="%s"' % (stroke, sw) if stroke else ""
    return '<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="%s"%s %s/>' % (x, y, w, h, rx, fill, st, extra)


def circle(cx, cy, r, fill, stroke=None, sw=0, extra=""):
    st = ' stroke="%s" stroke-width="%s"' % (stroke, sw) if stroke else ""
    return '<circle cx="%s" cy="%s" r="%s" fill="%s"%s %s/>' % (cx, cy, r, fill, st, extra)


def path(d, fill="none", stroke=None, sw=0, extra=""):
    st = ' stroke="%s" stroke-width="%s" stroke-linecap="round" stroke-linejoin="round"' % (stroke, sw) if stroke else ""
    return '<path d="%s" fill="%s"%s %s/>' % (d, fill, st, extra)


def line(x1, y1, x2, y2, stroke=INK, sw=6):
    return '<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="%s" stroke-linecap="round"/>' % (x1, y1, x2, y2, stroke, sw)


def arrow(x1, y1, x2, y2, stroke=INK, sw=8):
    import math
    a = math.atan2(y2 - y1, x2 - x1)
    h = 26
    p1 = (x2 - h * math.cos(a - 0.45), y2 - h * math.sin(a - 0.45))
    p2 = (x2 - h * math.cos(a + 0.45), y2 - h * math.sin(a + 0.45))
    return (line(x1, y1, x2 - 10 * math.cos(a), y2 - 10 * math.sin(a), stroke, sw) +
            '<polygon points="%s,%s %s,%s %s,%s" fill="%s"/>' % (x2, y2, p1[0], p1[1], p2[0], p2[1], stroke))


def glass(cx, base, w=200, h=260, liquid="#64b5f6", level=0.75, opacity=0.85, rim="#90a4ae"):
    """A drinking glass (slightly tapered). Returns svg for glass back + liquid + front outline."""
    x0, x1 = cx - w / 2, cx + w / 2
    top = base - h
    lw0 = x0 + 8
    ly = base - h * level
    inset = 10 * (1 - level) * 0 + 0
    body = "M%s %s L%s %s L%s %s L%s %s Z" % (x0, top, x0 + 18, base, x1 - 18, base, x1, top)
    # liquid polygon: interpolate edges
    def edge(y):
        f = (y - top) / h
        return x0 + 18 * f, x1 - 18 * f
    ax, bx = edge(ly)
    liq = "M%s %s L%s %s L%s %s L%s %s Z" % (ax, ly, x0 + 18, base, x1 - 18, base, bx, ly)
    return (path(liq, liquid, extra='opacity="%s"' % opacity) +
            path(body, "none", rim, 8) )


def bubbles_static(cx, cy, n=3):
    return ""


class Phase:
    """A group of actors that are hidden together at the end of a part of the lesson."""

    def __init__(self, sc, t_end, dur=0.4):
        self.sc, self.t_end, self.dur, self.ids = sc, t_end, dur, []

    def add(self, id_, inner=None, t_in=None, dur=0.5, **kw):
        """Create an actor (fading in at t_in if given) that disappears with the phase."""
        tr = self.sc.actor(id_, inner, **kw)
        if t_in is not None:
            tr.hidden_until(t_in, dur)
        self.ids.append(id_)
        return tr

    def adopt(self, id_):
        self.ids.append(id_)

    def close(self):
        for i in self.ids:
            self.sc.tracks[i].hide(self.t_end, self.dur)


def loop_rise(tr, t0, t_end, dy, dur=1.8, fade=0.3):
    """Something that rises dy, fades out, and starts again, until t_end."""
    tr.set("opacity", 0, 0)
    first = True
    while t0 + dur < t_end:
        tr.hold("opacity", t0)
        tr.set("opacity", t0 + fade, 0.9, "out")
        tr.set("opacity", t0 + dur - 0.1, 0.0, "in")
        if first:
            tr.set("y", t0, 0)
        else:
            tr.set("y", t0, 0, "step")
        tr.set("y", t0 + dur, dy, "linear")
        t0 += dur + 0.01
        first = False
