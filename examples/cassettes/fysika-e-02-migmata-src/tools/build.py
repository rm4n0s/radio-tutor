"""Builds the cassette folder: python3 build.py <work dir> <cassette folder> <src folder>

Needs piper (~/piper), ffmpeg and ffprobe. Narration is re-synthesised only for
sentences that changed (cached in the work dir by their text).
"""
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from lib import *
from icons import *
import l1, l2, l3
import exercises

LESSONS = [l1, l2, l3]
WORKSHEETS = [exercises.WORKSHEET_1, exercises.WORKSHEET_2, exercises.WORKSHEET_3]
VERSION = "1.0.0"

SOURCE_CREDIT = ("Βασισμένο στην ύλη του κεφαλαίου «Μίγματα» του βιβλίου «Φυσικά Ε΄ Δημοτικού – Ερευνώ και Ανακαλύπτω» "
                 "(ΙΤΥΕ Διόφαντος).")

GRADER = ("Μίλα στο παιδί με ζεστασιά και απλά λόγια, στον ενικό. Επαίνεσε την προσπάθεια, ακόμα κι όταν η απάντηση δεν είναι σωστή. "
          "Το παιδί είναι 10 έως 11 ετών και γράφει με το χέρι: μην βγάζεις λάθος για ορθογραφία, για τόνους ή για κακό γραφικό χαρακτήρα, "
          "αρκεί να καταλαβαίνεται η απάντηση. Δέξου κάθε σωστή απάντηση που λέει το ίδιο με άλλα λόγια. Όταν κάτι είναι λάθος, εξήγησε σε μία "
          "ή δύο προτάσεις τι να σκεφτεί, χωρίς να δώσεις αμέσως όλη τη λύση. Οι όροι του μαθήματος είναι: μίγμα, συστατικά, καθαρή ουσία, "
          "ετερογενές, ομογενές (διάλυμα), διαλύτης, διαλυμένη ουσία, ίζημα, διαλογή, διήθηση, κράμα.")


def write_json(path, obj):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(obj, f, ensure_ascii=False, indent=2)
        f.write("\n")


def write(path, s):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(s)


def prompt_svg(name):
    if name == "zesto-kryo":
        def spoons(cx, n, y):
            return "".join(circle(cx - 90 + 45 * (k % 5), y - 40 * (k // 5), 14, "#fff8e1", "#bf6a00", 4) for k in range(n))
        out = ['<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 480">',
               '<rect width="1000" height="480" fill="#ffffff"/>',
               glass_cup(250, 360, 240, 280, "#81d4fa", 0.7),
               "".join(circle(180 + 22 * k, 346 - (k % 2) * 8, 9, "#ffffff", "#78909c", 3) for k in range(4)),
               text(250, 420, "κρύο νερό", 40, "#0277bd"), text(250, 462, "3 κουταλάκια", 38, "#bf6a00"),
               glass_cup(750, 360, 240, 280, "#ffcc80", 0.7),
               "".join(path("M%s 130 q-14 -26 0 -46 q14 -20 0 -46" % (690 + 60 * i), "none", "#ef9a9a", 7) for i in range(2)),
               "".join(circle(680 + 22 * k, 346 - (k % 2) * 8, 9, "#ffffff", "#78909c", 3) for k in range(4)),
               text(750, 420, "ζεστό νερό", 40, "#c62828"), text(750, 462, "8 κουταλάκια", 38, "#bf6a00"),
               "</svg>"]
        return "\n".join(out) + "\n"
    if name == "pothari":
        out = ['<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 480">',
               '<rect width="1000" height="480" fill="#ffffff"/>',
               glass_cup(250, 360, 240, 280, "#81d4fa", 0.5),
               "".join(circle(185 + 30 * k, 345 - (k % 2) * 8, 12, "#ffffff", "#78909c", 3) for k in range(5)),
               text(250, 420, "μισογεμάτο ποτήρι", 38, "#0277bd"),
               text(250, 462, "λίγες κουταλιές αλάτι", 36, INK),
               glass_cup(750, 360, 240, 280, "#81d4fa", 0.9),
               text(750, 420, "γεμάτο ποτήρι", 38, "#0277bd"),
               text(750, 462, "πόσες κουταλιές;", 36, INK),
               "</svg>"]
        return "\n".join(out) + "\n"
    raise KeyError(name)


def cover():
    return "\n".join([
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300 400">',
        '<defs><linearGradient id="bg" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#b3e5fc"/><stop offset="1" stop-color="#fff8e1"/></linearGradient></defs>',
        '<rect width="300" height="400" fill="url(#bg)"/>',
        '<rect x="10" y="10" width="280" height="380" rx="26" fill="none" stroke="#ffffff" stroke-width="8"/>',
        text(150, 62, "Φυσικά Ε΄", 34, "#5c6bc0"),
        text(150, 126, "Μίγματα", 66, NAVY),
        text(150, 170, "και διαλύματα", 34, "#5c6bc0"),
        '<g transform="translate(150 285)">',
        '<path d="M-90 40 L90 40 A90 80 0 0 1 -90 40 Z" fill="#fb8c00" transform="translate(0 -20)"/>',
        '<ellipse cx="0" cy="20" rx="90" ry="18" fill="#ffe0b2"/>',
        '<path d="M-90 20 L90 20 A90 76 0 0 1 -90 20 Z" fill="#ffcc80" stroke="#e65100" stroke-width="5"/>',
        "".join('<circle cx="%s" cy="%s" r="%s" fill="%s"/>' % (x, y, r, c) for x, y, r, c in (
            (-50, 40, 12, "#e53935"), (-20, 52, 12, "#42a5f5"), (10, 38, 12, "#fdd835"), (40, 50, 12, "#e53935"),
            (-5, 66, 11, "#43a047"), (-60, 62, 10, "#fdd835"), (62, 36, 10, "#42a5f5"))),
        '<circle cx="-40" cy="-30" r="8" fill="#ffffff" stroke="#90a4ae" stroke-width="3"/>',
        '<circle cx="0" cy="-60" r="10" fill="#ffffff" stroke="#90a4ae" stroke-width="3"/>',
        '<circle cx="45" cy="-35" r="7" fill="#ffffff" stroke="#90a4ae" stroke-width="3"/>',
        "</g>",
        "</svg>"]) + "\n"


def main():
    work, out, src = [os.path.abspath(a) for a in sys.argv[1:4]]
    items = []
    script = ["# Μίγματα και διαλύματα: το κείμενο της αφήγησης\n",
              "Η αφήγηση φτιάχνεται από αυτές τις προτάσεις με το `build.py` (στον φάκελο `tools`), πρόταση προς πρόταση,",
              "με το πρόγραμμα κειμένου σε ομιλία **piper**, φωνή `el_GR-rapunzelina-low.onnx` (ταχύτητα `--length_scale 1.15`).",
              "Για να ξαναηχογραφηθεί με καλύτερη φωνή, άλλαξε το `PIPER`/`MODEL` στο `tools/lib.py`, ή ηχογράφησε μόνος σου κάθε πρόταση",
              "και χρησιμοποίησε τις ίδιες προτάσεις. Οι χρόνοι είναι σε δευτερόλεπτα από την αρχή της αφήγησης.\n",
              "Εντολή: `python3 tools/build.py <φάκελος εργασίας> <φάκελος κασέτας> <αυτός ο φάκελος>`\n"]
    total = 0.0
    lengths = {}
    for li, mod in enumerate(LESSONS):
        wdir = os.path.join(work, mod.ID)
        os.makedirs(wdir, exist_ok=True)
        nar = Narration(wdir, mod.SENTENCES)
        sc = mod.build(nar)
        ldir = os.path.join(out, "lessons", mod.ID)
        os.makedirs(ldir, exist_ok=True)
        ogg_len = nar.write_ogg(os.path.join(ldir, "narration.ogg"))
        write(os.path.join(ldir, "scene.svg"), sc.svg())
        write_json(os.path.join(ldir, "lesson.json"), sc.lesson_json(mod.TITLE, mod.SUMMARY, nar))
        lengths[mod.ID] = ogg_len
        total += ogg_len
        items.append({"type": "lesson", "id": mod.ID})
        script.append("\n## Μάθημα %d: %s (`%s`), %.1f s\n" % (li + 1, mod.TITLE, mod.ID, ogg_len))
        script.append("| από | έως | κλειδί | πρόταση |\n|---|---|---|---|")
        for it in nar.items:
            script.append("| %.2f | %.2f | %s | %s |" % (it["start"], it["end"], it["key"], it["text"]))
        for ex in WORKSHEETS[li]:
            edir = os.path.join(out, "exercises", ex["id"])
            d = {"format": 1, "title": ex["title"], "label": ex["label"], "prompt": ex["prompt"],
                 "reference_solution": ex["reference_solution"], "grading": ex["grading"],
                 "show_work": ex["show_work"], "max_attempts": 3, "hints": ex["hints"]}
            write_json(os.path.join(edir, "exercise.json"), d)
            if ex.get("svg"):
                write(os.path.join(edir, "prompt.svg"), prompt_svg(ex["svg"]))
            items.append({"type": "exercise", "id": ex["id"]})
    write(os.path.join(out, "cover.svg"), cover())
    manifest = {
        "format": 1,
        "id": "dev.radiotutor.examples.fysika-e.02-migmata",
        "version": VERSION,
        "title": "Μίγματα και διαλύματα",
        "description": ("Τι είναι ένα μίγμα, ετερογενή και ομογενή μίγματα, διαχωρισμός των συστατικών, διαλύματα και διαλύτες. "
                        "Τρία μαθήματα και τρία φύλλα ασκήσεων με μολύβι και χαρτί. " + SOURCE_CREDIT),
        "language": "el",
        "subject": "Φυσικά",
        "ages": [10, 11],
        "authors": ["Claude (Anthropic), για το Radio Tutor"],
        "source": "«Φυσικά Ε΄ Δημοτικού – Ερευνώ και Ανακαλύπτω» (ΙΤΥΕ Διόφαντος), κεφάλαιο «Μίγματα»: Βιβλίο Μαθητή σ. 18–23, Τετράδιο Εργασιών σ. 30–37",
        "grader_instructions": GRADER,
        "items": items,
    }
    write_json(os.path.join(out, "cassette.json"), manifest)
    script.append("\nΣυνολική διάρκεια αφήγησης: %.1f s.\n" % total)
    write(os.path.join(src, "script.md"), "\n".join(script))
    print("total narration %.1f s" % total, lengths)


main()
