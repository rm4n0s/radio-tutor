#!/usr/bin/env python3
"""Builds the cassette folder: narration (piper), scenes, lessons, exercises.

  WORK=<scratch dir> python3 build.py <out cassette folder> [lesson ids...]

The narration's text and timings are also written to script.md next to this file.
"""
import importlib
import json
import os
import sys

import icons
import lib

HERE = os.path.dirname(os.path.abspath(__file__))
LESSONS = ["lesson1", "lesson2", "lesson3", "lesson4", "lesson5"]


MANIFEST = {
    "format": 1,
    "id": "dev.radiotutor.examples.fysika-e.03-energeia",
    "version": "1.0.0",
    "title": "Φυσική Ε΄ Δημοτικού: Ενέργεια",
    "description": "Πέντε μαθήματα με αφήγηση και πέντε φύλλα εργασίας για την ενέργεια: οι μορφές της, οι αποθήκες της, οι μετατροπές της, η υποβάθμισή της και η ενέργεια από τις τροφές. Βασισμένο στην ύλη του κεφαλαίου «Ενέργεια» του βιβλίου «Φυσικά Ε΄ Δημοτικού – Ερευνώ και Ανακαλύπτω» (ΙΤΥΕ Διόφαντος). Τα κείμενα και τα σχέδια είναι δικά μας.",
    "language": "el",
    "subject": "Φυσική",
    "ages": [10, 11],
    "authors": ["Claude, ως παράδειγμα για το Radio Tutor"],
    "source": "Φυσικά Ε΄ Δημοτικού – Ερευνώ και Ανακαλύπτω (ΙΤΥΕ Διόφαντος), κεφάλαιο «Ενέργεια»: Βιβλίο Μαθητή σ. 24–31, Τετράδιο Εργασιών σ. 40–55",
    "grader_instructions": (
        "Μίλα στο παιδί στα ελληνικά, στον ενικό, με απλά λόγια και ζεστό τόνο, όπως μια φιλική δασκάλα. "
        "Επαίνεσε πρώτα την προσπάθεια, ακόμα και όταν η απάντηση δεν είναι σωστή, και μετά πες με μία ή δύο προτάσεις τι να ξανακοιτάξει, χωρίς να δώσεις ολόκληρη τη σωστή απάντηση. "
        "Το παιδί είναι δέκα έως έντεκα ετών και γράφει με το χέρι: μη μετράς τα ορθογραφικά λάθη και τους τόνους, αρκεί να καταλαβαίνεις τι εννοεί. "
        "Δέξου ισοδύναμες διατυπώσεις: «θερμότητα» ή «θερμική ενέργεια», «φωτεινή» ή «ενέργεια του φωτός», «δυναμική» ή «δυναμική (ελαστική)». "
        "Στις ερωτήσεις με «γιατί» αρκεί η σωστή ιδέα με δικά του λόγια. "
        "Μην ζητάς από το παιδί να κάνει πείραμα και μη συστήσεις ποτέ κάτι επικίνδυνο, ούτε δίαιτα ή νηστεία: οι ασκήσεις είναι μόνο με χαρτί και μολύβι."),
}


COVER = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300 400">'
         '<defs><linearGradient id="cv" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#fff59d"/><stop offset="1" stop-color="#ffcc80"/></linearGradient></defs>'
         '<rect width="300" height="400" fill="url(#cv)"/>'
         + icons.g(icons.sun(26, 12), 235, 70, 1.0)
         + icons.g(icons.glow(60), 150, 215) + icons.g(icons.bulb(True), 150, 215, 1.3)
         + icons.g(icons.spring(110, 6, 16, True), 70, 330, 1.0) + icons.g(icons.ball(18), 150, 330) + icons.g(icons.bolt(), 235, 320, 0.4)
         + lib.text(150, 55, "Ενέργεια", 50, "#bf360c")
         + lib.text(150, 385, "Φυσική Ε΄ Δημοτικού", 24, "#4e342e")
         + '</svg>\n')


def write_exercises(out, items):
    import exercises
    for lid, exs in exercises.WORKSHEETS.items():
        for eid, pic, ex in exs:
            d = os.path.join(out, "exercises", eid)
            os.makedirs(d, exist_ok=True)
            obj = {"format": 1}
            obj.update(ex)
            obj.setdefault("show_work", False)
            obj["max_attempts"] = 3
            order = ["format", "title", "label", "prompt", "reference_solution", "grading", "show_work", "max_attempts", "hints"]
            obj = {k: obj[k] for k in order}
            with open(os.path.join(d, "exercise.json"), "w", encoding="utf-8") as fh:
                json.dump(obj, fh, ensure_ascii=False, indent=2)
                fh.write("\n")
            if pic:
                with open(os.path.join(d, "prompt.svg"), "w", encoding="utf-8") as fh:
                    fh.write(pic)
            items.setdefault(lid, []).append(eid)


def main():
    out = os.path.abspath(sys.argv[1])
    only = sys.argv[2:]
    work = os.environ["WORK"]
    sys.path.insert(0, HERE)
    md = ["# Narration script: Ενέργεια (dev.radiotutor.examples.fysika-e.03-energeia)\n",
          "Voice: piper, model `el_GR-rapunzelina-low.onnx`, `--length-scale 1.1`, one WAV per sentence, "
          "joined with the pauses below and encoded with `ffmpeg -c:a libvorbis -q:a 2` (22050 Hz, mono).\n"
          "To re-record with another voice: change `PIPER` in `lib.py` and run `build.py` again; the timings "
          "are measured again and the scenes follow them.\n"]
    for name in LESSONS:
        if not os.path.exists(os.path.join(HERE, name + ".py")):
            continue
        mod = importlib.import_module(name)
        if only and mod.ID not in only:
            continue
        ldir = os.path.join(out, "lessons", mod.ID)
        os.makedirs(ldir, exist_ok=True)
        T = lib.synth(work, mod.ID, mod.SENTENCES, os.path.join(ldir, "narration.ogg"))
        L = mod.build(T)
        lj = L.lesson_json()
        with open(os.path.join(ldir, "lesson.json"), "w", encoding="utf-8") as fh:
            fh.write(lib.dump_json(lj))
        with open(os.path.join(ldir, "scene.svg"), "w", encoding="utf-8") as fh:
            fh.write(L.scene.svg())
        # preview frames (outside the cassette)
        pdir = os.path.join(work, "frames", mod.ID)
        os.makedirs(pdir, exist_ok=True)
        for key, _, _ in mod.SENTENCES:
            t = min(T.total, (T.start[key] + T.end[key]) / 2 + 0.0)
            tt = T.end[key] - 0.2
            with open(os.path.join(pdir, f"{key}.svg"), "w", encoding="utf-8") as fh:
                fh.write(lib.frame_svg(L.scene, lj, tt))
        md.append(f"\n## {mod.ID}: {mod.TITLE}\n\nLength: {T.total:.1f} s\n\n| # | start | end | pause after | sentence |\n|---|---|---|---|---|\n")
        for i, (key, text, pause) in enumerate(mod.SENTENCES):
            md.append(f"| {i+1} | {T.start[key]:.2f} | {T.end[key]:.2f} | {pause if i < len(mod.SENTENCES)-1 else lib.TAIL} | {text} |\n")
        print(f"{mod.ID}: {T.total:.1f} s, {len(lj['captions'])} captions, {len(lj['tracks'])} tracks")
    if not only:
        items = {}
        write_exercises(out, items)
        man = dict(MANIFEST)
        man["items"] = []
        for name in LESSONS:
            mod = importlib.import_module(name)
            man["items"].append({"type": "lesson", "id": mod.ID})
            for eid in items[mod.ID]:
                man["items"].append({"type": "exercise", "id": eid})
        with open(os.path.join(out, "cassette.json"), "w", encoding="utf-8") as fh:
            json.dump(man, fh, ensure_ascii=False, indent=2)
            fh.write("\n")
        with open(os.path.join(out, "cover.svg"), "w", encoding="utf-8") as fh:
            fh.write(COVER)
        with open(os.path.join(HERE, "script.md"), "w", encoding="utf-8") as fh:
            fh.write("".join(md))


if __name__ == "__main__":
    main()
