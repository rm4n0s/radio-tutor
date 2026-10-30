"""Lesson 1: energy has many faces (the introduction and the forms of energy)."""
import math
from lib import Scene, Lesson, text
from icons import *

ID = "energeia-morfes"
TITLE = "Η ενέργεια έχει πολλά πρόσωπα"
SUMMARY = "Χωρίς ενέργεια δεν αλλάζει τίποτα. Γνωρίζουμε τις επτά μορφές της ενέργειας."

SENTENCES = [
    ("s1", "Γεια σου! Σήμερα θα γνωρίσουμε μια πολύ σημαντική λέξη: την ενέργεια.", 0.6),
    ("s2", "Ένα φύλλο πέφτει, ένα αυτοκίνητο τρέχει, ένα φως ανάβει. Σε όλα αυτά κάτι αλλάζει.", 1.2),
    ("s3", "Για κάθε αλλαγή που γίνεται στη φύση, χρειάζεται ενέργεια.", 0.5),
    ("s4", "Χωρίς ενέργεια, δεν αλλάζει τίποτα!", 1.5),
    ("s5", "Εδώ και χιλιάδες χρόνια, οι άνθρωποι προσπαθούν να χρησιμοποιήσουν την ενέργεια.", 0.6),
    ("s6", "Πρώτα έμαθαν να ανάβουν φωτιά, για να ζεσταθούν και να φωτίζουν τη νύχτα.", 1.0),
    ("s7", "Ύστερα έφτιαξαν ανεμόμυλους και νερόμυλους, που παίρνουν ενέργεια από τον άνεμο και το νερό.", 1.2),
    ("s8", "Σήμερα έχουμε πολύπλοκες συσκευές, που αλλάζουν την ενέργεια από τη μία μορφή στην άλλη.", 1.0),
    ("s9", "Η ενέργεια έχει πολλά πρόσωπα. Τα πρόσωπα αυτά τα λέμε μορφές ενέργειας.", 1.2),
    ("s10", "Η ενέργεια που είναι κρυμμένη στις τροφές και στα καύσιμα, λέγεται χημική ενέργεια.", 0.5),
    ("s11", "Για παράδειγμα, το ψωμί, η μπαταρία και το πετρέλαιο έχουν χημική ενέργεια.", 1.0),
    ("s12", "Η ενέργεια που ταξιδεύει μέσα στα καλώδια ενός κυκλώματος, λέγεται ηλεκτρική ενέργεια.", 1.0),
    ("s13", "Η ενέργεια του φωτός λέγεται φωτεινή ενέργεια.", 1.0),
    ("s14", "Όταν η ενέργεια περνάει από ένα ζεστό σώμα σε ένα πιο κρύο, τη λέμε θερμότητα.", 1.0),
    ("s15", "Όταν στα βάθη ενός ατόμου σπάει ο πυρήνας του, ελευθερώνεται πυρηνική ενέργεια.", 1.0),
    ("s16", "Ένα σώμα που κινείται, για παράδειγμα ένα παιδί που τρέχει, έχει κινητική ενέργεια.", 1.0),
    ("s17", "Ένα σώμα που το πιέζουν ή το τραβούν, όπως ένα ελατήριο, έχει δυναμική ενέργεια.", 1.2),
    ("s18", "Η κινητική και η δυναμική ενέργεια είναι οι δύο βασικές μορφές της ενέργειας.", 1.5),
    ("s19", "Τώρα ξέρεις επτά μορφές ενέργειας. Μπορείς να τις βρεις γύρω σου;", 0.5),
]

CARD_W, CARD_H = 340, 320
CARDS = [  # id, label, colour, cx, cy
    ("chem", "Χημική", C_CHEM, 215, 290),
    ("elec", "Ηλεκτρική", C_ELEC, 605, 290),
    ("light", "Φωτεινή", C_LIGHT, 995, 290),
    ("heat", "Θερμότητα", C_HEAT, 1385, 290),
    ("nuc", "Πυρηνική", C_NUC, 410, 650),
    ("kin", "Κινητική", C_KIN, 800, 650),
    ("pot", "Δυναμική", C_POT, 1190, 650),
]


def card(cx, cy, label, col, inner):
    s = (f'<rect x="{cx-CARD_W/2}" y="{cy-CARD_H/2}" width="{CARD_W}" height="{CARD_H}" rx="30" fill="#fff" '
         f'stroke="{col}" stroke-width="10"/>')
    s += f'<rect x="{cx-CARD_W/2+5}" y="{cy+CARD_H/2-100}" width="{CARD_W-10}" height="95" rx="26" fill="{col}" opacity="0.18"/>'
    s += inner
    s += text(cx, cy + CARD_H / 2 - 34, label, 54, "#263238")
    return s


def build(T):
    sc = Scene()
    sc.define('<linearGradient id="sky1" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#bbdefb"/><stop offset="1" stop-color="#fffde7"/></linearGradient>')
    sc.static('<rect width="1600" height="900" fill="url(#sky1)"/>')
    sc.static('<ellipse cx="800" cy="900" rx="1000" ry="210" fill="#a5d6a7"/><rect y="760" width="1600" height="140" fill="#81c784"/>')

    # --- first part: nothing changes without energy
    sc.actor("sun", g(sun(70), 1400, 150), origin=(1400, 150))
    sc.actor("leaf", g(leaf(), 230, 160, 1.3), origin=(230, 160))
    sc.actor("car", g(car(), 160, 640, 1.2))
    sc.actor("cord", '<line x1="1020" y1="0" x2="1020" y2="260" stroke="#455a64" stroke-width="8"/>')
    sc.actor("bulb-off", g(bulb(False), 1020, 320, 1.1))
    sc.actor("bulb-on", g(glow(130), 1020, 310) + g(bulb(True), 1020, 320, 1.1))
    sc.actor_text("banner", 800, 830, "Χωρίς ενέργεια δεν αλλάζει τίποτα!", 62, "#1b5e20")

    # --- history
    sc.actor("fire", g(logs(), 300, 640, 0.9), origin=(300, 640))
    sc.actor("flame", g(flame(), 300, 560, 1.2), origin=(300, 600))
    sc.actor("tower", g(windmill_tower(), 800, 560))
    sc.actor("blades", g(windmill_blades(), 800, 530, 1.1), origin=(800, 530))
    sc.actor("water-wheel", g(water_wheel(), 1260, 580), origin=(1260, 580))
    sc.actor("stream", '<rect x="1090" y="600" width="340" height="110" rx="40" fill="#4fc3f7" opacity="0.85"/>'
             '<path d="M1120 640 q30 -16 60 0 t60 0 t60 0 t60 0" fill="none" stroke="#e1f5fe" stroke-width="7"/>')
    sc.actor_text("lab-fire", 300, 830, "φωτιά", 56, "#1b5e20")
    sc.actor_text("lab-wind", 800, 830, "άνεμος", 56, "#1b5e20")
    sc.actor_text("lab-water", 1260, 830, "νερό", 56, "#1b5e20")

    # --- devices
    sc.actor("dev-fan", g(fan_stand(), 380, 520, 1.1))
    sc.actor("dev-blades", g(fan_blades(), 380, 480, 1.0), origin=(380, 480))
    sc.actor("dev-bulb", g(glow(110), 800, 480) + g(bulb(True), 800, 490, 1.2), origin=(800, 480))
    sc.actor("dev-car", g(car("#3949ab"), 1180, 600, 1.2))
    sc.actor_text("lab-dev", 800, 830, "συσκευές και μηχανές", 60, "#1b5e20")

    # --- the faces
    sc.actor_text("title", 800, 100, "Μορφές ενέργειας", 76, "#0d47a1")
    inner = {
        "chem": g(barrel(), -75, -35, 0.78) + g(bread(), 70, -20, 0.7) + g(apple(), 90, 50, 0.6),
        "elec": g(pylon(), -60, -20, 0.75) + g(bolt(), 80, -20, 0.85),
        "light": g(glow(105), 0, -35) + g(bulb(True), 0, -30, 1.15),
        "heat": ('<rect x="-130" y="-10" width="95" height="85" rx="14" fill="#ef9a9a"/>' + g(heat_waves(), -82, -60, 0.6) +
                 arrow(-24, 34, 24, 34, "#455a64", 12, 28) +
                 '<rect x="36" y="-10" width="95" height="85" rx="14" fill="#90caf9"/>'),
        "nuc": g(atom(), 0, -35, 0.9),
        "kin": '<rect x="-140" y="40" width="280" height="10" rx="5" fill="#90a4ae"/>',
        "pot": '<rect x="-130" y="-60" width="22" height="110" rx="6" fill="#78909c"/>',
    }
    for cid, label, col, cx, cy in CARDS:
        sc.actor(f"card-{cid}", card(cx, cy, label, col, g(inner[cid], cx, cy)), origin=(cx, cy))
    sc.actor("ball-k", g(g(speed_lines(), -50, 0) + g(ball(34), 0, 0), 740, 625))
    sc.actor("spring-p", g(spring(170, 6, 28), 1175, 625), origin=(1090, 625))

    L = Lesson(ID, TITLE, SUMMARY, sc, T)
    L.sentences = [(k, t) for k, t, _ in SENTENCES]
    s, e = T.s, T.e
    end = T.total

    # sun rays turn slowly all the time
    L.spin("sun", 0.5, s("s9"), 70)
    L.fade_out("sun", s("s9"), 0.6)
    # s2: leaf falls, car drives, bulb lights up
    L.k("leaf", 0, x=0, y=0, rotate=0)
    L.k("leaf", s("s2", 0.2), x=0, y=0, rotate=0)
    L.k("leaf", s("s2", 1.4), x=60, y=170, rotate=110, ease="in-out")
    L.k("leaf", s("s2", 2.8), x=-20, y=330, rotate=-60, ease="in-out")
    L.k("leaf", s("s2", 4.2), x=40, y=500, rotate=40, ease="in-out")
    L.move("car", s("s2", 0.2), e("s2", -0.3), (0, 0), (780, 0), "in-out")
    L.fade_in("bulb-on", s("s2", 3.8), 0.8)
    L.write("banner", s("s4"), 1.4)
    L.k("banner", s("s5", -0.7), opacity=1)
    for a in ("leaf", "car", "cord", "bulb-off", "bulb-on", "banner"):
        L.fade_out(a, s("s5", -0.6), 0.5)
    # history
    L.pop("fire", s("s6"), 0.5)
    L.fade_in("flame", s("s6", 0.2), 0.5)
    L.k("flame", s("s6", 0.7), scale=1)
    for i in range(8):
        L.k("flame", s("s6", 1.0 + 0.5 * i), scale=1.12 if i % 2 == 0 else 0.92, ease="in-out")
    L.write("lab-fire", s("s6", 0.5), 0.7)
    L.fade_in("tower", s("s7"), 0.6)
    L.fade_in("blades", s("s7"), 0.6)
    L.spin("blades", s("s7"), s("s8", -0.5), 1100)
    L.fade_in("water-wheel", s("s7", 2.0), 0.6)
    L.fade_in("stream", s("s7", 2.0), 0.6)
    L.spin("water-wheel", s("s7", 2.0), s("s8", -0.5), 500)
    L.write("lab-wind", s("s7", 0.5), 0.7)
    L.write("lab-water", s("s7", 2.2), 0.7)
    for a in ("fire", "flame", "tower", "blades", "water-wheel", "stream", "lab-fire", "lab-wind", "lab-water"):
        L.fade_out(a, s("s8", -0.4) if a not in ("blades", "water-wheel") else s("s8", -0.4), 0.4)
    # devices
    L.fade_in("dev-fan", s("s8", 0.2), 0.5)
    L.fade_in("dev-blades", s("s8", 0.2), 0.5)
    L.spin("dev-blades", s("s8", 0.2), s("s9", -0.4), 1500)
    L.pop("dev-bulb", s("s8", 2.0), 0.5)
    L.fade_in("dev-car", s("s8", 3.5), 0.5)
    L.move("dev-car", s("s8", 3.5), s("s9", -0.4), (-120, 0), (60, 0), "in-out")
    L.write("lab-dev", s("s8", 0.5), 1.0)
    for a in ("dev-fan", "dev-blades", "dev-bulb", "dev-car", "lab-dev"):
        L.fade_out(a, s("s9", 0.1), 0.5)
    # title and the cards
    L.write("title", s("s9", 1.2), 1.4)
    when = {"chem": "s10", "elec": "s12", "light": "s13", "heat": "s14", "nuc": "s15", "kin": "s16", "pot": "s17"}
    for cid, key in when.items():
        L.pop(f"card-{cid}", s(key), 0.5)
    L.fade_in("ball-k", s("s16"), 0.5)
    L.move("ball-k", s("s16", 0.5), s("s17", -0.3), (-20, 0), (150, 0), "linear")
    L.fade_in("spring-p", s("s17"), 0.5)
    L.k("spring-p", s("s17", 0.6), scale=1)
    L.k("spring-p", s("s17", 2.0), scale=0.55, ease="in-out")
    L.k("spring-p", s("s17", 3.2), scale=1.0, ease="out")
    L.pulse("card-chem", s("s11", 0.2), 1, 1.1, 0.8)
    L.pulse("card-kin", s("s18"), 2, 1.1, 0.9)
    L.pulse("card-pot", s("s18"), 2, 1.1, 0.9)
    for i, cid in enumerate(when):
        L.bob(f"card-{cid}", s("s19", 0.5 + 0.15 * i), 1, 18, 0.5)
    return L
