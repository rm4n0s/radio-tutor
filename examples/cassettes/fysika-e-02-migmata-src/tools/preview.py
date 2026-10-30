"""Render frames of a lesson at given times (for checking by eye).

usage: preview.py <lesson dir> <out.png> t1 t2 ...   (needs google-chrome and ImageMagick)
"""
import json
import os
import subprocess
import sys
import tempfile
import xml.etree.ElementTree as ET

NS = "http://www.w3.org/2000/svg"
ET.register_namespace("", NS)
DEF = {"opacity": 1, "x": 0, "y": 0, "scale": 1, "rotate": 0, "reveal": 1}
EASE = {"linear": lambda u: u, "in": lambda u: u ** 3, "out": lambda u: 1 - (1 - u) ** 3,
        "in-out": lambda u: 4 * u ** 3 if u < .5 else 1 - (-2 * u + 2) ** 3 / 2, "step": lambda u: 1.0 if u >= 1 else 0.0}


def value(keys, prop, t):
    ks = [k for k in keys if prop in k]
    if not ks:
        return DEF[prop]
    if t <= ks[0]["t"]:
        return ks[0][prop]
    for a, b in zip(ks, ks[1:]):
        if t <= b["t"]:
            u = (t - a["t"]) / (b["t"] - a["t"])
            e = EASE[b.get("ease", "linear")](u)
            return a[prop] + (b[prop] - a[prop]) * e
    return ks[-1][prop]


def frame_svg(d, t):
    tree = ET.parse(os.path.join(d, "scene.svg"))
    root = tree.getroot()
    lesson = json.load(open(os.path.join(d, "lesson.json")))
    tracks = {tr["actor"]: tr for tr in lesson["tracks"]}
    defs = root.find("{%s}defs" % NS)
    for g in root.findall("{%s}g" % NS):
        gid = g.get("id")
        if not gid or gid not in tracks:
            continue
        tr = tracks[gid]
        v = {p: value(tr["keys"], p, t) for p in DEF}
        g.set("opacity", str(max(0, min(1, v["opacity"]))))
        ox, oy = tr.get("origin", [0, 0])
        g.set("transform", "translate(%s %s) translate(%s %s) rotate(%s) scale(%s) translate(%s %s)" % (
            v["x"], v["y"], ox, oy, v["rotate"], v["scale"], -ox, -oy))
        if "reveal_box" in tr and v["reveal"] < 1:
            x, y, w, h = tr["reveal_box"]
            f = tr.get("reveal_from", "left")
            r = v["reveal"]
            if f == "left":
                rc = (x, y, w * r, h)
            elif f == "right":
                rc = (x + w * (1 - r), y, w * r, h)
            elif f == "top":
                rc = (x, y, w, h * r)
            else:
                rc = (x, y + h * (1 - r), w, h * r)
            cid = "clip-" + gid
            cp = ET.SubElement(defs, "{%s}clipPath" % NS, id=cid)
            ET.SubElement(cp, "{%s}rect" % NS, x=str(rc[0]), y=str(rc[1]), width=str(rc[2]), height=str(rc[3]))
            # the actor's own transform moves the clip too; wrap in an outer group instead
            inner = ET.Element("{%s}g" % NS)
            for c in list(g):
                g.remove(c)
                inner.append(c)
            inner.set("clip-path", "url(#%s)" % cid)
            g.append(inner)
    return ET.tostring(root, encoding="unicode")


def main():
    d, out, times = sys.argv[1], sys.argv[2], [float(x) for x in sys.argv[3:]]
    tmp = tempfile.mkdtemp()
    pngs = []
    for i, t in enumerate(times):
        svg = frame_svg(d, t)
        html = os.path.join(tmp, "f%d.html" % i)
        open(html, "w").write('<html><body style="margin:0;background:#fff"><div style="width:1600px;height:900px">%s</div></body></html>' % svg.replace("<svg ", '<svg width="1600" height="900" ', 1))
        png = os.path.join(tmp, "f%d.png" % i)
        subprocess.run(["google-chrome", "--headless=new", "--no-sandbox", "--disable-gpu", "--hide-scrollbars",
                        "--window-size=1600,900", "--screenshot=" + png, "file://" + html],
                       check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=60)
        subprocess.run(["convert", png, "-resize", "800x450", "-gravity", "NorthWest", "-fill", "black", "-pointsize", "28",
                        "-annotate", "+8+4", "t=%.1f" % t, png], check=True)
        pngs.append(png)
    subprocess.run(["montage"] + pngs + ["-tile", "2x", "-geometry", "+4+4", out], check=True)


main()
