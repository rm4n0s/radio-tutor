"""Lesson 1: what a mixture is."""
from lib import *
from icons import *

ID = "ti-einai-migma"
TITLE = "Τι είναι ένα μίγμα;"
SUMMARY = "Καθαρές ουσίες και μίγματα: τα συστατικά, οι ιδιότητες και παραδείγματα από την κουζίνα."

SENTENCES = [
    ("s1", "Γεια σου! Σήμερα θα γνωρίσουμε τα μίγματα."),
    ("s2", "Τα υλικά γύρω μας είναι δύο ειδών: καθαρές ουσίες και μίγματα.", 1.2),
    # pure substances
    ("s3", "Ας ξεκινήσουμε με τις καθαρές ουσίες."),
    ("s4", "Το αλουμίνιο στα παράθυρα είναι καθαρή ουσία.", 0.8),
    ("s5", "Το ίδιο και ο χαλκός στα σύρματα.", 0.8),
    ("s6", "Και ο υδράργυρος στο θερμόμετρο.", 0.8),
    ("s7", "Αυτές οι ουσίες λέγονται χημικά στοιχεία.", 1.0),
    ("s8", "Καθαρή ουσία είναι και το αποσταγμένο νερό."),
    ("s9", "Τέτοιες ουσίες λέγονται χημικές ενώσεις.", 1.2),
    # mixtures
    ("s10", "Τις περισσότερες φορές όμως τα υλικά δεν είναι καθαρές ουσίες."),
    ("s11", "Είναι μίγματα.", 1.0),
    ("s12", "Όταν ανακατεύουμε δύο ή περισσότερες ουσίες, φτιάχνουμε ένα μίγμα.", 1.0),
    ("s13", "Οι ουσίες που ανακατεύουμε λέγονται συστατικά του μίγματος.", 1.5),
    # soda
    ("s14", "Ας δούμε ένα αναψυκτικό."),
    ("s15", "Στην ετικέτα του διαβάζουμε τα συστατικά του.", 0.9),
    ("s16", "Το νερό είναι υγρό.", 0.8),
    ("s17", "Η ζάχαρη είναι στερεή.", 0.8),
    ("s18", "Το διοξείδιο του άνθρακα είναι αέριο.", 0.9),
    ("s19", "Το αναψυκτικό είναι μίγμα από υγρό, στερεό και αέριο.", 1.5),
    # properties
    ("s20", "Ένα μίγμα κρατάει τις ιδιότητες των συστατικών του.", 0.9),
    ("s21", "Το αλάτι είναι αλμυρό.", 0.8),
    ("s22", "Αν το ρίξουμε στο νερό, το αλατόνερο είναι κι αυτό αλμυρό.", 1.2),
    ("s23", "Η ζάχαρη είναι γλυκιά και ο καφές έχει σκούρο χρώμα.", 0.8),
    ("s24", "Ο καφές με ζάχαρη είναι σκούρος και γλυκός.", 1.5),
    # kitchen
    ("s25", "Στην κουζίνα φτιάχνουμε συχνά μίγματα."),
    ("s26", "Η σαλάτα, το λαδόξιδο, ο χυμός πορτοκαλιού και το τσάι είναι μίγματα.", 1.2),
    ("s27", "Μίγμα φτιάχνουμε όταν ανακατεύουμε δύο ή περισσότερες ουσίες."),
    ("s28", "Στο επόμενο μάθημα θα δούμε πώς είναι τα μίγματα."),
]


import random


def build(nar):
    sc = Scene("#e8f5fe", "#fff8e1")
    sc.static(rect(0, 800, 1600, 100, "#ffe0b2", 0, None, 0, 'opacity="0.7"'))
    S, E, C = nar.s, nar.e, nar.cue

    def head(ph, id_, s, k):
        ph.add(id_, text(800, 105, s, 68, NAVY), S(k) - 0.1)

    # ---- phase 0: two kinds of materials
    ph = Phase(sc, E("s2") + 0.3)
    tr = ph.add("title", text(800, 210, "Μίγματα", 130, NAVY), 0.3, 0.8, origin=[800, 170])
    tr.scale(0.3, 1.1, 1.0)
    ta, tb = C("s2", "καθαρές") - 0.1, C("s2", "μίγματα") - 0.1
    ph.add("card-a", rect(220, 330, 500, 420, "#bbdefb", 40, "#1e88e5", 8) + coil(470, 470, 1.1) +
           text(470, 680, "καθαρές ουσίες", 54, "#0d47a1"), ta, 0.6)
    ph.add("card-b", rect(880, 330, 500, 420, "#ffe0b2", 40, "#fb8c00", 8) + bowl(1130, 470, 340, 140) +
           text(1130, 680, "μίγματα", 54, "#e65100"), tb, 0.6)
    ph.close()

    # ---- phase 1: pure substances
    ph = Phase(sc, E("s9") + 0.3)
    head(ph, "h-pure", "Καθαρές ουσίες", "s3")
    items = [("window", 250, window(250, 440), "αλουμίνιο", "s4"),
             ("coil", 600, coil(600, 440, 1.2), "χαλκός", "s5"),
             ("thermo", 950, thermometer(950, 440), "υδράργυρος", "s6"),
             ("drop", 1340, drop(1340, 470, 80), "αποσταγμένο νερό", "s8")]
    for name, x, svg, lab, k in items:
        t0 = S(k) + 0.1
        ph.add("pure-" + name, svg, t0, 0.5)
        ph.add("pure-%s-l" % name, text(x, 730, lab, 46, INK), t0 + 0.3, 0.5)
    br1 = (path("M130 300 L130 270 L1070 270 L1070 300", "none", "#2e7d32", 8) +
           text(600, 235, "χημικά στοιχεία", 52, "#2e7d32"))
    ph.add("br-elements", br1, reveal_box=[120, 190, 960, 120]).reveal(C("s7", "χημικά") - 0.2, E("s7"))
    br2 = (path("M1190 300 L1190 270 L1490 270 L1490 300", "none", "#6a1b9a", 8) +
           text(1340, 235, "χημική ένωση", 52, "#6a1b9a"))
    ph.add("br-compound", br2, reveal_box=[1180, 190, 330, 120]).reveal(C("s9", "χημικές") - 0.2, E("s9"))
    ph.close()

    # ---- phase 2: a mixture
    ph = Phase(sc, E("s13") + 0.4)
    head(ph, "h-mix", "Το μίγμα", "s10")
    cols = ["#e53935", "#42a5f5", "#fdd835"]
    t_in = S("s10") + 0.2
    iy = 290
    ing = [("red", 450, "".join(circle(450 + dx, iy + dy, 38, "#e53935", "#b71c1c", 5) for dx, dy in ((-42, 28), (42, 28), (0, -36)))),
           ("blue", 800, drop(800, iy + 10, 70)),
           ("yellow", 1150, sugar_pile(1150, iy, 1.6).replace("#fff8e1", "#fdd835").replace("#bcaaa4", "#f9a825"))]
    for name, x, svg in ing:
        ph.add("ing-" + name, svg, t_in, 0.6)
    by = 700
    ph.add("bowl-back", bowl_back(800, by, 520, 190), t_in, 0.6)
    rnd = random.Random(3)
    dots = "".join(circle(800 + rnd.randint(-170, 170), by + 5 + rnd.randint(0, 90), rnd.randint(11, 17), cols[i % 3])
                   for i in range(46))
    mix = ph.add("mix-dots", dots, reveal_box=[600, by - 10, 400, 130], reveal_from="bottom")
    ph.add("bowl-front", bowl_front(800, by, 520, 190, "#ffcc80").replace("<path", '<path fill-opacity="0.25"', 2), t_in, 0.6)
    ph.add("lab-mix", text(1330, by + 25, "μίγμα", 84, "#e65100") + arrow(1190, by + 5, 1080, by + 5, "#e65100", 8),
           reveal_box=[1060, by - 70, 520, 140], reveal_from="right").reveal(S("s11") + 0.1, S("s11") + 0.9)
    tm = C("s12", "ανακατεύουμε", 0.2)
    for i, (name, x, svg) in enumerate(ing):
        dot = circle(x, iy, 30, cols[i]) if name != "blue" else drop(x, iy + 6, 32)
        d = i * 0.35
        tr = ph.add("fly-" + name, dot, tm + d, 0.15)
        tr.move(tm + d, tm + d + 1.5, x=800 - x + (i - 1) * 60, y=by - iy + 20)
        tr.hide(tm + d + 1.3, 0.2)
    mix.reveal(tm + 1.4, tm + 2.6)
    brk = (path("M330 450 L330 480 L1270 480 L1270 450", "none", "#2e7d32", 8) +
           text(800, 555, "συστατικά του μίγματος", 56, "#2e7d32"))
    ph.add("br-comp", brk, reveal_box=[310, 430, 980, 160]).reveal(C("s13", "συστατικά") - 0.2, E("s13") - 0.3)
    ph.close()

    # ---- phase 3: a soft drink
    t_end = E("s19") + 1.0
    ph = Phase(sc, t_end, 0.5)
    head(ph, "h-soda", "Το αναψυκτικό", "s14")
    ph.add("bottle", soda_bottle(420, 850),
           S("s14") + 0.1, 0.6, origin=[420, 700])
    ph.add("card", rect(760, 230, 760, 560, "#ffffff", 30, "#90a4ae", 6) + text(1140, 310, "Συστατικά", 56, NAVY),
           reveal_box=[750, 220, 780, 580], reveal_from="top").reveal(S("s15") + 0.2, S("s15") + 1.2)
    rows = [("nero", 410, drop(850, 415, 34), "νερό", "υγρό", "#1565c0", "s16"),
            ("zahari", 560, cube(850, 560, 56), "ζάχαρη", "στερεό", "#bf6a00", "s17"),
            ("co2", 710, circle(850, 715, 22, "#ffffff", "#78909c", 5) + circle(868, 690, 12, "#ffffff", "#78909c", 4),
             "διοξείδιο του άνθρακα", "αέριο", "#2e7d32", "s18")]
    for name, y, ic, nm, st, col, k in rows:
        ph.add("row-" + name, ic + text(920, y - 4, nm, 42, INK, "start") + text(920, y + 46, st, 44, col, "start"),
               S(k) + 0.1, 0.5)
    for i, (bx, by, off) in enumerate([(395, 780, 0.0), (440, 800, 0.5), (415, 740, 1.0), (455, 690, 1.4)]):
        tr = ph.add("bubble-%d" % i, circle(bx, by, 12, "#ffffff", "#ffe0b2", 3) +
                    circle(bx + 25, by + 40, 8, "#ffffff", "#ffe0b2", 3))
        loop_rise(tr, S("s18") + off, t_end - 0.2, -230)
    ph.close()

    # ---- phase 4: properties are kept
    ph = Phase(sc, E("s24") + 1.0, 0.5)
    head(ph, "h-prop", "Οι ιδιότητες μένουν", "s20")
    ya, yb = 340, 670

    def tag(id_, x, y, s, t, col, anchor="middle", size=46):
        ph.add(id_, text(x, y, s, size, col, anchor), t, 0.5)

    def op(id_, x, y, s, t):
        ph.add(id_, text(x, y + 25, s, 80, INK), t, 0.4)
    # row A: salt + water = salt water
    ph.add("a-salt", salt_shaker(250, ya, 1.1), S("s21") + 0.1)
    tag("a-salt-l", 250, ya + 150, "αλμυρό", C("s21", "αλμυρό"), "#0277bd")
    op("a-plus", 430, ya, "+", C("s22", "νερό", -0.2))
    ph.add("a-water", glass_cup(610, ya + 110, 190, 230, "#64b5f6", 0.7), C("s22", "νερό", -0.2))
    op("a-eq", 800, ya, "=", C("s22", "αλατόνερο", -0.3))
    salty = glass_cup(990, ya + 110, 190, 230, "#80deea", 0.7) + "".join(
        circle(940 + 20 * i, ya + 60 + (i * 37) % 70, 5, "#ffffff") for i in range(8))
    ph.add("a-res", salty, C("s22", "αλατόνερο", -0.3))
    tag("a-res-l", 1260, ya + 20, "αλμυρό", C("s22", "κι αυτό"), "#0277bd", "start", 56)
    # row B: sugar + coffee = sweet dark coffee
    ph.add("b-sugar", cube(250, yb - 20, 90) + cube(300, yb + 30, 70) + cube(200, yb + 40, 60), S("s23") + 0.1)
    tag("b-sugar-l", 250, yb + 150, "γλυκιά", C("s23", "γλυκιά"), "#bf6a00")
    ph.add("b-coffee", cup(610, yb, "#4e342e", 1.0), C("s23", "ο καφές"))
    tag("b-coffee-l", 610, yb + 150, "σκούρος", C("s23", "σκούρο"), "#4e342e")
    op("b-plus", 430, yb, "+", C("s23", "ο καφές", -0.4))
    op("b-eq", 800, yb, "=", S("s24") + 0.2)
    ph.add("b-res", cup(990, yb, "#3e2723", 1.0) + cube(1030, yb - 52, 34), S("s24") + 0.4)
    tag("b-res-l", 1140, yb, "σκούρος", C("s24", "σκούρος"), "#4e342e", "start", 52)
    tag("b-res-l2", 1140, yb + 60, "και γλυκός", C("s24", "γλυκός", -0.2), "#bf6a00", "start", 52)
    ph.close()

    # ---- phase 5: kitchen
    ph = Phase(sc, E("s28") + 1.0, 0.8)
    head(ph, "h-kit", "Μίγματα στην κουζίνα", "s25")
    greens = "".join(circle(250 + dx, 465 + dy, r, c) for dx, dy, r, c in (
        (-90, 0, 40, "#66bb6a"), (-30, -25, 46, "#43a047"), (40, -10, 44, "#81c784"), (95, 5, 36, "#66bb6a"),
        (-50, 10, 22, "#e53935"), (60, 15, 20, "#e53935"), (10, -45, 18, "#fdd835")))
    salad = bowl_back(250, 520, 340, 150, "#a5d6a7") + greens + bowl_front(250, 520, 340, 150, "#26a69a")
    vin = (rect(570, 330, 100, 270, "#ffe082", 22, "#8d6e63", 6) + rect(570, 470, 100, 130, "#8d4a2f", 22) +
           rect(588, 290, 64, 50, "#d7ccc8", 8, "#8d6e63", 5) + rect(580, 270, 80, 28, "#c62828", 8) +
           rect(570, 465, 100, 12, "#6d3a22", 0))
    oj = glass_cup(990, 600, 200, 260, "#ffa726", 0.8) + orange_slice(1085, 350, 44)
    tea = cup(1350, 500, "#c68642", 1.0) + line(1350, 450, 1350, 400, "#78909c", 4) + rect(1330, 370, 40, 40, "#fff59d", 4, "#fbc02d", 3)
    for name, x, svg, lab, word in (("salad", 250, salad, "σαλάτα", "σαλάτα"), ("vin", 620, vin, "λαδόξιδο", "λαδόξιδο"),
                                    ("oj", 990, oj, "χυμός πορτοκαλιού", "χυμός"), ("tea", 1350, tea, "τσάι", "τσάι")):
        t0 = C("s26", word, -0.1)
        ph.add("k-" + name, svg, t0)
        ph.add("k-%s-l" % name, text(x, 740, lab, 46, INK), t0 + 0.2)
    ph.add("k-final", text(800, 835, "μίγμα = δύο ή περισσότερες ουσίες", 52, "#e65100"),
           reveal_box=[300, 790, 1000, 80]).reveal(C("s27", "δύο") - 0.6, E("s27"))
    ph.close()
    return sc
