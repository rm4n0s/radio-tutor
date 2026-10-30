"""Lesson 4: energy is degraded (useful energy turns into heat)."""
from lib import Scene, Lesson, text
from icons import *

ID = "energeia-ypovathmisi"
TITLE = "Η ενέργεια υποβαθμίζεται"
SUMMARY = "Κάθε φορά που η ενέργεια αλλάζει μορφή, ένα μέρος της γίνεται θερμότητα που δεν την αξιοποιούμε."

SENTENCES = [
    ("s1", "Όταν χρησιμοποιούμε μια συσκευή, η ενέργεια αλλάζει μορφή. Όμως δεν αλλάζει πάντα σε μορφή που μας χρειάζεται.", 0.8),
    ("s2", "Ας δούμε ένα φορτηγό. Το πετρέλαιο στη δεξαμενή του έχει χημική ενέργεια, που τη χρησιμοποιούμε εύκολα.", 1.0),
    ("s3", "Η μηχανή κάνει το φορτηγό να κινείται. Όμως ένα μέρος της ενέργειας γίνεται θερμότητα, στη μηχανή και στα λάστιχα που τρίβονται στον δρόμο.", 1.0),
    ("s4", "Αυτή τη θερμότητα δεν μπορούμε να τη χρησιμοποιήσουμε εύκολα. Σκορπίζεται στον αέρα γύρω μας.", 1.0),
    ("s5", "Λέμε ότι η ενέργεια υποβαθμίστηκε. Υποβάθμιση είναι η μετατροπή της ενέργειας σε μορφές που δεν μπορούμε να αξιοποιήσουμε.", 1.5),
    ("s6", "Το ίδιο συμβαίνει και με έναν ανεμιστήρα. Παίρνει ηλεκτρική ενέργεια, που τη χρησιμοποιούμε εύκολα.", 1.0),
    ("s7", "Τα πτερύγια γυρίζουν, αλλά ο ανεμιστήρας ζεσταίνεται λίγο. Ένα μέρος της ενέργειας υποβαθμίζεται σε θερμότητα.", 1.5),
    ("s8", "Τώρα φαντάσου ένα ποδήλατο ανάποδα, πάνω σε ένα θρανίο, με ένα λαμπάκι.", 0.8),
    ("s9", "Στη ρόδα ακουμπάει ο δυναμό. Είναι μια μικρή συσκευή, που μετατρέπει την κίνηση της ρόδας σε ηλεκτρική ενέργεια.", 1.0),
    ("s10", "Όταν η δασκάλα γυρίζει τη ρόδα αργά, το λαμπάκι φωτίζει λίγο.", 1.0),
    ("s11", "Όταν τη γυρίζει γρήγορα, το λαμπάκι φωτίζει πολύ περισσότερο.", 1.0),
    ("s12", "Η κινητική ενέργεια της ρόδας γίνεται ηλεκτρική στον δυναμό, και μετά φωτεινή στο λαμπάκι.", 1.2),
    ("s13", "Και εδώ ένα μέρος της ενέργειας γίνεται θερμότητα, γιατί ο δυναμό και το λαμπάκι ζεσταίνονται.", 1.2),
    ("s14", "Κάθε φορά που η ενέργεια αλλάζει μορφή, ένα μέρος της καταλήγει ως θερμότητα. Γι’ αυτό λέμε ότι η ενέργεια υποβαθμίζεται.", 1.5),
    ("s15", "Τα φωτοβολταϊκά πάνελ μετατρέπουν τη φωτεινή ενέργεια του Ήλιου σε ηλεκτρική.", 1.0),
    ("s16", "Η ηλεκτρική ενέργεια μπορεί μετά να γυρίσει ένα μικρό ανεμιστηράκι. Η φωτεινή ενέργεια γίνεται ηλεκτρική, και μετά κινητική.", 1.5),
    ("s17", "Στο σπίτι μας, ο μετρητής της ΔΕΗ δεν μετράει ενέργεια που χάνεται. Μετράει την ενέργεια που μετατρέπεται και υποβαθμίζεται σε θερμότητα.", 1.0),
    ("s18", "Η ΔΕΗ μας χρεώνει γι’ αυτή την ποσότητα της ενέργειας.", 1.0),
    ("s19", "Η συνολική ενέργεια διατηρείται, αλλά η χρήσιμη ενέργεια λιγοστεύει. Γι’ αυτό τη χρησιμοποιούμε με προσοχή.", 0.5),
]


def rising(L, actor, t0, t1, per=1.4, dy=-60):
    t = t0
    while t + per <= t1:
        L.k(actor, t, y=0, opacity=0)
        L.k(actor, t + 0.4 * per, opacity=1, ease="out")
        L.k(actor, t + per - 0.02, y=dy, opacity=0, ease="in")
        t += per


def build(T):
    sc = Scene()
    sc.define('<linearGradient id="bg4" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#e3f2fd"/><stop offset="1" stop-color="#fff3e0"/></linearGradient>')
    sc.static('<rect width="1600" height="900" fill="url(#bg4)"/>')
    sc.static('<rect y="730" width="1600" height="170" fill="#cfd8dc"/><rect y="730" width="1600" height="12" fill="#90a4ae"/>')

    # A: truck
    sc.actor("barrel", g(barrel(), 230, 560, 1.5), origin=(230, 560))
    sc.actor_text("lab-chem", 230, 790, "χημική", 58, C_CHEM)
    sc.actor("arr-main", arrow(370, 560, 560, 560, C_KIN, 24, 50))
    sc.actor("truck", g(truck(), 890, 520, 1.5), origin=(890, 520))
    sc.actor_text("lab-kin", 890, 790, "κινητική", 58, C_KIN)
    sc.actor("heat-eng", g(heat_waves(), 1060, 330, 0.9))
    sc.actor("heat-roll", g(heat_waves(), 680, 660, 0.6))
    sc.actor_text("lab-heat", 1360, 400, "θερμότητα", 58, C_HEAT)
    sc.actor_text("lab-degr", 800, 130, "υποβάθμιση της ενέργειας", 70, "#b71c1c")

    # B: fan
    sc.actor("bolt-b", g(bolt(), 230, 480, 1.4), origin=(230, 480))
    sc.actor_text("lab-elec", 230, 790, "ηλεκτρική", 58, C_ELEC)
    sc.actor("arr-fan", arrow(370, 520, 560, 520, C_ELEC, 24, 50))
    sc.actor("fan-stand", g(fan_stand(), 830, 480, 1.7))
    sc.actor("fan-blades", g(fan_blades(), 830, 435, 1.7), origin=(830, 435))
    sc.actor_text("lab-kin2", 830, 790, "κινητική", 58, C_KIN)
    sc.actor("heat-fan", g(heat_waves(), 1130, 330, 0.9))
    sc.actor_text("lab-heat2", 1330, 790, "θερμότητα", 58, C_HEAT)

    # C: bike with dynamo
    sc.actor("table", '<rect x="250" y="590" width="560" height="26" rx="8" fill="#8d6e63"/><rect x="290" y="616" width="24" height="120" fill="#6d4c41"/><rect x="746" y="616" width="24" height="120" fill="#6d4c41"/>')
    sc.actor("bike", g(bike_frame(), 530, 470, 1.0, rot=180))
    sc.actor("wheel-f", g(wheel(88), 640, 410), origin=(640, 410))
    sc.actor("wheel-r", g(wheel(88), 420, 410), origin=(420, 410))
    sc.actor("dynamo", g(tiny_dynamo(), 740, 410, 1.2) + '<path d="M740 440 L740 600 L1180 600 L1180 508" fill="none" stroke="#455a64" stroke-width="9" stroke-linejoin="round"/>')
    sc.actor("lamp-off", g(bulb(False), 1180, 410, 1.3))
    sc.actor("lamp-dim", g(glow(130), 1180, 395) + g(bulb(True), 1180, 410, 1.3))
    sc.actor("lamp-bright", g(glow(190), 1180, 395) + g(glow(130), 1180, 395) + g(bulb(True), 1180, 410, 1.3))
    sc.actor_text("c-kin", 330, 810, "κινητική", 54, C_KIN)
    sc.actor_text("c-elec", 800, 810, "ηλεκτρική", 54, C_ELEC)
    sc.actor_text("c-light", 1270, 810, "φωτεινή", 54, "#ef6c00")
    sc.actor("c-arrows", arrow(480, 785, 620, 785, "#455a64", 12, 30) + arrow(990, 785, 1100, 785, "#455a64", 12, 30))
    sc.actor("heat-c", g(heat_waves(), 1420, 480, 0.8) + text(1420, 590, "θερμότητα", 46, C_HEAT))

    # D: solar panel and fan
    sc.actor("p-sun", g(sun(55), 250, 170), origin=(250, 170))
    sc.actor_text("p-l1", 250, 330, "φωτεινή", 54, "#ef6c00")
    sc.actor("p-panel", g(panel(), 250, 530, 1.6), origin=(250, 530))
    sc.actor("p-arrow", arrow(470, 540, 880, 540, C_ELEC, 22, 46))
    sc.actor_text("p-l2", 675, 500, "ηλεκτρική", 54, C_ELEC)
    sc.actor("p-stand", g(fan_stand(), 1130, 520, 1.4))
    sc.actor("p-blades", g(fan_blades(), 1130, 484, 1.4), origin=(1130, 484))
    sc.actor_text("p-l3", 1130, 790, "κινητική", 54, C_KIN)

    # E: the electricity meter
    sc.actor("home", g(house(), 380, 520, 2.0), origin=(380, 520))
    sc.actor("meter", '<rect x="150" y="470" width="110" height="130" rx="12" fill="#eceff1" stroke="#455a64" stroke-width="7"/><circle cx="205" cy="520" r="36" fill="#fff" stroke="#455a64" stroke-width="5"/>')
    sc.actor("needle", '<line x1="205" y1="520" x2="205" y2="490" stroke="#c62828" stroke-width="6" stroke-linecap="round"/>', origin=(205, 520))
    sc.actor("heat-home", g(heat_waves(), 560, 420, 0.8))
    sc.actor_text("m1", 1130, 330, "Ο μετρητής μετράει", 54, "#263238")
    sc.actor_text("m2", 1130, 410, "την ενέργεια που", 54, "#263238")
    sc.actor_text("m3", 1130, 490, "μετατρέπεται στο σπίτι.", 54, "#263238")
    sc.actor_text("m4", 800, 860, "Χρησιμοποίησέ την με μέτρο!", 70, "#1b5e20")

    L = Lesson(ID, TITLE, SUMMARY, sc, T)
    L.sentences = [(k, t) for k, t, _ in SENTENCES]
    s, e = T.s, T.e
    d = lambda k: e(k) - s(k)

    # A
    L.pop("barrel", s("s2"), 0.5)
    L.write("lab-chem", s("s2", 0.5), 0.9)
    L.fade_in("arr-main", s("s3", 0.0), 0.6)
    L.pop("truck", s("s3", 0.2), 0.5)
    L.move("truck", s("s3", 1.0), e("s4"), (0, 0), (60, 0), "in-out")
    L.write("lab-kin", s("s3", 1.5), 0.9)
    rising(L, "heat-eng", s("s3", 3.0), e("s5"))
    rising(L, "heat-roll", s("s3", 4.0), e("s5"))
    L.write("lab-heat", s("s4"), 1.0)
    L.write("lab-degr", s("s5", 0.5), 2.0)
    for a in ("barrel", "lab-chem", "arr-main", "truck", "lab-kin", "lab-heat", "lab-degr"):
        L.fade_out(a, s("s6", -0.4), 0.5)
    # B
    L.pop("bolt-b", s("s6"), 0.5)
    L.write("lab-elec", s("s6", 0.4), 0.9)
    L.fade_in("arr-fan", s("s6", 2.0), 0.5)
    L.fade_in("fan-stand", s("s6", 2.5), 0.5)
    L.fade_in("fan-blades", s("s6", 2.5), 0.5)
    L.spin("fan-blades", s("s6", 2.8), e("s7"), 2600)
    L.write("lab-kin2", s("s7", 0.2), 0.9)
    rising(L, "heat-fan", s("s7", 2.5), e("s7"))
    L.write("lab-heat2", s("s7", 2.8), 1.0)
    for a in ("bolt-b", "lab-elec", "arr-fan", "fan-stand", "fan-blades", "lab-kin2", "lab-heat2"):
        L.fade_out(a, s("s8", -0.4), 0.5)
    # C
    for a in ("table", "bike", "wheel-f", "wheel-r", "lamp-off"):
        L.fade_in(a, s("s8"), 0.6)
    L.fade_in("dynamo", s("s9"), 0.6)
    for w in ("wheel-f", "wheel-r"):
        L.spin(w, s("s10"), e("s10"), 120)
        L.k(w, s("s11"), rotate=120)
        L.k(w, e("s11"), rotate=120 + 1500, ease="linear")
        L.k(w, e("s11", 3.0), rotate=120 + 1500 + 700, ease="out")
    L.fade_in("lamp-dim", s("s10", 0.6), 0.8)
    L.fade_out("lamp-dim", s("s11", 0.6), 0.6)
    L.fade_in("lamp-bright", s("s11", 0.6), 0.8)
    L.write("c-kin", s("s12"), 0.8)
    L.fade_in("c-arrows", s("s12", 1.0), 0.5)
    L.write("c-elec", s("s12", 2.0), 0.9)
    L.write("c-light", s("s12", 4.0), 0.9)
    rising(L, "heat-c", s("s13", 0.5), e("s14", -2.0))
    for a in ("table", "bike", "wheel-f", "wheel-r", "lamp-off", "dynamo", "lamp-bright", "c-kin", "c-elec", "c-light", "c-arrows"):
        L.fade_out(a, s("s15", -0.4), 0.5)
    # D
    L.pop("p-sun", s("s15"), 0.5)
    L.spin("p-sun", s("s15"), e("s16"), 90)
    L.write("p-l1", s("s15", 0.5), 0.8)
    L.pop("p-panel", s("s15", 1.5), 0.5)
    L.fade_in("p-arrow", s("s16"), 0.6)
    L.write("p-l2", s("s16", 0.6), 0.9)
    L.fade_in("p-stand", s("s16", 2.0), 0.5)
    L.fade_in("p-blades", s("s16", 2.0), 0.5)
    L.spin("p-blades", s("s16", 2.5), e("s16"), 1800)
    L.write("p-l3", s("s16", 4.5), 0.9)
    for a in ("p-sun", "p-l1", "p-panel", "p-arrow", "p-l2", "p-stand", "p-blades", "p-l3"):
        L.fade_out(a, s("s17", -0.4), 0.5)
    # E
    L.pop("home", s("s17"), 0.5)
    L.fade_in("meter", s("s17", 0.5), 0.5)
    L.fade_in("needle", s("s17", 0.5), 0.5)
    L.spin("needle", s("s17", 1.0), e("s18"), 1000)
    L.write("m1", s("s17", 1.0), 1.0)
    L.write("m2", s("s17", 2.5), 1.0)
    L.write("m3", s("s17", 4.0), 1.2)
    rising(L, "heat-home", s("s17", 5.5), e("s18"))
    L.write("m4", s("s19", 1.0), 1.8)
    return L
