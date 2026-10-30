"""Lesson 5: food and energy."""
from lib import Scene, Lesson, text
from icons import *

ID = "energeia-trofes"
TITLE = "Τροφές και ενέργεια"
SUMMARY = "Η ενέργεια του σώματός μας έρχεται από τις τροφές. Πόση ενέργεια χρειαζόμαστε και πόση παίρνουμε."

SENTENCES = [
    ("s1", "Έχεις νιώσει ποτέ πείνα, και να μην έχεις διάθεση για παιχνίδι; Σου έλειπε ενέργεια!", 0.8),
    ("s2", "Το σώμα μας είναι ένας ακούραστος μετατροπέας ενέργειας. Την ενέργεια που χρειαζόμαστε την παίρνουμε από τις τροφές.", 1.0),
    ("s3", "Στις τροφές υπάρχει αποθηκευμένη χημική ενέργεια. Όταν τις χωνεύουμε, η ενέργεια αυτή ελευθερώνεται.", 1.2),
    ("s4", "Στη συνέχεια, το σώμα την αλλάζει σε άλλες μορφές ενέργειας.", 0.8),
    ("s5", "Σε θερμότητα, για να μένει ζεστό το σώμα μας.", 1.2),
    ("s6", "Σε κινητική ενέργεια, όταν τρέχουμε, περπατάμε ή γράφουμε.", 1.2),
    ("s7", "Σε ηλεκτρική ενέργεια, για να στέλνει ο εγκέφαλος μηνύματα σε όλο το σώμα.", 1.2),
    ("s8", "Και σε χημική ενέργεια, για να χτίζονται καινούργια κύτταρα και να αποθηκεύεται λίπος.", 1.5),
    ("s9", "Οι διαφορετικές τροφές μάς δίνουν διαφορετική ποσότητα ενέργειας.", 1.0),
    ("s10", "Στη συσκευασία πολλών τροφών γράφει πόση ενέργεια έχουν. Η ενέργεια μετριέται σε κιλοτζάουλ.", 1.0),
    ("s11", "Παλιότερα χρησιμοποιούσαν τις χιλιοθερμίδες. Μία χιλιοθερμίδα είναι περίπου τέσσερα κόμμα δύο κιλοτζάουλ.", 1.2),
    ("s12", "Το λάδι και η σοκολάτα έχουν πολλή ενέργεια. Τα μήλα και η σαλάτα έχουν λίγη.", 1.5),
    ("s13", "Δεν χρειάζονται όλοι την ίδια ενέργεια. Όσο περισσότερο κινούμαστε, τόσο περισσότερη ενέργεια χρειαζόμαστε.", 1.0),
    ("s14", "Ένας αθλητής που προπονείται πολλές ώρες, χρειάζεται πολύ περισσότερη ενέργεια από έναν υπάλληλο γραφείου.", 1.2),
    ("s15", "Ακόμα και όταν κάθεσαι, χρειάζεσαι ενέργεια, για να αναπνέεις και για να χτυπάει η καρδιά σου.", 1.5),
    ("s16", "Αν παίρνουμε από τις τροφές περισσότερη ενέργεια από όση χρειαζόμαστε, το σώμα αποθηκεύει την ενέργεια που περισσεύει, σε μορφή λίπους. Παχαίνουμε!", 1.2),
    ("s17", "Αν παίρνουμε λιγότερη, το σώμα χρησιμοποιεί το λίπος σαν αποθήκη, και αδυνατίζουμε. Όμως οι εξαντλητικές δίαιτες είναι επικίνδυνες για την υγεία.", 1.2),
    ("s18", "Η σωστή διατροφή δίνει κάθε μέρα όση ενέργεια χρειαζόμαστε, ούτε περισσότερη, ούτε λιγότερη.", 1.5),
    ("s19", "Σκέψου πόσο κινήθηκες σήμερα, και τι έφαγες. Πήρες την ενέργεια που χρειάζεσαι;", 0.5),
]

FOODS = [  # id, name, kJ per 100 g, icon, scale
    ("oil", "λάδι", 3800, oil_bottle, 0.6),
    ("choc", "σοκολάτα", 2250, chocolate, 0.55),
    ("rice", "ρύζι", 1500, rice, 0.6),
    ("bread", "ψωμί", 1050, bread, 0.55),
    ("apple", "μήλο", 200, apple, 0.65),
    ("salad", "σαλάτα", 80, salad, 0.6),
]
ROW_Y = [185, 295, 405, 515, 625, 735]
BAR_X, K = 500, 0.2


def build(T):
    sc = Scene()
    sc.define('<linearGradient id="bg5" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#e8f5e9"/><stop offset="1" stop-color="#fffde7"/></linearGradient>')
    sc.static('<rect width="1600" height="900" fill="url(#bg5)"/>')

    # A: hungry child and the energy bar
    sc.actor("floor", '<rect y="745" width="1600" height="155" fill="#d7ccc8"/>')
    sc.actor("ebar", '<rect x="150" y="110" width="520" height="120" rx="22" fill="#fff" stroke="#455a64" stroke-width="10"/><rect x="670" y="145" width="26" height="50" rx="8" fill="#455a64"/>')
    sc.actor("efill", '<rect x="164" y="124" width="492" height="92" rx="12" fill="#66bb6a"/>', box=[160, 120, 500, 100])
    sc.actor_text("lab-en", 410, 300, "ενέργεια", 60, "#2e7d32")
    sc.actor("kid-tired", g(person("#1e88e5", tired=True), 410, 610, 1.4), origin=(410, 610))
    sc.actor("kid-happy", g(person("#1e88e5"), 410, 610, 1.4), origin=(410, 610))
    sc.actor("food", g(bread(), 960, 690, 1.1) + g(apple(), 1130, 690, 1.2) + g(cheese(), 1330, 690, 1.0), origin=(1130, 690))

    # B: the body changes the energy
    sc.actor_text("b-title", 800, 100, "Το σώμα αλλάζει την ενέργεια των τροφών", 56, "#1b5e20")
    sc.actor("body", g(person("#43a047"), 800, 500, 1.3), origin=(800, 500))
    dest = [("heat", 290, 280, "θερμότητα", C_HEAT, g(heat_waves(), 0, 0, 1.0), (690, 400), (440, 300)),
            ("kin", 1310, 280, "κινητική", C_KIN, g(person("#fb8c00", pose="run"), 0, 20, 0.6), (910, 400), (1160, 300)),
            ("elec", 290, 650, "ηλεκτρική", C_ELEC, g(brain(), 0, 0, 1.1) + g(bolt(), 80, 0, 0.5), (700, 560), (440, 650)),
            ("chem", 1310, 650, "χημική", C_CHEM, g(cell(), -60, 0, 1.0) + g(cell(), 40, -20, 0.8) + g(cell(), 50, 40, 0.7), (900, 560), (1160, 650))]
    for did, cx, cy, label, col, icon, a0, a1 in dest:
        sc.actor(f"d-{did}", g(icon, cx, cy - 20) + text(cx, cy + 140, label, 56, col), origin=(cx, cy))
        sc.actor(f"a-{did}", arrow(a0[0], a0[1], a1[0], a1[1], col, 18, 42))

    # C: foods
    sc.actor_text("c-title", 800, 90, "Ενέργεια σε εκατό γραμμάρια τροφής", 58, "#33691e")
    for (fid, name, kj, icon, sz), y in zip(FOODS, ROW_Y):
        sc.actor(f"f-{fid}", g(icon(), 120, y, sz * 0.9) + text(210, y + 16, name, 48, "#263238", "start"), origin=(300, y))
        w = kj * K
        col = "#e53935" if kj >= 2000 else ("#fb8c00" if kj >= 1000 else "#43a047")
        sc.actor(f"bar-{fid}", f'<rect x="{BAR_X}" y="{y-34}" width="{w}" height="68" rx="12" fill="{col}"/>' +
                 text(BAR_X + w + 16, y + 16, f"{kj} kJ", 46, "#263238", "start"),
                 box=[BAR_X - 10, y - 50, int(w + 260), 100])
    sc.actor("c-conv", text(800, 865, "1 χιλιοθερμίδα ≈ 4,2 kJ", 58, "#6a1b9a"), origin=(800, 840), box=[400, 805, 800, 80])

    # D: athlete and office worker
    sc.actor("ath", g(person("#fb8c00", pose="run"), 190, 280, 0.9) + text(380, 190, "αθλητής", 54, "#e65100", "start"), origin=(190, 280))
    sc.actor("off", g(person("#78909c"), 190, 640, 0.8) + '<rect x="100" y="640" width="190" height="14" rx="6" fill="#8d6e63"/>' + text(380, 550, "υπάλληλος γραφείου", 54, "#37474f", "start"), origin=(190, 640))
    sc.actor("bar-ath", '<rect x="380" y="230" width="1000" height="100" rx="16" fill="#fb8c00"/>', box=[370, 220, 1020, 120])
    sc.actor("bar-off", '<rect x="380" y="590" width="440" height="100" rx="16" fill="#1e88e5"/>', box=[370, 580, 460, 120])
    sc.actor("base-ath", '<rect x="380" y="230" width="300" height="100" rx="16" fill="#90a4ae"/>')
    sc.actor("base-off", '<rect x="380" y="590" width="300" height="100" rx="16" fill="#90a4ae"/>')
    sc.actor_text("lab-base", 800, 790, "γκρι: ενέργεια για αναπνοή, καρδιά, εγκέφαλο", 46, "#455a64")

    # E: balance
    sc.actor_text("e-title", 800, 100, "Ενέργεια που παίρνουμε και ενέργεια που χρειαζόμαστε", 52, "#1b5e20")
    sc.actor("pivot", '<path d="M800 620 L720 780 L880 780 Z" fill="#78909c"/>')
    sc.actor("beam", '<rect x="330" y="600" width="940" height="30" rx="12" fill="#8d6e63"/>' + g(apple(), 430, 565, 0.9) +
             g(person("#1e88e5", pose="run"), 1170, 535, 0.5) +
             text(480, 440, "παίρνουμε", 50, "#2e7d32", "middle") + text(1120, 440, "χρειαζόμαστε", 50, "#c62828", "middle"), origin=(800, 615))
    sc.actor("fat", '<ellipse cx="800" cy="230" rx="190" ry="75" fill="#fff59d" stroke="#f9a825" stroke-width="8"/>' + text(800, 248, "λίπος", 56, "#e65100"), origin=(800, 230))
    sc.actor_text("e-ok", 800, 260, "Ισορροπία!", 90, "#2e7d32")

    L = Lesson(ID, TITLE, SUMMARY, sc, T)
    L.sentences = [(k, t) for k, t, _ in SENTENCES]
    s, e = T.s, T.e
    d = lambda k: e(k) - s(k)

    # A
    L.fade_in("floor", s("s1"), 0.3)
    L.fade_out("floor", s("s9", -0.4), 0.5)
    L.fade_in("pivot", s("s16"), 0.5)
    L.fade_in("ebar", s("s1"), 0.6)
    L.fade_in("efill", s("s1"), 0.6)
    L.k("efill", s("s1"), reveal=0.12)
    L.k("efill", s("s1", 0.7), reveal=0.12)
    L.write("lab-en", s("s1", 1.0), 0.8)
    L.fade_in("kid-tired", s("s1"), 0.6)
    L.fade_in("food", s("s2", 1.0), 0.6)
    L.k("food", s("s3", 0.0), x=0, y=0)
    L.k("food", s("s3", 2.5), x=-380, y=-20, ease="in-out")
    L.k("food", s("s3", 3.3), x=-380, y=-20, opacity=0)
    L.k("efill", s("s3", 3.0), reveal=0.12)
    L.k("efill", s("s3", 5.0), reveal=1.0, ease="out")
    L.fade_out("kid-tired", s("s3", 2.6), 0.4)
    L.fade_in("kid-happy", s("s3", 3.0), 0.5)
    L.bob("kid-happy", s("s3", 4.5), 1, 20, 0.5)
    for a in ("ebar", "efill", "lab-en", "kid-happy", "food"):
        if a == "food":
            continue
        L.fade_out(a, s("s4", -0.4), 0.5)
    # B
    L.fade_in("b-title", s("s4"), 0.6)
    L.fade_in("body", s("s4", 0.3), 0.6)
    for did, key in (("heat", "s5"), ("kin", "s6"), ("elec", "s7"), ("chem", "s8")):
        L.fade_in(f"a-{did}", s(key), 0.5)
        L.pop(f"d-{did}", s(key, 0.4), 0.5)
    L.pulse("body", s("s8", 3.5), 1, 1.08, 0.8)
    for a in ["b-title", "body"] + [f"{p}-{x}" for x in ("heat", "kin", "elec", "chem") for p in ("a", "d")]:
        L.fade_out(a, s("s9", -0.4), 0.5)
    # C
    L.write("c-title", s("s9", 0.3), 1.5)
    t0, t1 = s("s9", 2.5), e("s10", -0.5)
    for i, (fid, *_r) in enumerate(FOODS):
        t = t0 + (t1 - t0) * i / len(FOODS)
        L.rise(f"f-{fid}", t, 0.5, 30)
        L.write(f"bar-{fid}", t + 0.3, 1.2)
    L.write("c-conv", s("s11", 2.0), 1.6)
    L.pulse("f-oil", s("s12"), 1, 1.1, 0.8)
    L.pulse("f-choc", s("s12", 0.8), 1, 1.1, 0.8)
    L.pulse("f-apple", s("s12", 3.0), 1, 1.1, 0.8)
    L.pulse("f-salad", s("s12", 3.8), 1, 1.1, 0.8)
    allc = ["c-title", "c-conv"] + [f"{p}-{f_[0]}" for f_ in FOODS for p in ("f", "bar")]
    for a in allc:
        L.fade_out(a, s("s13", -0.4), 0.5)
    # D
    L.fade_in("ath", s("s13", 0.3), 0.6)
    L.fade_in("off", s("s13", 2.5), 0.6)
    L.write("bar-ath", s("s14", 0.5), 1.8)
    L.write("bar-off", s("s14", 3.5), 1.4)
    L.fade_in("base-ath", s("s15", 0.5), 0.6)
    L.fade_in("base-off", s("s15", 0.5), 0.6)
    L.write("lab-base", s("s15", 1.5), 2.0)
    for a in ("ath", "off", "bar-ath", "bar-off", "base-ath", "base-off", "lab-base"):
        L.fade_out(a, s("s16", -0.4), 0.5)
    # E
    L.write("e-title", s("s16"), 1.8)
    L.fade_in("beam", s("s16", 0.2), 0.5)
    L.k("beam", s("s16", 0.5), rotate=0)
    L.k("beam", s("s16", 2.5), rotate=-9, ease="in-out")
    L.pop("fat", s("s16", 6.0), 0.6)
    L.k("beam", s("s17", 0.0), rotate=-9)
    L.k("beam", s("s17", 2.0), rotate=9, ease="in-out")
    L.fade_out("fat", s("s17", 1.0), 0.8)
    L.k("beam", s("s18", 0.0), rotate=9)
    L.k("beam", s("s18", 1.8), rotate=0, ease="in-out")
    L.write("e-ok", s("s18", 2.5), 1.2)
    L.bob("beam", s("s19", 0.5), 1, 12, 0.6)
    return L
