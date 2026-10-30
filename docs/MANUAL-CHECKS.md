# Manual checks

Checks that need a person (a real display, real sound, a phone, a provider). Each item has the step id and the exact commands. When you have done one, tell the assistant the result; it is recorded in the step's entry in `docs/IMPLEMENTED.md` and the item is removed.

The checks are ordered by step.

## 3.3 A real provider answers the test image

Needs an AI provider you trust with a picture of "2 + 3 = 5" (a local Ollama with a vision model, or a cloud key). Nothing of yours is sent besides that picture. For a local Ollama:

```
RADIO_TUTOR_AI_BASE_URL=http://localhost:11434/v1 RADIO_TUTOR_AI_MODEL=qwen2.5vl:7b \
  go test -tags radiotutor_ai -run TestRealProvider -count=1 -v ./internal/core/providers
```

For a cloud provider add `RADIO_TUTOR_AI_KEY=<key>` and `RADIO_TUTOR_AI_KIND=cloud`. The test passes when a valid reply comes back; it prints the verdict (expected: `correct`), the model, the time and whether the app had to use a weaker structured output mode. It does not touch your Radio Tutor folders.

## 4.1 Phone server

A real phone on the same Wi-Fi as this computer. The firewall must allow port 8737 (the commands are in the README, "Let the phone reach the computer"; remove the rule again afterwards if you do not want it). The test prints the URL; it ends when the photos arrive or after five minutes, and keeps them in a temporary folder that `go test` removes at the end.

1. **The page on Android Chrome and iOS Safari.**
   `go test -tags radiotutor_phone ./internal/core/phone -run TestServeForRealPhone -v` (set `RADIO_TUTOR_LANG=el` for Greek, `RADIO_TUTOR_INTERFACE=<name>` to pick the Wi-Fi interface)
   Type the printed URL on the phone. Does the page show the title (with the odd characters as text), both exercises with their badges, and the big button? Does the button open the camera? Take 2 photos, remove one with the ✕, take another, press Send: does the progress bar move, and does the page then say "Sent! Look at the computer." while the test log says "received 2 photos"? Reload the page afterwards: it must say the link has expired.

## 5.1 Real sound through the audio server

Needs a running PulseAudio or PipeWire (with `pipewire-pulse`) server on your desktop session.

```
go test -tags radiotutor_audio -run TestRealServerTone -count=1 -v ./internal/core/audio
```

You should hear a 1 s tone (440 Hz, quiet), with a gap of about 0.3 s in the middle where the test pauses and resumes it. The test passes when no server error occurs, the position never goes backwards, the pause holds the position, and the clip ends by itself after about 1.3 s of wall time. If no sound server answers, the test fails with the reason the app would show as "No sound device found".

Also look at the volume control of your desktop while it runs: the stream is named "Radio Tutor".

## 6.1 App shell and login

Both checks use throw-away data folders (`scripts/isolated.sh`), never your real ones.

1. **First run, login and the Greek UI on a real display.**
   `scripts/isolated.sh -- go run ./cmd/radio-tutor`
   Create the parent account, log out, switch EN / ΕΛ on the login screen, log in again. Does every Greek text show real letters (no empty boxes), and does the window feel right (buttons easy to hit, nothing cut off at 800x600)? Close the window with the close button.
2. **Second instance.** In one terminal, with throw-away folders shared by both runs:
   `export XDG_CONFIG_HOME=$(mktemp -d) XDG_DATA_HOME=$(mktemp -d) XDG_STATE_HOME=$(mktemp -d) XDG_CACHE_HOME=$(mktemp -d)`
   `go run ./cmd/radio-tutor &`
   `go run ./cmd/radio-tutor; echo "exit $?"`
   The second run shows a small window "Radio Tutor is already running"; OK closes it, and the terminal prints "Radio Tutor is already running" and `exit 2`. Then close the first window and check that `jobs` shows nothing left.

## 6.2 Admin screens

Uses throw-away data folders (`scripts/isolated.sh`), never your real ones.

1. **Import with the real file dialog, and Test against a real provider.**
   `scripts/isolated.sh -- go run ./cmd/radio-tutor`
   Create the parent account. Cassettes: Import a cassette and choose `testdata/cassettes/sample.cassette` (the file dialog opens, the cassette appears with its version; Remove it again with and without the checkbox). Providers: Add from the Ollama preset (or any local server you run) or from a cloud preset with your own key, press Test: does it show "It works", the verdict "correct", the time and the model? Try a wrong key and see the plain-words error. Students: add a child, hide a cassette, log in as the child and check the shelf. Does everything look right at 800x600, in English and in Greek?

## 6.4 The lesson player with real sound

Uses throw-away data folders and the sample cassette (`testdata/cassettes/sample.cassette`, imported with **Import a cassette**). Needs a PulseAudio or PipeWire session.

```
scripts/isolated.sh -- go run ./cmd/radio-tutor
```

Create the parent account, add a student, import a cassette, log in as the student and open a lesson. Does the picture fit the window with bars at the sides or above and below (resize the window to check), and do the actors move in step with the narration (not ahead of it, not behind it)? Press Pause in the middle, wait, and press Play: does the sound and the picture continue together? Press Stop: does the picture go back to the start? Press Space and Escape. Let it play to the end: do you hear it finish, does the **Next** button appear, and is the lesson marked done on the cassette screen? Then stop the audio server (`systemctl --user stop pipewire-pulse.socket pipewire-pulse.service` or `pulseaudio -k`), start the app again and open a lesson: the notice "No sound device found. The lesson plays without sound." must show once, and the lesson must still play (without sound) to its end. Restart the audio server afterwards.

## 6.5 Worksheet, QR code and prompt audio

Needs a real display, a phone on the same Wi-Fi, a sound device and (for the last item) an AI provider you trust. The app runs on throw-away data folders (`scripts/isolated.sh`). Import `testdata/cassettes/sample.cassette` first (see 6.2).

1. **The QR code scans.** Open a worksheet as a student, press **Check my answers**, and scan the code with the phone's camera app. Does the phone open the page that lists the selected exercises (open the port as in 4.1 if not)? Compare the address under the code with the one in the phone's browser.
2. **Prompt audio.** On a worksheet whose exercise has `prompt.ogg`, press **Listen**: you should hear the prompt and the button goes back to **Listen** when it ends; **Stop** stops it at once.
3. **The whole flow with a real provider.** Send a photo of your own solution and read the results screen: marks, feedback, the star and the new hint.

## 7.1 Sample cassette: listen to it

The narration is made by `espeak-ng`; the check is that it is clear enough for a child and that the animation follows it. Never run this on a machine where sound would disturb anyone: it plays on the real speakers.

```
ffplay -nodisp -autoexit testdata/cassettes/sample/lessons/what-is-a-half/narration.ogg
ffplay -nodisp -autoexit testdata/cassettes/sample/lessons/quarters/narration.ogg
```

Are the sentences clear at this speed (140 words per minute, `"speed"` in `testdata/cassettes/sample-src/script.json`)? Then play both lessons in the app (import `testdata/cassettes/sample.cassette` into a throw-away data folder with `scripts/isolated.sh`) and check that each change on the scene happens while its sentence is spoken.

## 7.2 Full walkthrough on a real display with a real phone and a real provider

The automated end-to-end tests use a fake sink, a fake provider and a test upload. This check is the same story with the real things. Needs a real display, a sound device, a phone on the same Wi-Fi as this computer, and either a local Ollama with a vision model (nothing leaves the computer) or a cloud key you trust with a photo of a child's paper. Open the port as in 4.1 if the phone cannot reach the page. Everything runs on throw-away data folders.

1. Make sure the sample exists: `ls testdata/cassettes/sample.cassette` (rebuild with `scripts/make-sample-cassette.sh` if not). For Ollama: `ollama pull qwen2.5vl:7b` and make sure `ollama serve` runs.
2. `scripts/isolated.sh -- go run ./cmd/radio-tutor`
3. First run: create the parent account (username `mum`, a password of 8 or more characters).
4. **AI providers** -> Add -> the Ollama preset (or a cloud preset and your key) -> Save -> **Test**: it must say it works with the verdict "correct". Press **Set active**. Go back.
5. **Students** -> Add a student (`mia`, password `abcd`). Go back. **Cassettes** -> **Import a cassette** and choose `testdata/cassettes/sample.cassette` (the file dialog opens): "Halves and quarters" appears. Go back, **Log out**.
6. Log in as `mia`. Open "Halves and quarters". Lesson 2 ("Quarters") must be locked. Open the first lesson and press Play: you hear the narration and the picture follows it. Let it play to the end; **Next** appears.
7. Next opens the first worksheet (exercises 1 and 2, both ticked). On paper, draw a circle, cut it into two equal halves with one line and colour one half (exercise 1), and write `A` and `1/2` for exercise 2. Press **Check my answers**.
8. Scan the QR code with the phone (or type the address under it). The page lists both exercises. Take one photo of the page with both solutions (or two photos), press Send. The computer shows the grading indicator, then the results screen with a mark, feedback and a star for each exercise passed. If a mark is not what you expect, read the feedback: it is the provider's judgment.
9. Press **Back to the worksheet**, then the back arrow. When both exercises are passed, "Quarters" is unlocked (no lock on its row). If one failed, solve it again and send again; the hint of that exercise appears after a failed attempt.
10. **Log out**, log in as `mum`, **Progress** -> `mia` -> "Halves and quarters": the submission is listed with the provider and its kind; open it, check the photo thumbnail, press it to zoom and pan, and read the transcription and the notes for the parent. Close the window with the close button.
11. `pgrep -a radio-tutor` must show nothing left.

## 7.3 The README, followed on a clean account

Uses throw-away data folders for the app (`scripts/isolated.sh`) and a computer where you are allowed to open a firewall port. Read `README.md` from the top as a parent would and do what it says.

1. Build from a clone (`go build -o radio-tutor ./cmd/radio-tutor`) and run `./radio-tutor version`. Once a tag is pushed, also `go install github.com/rm4n0s/radio-tutor/cmd/radio-tutor@latest` and check that `radio-tutor version` prints the tag.
2. Validate and import `testdata/cassettes/sample.cassette` as the README says.
3. Open the port with the README's firewalld commands, check it with `sudo firewall-cmd --list-ports`, and that the phone reaches the page (4.1). Remove the rule again if you do not want it.
4. Reset a password: `scripts/isolated.sh --keep -- go run ./cmd/radio-tutor`, create a parent account, close the window; the kept folder is printed at the end (`isolated.sh: kept <root>`). Then
   `XDG_CONFIG_HOME=<root>/config XDG_DATA_HOME=<root>/data XDG_STATE_HOME=<root>/state go run ./cmd/radio-tutor reset-password <username>`
   and run the app again with the same three variables: the new password must log in. Delete `<root>` afterwards.
5. Is any sentence of the README wrong or unclear for someone who has never seen the app?
