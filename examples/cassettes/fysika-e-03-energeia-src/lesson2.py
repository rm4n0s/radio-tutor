"""Lesson 2: energy is stored (energy stores, sources, using energy wisely)."""
from lib import Scene, Lesson, text
from icons import *

ID = "energeia-apothikes"
TITLE = "Η ενέργεια αποθηκεύεται"
SUMMARY = "Οι αποθήκες της ενέργειας: ο Ήλιος, το πετρέλαιο, το νερό, η μπαταρία και το ελατήριο."

SENTENCES = [
    ("s1", "Ξέρεις τι είναι μια αποθήκη; Είναι ένας χώρος, όπου φυλάμε πράγματα για να τα χρησιμοποιήσουμε αργότερα.", 0.8),
    ("s2", "Και η ενέργεια μπορεί να αποθηκευτεί! Τις αποθήκες της ενέργειας τις λέμε πηγές ενέργειας.", 1.0),
    ("s3", "Η φύση έχει φτιάξει πολλές τέτοιες αποθήκες.", 0.8),
    ("s4", "Ο Ήλιος μας στέλνει φωτεινή ενέργεια, και τα φυτά τη χρησιμοποιούν για να μεγαλώσουν.", 1.2),
    ("s5", "Το πετρέλαιο και τα κάρβουνα είναι αποθήκες χημικής ενέργειας. Η ενέργειά τους ήρθε από τον Ήλιο πριν από εκατομμύρια χρόνια.", 1.2),
    ("s6", "Το νερό που πέφτει ορμητικά από έναν καταρράκτη έχει κινητική ενέργεια, και μπορεί να γυρίσει έναν νερόμυλο.", 1.5),
    ("s7", "Κάποιες φορές αποθηκεύουμε εμείς την ενέργεια, για να τη χρησιμοποιήσουμε αργότερα.", 1.0),
    ("s8", "Η μπαταρία έχει αποθηκευμένη χημική ενέργεια.", 0.6),
    ("s9", "Όταν τη συνδέσουμε με ένα λαμπάκι, η ενέργειά της γίνεται ηλεκτρική και μετά φωτεινή.", 1.5),
    ("s9b", "Μπαταρίες έχουν οι φακοί, τα τηλεχειριστήρια, τα ρολόγια και πολλά παιχνίδια.", 1.0),
    ("s10", "Ένα ελατήριο που το πιέζουμε ή το τεντώνουμε, αποθηκεύει δυναμική ενέργεια.", 1.0),
    ("s11", "Όταν κουρδίζουμε ένα παιχνίδι, αποθηκεύουμε ενέργεια στο ελατήριό του.", 1.0),
    ("s12", "Μετά το ελατήριο ξετυλίγεται σιγά σιγά, και η ενέργεια γίνεται κινητική. Το παιχνίδι κινείται!", 1.5),
    ("s13", "Και το αυτοκίνητο έχει μια αποθήκη: τη δεξαμενή με τη βενζίνη, που κρύβει χημική ενέργεια.", 1.5),
    ("s14", "Είναι όμως η ενέργεια ανεξάντλητη; Όχι! Οι αποθήκες της Γης με χρήσιμη ενέργεια έχουν όριο.", 1.0),
    ("s15", "Όταν μετατρέπουμε πολλή ενέργεια, επιβαρύνουμε και το περιβάλλον.", 0.8),
    ("s16", "Γι’ αυτό πρέπει να χρησιμοποιούμε την ενέργεια σωστά και με μέτρο.", 1.5),
    ("s17", "Θυμήσου μια πολύ σημαντική αρχή: η ενέργεια δεν εμφανίζεται από το τίποτα, ούτε εξαφανίζεται.", 1.0),
    ("s18", "Δεν μπορούμε να τη δημιουργήσουμε. Μπορούμε όμως να την αποθηκεύσουμε και να την αλλάξουμε σε άλλη μορφή.", 1.0),
    ("s19", "Η ενέργεια που χρησιμοποιούμε έρχεται πάντα από μια αποθήκη.", 0.5),
]

SH1, SH2 = 400, 790     # shelf boards
X1 = [190, 520, 800, 1090, 1390]   # sun, oil, coal, waterfall, mill


def label(sc, aid, x, y, s, col="#3e2723"):
    sc.actor_text(aid, x, y, s, 42, col)


def build(T):
    sc = Scene()
    sc.define('<linearGradient id="wall2" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#fff3e0"/><stop offset="1" stop-color="#ffe0b2"/></linearGradient>')
    sc.static('<rect width="1600" height="900" fill="url(#wall2)"/>')
    sc.static(f'<rect x="60" y="{SH1}" width="1480" height="26" rx="8" fill="#8d6e63"/><rect x="60" y="{SH2}" width="1480" height="26" rx="8" fill="#8d6e63"/>')
    sc.static(f'<rect x="100" y="{SH1+26}" width="22" height="70" fill="#6d4c41"/><rect x="1512" y="{SH1+26}" width="22" height="70" fill="#6d4c41"/>')
    sc.actor_text("title", 800, 100, "Αποθήκες ενέργειας", 76, "#bf360c")
    sc.actor_text("sub1", 800, 165, "Οι αποθήκες της φύσης", 52, "#5d4037")
    sc.actor_text("sub2", 800, 590, "Οι αποθήκες που φτιάχνουμε εμείς", 52, "#5d4037")

    # natural stores (shelf 1)
    sc.actor("sun", g(sun(50), X1[0], 300, 1.1), origin=(X1[0], 300))
    sc.actor("plant", g(plant(), X1[0] + 150, 320, 0.8), origin=(X1[0] + 150, 320))
    sc.actor("oil", g(barrel(), X1[1], 305, 1.1), origin=(X1[1], 305))
    sc.actor("coal", g(coal(), X1[2], 340, 0.8), origin=(X1[2], 340))
    sc.actor("fall", g(waterfall(), X1[3], 305, 1.0), origin=(X1[3], 305))
    sc.actor("drops", "".join(f'<line x1="{X1[3]+dx}" y1="{y}" x2="{X1[3]+dx}" y2="{y+28}" stroke="#e1f5fe" stroke-width="9" stroke-linecap="round"/>'
                              for dx, y in [(-18, 250), (0, 300), (18, 260)]))
    sc.actor("mill", g(water_wheel(), X1[4], 310, 0.85), origin=(X1[4], 310))
    label(sc, "lab-sun", X1[0] + 60, 470, "Ήλιος")
    label(sc, "lab-oil", X1[1], 470, "πετρέλαιο")
    label(sc, "lab-coal", X1[2], 470, "κάρβουνο")
    label(sc, "lab-fall", X1[3], 470, "καταρράκτης")
    label(sc, "lab-mill", X1[4], 470, "νερόμυλος")

    # stores we make (shelf 2)
    xb = 280
    sc.actor("battery", g(battery(150, 80), xb, 735), origin=(xb, 735))
    sc.actor("wire", f'<path d="M{xb+80} 735 L{xb+140} 735 L{xb+140} 700" fill="none" stroke="#455a64" stroke-width="8"/>')
    sc.actor("lamp-off", g(bulb(False), xb + 140, 700, 0.8))
    sc.actor("lamp-on", g(glow(90), xb + 140, 690) + g(bulb(True), xb + 140, 700, 0.8))
    xs = 760
    sc.actor("toy", g(toy_car(), xs + 120, 735, 0.9), origin=(xs + 120, 735))
    sc.actor("spring", g(spring(150, 6, 24), xs - 90, 735), origin=(xs - 165, 735))
    sc.actor("toy-wall", f'<rect x="{xs-190}" y="690" width="22" height="90" rx="6" fill="#78909c"/>')
    xt = 1330
    sc.actor("can", g(jerrycan(), xt, 705, 0.85), origin=(xt, 705))
    label(sc, "lab-bat", xb + 40, 850, "μπαταρία")
    label(sc, "lab-spr", xs + 20, 850, "ελατήριο")
    label(sc, "lab-can", xt, 850, "βενζίνη")
    sc.actor_text("rule", 800, 250, "Η ενέργεια δεν εμφανίζεται από το τίποτα!", 60, "#b71c1c")
    sc.actor_text("rule2", 800, 340, "Δεν δημιουργείται, δεν εξαφανίζεται, αλλάζει μορφή.", 54, "#4e342e")

    L = Lesson(ID, TITLE, SUMMARY, sc, T)
    L.sentences = [(k, t) for k, t, _ in SENTENCES]
    s, e = T.s, T.e
    L.write("title", s("s1"), 1.4)
    L.fade_in("sub1", s("s3"), 0.6)
    L.pop("sun", s("s4"), 0.6)
    L.spin("sun", s("s4"), s("s6"), 60)
    L.pop("plant", s("s4", 3.0), 0.6)
    L.pop("oil", s("s5"), 0.6)
    L.pop("coal", s("s5", 2.0), 0.6)
    L.pop("fall", s("s6"), 0.6)
    L.fade_in("drops", s("s6", 0.4), 0.4)
    for i in range(10):
        L.k("drops", s("s6", 0.9 + 0.4 * i), y=0 if i % 2 == 0 else 40, ease="linear")
    L.pop("mill", s("s6", 4.0), 0.6)
    L.spin("mill", s("s6", 4.0), s("s7"), 240)
    L.write("lab-sun", s("s4"), 0.8)
    L.write("lab-oil", s("s5"), 1.0)
    L.write("lab-coal", s("s5", 2.0), 0.9)
    L.write("lab-fall", s("s6"), 1.2)
    L.write("lab-mill", s("s6", 4.2), 0.9)
    L.fade_in("sub2", s("s7"), 0.6)
    L.pop("battery", s("s8"), 0.5)
    L.write("lab-bat", s("s8", 0.5), 0.8)
    L.fade_in("wire", s("s9"), 0.5)
    L.fade_in("lamp-off", s("s9"), 0.5)
    L.fade_in("lamp-on", s("s9", 1.5), 0.7)
    L.pulse("battery", s("s9b"), 2, 1.12, 0.9)
    L.fade_in("toy-wall", s("s10"), 0.5)
    L.fade_in("spring", s("s10"), 0.5)
    L.write("lab-spr", s("s10", 0.4), 0.9)
    L.k("spring", s("s10", 0.6), scale=1)
    L.k("spring", s("s10", 3.4), scale=0.55, ease="in-out")      # being wound up
    L.k("spring", s("s12", 0.6), scale=0.55)
    L.k("spring", e("s12", -0.8), scale=1.0, ease="in-out")      # unwinds slowly
    L.pop("toy", s("s11"), 0.5)
    L.move("toy", s("s12", 0.6), e("s12", 0.3), (0, 0), (150, 0), "in-out")
    L.pop("can", s("s13"), 0.5)
    L.write("lab-can", s("s13", 0.5), 0.8)
    # clear the shelves for the rule
    for a in ("sub1", "sub2", "lab-sun", "lab-oil", "lab-coal", "lab-fall", "lab-mill", "sun", "plant", "oil", "coal", "fall", "drops", "mill"):
        L.fade_out(a, s("s17", -0.3), 0.7) if a not in ("sun", "mill") else None
    L.fade_out("sun", s("s17", -0.3), 0.7)
    L.fade_out("mill", s("s17", -0.3), 0.7)
    L.write("rule", s("s17", 1.0), 2.4)
    L.write("rule2", s("s18", 0.8), 2.4)
    L.bob("battery", s("s19"), 1, 20, 0.6)
    L.bob("can", s("s19", 0.2), 1, 20, 0.6)
    L.bob("toy", s("s19", 0.4), 1, 20, 0.6)
    return L
