# The cassette format, version 1

A **cassette** is a zip file with the extension `.cassette` that holds lessons and exercises for [Radio Tutor](https://github.com/rm4n0s/radio-tutor). This document is the complete definition of the format; it is part of Radio Tutor's specification. It is written so that an AI agent can produce a valid cassette from it alone.

Check a cassette with `radio-tutor validate <file.cassette|folder>` and build the zip with `radio-tutor pack <folder> <out.cassette>`.

## 1. Layout

```
cassette.json                     the manifest (§3)
cover.svg                         the cover shown on the shelf (§4)
lessons/<lesson-id>/
    lesson.json                   title, timeline and captions (§5.3, §5.4)
    scene.svg                     the scene (§5.1)
    narration.ogg                 the narration, Ogg Vorbis (§6)
exercises/<exercise-id>/
    exercise.json                 the exercise (§7)
    prompt.svg                    optional: a picture for the exercise
    prompt.ogg                    optional: the exercise read aloud
```

Nothing else may be in the zip: the validator reports any other file (including `.DS_Store`, `__MACOSX/`, `Thumbs.db`). Every lesson and exercise listed in the manifest has its folder, and every folder is listed.

## 2. Common rules

- All text files are UTF-8 without a byte order mark. JSON files are objects; unknown fields are errors (catches typos).
- **Ids** of lessons and exercises: `^[a-z0-9][a-z0-9-]{0,62}$`, unique across the whole cassette (a lesson and an exercise cannot share an id).
- **The cassette id** is reverse-domain style: `^[a-z0-9]+(\.[a-z0-9][a-z0-9-]*)+$`, e.g. `org.family-smith.math.fractions-1`. Two different cassettes never share an id.
- **The version** is `MAJOR.MINOR.PATCH` (numbers only). A cassette with a new version of the same id replaces the old one, and the child's progress is kept **by id**. Therefore:
  - **Never rename an id** in a new version.
  - **Never reuse an id for different content.** If an exercise changes so that an old answer would no longer be right, give it a new id.
  - Removing an item is allowed; its progress is ignored.
- Times are seconds as JSON numbers (`3.25`), from the start of the narration.
- Text shown to the child is plain text (no Markdown, no HTML). A newline (`\n`) is a line break.

## 3. The manifest: `cassette.json`

```json
{
  "format": 1,
  "id": "org.family-smith.math.fractions-1",
  "version": "1.0.0",
  "title": "Fractions, part 1",
  "description": "What a fraction is, and adding fractions with the same denominator.",
  "language": "el",
  "subject": "math",
  "ages": [8, 10],
  "authors": ["Claude, for the Smith family"],
  "source": "Μαθηματικά Γ' Δημοτικού, κεφάλαιο 12",
  "grader_instructions": "Speak to the child warmly and simply, in Greek. Praise effort. Accept any correct equivalent form unless an exercise says otherwise.",
  "items": [
    {"type": "lesson",   "id": "what-is-a-fraction"},
    {"type": "exercise", "id": "shade-the-half"},
    {"type": "exercise", "id": "name-the-fraction"},
    {"type": "lesson",   "id": "adding-fractions"},
    {"type": "exercise", "id": "write-the-sum"}
  ]
}
```

| Field | Required | Rules |
|---|---|---|
| `format` | yes | `1` |
| `id` | yes | §2 |
| `version` | yes | §2 |
| `title` | yes | 1 to 80 characters |
| `description` | no | at most 500 characters |
| `language` | yes | a BCP 47 language tag of the content, e.g. `en`, `el`, `en-GB` |
| `subject` | yes | 1 to 40 characters, free text (e.g. `math`, `history`) |
| `ages` | yes | `[min, max]`, integers from 3 to 18, min ≤ max |
| `authors` | no | list of strings |
| `source` | no | the book or material the cassette is based on, at most 300 characters |
| `grader_instructions` | no | at most 4000 characters: how the AI grader should speak and what it should accept, for every exercise (§7.3) |
| `items` | yes | 1 to 500 entries, each `{"type": "lesson" or "exercise", "id": ...}`; the order is the order the child follows |

**Order and unlocking.** The child must finish items in order: a lesson unlocks when every lesson before it was played to the end and every exercise before it was solved correctly. Consecutive exercises form a **worksheet**: they are shown together, solved on the same paper, and photographed together. Put a worksheet of 1 to 5 exercises after each lesson that teaches what they practise. The first item should be a lesson.

## 4. The cover: `cover.svg`

An SVG picture for the shelf, with a `viewBox` of aspect ratio 3:4 (e.g. `0 0 300 400`). Same SVG rules as §5.2.

## 5. Lessons

A lesson is one **scene** (a picture whose parts move), one **narration** track, and **captions**. The narration is the clock: when the child presses Play, the narration plays and the scene moves in step with it; Pause and Stop pause it or go back to the start. A lesson is done when the narration has played to its end. Aim for 1 to 5 minutes per lesson.

### 5.1 The scene: `scene.svg`

An ordinary SVG document. The parts that move are **actors**: each actor is a **top-level `<g>` with an `id`** (a direct child of the root `<svg>` element).

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1600 900">
  <defs>
    <linearGradient id="sky">...</linearGradient>
  </defs>
  <rect width="1600" height="900" fill="url(#sky)"/>   <!-- static: drawn as it is -->
  <g id="pizza">  ... </g>                              <!-- actor "pizza" -->
  <g id="slice">  ... </g>                              <!-- actor "slice" -->
  <g id="label-half">
    <text x="800" y="800" font-size="72" text-anchor="middle">1/2</text>
  </g>
</svg>
```

- The root has a `viewBox` with an aspect ratio between 4:3 and 21:9; use `0 0 1600 900` unless there is a reason not to. Coordinates in the timeline are in view box units.
- Top-level children are drawn in document order. Each top-level `<g id>` is an actor; every other top-level element (and a `<g>` without an id) is static. Elements inside an actor do not move separately: to move something on its own, make it its own top-level group.
- The `<defs>` and `<style>` elements (top-level) are shared by all actors: gradients, patterns, clip paths, markers and symbols defined there can be used by any actor through `url(#id)` and `<use href="#id">`.
- Actor ids follow the id rule of §2 and are unique in the scene. At most 64 actors and 5 MiB per scene.
- Text: use `font-family="sans-serif"`, `serif` or `monospace` (the app's own fonts; they cover Latin, Greek and Cyrillic). Make text large (at least 40 units in a 1600×900 scene) because children read it from a distance.

### 5.2 What SVG may contain

Shapes, paths, strokes and dashes, fills, linear and radial gradients, patterns, clip paths, masks, opacity, transforms, `<use>`, markers, text (`<text>`, `<tspan>`), `<style>` with CSS selectors, `<image>` only with a `data:image/png` or `data:image/jpeg` URL, and the common filters (`feGaussianBlur`, `feOffset`, `feFlood`, `feComposite`, `feMerge`, `feColorMatrix`, `feBlend`, `feDropShadow`).

Not allowed (the validator rejects them): `<script>`, event attributes (`onclick`, ...), SMIL animation (`<animate>`, `<animateTransform>`, `<animateMotion>`, `<set>`), CSS animations and transitions, `<foreignObject>`, references to external files or URLs (`href` must be `#id` or a `data:` image), `textLength`, vertical text, and the filter primitives `feImage`, `feTile`, `feTurbulence`, `feConvolveMatrix`, `feDisplacementMap` and the lighting filters.

### 5.3 The lesson file: `lesson.json`

```json
{
  "format": 1,
  "title": "What is a fraction?",
  "summary": "We cut a pizza into equal pieces.",
  "tracks": [
    {
      "actor": "pizza",
      "keys": [
        {"t": 0,   "opacity": 0},
        {"t": 1.5, "opacity": 1, "ease": "out"}
      ]
    },
    {
      "actor": "slice",
      "origin": [800, 450],
      "keys": [
        {"t": 6.0, "x": 0,   "rotate": 0},
        {"t": 8.0, "x": 240, "rotate": 15, "ease": "in-out"}
      ]
    },
    {
      "actor": "label-half",
      "reveal_box": [650, 720, 300, 110],
      "reveal_from": "left",
      "keys": [
        {"t": 9.0,  "reveal": 0},
        {"t": 10.5, "reveal": 1}
      ]
    }
  ],
  "captions": [
    {"start": 0.0, "end": 5.6,  "text": "Here is a pizza. Let's share it fairly."},
    {"start": 5.8, "end": 8.6,  "text": "We cut it into two equal pieces."},
    {"start": 8.8, "end": 12.0, "text": "Each piece is one half: 1/2."}
  ]
}
```

| Field | Required | Rules |
|---|---|---|
| `format` | yes | `1` |
| `title` | yes | 1 to 80 characters |
| `summary` | no | at most 300 characters |
| `tracks` | yes | at most one track per actor; may be empty |
| `captions` | yes | §5.5; at least one |

### 5.4 Tracks and keys

A **track** animates one actor:

| Field | Required | Rules |
|---|---|---|
| `actor` | yes | the id of a top-level `<g>` of `scene.svg` |
| `origin` | when a key sets `scale` or `rotate` | `[x, y]` in view box units: the point the actor rotates and scales around (usually its center) |
| `reveal_box` | when a key sets `reveal` | `[x, y, width, height]` in view box units, inside the view box: the area the reveal wipes over (the actor's bounds) |
| `reveal_from` | no | `left` (default), `right`, `top` or `bottom`: the side the reveal starts from |
| `keys` | yes | 1 to 1000 keys with strictly increasing `t` |

A **key** has `t` (required, ≥ 0 and at most the narration's length) and any of these properties:

| Property | Meaning | Default |
|---|---|---|
| `opacity` | 0 (invisible) to 1 (opaque) | 1 |
| `x`, `y` | move by this many view box units (right, down) from where the scene draws the actor | 0 |
| `scale` | uniform scale around `origin`, greater than 0, at most 20 | 1 |
| `rotate` | degrees, clockwise, around `origin` | 0 |
| `reveal` | 0 (hidden) to 1 (fully shown): the fraction of `reveal_box` shown, from `reveal_from`; use it to "write" text or "draw" a line | 1 |
| `ease` | how the properties this key sets arrive from the previous key that set them: `linear` (default), `in`, `out`, `in-out` (cubic), `step` (jump at this key) | `linear` |

Each property is animated on its own: between two keys that set it, the value moves from the first to the second with the second key's `ease`. Before the first key that sets a property, it has that key's value; after the last, it keeps the last value. A property no key sets keeps its default. Transforms apply in this order: scale and rotate around `origin`, then move by `x`, `y`.

Tip: to make an actor appear at 5 seconds, give it `{"t": 0, "opacity": 0}`, `{"t": 5, "opacity": 0}`, `{"t": 5.5, "opacity": 1}`; to make it pop in, use `"ease": "step"` on the second key.

### 5.5 Captions

Each caption is `{"start", "end", "text"}`: shown from `start` to `end` (`start < end`). Captions do not overlap, are in order, and lie within the narration. Text: 1 to 200 characters, at most two short lines, matching what the narration says at that time. Captions are what a child who cannot hear (or the sound is off) reads, so every spoken sentence has its caption.

## 6. Narration: `narration.ogg`

- Ogg Vorbis, mono or stereo, 22050, 44100 or 48000 Hz, at most 10 minutes and 20 MiB. `ffmpeg -i in.wav -c:a libvorbis -q:a 4 narration.ogg` makes one.
- The narration's length is the lesson's length. Every key `t` and every caption `end` is at most the narration's length (the validator allows 0.05 s of rounding).
- Speak slowly and clearly, for the ages of the manifest; leave short pauses where something on the scene moves.

## 7. Exercises

An exercise is solved **on paper**. The child writes the exercise's **label** next to the solution, takes photos, and an AI grader reads them. Several exercises are usually solved on the same page, so the grader must tell them apart: labels and clearly different exercises help it.

### 7.1 Writing good exercises

- The answer must be something a child can write or draw on paper and a camera can read: numbers, words, short sentences, a simple drawing or a marked figure.
- Each exercise asks one clear thing, at the level of the lesson before it.
- Give a `reference_solution` that is exactly right, and `grading` instructions that say what counts as correct, what counts as partial, which equivalent answers are accepted, and the common mistakes with what to tell the child about each.
- Exercises of the same worksheet should look different on paper (different kinds of answer, or clearly different numbers), so they cannot be confused if a label is missing.
- If the child must copy something (a figure to shade), the exercise prompt says so ("Draw a circle, cut it into 4 equal parts, shade 1 part").

### 7.2 The exercise file: `exercise.json`

```json
{
  "format": 1,
  "title": "Write the sum",
  "label": "3",
  "prompt": "Add the fractions and write the result:\n1/4 + 2/4 = ?",
  "reference_solution": "3/4",
  "grading": "Correct: 3/4, or 0.75, or the words 'three quarters'. Partial: the right numerator with a wrong denominator (3/8) - tell the child the bottom number stays the same when we add quarters. Incorrect: anything else.",
  "show_work": false,
  "max_attempts": 3,
  "hints": [
    "Look at the bottom numbers. Are they the same?",
    "When the bottom numbers are the same, add only the top numbers."
  ]
}
```

| Field | Required | Rules |
|---|---|---|
| `format` | yes | `1` |
| `title` | yes | 1 to 80 characters |
| `label` | no | 1 to 6 characters of letters, digits and `.` or `-`; unique in the cassette. Default: the exercise's position among all the exercises of the cassette, counting from 1 (`"1"`, `"2"`, ...). The child writes it next to the solution, so keep it short |
| `prompt` | yes | 1 to 2000 characters: what the child sees and does |
| `reference_solution` | yes | 1 to 4000 characters: the right answer, with the working if `show_work` is true |
| `grading` | yes | 1 to 4000 characters: instructions to the grader for this exercise (§7.3) |
| `show_work` | no | `true` when the working is graded too, not only the final answer. Default `false` |
| `max_attempts` | no | 0 to 20; 0 means no limit. Default 3. After this many wrong tries a parent looks at the exercise |
| `hints` | no | 0 to 5 hints of at most 300 characters; after each wrong try the next hint is shown |

`prompt.svg` (optional) is a picture shown with the prompt (same rules as §5.2; any `viewBox`; no actors, nothing moves). `prompt.ogg` (optional) is the prompt read aloud (same audio rules as §6, at most 1 minute), for children who cannot read yet.

### 7.3 How the grader uses the cassette

Radio Tutor sends the grader the photos and, for every exercise of the submission, its label, id, title, prompt, reference solution, grading instructions and `show_work`, plus the manifest's `language`, `ages` and `grader_instructions`. The grader answers, for each exercise, one of `correct`, `partial`, `incorrect`, `unreadable` (the solution is there but cannot be read) or `not_found` (no solution for it on the photos), a transcription, short feedback for the child, and notes for the parent. Only `correct` solves an exercise.

So `grader_instructions` and `grading` should say *how to judge and how to talk*, not repeat these rules. They are data for the grader: they cannot change the answer format or ask for anything else.

## 8. Versions of the format

`format` is 1 in every JSON file. A future format 2 will be read by newer versions of Radio Tutor; version 1 apps refuse it with "This cassette needs a newer Radio Tutor."

## 9. Limits

- Zip: at most 512 MiB, at most 2000 entries, at most 1 GiB uncompressed; each entry at most 200 MiB uncompressed and at most 100 times its compressed size.
- Entry names: relative, `/` separated, only `a-z 0-9 . - _ /`, no `..`, no empty parts, no leading `/`, unique; no symlinks or other special files; folders may be listed as entries or not.
- `cassette.json`, `lesson.json` and `exercise.json`: at most 1 MiB each. SVG files: at most 5 MiB each.

## 10. Validation

`radio-tutor validate` checks, and reports every problem it finds (not only the first) as `path: message`:

1. The zip limits and entry names (§9), and that only the files of §1 are present.
2. Each JSON file: valid UTF-8 JSON, no unknown fields, every field's rules (§3, §5.3, §5.4, §5.5, §7.2).
3. The manifest's items: ids valid and unique, each with its folder, each folder listed, the first item a lesson (a warning, not an error), at least one item.
4. Labels unique (after defaults are applied).
5. Each SVG: parses, has a `viewBox` (scenes: aspect ratio rule), no forbidden elements or attributes (§5.2), no external references, at most 64 actors, ids of actors valid and unique; Radio Tutor's renderer draws every layer without an error.
6. Each track's actor exists in the scene; `origin` and `reveal_box` present when needed; `reveal_box` inside the view box; keys strictly increasing; property values in range.
7. Each Ogg file decodes, its rate and channels are allowed, its length is within the limits; every key `t` and caption `end` ≤ the narration's length + 0.05 s; captions ordered and not overlapping.

A cassette is valid when there are no errors; warnings are printed but do not fail it.
