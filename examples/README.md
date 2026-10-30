# Example cassettes / Παραδείγματα κασετών

## Ελληνικά

Εδώ βρίσκονται έτοιμες κασέτες για το Radio Tutor, για παιδιά της Ε΄ Δημοτικού (10 έως 11 ετών). Ακολουθούν τα κεφάλαια του βιβλίου «Φυσικά Ε΄ Δημοτικού – Ερευνώ και Ανακαλύπτω» (ΙΤΥΕ Διόφαντος) με την ίδια σειρά, αλλά με **δικά μας λόγια και δικά μας σχέδια**: δεν αντιγράφουμε προτάσεις, ασκήσεις ή εικόνες των βιβλίων. Κάθε κασέτα έχει μαθήματα με κινούμενη σκηνή, αφήγηση και υπότιτλους, και ασκήσεις που λύνονται στο χαρτί και βαθμολογούνται από μια φωτογραφία.

| # | Κασέτα | Φάκελος |
|---|---|---|
| 01 | Υλικά σώματα | `cassettes/fysika-e-01-ylika-somata` |
| 02 | Μίγματα | `cassettes/fysika-e-02-migmata` |
| 03 | Ενέργεια | `cassettes/fysika-e-03-energeia` |

Για κάθε κασέτα υπάρχουν: ο φάκελος της κασέτας, το πακεταρισμένο αρχείο `.cassette` δίπλα του και ο φάκελος `…-src` με το κείμενο της αφήγησης (`script.md`, `script.json`).

**Η αφήγηση** γίνεται με το [piper](https://github.com/rhasspy/piper) και τη φωνή `el_GR-rapunzelina-low`. Μπορεί να ηχογραφηθεί ξανά από τα κείμενα, με άλλη φωνή ή με αλλαγμένες προτάσεις: δες το `script.md` κάθε κασέτας.

**Πώς εισάγεις μια κασέτα:** άνοιξε το Radio Tutor, πήγαινε στις Κασέτες, Εισαγωγή κασέτας, και διάλεξε ένα αρχείο `.cassette` από τον φάκελο `cassettes/`. Μπορείς πρώτα να την ελέγξεις από το τερματικό: `go run ./cmd/radio-tutor validate examples/cassettes/fysika-e-01-ylika-somata.cassette`. Δες την κασέτα εσύ πριν τη δει το παιδί.

## English

These are ready-made cassettes for Radio Tutor, for Greek 5th-grade children (ages 10 to 11). They follow the chapters of the book «Φυσικά Ε΄ Δημοτικού – Ερευνώ και Ανακαλύπτω» («Physics, 5th grade – I investigate and discover», ITYE Diophantos) in the same order, but in **our own words and our own drawings**: no sentences, exercises or pictures of the books are copied. Each cassette has lessons (an animated scene with narration and captions) and exercises that the child solves on paper and that are graded from a photo.

| # | Cassette | Folder |
|---|---|---|
| 01 | Υλικά σώματα (Material bodies) | `cassettes/fysika-e-01-ylika-somata` |
| 02 | Μίγματα (Mixtures) | `cassettes/fysika-e-02-migmata` |
| 03 | Ενέργεια (Energy) | `cassettes/fysika-e-03-energeia` |

For each cassette there are: the cassette folder, the packed `.cassette` file next to it, and the `…-src` folder with the narration text (`script.md`, `script.json`).

**The narration** is made with [piper](https://github.com/rhasspy/piper) and the voice `el_GR-rapunzelina-low`. It can be recorded again from the scripts, with another voice or with changed sentences: see the `script.md` of each cassette.

**To import a cassette:** open Radio Tutor, go to Cassettes, Import a cassette, and choose a `.cassette` file from the `cassettes/` folder. You can check it first from a terminal: `go run ./cmd/radio-tutor validate examples/cassettes/fysika-e-01-ylika-somata.cassette`. Watch the cassette yourself before your child does.

`cmd/radio-tutor/examples_test.go` validates every `.cassette` in this folder.
