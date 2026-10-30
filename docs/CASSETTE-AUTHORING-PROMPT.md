# Making a cassette with an AI agent

This page is for parents. It contains a prompt you give to an AI agent that can work with files and run commands on your computer (for example Claude Code, or another coding agent). The agent reads your book or subject, writes the lessons and exercises, makes the narration with a text-to-speech program, and packs everything into a `.cassette` file that you import into Radio Tutor.

## What you need

- An AI agent that can create files and run commands in a folder (a chat window that cannot run commands is not enough, because the narration audio has to be produced and measured).
- `ffmpeg` (with Vorbis support) and a text-to-speech program: `espeak-ng` (installed on most Linux systems, sounds robotic) or `piper` (much more natural; voices for many languages). You can also record the narration yourself (see the note at the end).
- The `radio-tutor` program, to check and pack the cassette. Run `radio-tutor version` to see that it is installed.
- The material: a chapter of a book (PDF, photos of the pages, or typed text), or just a subject and what the child should learn.

The material stays on your computer and with the AI provider your agent uses. Use books you own for your own family; don't share cassettes made from copyrighted books.

## How to use it

1. Make an empty folder, put the material in it (e.g. `source/chapter-12.pdf`), and start the agent in that folder.
2. Copy the prompt below, replace every `{{...}}` with your answer, and give it to the agent.
3. When the agent finishes, it tells you the path of the `.cassette` file. Import it in Radio Tutor (Cassettes → Import), and watch the first lesson yourself before your child does.

To change something later, ask the same agent ("make the second lesson slower", "add two harder exercises after lesson 3"). It must raise the cassette's version and keep the ids of what did not change, so your child's progress is kept.

---

## The prompt

```text
You are making a "cassette" for Radio Tutor, a desktop tutoring app for children.
A cassette is a zip file of animated, narrated lessons and paper-and-pencil
exercises. You will produce one cassette file in this folder.

## What the cassette is about

- Material: {{SOURCE}}
  (for example: "the PDF source/chapter-12.pdf, pages 80-95", or
  "the photos in source/ of chapter 4 of my child's history book", or
  "no book: teach long division with remainders")
- What my child should be able to do at the end: {{GOALS}}
- My child's age: {{AGE}}. School year / level: {{LEVEL}}
- Language of the lessons and exercises: {{LANGUAGE}} (BCP 47 tag, e.g. el, en)
- Size: about {{NUMBER_OF_LESSONS}} lessons, each followed by a worksheet of
  {{EXERCISES_PER_WORKSHEET}} exercises
- How the narrator and the grader should talk to my child: {{TONE}}
  (for example: "warm, simple words, a little humor, praise effort")
- Anything to avoid or to stress: {{NOTES}}
- Cassette id: {{CASSETTE_ID}} (reverse-domain style, e.g.
  org.family-papadopoulos.math.fractions-1) and version: {{VERSION}}
  (1.0.0 for a new cassette)
- Text-to-speech: {{TTS}} (for example: "piper with the voice
  el_GR-rapunzelina-low.onnx in ~/piper", or "espeak-ng -v el")
- The radio-tutor program: {{RADIO_TUTOR}} (for example: "radio-tutor")

## The format

The complete format is defined in
https://github.com/rm4n0s/radio-tutor/blob/main/docs/CASSETTE-FORMAT.md
Read all of it before you start (if you cannot open the URL, ask me to put
the file in this folder). Follow it exactly: unknown fields, missing files,
SMIL or CSS animation, scripts and external references are all errors.

## How to work

1. Read the material. Write `plan.md`: the lessons in order (id, title, what
   it teaches, the scene idea in one sentence), and for each worksheet its
   exercises (id, label, what it asks, the answer). Keep each lesson to 1-5
   minutes of narration and one idea. Follow the order the material teaches.
   Show me nothing yet; continue.

2. For each lesson, write the narration script first, as a list of short
   sentences (one caption each, at most about 15 words). Speak to a child of
   {{AGE}}: simple words, one idea per sentence, examples from a child's life.
   Text-to-speech programs read digits and symbols badly ("1/4" may be read
   as "one slash four"), so write numbers and fractions in words in the
   sentences ("one over four", "one quarter"). A sentence's text is also its
   caption, so it must read well on the screen.

3. Make the narration audio sentence by sentence, so you know the timing:
   - synthesize each sentence to its own WAV file with the TTS program (for
     espeak-ng: `espeak-ng -v en-us -s 140 -w 01.wav "the sentence"`; a speed
     of 140 words per minute is good for children), and convert every file to
     the same format, e.g. `ffmpeg -i 01.wav -ac 1 -ar 22050 -c:a pcm_s16le
     s01.wav`, because the next step needs identical formats;
   - measure each file's length with
     `ffprobe -v error -show_entries format=duration -of csv=p=0 s01.wav`;
   - make the silences with `ffmpeg -f lavfi -i anullsrc=r=22050:cl=mono
     -t 0.4 -c:a pcm_s16le gap.wav`: 0.4 s between sentences (longer, 1-2 s,
     where the scene shows something the child should look at), about 0.5 s
     before the first sentence and 2 s after the last one, so that the last
     change on the scene happens while the narration is still playing;
   - join the pieces in order and encode in one command, with a list file
     whose lines are `file '/full/path/s01.wav'` (silences included):
     `ffmpeg -f concat -safe 0 -i list.txt -ac 1 -ar 44100 -c:a libvorbis
     -q:a 4 lessons/<id>/narration.ogg` (for speech, `-ar 22050 -q:a 2` makes
     a much smaller file that is still clear);
   - measure the final file's length again: it is the sum of the pieces.
   From the sentence lengths and pauses, compute each sentence's start and
   end time. These are the captions, and the times the scene changes. Do the
   sums in a small script, not by hand, and keep it: if a sentence changes,
   run it again.
   Keep the work files (WAVs, scripts) outside the cassette folder: any other
   file inside it is an error.

4. Draw the scene `scene.svg` (viewBox 0 0 1600 900): a calm background
   (static), and every part that must move, appear, or be "written" as its
   own top-level <g id="...">. Big shapes, strong colors, big text (at least
   40 units, font-family sans-serif). Draw with simple paths, circles,
   rectangles and text; no external images. Two tips: an object that is cut
   into pieces (a pizza, a cake) is drawn once inside `<defs>` and shown by
   several actors with `<use href="#art" clip-path="url(#clip-left)"/>`, each
   clipping a different part; and the width of a bold sans-serif text is
   about 0.55 x font-size x number of characters, which you need for the
   `reveal_box` of a written text.

5. Write `lesson.json`: the captions from step 3, and tracks that make the
   scene follow the narration: an actor appears (opacity) or is written
   (reveal) when the sentence that talks about it starts, things move while
   they are explained, and nothing moves while the child should read.
   Give `origin` (the actor's center) to every track that scales or rotates,
   and `reveal_box` (the actor's bounds) to every track that reveals.
   Every time must be within the narration's length. A key's time is the
   start or end of a sentence plus a small offset ("sentence 4 starts at
   12.0 s, so the cake is cut at 12.0 s"): generate `lesson.json` from the
   sentence times with your script instead of typing the times, and move an
   actor in pairs of keys (the value before, the value after), because a
   property moves from its previous key to the next one that sets it.
   To make something vanish and something else take its place at the same
   moment (a whole pizza that is cut into halves), use `"ease": "step"` on
   the key that changes `opacity`.

6. Write the exercises (`exercise.json`, optional `prompt.svg`). Rules:
   - solvable with pencil and paper, and readable from a phone photo;
   - one clear question each, practising the lesson before it;
   - short labels the child writes next to the answer ("1", "2", ... are
     fine; they must be unique in the cassette);
   - exercises of the same worksheet must look different on paper, so the
     grader can tell them apart even if a label is missing;
   - `reference_solution` exactly right; `grading` says what is correct,
     what is partial, which equivalent forms are accepted, and the common
     mistakes with what to tell the child for each; set `show_work` only
     when the working matters;
   - 1-3 hints that lead the child step by step without giving the answer;
   - for a child who cannot read well yet, add `prompt.ogg` (the prompt read
     aloud, made like the narration).

7. Write `cassette.json` (with `grader_instructions` in the tone above) and
   `cover.svg` (viewBox 0 0 300 400, the title in big letters and a simple
   picture).

8. Check and pack:
   `{{RADIO_TUTOR}} validate <cassette folder>` — it prints every problem
   (warnings too) as `path: message`; fix them all and run it again until it
   prints no problem and ends with "is valid (0 errors, 0 warnings)".
   Then `{{RADIO_TUTOR}} pack <cassette folder> <id>-<version>.cassette`
   (add `--force` to replace a file that exists, when you make a new version).

9. Before you finish, check your own work:
   - read every caption against the narration script;
   - for each lesson, list the times of the main tracks next to the captions
     and make sure each change happens while its sentence is spoken;
   - solve every exercise yourself and compare with `reference_solution`;
   - check that every exercise has been taught in a lesson before it.

10. Tell me: the path of the .cassette file, the list of lessons and
    exercises (titles and labels), the total narration time, and anything in
    the material you left out or were unsure about.

When you change an existing cassette later: raise the version, never rename
an id, never reuse an id for an exercise whose answer changed (make a new id),
and run validate and pack again.
```

---

## Recording the narration yourself

If you prefer your own voice: ask the agent to stop after step 2 and give you the sentences as a numbered list. Record each sentence as its own file (`01.wav`, `02.wav`, ... with any recorder), put them in a folder `recordings/<lesson-id>/`, and tell the agent to use them in step 3 instead of the text-to-speech program.
