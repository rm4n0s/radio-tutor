"""Lesson 2: heterogeneous mixtures and how to separate them."""
import random
from lib import *
from icons import *

ID = "eterogena-kai-diaxorismos"
TITLE = "Μελετώντας τα μίγματα"
SUMMARY = "Ετερογενή μίγματα, ίζημα και ανακίνηση, διαλογή και διήθηση, τα σκουπίδια ως μίγμα."

SENTENCES = [
    ("a1", "Κάθε μίγμα έχει δύο ή περισσότερα συστατικά."),
    ("a2", "Σε μερικά μίγματα βλέπουμε τα συστατικά με το μάτι ή με μικροσκόπιο."),
    ("a3", "Αυτά τα μίγματα λέγονται ετερογενή.", 1.0),
    ("a4", "Για παράδειγμα, στο μπετόν των σπιτιών βλέπουμε χαλίκια, άμμο και τσιμέντο.", 1.5),
    ("b1", "Στη συσκευασία ενός χυμού διαβάζουμε: ανακινήστε καλά πριν από τη χρήση."),
    ("b2", "Γιατί όμως πρέπει να τον ανακινήσουμε;", 0.9),
    ("b3", "Αν αφήσουμε τον χυμό για αρκετή ώρα, στερεά κομμάτια πέφτουν στον πάτο.", 1.4),
    ("b4", "Αυτό το στερεό στρώμα λέγεται ίζημα.", 1.2),
    ("b5", "Όταν ανακινούμε, το ίζημα ανακατεύεται ξανά με το υγρό.", 1.5),
    ("c1", "Κάποιες φορές θέλουμε μόνο ένα συστατικό ενός μίγματος."),
    ("c2", "Τότε πρέπει να διαχωρίσουμε τα συστατικά του.", 0.8),
    ("c3", "Υπάρχουν πολλές μέθοδοι διαχωρισμού, απλές και πιο σύνθετες.", 1.5),
    ("c4", "Μια απλή μέθοδος είναι η διαλογή."),
    ("c5", "Όταν μαγειρεύουμε φακές, βγάζουμε τα πετραδάκια με το χέρι.", 1.0),
    ("c6", "Έτσι ξεχωρίζουμε τα πετραδάκια από τις φακές.", 1.5),
    ("c7", "Μια άλλη απλή μέθοδος είναι η διήθηση, ή αλλιώς φιλτράρισμα."),
    ("c8", "Ο χυμός περνάει από το σουρωτήρι, και τα στερεά κομμάτια μένουν πάνω.", 1.0),
    ("c9", "Το ίδιο γίνεται και στον καφέ φίλτρου.", 1.5),
    ("c10", "Πιο σύνθετες μέθοδοι είναι η φυγοκέντριση, η απόσταξη και η χρωματογραφία.", 0.8),
    ("c11", "Τις χρησιμοποιούν σε ειδικές εγκαταστάσεις.", 1.5),
    ("d1", "Αν μπορούσαμε να μεγεθύνουμε ένα μίγμα, θα βλέπαμε μικρά σωματίδια."),
    ("d2", "Είναι τα μόρια των ουσιών που έχουν ανακατευτεί.", 0.8),
    ("d3", "Κινούνται συνεχώς, προς όλες τις κατευθύνσεις.", 1.5),
    ("e1", "Και τα σκουπίδια είναι ένα ετερογενές μίγμα."),
    ("e2", "Έχουν πολλά διαφορετικά συστατικά, και μερικά μπορούν να γίνουν πάλι χρήσιμα.", 0.8),
    ("e3", "Για να τα αξιοποιήσουμε, πρέπει πρώτα να τα ξεχωρίσουμε.", 1.0),
    ("e4", "Πριν πετάξουμε τα σκουπίδια, κάνουμε διαλογή."),
    ("e5", "Το γυαλί, το χαρτί και το αλουμίνιο πηγαίνουν στον δικό τους κάδο.", 1.5),
    ("e6", "Σε πολλές χώρες ξεχωρίζουν και τα υπολείμματα φαγητού.", 1.0),
    ("e7", "Έτσι μειώνεται το κόστος επεξεργασίας των σκουπιδιών.", 1.5),
    ("f1", "Θυμήσου: στα ετερογενή μίγματα φαίνονται τα συστατικά."),
    ("f2", "Τα ξεχωρίζουμε με διαλογή ή με διήθηση."),
]


def scatter(rnd, cx, cy, rx, ry, n, makers):
    out = ""
    pts = []
    while len(pts) < n:
        x, y = rnd.uniform(-1, 1), rnd.uniform(-1, 0.1)
        if x * x + y * y <= 1:
            pts.append((cx + x * rx, cy + y * ry))
    for i, (x, y) in enumerate(pts):
        out += makers[i % len(makers)](x, y, rnd)
    return out


def mk_bean(x, y, r):
    return bean(x, y, 1.3, "#8d6e63", r.randint(0, 170))


def mk_lentil(x, y, r):
    return circle(x, y, 12, "#e08a1e", "#a65f00", 2)


def mk_pebble(x, y, r):
    return pebble(x, y, 15, "#9e9e9e")


def build(nar):
    sc = Scene("#eaf6ee", "#fffde7")
    sc.static(rect(0, 820, 1600, 80, "#d7ccc8", 0, None, 0, 'opacity="0.6"'))
    S, E, C = nar.s, nar.e, nar.cue
    rnd = random.Random(7)
    # A scene may have at most 64 actors (CASSETTE-FORMAT 5.1), so related drawings share one actor.

    def head(ph, id_, s, k):
        ph.add(id_, text(800, 105, s, 68, NAVY), S(k) - 0.1)

    # ---------- A: heterogeneous mixtures
    ph = Phase(sc, E("a4") + 0.3)
    head(ph, "h-het", "Ετερογενή μίγματα", "a1")
    heap = scatter(rnd, 430, 640, 190, 90, 26, [mk_bean, mk_lentil, mk_pebble, mk_lentil])
    ph.add("a-bowl", bowl_back(430, 650, 470, 170) + heap + bowl_front(430, 650, 470, 170, "#8e24aa"), S("a1") + 0.3, 0.6)
    lens_back, lens_front = magnifier(0, 0, 120, "#ffffff")
    big = (bean(-40, -20, 4.0, "#8d6e63", 30) + circle(40, 10, 40, "#e08a1e", "#a65f00", 5) + pebble(-10, 55, 52, "#9e9e9e"))
    lens = lens_back.replace('fill="#ffffff"', 'fill="#e3f2fd"') + big + lens_front
    tl = C("a2", "μικροσκόπιο", -2.5)
    tr = ph.add("a-lens", '<g transform="translate(430 500)">%s</g>' % lens, tl, 0.5, origin=[430, 500])
    tr.set("x", tl, 380)
    tr.set("x", tl + 1.3, 0, "out")
    ph.add("a-lab-het", text(430, 345, "ετερογενές", 72, "#2e7d32"), reveal_box=[200, 270, 460, 100]).reveal(C("a3", "ετερογενή") - 0.3, E("a3"))
    conc = rect(940, 480, 440, 280, "#bdbdbd", 14, "#757575", 6)
    for i in range(30):
        x, y = 960 + rnd.random() * 400, 500 + rnd.random() * 240
        conc += circle(round(x), round(y), 4, "#e0c9a6")
    for (x, y, r) in ((1010, 560, 28), (1130, 640, 34), (1260, 540, 26), (1060, 700, 24), (1300, 690, 30), (1190, 510, 22)):
        conc += pebble(x, y, r, "#8d6e63")
    ph.add("a-conc", conc + text(1160, 810, "μπετόν", 46, INK), C("a4", "μπετόν") - 0.1, 0.6)
    labs = [("hal", 1010, "χαλίκια", (1050, 548)), ("amm", 1160, "άμμος", (1160, 590)), ("tsi", 1320, "τσιμέντο", (1340, 590))]
    for nm, x, s_, (ax, ay) in labs:
        t0 = C("a4", {"άμμος": "άμμο"}.get(s_, s_), -0.1)
        dy = 0 if nm != "amm" else -30
        ph.add("a-l-" + nm, text(x, 400 + dy, s_, 50, "#6a1b9a") + arrow(x, 415 + dy, ax, ay - 20, "#6a1b9a", 6), t0, 0.5)
    ph.close()

    # ---------- B: sediment
    ph = Phase(sc, E("b5") + 0.5)
    head(ph, "h-sed", "Ο χυμός με το ίζημα", "b1")
    gx, gb, gw, gh, gk = 800, 830, 300, 440, 22
    top = gb - gh

    def edge(y):
        f = (y - top) / gh
        return gx - gw / 2 + gk * f, gx + gw / 2 - gk * f
    lvl = top + 50
    ax, bx = edge(lvl)
    liq = path("M%s %s L%s %s L%s %s L%s %s Z" % (ax, lvl, gx - gw / 2 + gk, gb, gx + gw / 2 - gk, gb, bx, lvl), "#ffb74d", extra='opacity="0.9"')
    dots = ""
    rr = random.Random(11)
    for i in range(34):
        y = rr.uniform(lvl + 25, gb - 30)
        l, r_ = edge(y)
        dots += circle(round(rr.uniform(l + 18, r_ - 18)), round(y), rr.randint(7, 11), "#6d4c41")
    sy = gb - 80
    l1, r1 = edge(sy)
    sed = path("M%s %s L%s %s L%s %s L%s %s Z" % (l1, sy, gx - gw / 2 + gk, gb, gx + gw / 2 - gk, gb, r1, sy), "#6d4c41")
    front = path("M%s %s L%s %s L%s %s L%s %s" % (gx - gw / 2, top, gx - gw / 2 + gk, gb, gx + gw / 2 - gk, gb, gx + gw / 2, top), "none", "#78909c", 9)
    t_sh = C("b5", "ανακινούμε", -0.1)
    parts = []
    parts.append(ph.add("g-liq", liq, S("b1") + 0.2, 0.6, origin=[gx, gb]))
    parts.append(ph.add("g-dots", dots, S("b1") + 0.2, 0.6, origin=[gx, gb]))
    parts.append(ph.add("g-sed", sed, reveal_box=[gx - gw / 2, sy - 5, gw, 100], reveal_from="bottom", origin=[gx, gb]))
    parts.append(ph.add("g-front", front, S("b1") + 0.2, 0.6, origin=[gx, gb]))
    t3, t3e = S("b3") + 0.5, E("b3") + 0.5
    sc.tracks["g-dots"].hold("opacity", t3)
    sc.tracks["g-dots"].set("opacity", t3e, 0.0, "in-out")
    sc.tracks["g-sed"].reveal(t3, t3e)
    for t in parts:
        t.hold("rotate", t_sh)
        for k, v in enumerate((12, -12, 12, -12, 10, -10, 0)):
            t.set("rotate", t_sh + 0.25 + 0.4 * k, v, "in-out")
    sc.tracks["g-sed"].hold("opacity", t_sh + 0.3)
    sc.tracks["g-sed"].set("opacity", t_sh + 2.0, 0.0, "in-out")
    sc.tracks["g-dots"].hold("opacity", t_sh + 0.3)
    sc.tracks["g-dots"].set("opacity", t_sh + 2.0, 1.0, "in-out")
    sticker = (rect(110, 400, 420, 190, "#ffffff", 20, "#ef6c00", 8) + text(320, 480, "Ανακινήστε", 56, "#ef6c00") +
               text(320, 550, "καλά", 56, "#ef6c00"))
    st = ph.add("b-sticker", sticker, S("b1") + 0.3, 0.6, origin=[320, 495])
    st.wiggle(S("b1") + 1.2, 0.5, 2, 8, "rotate")
    ph.add("b-lab-sed", text(1230, 740, "ίζημα", 76, "#6d4c41", "middle") + arrow(1130, 745, 960, 790, "#6d4c41", 8),
           reveal_box=[900, 660, 450, 150], reveal_from="right").reveal(C("b4", "ίζημα") - 0.3, E("b4"))
    ph.close()

    # ---------- C: separation methods (one heading for the whole part)
    hp = Phase(sc, E("c11") + 0.3)
    head(hp, "h-sep", "Διαχωρισμός των συστατικών", "c1")
    hp.close()
    cols = ["#e53935", "#42a5f5", "#fdd835"]
    ph = Phase(sc, E("c3") + 0.3)
    mixd = "".join(circle(380 + rnd.randint(-120, 120), 600 + rnd.randint(0, 70), 13, cols[i % 3]) for i in range(30))
    ph.add("c-bowl", bowl_back(380, 580, 400, 190) + mixd +
           bowl_front(380, 580, 400, 190, "#ffcc80").replace("<path", '<path fill-opacity="0.25"', 2), S("c1") + 0.2, 0.5)
    ph.add("c-arrow", arrow(620, 640, 900, 640, "#2e7d32", 12), reveal_box=[600, 580, 320, 120]).reveal(S("c2") + 0.2, S("c2") + 1.2)
    sep_t = S("c2") + 1.2
    ph.add("c-out-red", bowl(1130, 600, 220, 100, "#ef9a9a") + "".join(circle(1090 + 25 * i, 560 + (i % 2) * 14, 16, "#e53935") for i in range(4)), sep_t, 0.5)
    ph.add("c-out-blue", bowl(1400, 600, 220, 100, "#90caf9") + "".join(circle(1360 + 25 * i, 560 + (i % 2) * 14, 16, "#42a5f5") for i in range(4)) +
           text(1260, 770, "ξεχωριστά συστατικά", 48, "#2e7d32"), sep_t + 0.4, 0.5)
    ph.close()

    # sorting
    ph = Phase(sc, E("c6") + 0.3)
    lent = "".join(circle(430 + rnd.randint(-150, 150), 640 + rnd.randint(-45, 15), 12, "#e08a1e", "#a65f00", 2) for i in range(40))
    stones = [(360, 600), (440, 580), (500, 620)]
    ph.add("s-bowl", bowl_back(430, 650, 470, 170) + lent + bowl_front(430, 650, 470, 170, "#8e24aa"), S("c4") + 0.2, 0.5)
    ph.add("s-plate", '<ellipse cx="1250" cy="730" rx="170" ry="34" fill="#b0bec5" stroke="#78909c" stroke-width="6"/>' +
           text(1250, 800, "πετραδάκια", 46, INK), S("c5") + 0.1, 0.5)
    for i, (x, y) in enumerate(stones):
        tr = ph.add("s-stone-%d" % i, pebble(x, y, 20, "#757575"), S("c4") + 0.2, 0.5)
        t0 = C("c5", "πετραδάκια", 0.2) + i * 0.8
        tr.move(t0, t0 + 0.35, y=-110, ease="out")
        tr.move(t0 + 0.35, t0 + 1.0, x=1200 + 40 * i - x, y=715 - y, ease="in-out")
    ph.add("s-lab-d", text(430, 360, "διαλογή", 80, "#2e7d32"), reveal_box=[200, 280, 460, 120]).reveal(C("c4", "διαλογή") - 0.3, E("c4"))
    ph.close()

    # filtering
    ph = Phase(sc, E("c9") + 0.3)
    gx2, gb2 = 800, 880
    ph.add("f-glass", glass_cup(gx2, gb2, 260, 260, "#ffb74d", 0.001), C("c7", "διήθηση", -0.2), 0.5)
    fill = rect(gx2 - 118, gb2 - 160, 236, 160, "#ffb74d", 0, None, 0, 'opacity="0.9"')
    f_liq = ph.add("f-liq", fill, reveal_box=[gx2 - 120, gb2 - 165, 240, 170], reveal_from="bottom")
    sieve = ('<ellipse cx="800" cy="430" rx="200" ry="30" fill="#eceff1" stroke="#78909c" stroke-width="7"/>' +
             path("M600 430 A200 150 0 0 0 1000 430 Z", "#cfd8dc", "#78909c", 7) +
             "".join(line(640 + 40 * i, 470, 700 + 36 * i, 560, "#90a4ae", 3) for i in range(7)) +
             rect(1000, 420, 150, 22, "#78909c", 11))
    ph.add("f-sieve", sieve, C("c7", "διήθηση", -0.2), 0.5)
    pulp = "".join(circle(740 + rnd.randint(0, 120), 430 + rnd.randint(-5, 20), rnd.randint(9, 14), "#ef6c00") for i in range(14))
    ph.add("f-pulp", pulp, C("c8", "χυμός") - 0.2, 0.5)
    ph.add("f-stream", rect(786, 200, 28, 215, "#ffb74d", 8), reveal_box=[780, 190, 40, 235], reveal_from="top").reveal(C("c8", "χυμός"), C("c8", "χυμός") + 1.0)
    for i, off in enumerate((0.0, 0.7)):
        tr = ph.add("f-drop-%d" % i, circle(790 + 15 * i, 580, 11, "#ffb74d"))
        loop_rise(tr, C("c8", "περνάει") + 0.6 + off, E("c9") - 0.1, 220, 1.5, 0.2)
    f_liq.reveal(C("c8", "περνάει"), E("c9") - 0.5)
    ph.add("f-lab-p", text(1150, 520, "μένουν πάνω", 46, "#bf360c") + arrow(1090, 495, 930, 440, "#bf360c", 6), C("c8", "μένουν") - 0.2, 0.5)
    ph.close()

    # complex methods
    ph = Phase(sc, E("c11") + 0.3)
    for i, nm in enumerate(("φυγοκέντριση", "απόσταξη", "χρωματογραφία")):
        y = 300 + i * 150
        ph.add("cx-%d" % i, rect(160, y - 70, 640, 110, "#e8eaf6", 30, "#5c6bc0", 6) + circle(220, y - 15, 24, "#5c6bc0") + text(280, y + 10, nm, 62, NAVY, "start"),
               C("c10", nm, -0.15), 0.5)
    fac = (rect(1000, 480, 440, 280, "#90a4ae", 8, "#546e7a", 6) + rect(1320, 340, 70, 150, "#78909c", 4, "#546e7a", 6) +
           "".join(rect(1040 + 100 * i, 540, 70, 70, "#fff59d", 6, "#546e7a", 4) for i in range(4)) +
           rect(1160, 660, 110, 100, "#546e7a", 6) + text(1220, 810, "ειδικές εγκαταστάσεις", 46, INK))
    ph.add("cx-fac", fac, C("c11", "ειδικές") - 0.2, 0.6)
    for i, (dx, dy) in enumerate(((0, 0), (30, -70))):
        tr = ph.add("cx-smoke-%d" % i, circle(1355 + dx, 300 + dy, 36 - 4 * i, "#eceff1", "#b0bec5", 4))
        loop_rise(tr, C("c11", "ειδικές") + 0.3 + i * 0.8, E("c11") + 0.2, -90, 2.0, 0.4)
    ph.close()

    # ---------- D: particles
    ph = Phase(sc, E("d3") + 0.5)
    head(ph, "h-part", "Τα σωματίδια του μίγματος", "d1")
    lb, lf = magnifier(800, 520, 260, "#e3f2fd")
    ph.add("d-lens", lb + lf, S("d1") + 0.2, 0.6)
    colsd = ["#e53935", "#1e88e5", "#43a047", "#fb8c00"]
    for i in range(5):
        a = i * 1.26 + 0.3
        rad = 60 + (i % 2) * 90
        px, py = 800 + rad * math.cos(a), 520 + rad * math.sin(a)
        grp = circle(px, py, 24, colsd[i % 4], "#37474f", 3) + circle(px + 38, py + 22, 24, colsd[(i + 1) % 4], "#37474f", 3)
        tr = ph.add("d-p-%d" % i, grp, S("d1") + 0.5 + 0.1 * i, 0.5)
        tt = C("d2", "μόρια") - 0.5
        x = y = 0
        while tt < E("d3") + 0.3:
            nx, ny = rnd.randint(-60, 60), rnd.randint(-60, 60)
            tr.set("x", tt, x, "in-out") if tr.last_t("x") < tt else None
            tr.set("y", tt, y, "in-out") if tr.last_t("y") < tt else None
            x, y = nx, ny
            tt += 0.5
            tr.set("x", tt, x, "in-out")
            tr.set("y", tt, y, "in-out")
    ph.close()

    # ---------- E: rubbish
    ph = Phase(sc, E("e7") + 0.5)
    head(ph, "h-rub", "Τα σκουπίδια", "e1")
    bins = [("glass", 760, "#2e7d32", "γυαλί"), ("paper", 1000, "#1565c0", "χαρτί"),
            ("alu", 1240, "#f9a825", "αλουμίνιο"), ("food", 1480, "#6d4c41", "φαγητό")]
    for nm, x, col, lab in bins:
        svg = (rect(x - 90, 480, 180, 250, col, 16, "#263238", 5) + rect(x - 105, 455, 210, 40, col, 12, "#263238", 5) +
               rect(x - 60, 540, 120, 14, "#ffffff", 7, None, 0, 'opacity="0.5"') + text(x, 785, lab, 42, INK))
        ph.add("bin-" + nm, svg, C("e5", "γυαλί") - 1.5 if nm != "food" else C("e6", "υπολείμματα") - 1.0, 0.5)
    pile = ""
    for i in range(18):
        pile += rect(round(110 + rnd.random() * 400), round(560 + rnd.random() * 170), rnd.randint(30, 60), rnd.randint(20, 40),
                     rnd.choice(["#90a4ae", "#a1887f", "#bcaaa4", "#78909c", "#c5e1a5"]), 6, "#455a64", 3)
    ph.add("r-pile", pile, S("e1") + 0.3, 0.6)
    items = [("glass", 220, 560, bottle_icon, C("e5", "γυαλί", 0.0), 760),
             ("paper", 330, 560, paper_icon, C("e5", "χαρτί", 0.0), 1000),
             ("alu", 440, 560, can_icon, C("e5", "αλουμίνιο", 0.0), 1240),
             ("food", 550, 560, peel_icon, C("e6", "υπολείμματα", 0.2), 1480)]
    for nm, x, y, fn, t0, bx in items:
        tr = ph.add("r-" + nm, fn(x, y), S("e2") + 0.2, 0.5, origin=[x, y])
        tr.move(t0, t0 + 0.6, y=-120, ease="out")
        tr.move(t0 + 0.6, t0 + 1.6, x=bx - x, y=-60, ease="in-out")
        tr.hold("opacity", t0 + 1.4)
        tr.set("opacity", t0 + 1.8, 0, "in")
    ph.close()

    # ---------- F: remember
    ph = Phase(sc, E("f2") + 1.8, 0.6)
    head(ph, "h-rem", "Θυμήσου", "f1")
    ph.add("rem-1", rect(200, 280, 1200, 170, "#e8f5e9", 36, "#43a047", 7) +
           text(800, 395, "ετερογενές μίγμα: βλέπουμε τα συστατικά", 54, "#1b5e20"), S("f1") + 0.1, 0.6)
    ph.add("rem-2", rect(200, 520, 1200, 170, "#fff3e0", 36, "#fb8c00", 7) +
           text(800, 635, "τα ξεχωρίζουμε: διαλογή ή διήθηση", 54, "#e65100"), S("f2") - 0.1, 0.6)
    ph.close()
    return sc


def bottle_icon(x, y):
    return (path("M%s %s L%s %s Q%s %s %s %s L%s %s L%s %s L%s %s Q%s %s %s %s L%s %s Z" % (
        x - 12, y - 60, x - 12, y - 25, x - 28, y - 10, x - 28, y + 10, x - 28, y + 50, x + 28, y + 50, x + 28, y + 10,
        x + 28, y - 10, x + 12, y - 25, x + 12, y - 60), "#81c784", "#2e7d32", 5) + rect(x - 14, y - 70, 28, 14, "#8d6e63", 3))


def paper_icon(x, y):
    return (rect(x - 40, y - 35, 80, 70, "#fafafa", 4, "#607d8b", 5) +
            "".join(line(x - 28, y - 18 + 14 * i, x + 28, y - 18 + 14 * i, "#90a4ae", 4) for i in range(4)))


def can_icon(x, y):
    return (rect(x - 24, y - 36, 48, 72, "#cfd8dc", 6, "#546e7a", 5) + rect(x - 24, y - 12, 48, 24, "#ef5350", 0) +
            '<ellipse cx="%s" cy="%s" rx="24" ry="7" fill="#b0bec5" stroke="#546e7a" stroke-width="4"/>' % (x, y - 36))


def peel_icon(x, y):
    return (path("M%s %s Q%s %s %s %s Q%s %s %s %s Q%s %s %s %s Z" % (x - 40, y + 10, x - 10, y - 50, x + 40, y - 20, x + 10, y - 5, x + 30, y + 30, x - 5, y + 10, x - 40, y + 10),
                 "#fdd835", "#f9a825", 5))
