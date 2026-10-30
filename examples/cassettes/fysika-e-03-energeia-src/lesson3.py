"""Lesson 3: energy keeps changing form (energy converters, conservation)."""
from lib import Scene, Lesson, text
from icons import *

ID = "energeia-metatropes"
TITLE = "Η ενέργεια αλλάζει συνεχώς μορφή"
SUMMARY = "Οι συσκευές είναι μετατροπείς ενέργειας: αλλάζουν την ενέργεια από μία μορφή σε μία άλλη."

SENTENCES = [
    ("s1", "Όλες οι συσκευές που χρησιμοποιούμε χρειάζονται ενέργεια, για να λειτουργήσουν.", 0.8),
    ("s2", "Κάθε συσκευή αλλάζει την ενέργεια από μία μορφή σε μία άλλη. Γι’ αυτό τη λέμε μετατροπέα ενέργειας.", 1.0),
    ("s3", "Ας δούμε ένα πείραμα με ένα ελατήριο. Το πιέζουμε και το κρατάμε. Έχει αποθηκευμένη δυναμική ενέργεια.", 1.0),
    ("s4", "Μπροστά του υπάρχει ένα μικρό χάρτινο μπαλάκι. Τώρα αφήνουμε το ελατήριο απότομα.", 0.7),
    ("s5", "Το ελατήριο τινάζεται και σπρώχνει το μπαλάκι μακριά! Η δυναμική ενέργεια έγινε κινητική.", 1.5),
    ("s6", "Ας δούμε και ένα κύκλωμα, με μια μπαταρία, καλώδια και ένα λαμπάκι. Το λαμπάκι φωτοβολεί.", 1.0),
    ("s7", "Η χημική ενέργεια της μπαταρίας γίνεται ηλεκτρική στα καλώδια, και μετά φωτεινή στο λαμπάκι.", 1.0),
    ("s8", "Αν αποσυνδέσουμε την μπαταρία, το λαμπάκι σβήνει. Χωρίς ενέργεια, δεν γίνεται καμία αλλαγή!", 1.5),
    ("s9", "Για να σημειώνουμε τις αλλαγές, γράφουμε: από ποια μορφή ενέργειας, σε ποια μορφή ενέργειας.", 1.2),
    ("s10", "Ο ηλεκτρικός ανεμιστήρας μετατρέπει την ηλεκτρική ενέργεια σε κινητική.", 1.2),
    ("s11", "Το καμινέτο μετατρέπει τη χημική ενέργεια σε θερμότητα και φως.", 1.2),
    ("s12", "Το αυτοκίνητο μετατρέπει τη χημική ενέργεια της βενζίνης σε κινητική ενέργεια και θερμότητα.", 1.2),
    ("s13", "Το κουρδισμένο παιχνίδι μετατρέπει τη δυναμική ενέργεια του ελατηρίου σε κινητική.", 1.5),
    ("s14", "Σε μεγάλα εργοστάσια η ενέργεια μετατρέπεται σε ηλεκτρική, και ταξιδεύει μέσα από τα καλώδια της ΔΕΗ μέχρι το σπίτι μας.", 1.5),
    ("s15", "Μια σημαντική αρχή: η ενέργεια δεν χάνεται και δεν δημιουργείται, απλώς αλλάζει μορφή.", 1.2),
    ("s16", "Έτσι, η ενέργεια αλλάζει συνεχώς μορφή, γύρω μας και μέσα στη φύση.", 1.0),
    ("s17", "Πολλές φορές οι αλλαγές είναι ευεργετικές, όπως η βροχή, ο άνεμος και η ανάπτυξη των φυτών.", 1.2),
    ("s18", "Άλλες φορές είναι καταστροφικές, όπως μια πυρκαγιά, ένας σεισμός ή ένας τυφώνας.", 1.2),
    ("s19", "Μπορείς να βρεις μια συσκευή στο σπίτι σου και να πεις, από ποια μορφή σε ποια αλλάζει την ενέργεια;", 0.5),
]


def factory():
    return ('<rect x="-110" y="-20" width="220" height="130" fill="#90a4ae"/><rect x="-80" y="-80" width="36" height="70" fill="#78909c"/>'
            '<rect x="-20" y="-60" width="36" height="50" fill="#78909c"/><rect x="40" y="10" width="40" height="50" fill="#e1f5fe"/>'
            '<rect x="-90" y="10" width="40" height="50" fill="#e1f5fe"/>')


def build(T):
    sc = Scene()
    sc.define('<linearGradient id="bg3" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#e8eaf6"/><stop offset="1" stop-color="#fff8e1"/></linearGradient>')
    sc.static('<rect width="1600" height="900" fill="url(#bg3)"/>')

    # A: spring experiment
    sc.actor("desk", '<rect x="150" y="640" width="1300" height="40" rx="10" fill="#8d6e63"/><rect x="250" y="680" width="30" height="200" fill="#6d4c41"/>'
             '<rect x="1320" y="680" width="30" height="200" fill="#6d4c41"/><rect x="268" y="540" width="32" height="104" rx="8" fill="#78909c"/>')
    sc.actor("spr", g(spring(240, 7, 34), 420, 592) + '<rect x="540" y="548" width="18" height="90" rx="6" fill="#455a64"/>', origin=(300, 592))
    sc.actor("pball", g(paper_ball(34), 592, 604) , origin=(592, 604))
    sc.actor_text("lab-pot", 420, 470, "δυναμική ενέργεια", 56, C_POT)
    sc.actor_text("lab-kin", 1100, 470, "κινητική ενέργεια", 56, C_KIN)
    sc.actor("arr-a", arrow(630, 400, 900, 400, "#455a64", 14, 36))

    # B: circuit
    sc.actor("bat", g(battery(170, 90), 450, 650), origin=(450, 650))
    sc.actor("wire-a", '<path d="M553 650 L1128 650 L1128 606" fill="none" stroke="#455a64" stroke-width="9" stroke-linejoin="round"/>'
             '<path d="M300 650 L300 790 L1250 790 L1250 650 L1172 650 L1172 606" fill="none" stroke="#455a64" stroke-width="9" stroke-linejoin="round"/>')
    sc.actor("wire-cut", '<path d="M365 650 L300 650" fill="none" stroke="#455a64" stroke-width="9" stroke-linecap="round"/>', origin=(300, 650))
    sc.actor("lamp-off", g(bulb(False), 1150, 510, 1.2))
    sc.actor("lamp-on", g(glow(130), 1150, 495) + g(bulb(True), 1150, 510, 1.2))
    sc.actor_text("lab-chem", 450, 560, "χημική", 56, C_CHEM)
    sc.actor_text("lab-elec", 800, 600, "ηλεκτρική", 56, C_ELEC)
    sc.actor_text("lab-light", 1150, 340, "φωτεινή", 56, "#ef6c00")

    # C: from -> to rows
    sc.actor_text("fromto", 800, 120, "από  →  σε", 80, "#283593")
    rows = [
        ("fan", 270, "ανεμιστήρας", "ηλεκτρική → κινητική", (g(fan_stand(), 0, 10, 0.5) + g(fan_blades(), 0, -10, 0.5)), C_ELEC),
        ("stove", 440, "καμινέτο", "χημική → θερμότητα και φως", g(camp_stove(), 0, 0, 0.75), C_CHEM),
        ("car", 610, "αυτοκίνητο", "χημική → κινητική και θερμότητα", g(car(), 0, 0, 0.6), C_CHEM),
        ("toy", 780, "παιχνίδι", "δυναμική → κινητική", g(toy_car(), 0, 0, 0.95), C_POT),
    ]
    for rid, y, name, conv, icon, col in rows:
        sc.actor(f"row-{rid}", g(icon, 190, y) + text(330, y - 8, name, 54, "#263238", "start") + text(330, y + 52, conv, 48, col, "start"),
                 origin=(800, y))

    # D: the grid
    sc.actor("factory", g(factory(), 280, 560, 1.4), origin=(280, 560))
    sc.actor("grid", g(pylon(), 800, 520, 1.3) + '<path d="M420 530 Q620 470 710 470 M890 470 Q1000 470 1150 560" fill="none" stroke="#455a64" stroke-width="6"/>', origin=(800, 520))
    sc.actor("home", g(house(), 1340, 570, 1.3), origin=(1340, 570))
    sc.actor("bolt-t", g(bolt(), 0, 0, 0.5) , origin=(300, 700))
    sc.items[-1] = ("bolt-t", g(bolt(), 300, 700, 0.55))
    sc.actor_text("lab-fac", 280, 800, "εργοστάσιο", 50, "#37474f")
    sc.actor_text("lab-grid", 800, 800, "δίκτυο ΔΕΗ", 50, "#37474f")
    sc.actor_text("lab-home", 1340, 800, "σπίτι", 50, "#37474f")

    # E: principle and nature
    sc.actor_text("p1", 800, 140, "Δεν χάνεται.", 70, "#b71c1c")
    sc.actor_text("p2", 800, 240, "Δεν δημιουργείται.", 70, "#e65100")
    sc.actor_text("p3", 800, 340, "Αλλάζει μορφή!", 70, "#1b5e20")
    sc.actor("cloud", g(cloud("#b0bec5"), 400, 590, 1.7), origin=(400, 590))
    sc.actor("rain", g(rain(), 400, 700, 1.5))
    sc.actor("plant", g(plant(), 800, 730, 1.5), origin=(800, 880))
    sc.actor_text("lab-good", 640, 450, "ευεργετικές", 60, "#2e7d32")
    sc.actor("home2", g(house(), 1250, 690, 1.4), origin=(1250, 850))
    sc.actor("crack", '<path d="M1080 850 L1180 825 L1160 865 L1260 848 L1240 895 L1400 862" fill="none" stroke="#3e2723" stroke-width="10" stroke-linejoin="round"/>')
    sc.actor_text("lab-bad", 1250, 450, "καταστροφικές", 60, "#c62828")

    L = Lesson(ID, TITLE, SUMMARY, sc, T)
    L.sentences = [(k, t) for k, t, _ in SENTENCES]
    s, e = T.s, T.e
    d = lambda k: e(k) - s(k)

    # A
    L.fade_in("desk", s("s3", -0.2), 0.5)
    L.fade_in("spr", s("s3"), 0.5)
    L.k("spr", s("s3", 0.8), scale=1)
    L.k("spr", s("s3", 2.0), scale=0.5, ease="in-out")
    L.write("lab-pot", s("s3", 3.0), 1.0)
    L.pop("pball", s("s4"), 0.5)
    L.k("spr", s("s5"), scale=0.5)
    L.k("spr", s("s5", 0.18), scale=1.0, ease="out")
    L.k("pball", s("s5", 0.0), x=0, y=0, rotate=0)
    L.k("pball", s("s5", 0.18), x=0, y=0, rotate=0, ease="linear")
    L.k("pball", s("s5", 2.2), x=620, y=0, rotate=900, ease="out")
    L.write("lab-kin", s("s5", 1.2), 1.0)
    L.fade_in("arr-a", s("s5", 3.2), 0.6)
    for a in ("desk", "spr", "pball", "lab-pot", "lab-kin", "arr-a"):
        L.fade_out(a, s("s6", -0.5), 0.5)
    # B
    L.fade_in("bat", s("s6"), 0.5)
    L.fade_in("wire-a", s("s6"), 0.5)
    L.fade_in("wire-cut", s("s6"), 0.5)
    L.fade_in("lamp-off", s("s6"), 0.5)
    L.fade_in("lamp-on", s("s6", 3.6), 0.6)
    L.write("lab-chem", s("s7"), 0.8)
    L.write("lab-elec", s("s7", 0.45 * d("s7")), 0.9)
    L.write("lab-light", s("s7", 0.75 * d("s7")), 0.8)
    L.spin("wire-cut", s("s8", 0.2), s("s8", 1.0), -55, "out")
    L.fade_out("lamp-on", s("s8", 1.0), 0.3)
    for a in ("bat", "wire-a", "wire-cut", "lamp-off", "lab-chem", "lab-elec", "lab-light"):
        L.fade_out(a, s("s9", -0.5), 0.5)
    # C
    L.write("fromto", s("s9", 1.5), 1.2)
    for rid, key in (("fan", "s10"), ("stove", "s11"), ("car", "s12"), ("toy", "s13")):
        L.rise(f"row-{rid}", s(key), 0.6, 40)
    for a in ("fromto", "row-fan", "row-stove", "row-car", "row-toy"):
        L.fade_out(a, s("s14", -0.4), 0.5)
    # D
    L.pop("factory", s("s14"), 0.5)
    L.write("lab-fac", s("s14", 0.3), 0.8)
    L.pop("grid", s("s14", 2.0), 0.5)
    L.write("lab-grid", s("s14", 2.3), 0.9)
    L.pop("home", s("s14", 5.0), 0.5)
    L.write("lab-home", s("s14", 5.3), 0.8)
    L.k("bolt-t", s("s14", 3.0), opacity=0, x=0, y=0)
    L.k("bolt-t", s("s14", 3.3), opacity=1, x=0, y=0)
    L.k("bolt-t", s("s14", 7.0), opacity=1, x=1040, y=-60, ease="in-out")
    L.k("bolt-t", s("s14", 7.4), opacity=0, x=1040, y=-60)
    for a in ("factory", "grid", "home", "lab-fac", "lab-grid", "lab-home", "bolt-t"):
        L.fade_out(a, s("s15", -0.4), 0.5)
    # E
    L.write("p1", s("s15", 1.0), 1.0)
    L.write("p2", s("s15", 2.2), 1.2)
    L.write("p3", s("s15", 3.8), 1.0)
    L.pop("cloud", s("s17"), 0.6)
    L.fade_in("rain", s("s17", 0.6), 0.5)
    for i in range(12):
        L.k("rain", s("s17", 1.1 + 0.35 * i), y=0 if i % 2 == 0 else 30, ease="linear")
    L.write("lab-good", s("s17", 0.3), 0.9)
    L.k("plant", s("s17", 3.0), opacity=0, scale=0.2)
    L.k("plant", s("s17", 5.0), opacity=1, scale=1.0, ease="out")
    L.pop("home2", s("s18"), 0.5)
    L.write("lab-bad", s("s18", 0.3), 1.0)
    L.fade_in("crack", s("s18", 2.0), 0.4)
    L.k("home2", s("s18", 1.8), x=0)
    for i in range(8):
        L.k("home2", s("s18", 2.0 + 0.2 * i), x=(10 if i % 2 == 0 else -10), ease="in-out")
    L.k("home2", s("s18", 3.8), x=0)
    return L
