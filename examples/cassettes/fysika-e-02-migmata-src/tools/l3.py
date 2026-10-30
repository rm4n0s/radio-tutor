"""Lesson 3: solutions."""
import random
from lib import *
from icons import *

ID = "dialymata"
TITLE = "Διαλύματα"
SUMMARY = "Ομογενή μίγματα, διαλύτης και διαλυμένες ουσίες, κράματα, πόση ουσία διαλύεται και το νερό ως διαλύτης."

SENTENCES = [
    ("g1", "Σε μερικά μίγματα δεν μπορούμε να δούμε τα συστατικά, ούτε με μικροσκόπιο."),
    ("g2", "Τα λέμε ομογενή μίγματα, ή αλλιώς διαλύματα.", 1.0),
    ("g3", "Ρίχνουμε αλάτι στο νερό και ανακατεύουμε."),
    ("g4", "Το αλάτι διαλύεται, και δεν το βλέπουμε πια.", 1.2),
    ("g5", "Το αλατόνερο, το κρασί και ο αέρας που αναπνέουμε είναι διαλύματα.", 1.5),
    ("g6", "Το συστατικό με τη μεγαλύτερη ποσότητα λέγεται διαλύτης.", 0.9),
    ("g7", "Τα υπόλοιπα συστατικά λέγονται διαλυμένες ουσίες.", 1.0),
    ("g8", "Στο τσάι με ζάχαρη, το νερό είναι ο διαλύτης και η ζάχαρη η διαλυμένη ουσία.", 1.5),
    ("g9", "Δεν είναι όλα τα διαλύματα υγρά."),
    ("g10", "Ο καθαρός αέρας είναι ένα αέριο διάλυμα.", 0.8),
    ("g11", "Τα χρυσά κοσμήματα είναι στερεά διαλύματα."),
    ("g12", "Φτιάχνονται από χρυσό ανακατεμένο με άλλο μέταλλο, συνήθως χαλκό.", 0.8),
    ("g13", "Τα μίγματα των μετάλλων λέγονται κράματα.", 0.9),
    ("g14", "Ο καθαρός χρυσός είναι πολύ μαλακός και φθείρεται γρήγορα.", 1.5),
    ("g15", "Το χάλκινο μετάλλιο είναι από μπρούντζο, ένα κράμα χαλκού και κασσίτερου.", 1.5),
    ("g16", "Μπορούμε να διαλύσουμε όση ζάχαρη θέλουμε στο τσάι;", 0.9),
    ("g17", "Δεν μπορούμε. Αν ρίχνουμε συνέχεια ζάχαρη, κάποια στιγμή δεν διαλύεται πια."),
    ("g18", "Κατακάθεται στον πάτο του ποτηριού."),
    ("g19", "Αυτή η ποσότητα λέγεται ίζημα.", 1.5),
    ("g20", "Η ποσότητα που διαλύεται εξαρτάται από τον όγκο και τη θερμοκρασία του διαλύτη.", 1.0),
    ("g21", "Όσο περισσότερος είναι ο διαλύτης, τόσο περισσότερη ουσία διαλύεται.", 1.5),
    ("g22", "Και όσο πιο ζεστός είναι, τόσο περισσότερη.", 1.0),
    ("g23", "Στο ζεστό τσάι διαλύεται περισσότερη ζάχαρη απ' ό,τι στο κρύο.", 0.8),
    ("g24", "Προσοχή όμως: η πολλή ζάχαρη κάνει κακό στα δόντια.", 1.5),
    ("g25", "Όταν ζεσταίνεται το τσάι, τα μόρια κινούνται πιο γρήγορα."),
    ("g26", "Έτσι ανακατεύονται πιο εύκολα με τα μόρια του διαλύτη.", 1.5),
    ("g27", "Το νερό είναι ο καλύτερος διαλύτης, και το λέμε παγκόσμιο διαλύτη.", 0.9),
    ("g28", "Διαλύει στερεά, υγρά και αέρια.", 1.0),
    ("g29", "Στη θάλασσα έχουν διαλυθεί αλάτι και οξυγόνο.", 0.9),
    ("g30", "Τα ψάρια παίρνουν το οξυγόνο από το νερό με τα βράγχιά τους.", 1.5),
    ("g31", "Το μόριο του νερού έχει δύο άτομα υδρογόνου και ένα άτομο οξυγόνου.", 1.5),
    ("g32", "Θυμήσου: στο διάλυμα δεν βλέπουμε τα συστατικά."),
    ("g33", "Έχει έναν διαλύτη και διαλυμένες ουσίες."),
    ("g34", "Διαλύεται περισσότερη ουσία όταν ο διαλύτης είναι πιο πολύς και πιο ζεστός."),
]


def wine_glass(cx, cy, wine="#8e1b3a"):
    return (path("M%s %s L%s %s Q%s %s %s %s Q%s %s %s %s Z" % (cx - 70, cy - 100, cx + 70, cy - 100, cx + 70, cy + 10, cx, cy + 35, cx - 70, cy + 10, cx - 70, cy - 100), "#fafafa", "#78909c", 6) +
            path("M%s %s L%s %s Q%s %s %s %s Q%s %s %s %s Z" % (cx - 68, cy - 50, cx + 68, cy - 50, cx + 68, cy + 10, cx, cy + 33, cx - 68, cy + 10, cx - 68, cy - 50), wine) +
            rect(cx - 6, cy + 30, 12, 85, "#cfd8dc", 3, "#78909c", 4) + '<ellipse cx="%s" cy="%s" rx="55" ry="12" fill="#cfd8dc" stroke="#78909c" stroke-width="4"/>' % (cx, cy + 118))


def cloud(cx, cy):
    return ("".join(circle(cx + dx, cy + dy, r, "#ffffff", "#90a4ae", 5) for dx, dy, r in ((-60, 10, 45), (0, -20, 60), (65, 5, 48), (20, 25, 40))) +
            "".join(line(cx - 110 + 20 * i, cy + 80 + 18 * i, cx + 50 + 20 * i, cy + 80 + 18 * i, "#90caf9", 6) for i in range(2)))


def ring(cx, cy):
    out = circle(cx, cy, 95, "none", "#fbc02d", 30)
    out += circle(cx, cy, 95, "none", "#ffe082", 8, 'stroke-dasharray="40 30"')
    for k in range(10):
        a = k * 0.628
        out += circle(cx + 95 * math.cos(a), cy + 95 * math.sin(a), 9, "#fbc02d" if k % 2 else "#c2703a")
    out += path("M%s %s L%s %s L%s %s L%s %s Z" % (cx - 28, cy - 95 - 40, cx + 28, cy - 95 - 40, cx + 14, cy - 95 - 10, cx - 14, cy - 95 - 10), "#4fc3f7", "#0288d1", 4)
    return out


def medal(cx, cy, col, edge, rib):
    return (path("M%s %s L%s %s L%s %s L%s %s Z" % (cx - 40, cy - 130, cx - 5, cy - 130, cx + 15, cy - 50, cx - 25, cy - 50), rib) +
            path("M%s %s L%s %s L%s %s L%s %s Z" % (cx + 40, cy - 130, cx + 5, cy - 130, cx - 15, cy - 50, cx + 25, cy - 50), rib, extra='opacity="0.8"') +
            circle(cx, cy, 70, col, edge, 8) + circle(cx, cy, 48, "none", edge, 5))


def tooth(cx, cy):
    d = ("M{0} {1} Q{2} {3} {4} {5} Q{6} {7} {8} {9} Q{10} {11} {12} {13} L{14} {15} Q{16} {17} {18} {19} "
         "Q{20} {21} {22} {23} L{24} {25} Q{26} {27} {28} {29} Z").format(
        cx - 70, cy - 40, cx - 70, cy - 95, cx - 20, cy - 85, cx, cy - 75, cx + 20, cy - 85, cx + 70, cy - 95, cx + 70, cy - 40,
        cx + 55, cy + 40, cx + 40, cy + 110, cx + 25, cy + 60, cx, cy + 40, cx - 25, cy + 60, cx - 40, cy + 110, cx - 55, cy + 40,
        cx - 70, cy - 40, cx - 70, cy - 40)
    return path(d, "#ffffff", "#78909c", 6)


def warn(cx, cy):
    return (path("M%s %s L%s %s L%s %s Z" % (cx, cy - 60, cx + 60, cy + 45, cx - 60, cy + 45), "#ffeb3b", "#c62828", 8) + text(cx, cy + 28, "!", 70, "#c62828"))


def walk(tr, t0, t1, amp, step, rnd):
    """Random walk of x and y between t0 and t1."""
    x = y = 0
    t = t0
    tr.hold("x", t)
    tr.hold("y", t)
    while t + step <= t1:
        t += step
        x, y = rnd.randint(-amp, amp), rnd.randint(-amp, amp)
        tr.set("x", t, x, "in-out")
        tr.set("y", t, y, "in-out")


def build(nar):
    sc = Scene("#e8f0fe", "#fff9e6")
    sc.static(rect(0, 820, 1600, 80, "#cfd8dc", 0, None, 0, 'opacity="0.55"'))
    S, E, C = nar.s, nar.e, nar.cue
    rnd = random.Random(5)
    # A scene may have at most 64 actors (CASSETTE-FORMAT 5.1): one heading per part, related drawings share an actor.

    def head(t_end, id_, s, k, dt=-0.1):
        hp = Phase(sc, t_end)
        hp.add(id_, text(800, 105, s, 68, NAVY), S(k) + dt)
        hp.close()

    # ---------- A: homogeneous mixture, salt dissolves
    head(E("g5") + 0.5, "h-hom", "Ομογενή μίγματα", "g1")
    ph = Phase(sc, E("g4") + 0.4)
    ph.add("a-glass", glass_cup(520, 800, 300, 340, "#81d4fa", 0.75), S("g1") + 0.3, 0.6)
    grains = "".join(circle(430 + 18 * i + rnd.randint(-6, 6), 330 + rnd.randint(-30, 30), 10, "#ffffff", "#90a4ae", 3) for i in range(9))
    gr = ph.add("a-grains", grains)
    t3 = S("g3") + 0.3
    gr.set("opacity", 0, 0)
    gr.set("opacity", t3, 0)
    gr.set("opacity", t3 + 0.3, 1, "out")
    gr.set("y", t3, 0)
    gr.set("y", t3 + 1.6, 260, "in")
    gr.set("opacity", t3 + 1.7, 1, "linear")
    gr.set("opacity", C("g4", "διαλύεται") + 1.5, 0, "in-out")
    lb, lf = magnifier(1100, 500, 190, "#b3e5fc")
    ph.add("a-lens", lb + lf, S("g4") + 0.2, 0.6)
    ph.add("a-lens-t", text(1100, 800, "δεν ξεχωρίζουν τα συστατικά", 44, "#0277bd"), S("g4") + 1.5, 0.6)
    ph.add("a-lab", text(1100, 250, "ομογενές = διάλυμα", 62, "#2e7d32"), reveal_box=[760, 190, 680, 90]).reveal(C("g2", "ομογενή") - 0.2, E("g2"))
    ph.close()

    ph = Phase(sc, E("g5") + 0.5)
    ex = [("e-salt", 300, glass_cup(300, 740, 220, 250, "#81d4fa", 0.75), "αλατόνερο", "αλατόνερο"),
          ("e-wine", 800, wine_glass(800, 600), "κρασί", "κρασί"),
          ("e-air", 1300, cloud(1300, 580), "αέρας", "αέρας")]
    for nm, x, svg, lab, w in ex:
        ph.add(nm, svg + text(x, 800, lab, 56, INK), C("g5", w, -0.2), 0.6)
    ph.close()

    # ---------- B: solvent and dissolved substance
    ph = Phase(sc, E("g8") + 0.6)
    ph.add("h-solv", text(800, 105, "Διαλύτης και διαλυμένη ουσία", 68, NAVY), S("g6") - 0.1)
    sp = [(520 + rnd.randint(-130, 130), 520 + rnd.randint(-14, 14)) for _ in range(10)]
    tea = (cup(520, 620, "#c68642", 2.0) + "".join(circle(x, y, 8, "#ffe0b2", None, 0, 'opacity="0.95"') for x, y in sp))
    ph.add("b-tea", tea, S("g6") + 0.2, 0.7)
    ph.add("b-solv", text(900, 360, "διαλύτης", 76, "#1565c0", "start") + text(900, 430, "το νερό", 52, "#1565c0", "start") +
           arrow(880, 395, 700, 505, "#1565c0", 8), reveal_box=[860, 280, 600, 190]).reveal(C("g6", "διαλύτης") - 0.1, E("g6"))
    ph.add("b-diss", text(900, 660, "διαλυμένη ουσία", 70, "#bf6a00", "start") + text(900, 730, "η ζάχαρη", 52, "#bf6a00", "start") +
           arrow(880, 650, 640, 540, "#bf6a00", 8), reveal_box=[860, 580, 700, 190]).reveal(S("g7") + 0.2, S("g7") + 1.4)
    ph.close()

    # ---------- C: gas and solid solutions, metals
    head(E("g15") + 1.5, "h-types", "Υγρά, αέρια και στερεά διαλύματα", "g9")
    ph = Phase(sc, E("g14") + 0.4)
    cards = [("liq", 280, "υγρό", "#e3f2fd", "#1e88e5", glass_cup(280, 640, 190, 220, "#81d4fa", 0.75), "αλατόνερο", "g9", "Δεν"),
             ("gas", 800, "αέριο", "#f1f8e9", "#43a047", cloud(800, 520), "ο καθαρός αέρας", "g10", "αέρας"),
             ("sol", 1320, "στερεό", "#fff8e1", "#f9a825", ring(1320, 540), "χρυσό κόσμημα", "g11", "χρυσά")]
    for nm, x, ttl, bg, ec, ic, lab, k, w in cards:
        t0 = C(k, w, -0.2) if nm != "liq" else S("g9") + 1.4
        ph.add("c-" + nm, rect(x - 230, 200, 460, 560, bg, 36, ec, 8) + text(x, 290, ttl, 64, ec) + ic + text(x, 720, lab, 44, INK), t0, 0.7)
    ph.add("c-krama", text(1320, 812, "κράμα", 56, "#e65100"), reveal_box=[1170, 762, 300, 65]).reveal(C("g13", "κράματα") - 0.2, E("g13"))
    ph.close()

    ph = Phase(sc, E("g15") + 1.5)
    meds = [("gold", 400, "#ffd54f", "#f9a825", "#c62828", "χρυσό"), ("silver", 800, "#cfd8dc", "#78909c", "#1565c0", "αργυρό"),
            ("bronze", 1200, "#cd7f32", "#8d4e1a", "#2e7d32", "χάλκινο")]
    for i, (nm, x, col, edge, rib, lab) in enumerate(meds):
        ph.add("m-" + nm, medal(x, 470, col, edge, rib) + text(x, 640, lab, 52, INK), S("g15") + 0.1 + 0.4 * i, 0.6)
    ph.add("m-note", text(1200, 730, "μπρούντζος = χαλκός + κασσίτερος", 46, "#8d4e1a"), reveal_box=[780, 690, 820, 70]).reveal(C("g15", "μπρούντζο") - 0.2, E("g15"))
    ph.close()

    # ---------- D: limit and sediment
    ph = Phase(sc, E("g19") + 0.5)
    ph.add("h-lim", text(800, 105, "Το όριο της διάλυσης", 68, NAVY), S("g16") - 0.1)
    ph.add("d-glass", glass_cup(800, 810, 320, 400, "#ffcc80", 0.8), S("g16") + 0.2, 0.6)
    t_stop = C("g17", "δεν διαλύεται")
    for i, (cx, off) in enumerate(((770, 0.0), (810, 0.9))):
        tr = ph.add("d-cube-%d" % i, cube(cx, 360, 50), None)
        loop_rise(tr, S("g16") + 1.5 + off, t_stop, 330, 1.8, 0.25)
    sed = "".join(cube(700 + 36 * k + (k % 2) * 6, 790 - (k % 3) * 8, 34) for k in range(7))
    ph.add("d-sed", sed, reveal_box=[690, 740, 260, 70], reveal_from="bottom").reveal(t_stop, t_stop + 1.4)
    ph.add("d-q", text(330, 480, "όση θέλουμε;", 64, "#6a1b9a"), reveal_box=[130, 420, 440, 90]).reveal(S("g16") + 0.8, S("g16") + 2.0)
    ph.add("d-lab", text(1250, 740, "ίζημα", 80, "#8d6e63") + arrow(1130, 750, 960, 790, "#8d6e63", 8),
           reveal_box=[900, 660, 450, 150], reveal_from="right").reveal(C("g19", "ίζημα") - 0.3, E("g19"))
    ph.close()

    # ---------- E: volume and temperature
    head(E("g24") + 0.7, "h-amount", "Πόση ουσία διαλύεται;", "g20")
    ph = Phase(sc, E("g21") + 0.5)
    ph.add("v-glasses", glass_cup(480, 780, 280, 340, "#81d4fa", 0.45) + glass_cup(1120, 780, 280, 340, "#81d4fa", 0.9), S("g20") + 0.3, 0.6)
    ph.add("v-l", "".join(circle(430 + 30 * (i % 3), 690 + 30 * (i // 3), 9, "#ffffff", "#0288d1", 3) for i in range(4)) +
           text(480, 330, "λίγη ουσία", 54, "#0277bd"), C("g21", "Όσο") + 0.5, 0.6)
    ph.add("v-r", "".join(circle(1050 + 30 * (i % 5), 660 + 30 * (i // 5), 9, "#ffffff", "#0288d1", 3) for i in range(10)) +
           text(1120, 330, "περισσότερη ουσία", 54, "#0277bd"), C("g21", "τόσο") + 0.3, 0.6)
    ph.close()

    ph = Phase(sc, E("g24") + 0.7)
    steam = "".join(path("M%s %s q-20 -40 0 -70 q20 -30 0 -70" % (850 + 60 * i, 420), "none", "#ef9a9a", 8) for i in range(3))
    ph.add("t-cold", cup(300, 540, "#4fc3f7", 1.5) + cube(250, 470, 44, "#e1f5fe", "#4fc3f7") + cube(320, 480, 40, "#e1f5fe", "#4fc3f7") +
           text(300, 700, "κρύο", 56, "#0277bd"), S("g22") + 0.1, 0.6)
    ph.add("t-hot", cup(900, 540, "#ef6c00", 1.5) + steam + text(900, 700, "ζεστό", 56, "#c62828"), S("g22") + 0.6, 0.6)
    tb = C("g23", "Στο ζεστό")
    ph.add("t-bar-c", rect(540, 660, 100, 80, "#ffcc80", 6, "#bf6a00", 5), reveal_box=[535, 650, 110, 100], reveal_from="bottom").reveal(tb, tb + 0.8)
    ph.add("t-bar-h", rect(1140, 500, 100, 240, "#ffcc80", 6, "#bf6a00", 5), reveal_box=[1135, 490, 110, 260], reveal_from="bottom").reveal(tb + 0.8, tb + 2.0)
    ph.add("t-bar-l", text(590, 800, "λίγη ζάχαρη", 40, "#bf6a00") + text(1190, 800, "περισσότερη ζάχαρη", 40, "#bf6a00"), tb, 0.6)
    ph.add("t-tooth", tooth(1450, 560) + warn(1450, 380), C("g24", "Προσοχή") - 0.1, 0.5)
    ph.close()

    # ---------- F: molecules move faster
    ph = Phase(sc, E("g26") + 0.5)
    ph.add("h-mol", text(800, 105, "Τα μόρια κινούνται", 68, NAVY), S("g25") - 0.1)
    ph.add("m-box-c", rect(160, 230, 560, 460, "#e1f5fe", 30, "#4fc3f7", 8) + text(440, 760, "κρύο: αργά", 52, "#0277bd"), S("g25") + 0.1, 0.5)
    ph.add("m-box-h", rect(880, 230, 560, 460, "#ffebee", 30, "#ef5350", 8) + text(1160, 760, "ζεστό: γρήγορα", 52, "#c62828"), S("g25") + 0.1, 0.5)
    colsm = ["#e53935", "#1e88e5", "#43a047", "#fb8c00"]
    for i in range(3):
        for side, bx, step, amp in (("c", 440, 1.4, 50), ("h", 1160, 0.35, 70)):
            px, py = bx - 140 + i * 140, 340 + (i % 2) * 130
            tr = ph.add("m-%s-%d" % (side, i), circle(px, py, 26, colsm[(i + (side == "h")) % 4], "#37474f", 3) +
                        circle(px + 60, py + 110, 26, colsm[(i + 2 + (side == "h")) % 4], "#37474f", 3), S("g25") + 0.5, 0.5)
            walk(tr, S("g25") + 1.0, E("g26") + 0.3, amp, step, rnd)
    ph.close()

    # ---------- G: water, the best solvent
    head(E("g31") + 0.8, "h-wat", "Το νερό, ο καλύτερος διαλύτης", "g27")
    ph = Phase(sc, E("g30") + 0.7)
    sea = (rect(0, 300, 1600, 520, "#4fc3f7", 0, None, 0, 'opacity="0.55"') +
           path("M0 300 Q100 270 200 300 T400 300 T600 300 T800 300 T1000 300 T1200 300 T1400 300 T1600 300 L1600 330 L0 330 Z", "#29b6f6"))
    ph.add("w-sea", sea, S("g27") + 0.4, 0.8)
    ph.add("w-salt", "".join(circle(150 + 120 * i + rnd.randint(-20, 20), 420 + rnd.randint(0, 330), 8, "#ffffff", "#78909c", 3) for i in range(13)) +
           text(260, 840, "αλάτι", 48, INK), C("g29", "αλάτι") - 0.2, 0.6)
    ph.add("w-oxy", "".join(circle(190 + 140 * i + rnd.randint(-20, 20), 380 + rnd.randint(0, 340), 12, "#e1f5fe", "#0288d1", 3) for i in range(11)) +
           text(1340, 840, "οξυγόνο", 48, "#0277bd"), C("g29", "οξυγόνο") - 0.3, 0.6)
    fish1 = ph.add("w-fish", fish(500, 560, 1.6), C("g30", "ψάρια") - 0.4, 0.6)
    fish1.move(C("g30", "ψάρια") - 0.4, E("g30"), x=650, y=-40)
    ph.add("w-states", text(800, 215, "στερεά, υγρά, αέρια", 56, "#01579b"), reveal_box=[400, 160, 800, 85]).reveal(S("g28") + 0.1, E("g28"))
    ph.close()

    ph = Phase(sc, E("g31") + 0.8)
    ph.add("n-bo", line(800, 450, 640, 560, "#90a4ae", 14) + line(800, 450, 960, 560, "#90a4ae", 14) +
           circle(800, 440, 100, "#ef5350", "#b71c1c", 7) + text(800, 470, "O", 90, "#ffffff") +
           text(800, 790, "μόριο του νερού", 56, "#01579b"), S("g31") + 0.3, 0.6, origin=[800, 440])
    ph.add("n-h", circle(640, 580, 62, "#eceff1", "#607d8b", 7) + text(640, 604, "H", 70, "#37474f") +
           circle(960, 580, 62, "#eceff1", "#607d8b", 7) + text(960, 604, "H", 70, "#37474f"), C("g31", "δύο") - 0.1, 0.5)
    ph.add("n-lo", text(1140, 460, "οξυγόνο", 54, "#b71c1c", "start") + arrow(1120, 440, 915, 440, "#b71c1c", 6), C("g31", "οξυγόνου") - 0.3, 0.6)
    ph.add("n-lh", text(400, 780, "υδρογόνο", 54, "#37474f") + arrow(500, 740, 590, 650, "#37474f", 6), C("g31", "υδρογόνου") - 0.3, 0.6)
    ph.close()

    # ---------- H: remember
    ph = Phase(sc, E("g34") + 1.8, 0.6)
    ph.add("h-rem", text(800, 105, "Θυμήσου", 68, NAVY), S("g32") - 0.1)
    ph.add("r-1", rect(180, 220, 1240, 150, "#e3f2fd", 36, "#1e88e5", 7) +
           text(800, 322, "διάλυμα: δεν βλέπουμε τα συστατικά", 56, "#0d47a1"), S("g32") + 0.1, 0.6)
    ph.add("r-2", rect(180, 410, 1240, 150, "#fff3e0", 36, "#fb8c00", 7) +
           text(800, 512, "διαλύτης + διαλυμένες ουσίες", 56, "#e65100"), S("g33") + 0.1, 0.6)
    ph.add("r-3", rect(180, 600, 1240, 150, "#e8f5e9", 36, "#43a047", 7) +
           text(800, 702, "περισσότερος και πιο ζεστός διαλύτης", 52, "#1b5e20"), S("g34") + 0.2, 0.6)
    ph.close()
    return sc
