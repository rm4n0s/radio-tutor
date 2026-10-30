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
   Type the printed URL on the phone. Does the page show the title (with the odd characters as text), both exercises with their badges, and the big button? Does the button open the camera? Take 2 photos, remove one with the ✕, take another, press Send: does the progress bar move, and does the page then say "Sent! Look at the computer." while the test log says "received 2 photos"? The test then offers a second submission (exercise 3) on the same page: after about ten seconds of "Sent!" the page must switch to it by itself, with no reload and no new scan, and a second Send must work. Reload the page: it shows the waiting or the exercises, not an error. (The test ends after the second submission arrives; the link then ends and a reload says it has expired.)

## 5.1 Real sound through the audio server

Needs a running PulseAudio or PipeWire (with `pipewire-pulse`) server on your desktop session.

```
go test -tags radiotutor_audio -run TestRealServerTone -count=1 -v ./internal/core/audio
```

You should hear a 1 s tone (440 Hz, quiet), with two gaps where the test pauses and resumes it: about 0.3 s at 0.3 s into the tone and about 0.2 s at 0.7 s. Each pause corks the stream and each resume uncorks the same stream. The test passes when no server error occurs, the position never goes backwards, each pause holds the position, each resume succeeds, and the clip ends by itself after about 1.5 s of wall time. If no sound server answers, the test fails with the reason the app would show as "No sound device found".

Also look at the volume control of your desktop while it runs: the stream is named "Radio Tutor".

## 5.2 Pause and resume on the copy of the pulse library

The copy in `internal/core/pulse` fixes the deadlock of upstream's connection loop that made Radio Tutor close and reopen the stream on every pause. This checks that corking and uncorking one stream works against your real server (PipeWire's `pipewire-pulse` is the one that showed the bug). Run the test of 5.1 three times in a row, and listen for a click, a repeated or skipped piece of the tone around the gaps, or a delay of seconds at the end:

```
go test -tags radiotutor_audio -run TestRealServerTone -count=3 -v ./internal/core/audio
```

Then in the app (`scripts/isolated.sh -- go run ./cmd/radio-tutor`, the sample cassette): open a lesson, press Pause and Play ten times quickly and then let it play to the end, and press Stop in the middle of another play. Pause and Play must answer at once (no wait of 2 s or 4 s), the narration must continue exactly where it stopped, and Stop and the end of the lesson must be prompt.

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

## 6.5 (phone link) The phone page stays open between worksheets

Needs a real display, a phone on the same Wi-Fi (the port open as in 4.1), a sound device is not needed. An AI provider is needed for the grading; you may use the fake path by cancelling instead of sending in steps 6 and 9 if you only want to check the page. The app runs on throw-away data folders (`scripts/isolated.sh`). Import `testdata/cassettes/sample.cassette` first (see 6.2) and add a student `mia`.

1. `scripts/isolated.sh -- go run ./cmd/radio-tutor`, log in as `mia`, open a worksheet and press **Check my answers**. The QR code is large; the screen does not say "Your phone is ready".
2. Scan the code once with the phone's camera app. The phone shows the exercises of the worksheet; the computer says "Your phone opened the page."
3. Take a photo, press Send. The phone says "Sent! Look at the computer. Leave this page open for the next one." (in Greek: «Στάλθηκε! Κοίτα τον υπολογιστή. Άφησε αυτή τη σελίδα ανοιχτή για την επόμενη.»). The computer grades and shows the results.
4. **Keep the page open on the phone** (do not close the tab). On the computer press **Back to the worksheet**, go to another worksheet (or the same one if exercises are still open) and press **Check my answers**. The computer says "Your phone is ready: the page updated by itself", the address under the smaller code is the same as before, and **the phone page switches to the new exercises by itself**, without a reload and without scanning. Send a photo: it works as before.
5. Press **Check my answers** once more and then **Cancel** on the computer: the phone page goes back to "Waiting for the next exercise…" by itself within a few seconds.
6. Press **Check my answers** again, then lock the phone for about a minute and unlock it. Within a few seconds the page still shows the exercises (or the waiting message if you cancelled meanwhile) and works: send a photo. Repeat with Wi-Fi switched off on the phone for 20 seconds: the page shows "The connection was lost. Trying again…" and recovers when the Wi-Fi is back, with no reload.
7. Cancel, then **Log out** on the computer. On the phone, within about 30 seconds (or at once when you unlock it or switch back to the tab) the page says "This link has expired". Reload it: the same. Log in as `mia` again and **Check my answers**: the QR code is large again (a new link); the old phone page stays expired.
8. `pgrep -a radio-tutor` must show nothing left.

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

## Volume setting (after 7.3): the slider is audible

Needs a running PulseAudio or PipeWire server and real speakers (on Windows, the sound check above first). Throw-away data folders:

```
scripts/isolated.sh -- go run ./cmd/radio-tutor
```

Make a parent account, import `testdata/cassettes/sample.cassette`, and open Settings. Set the volume to 100, then to 50 and to 20 (drag the slider and let go; "Saved." appears), and play a lesson and the speaker button of an exercise after each change. Is each step quieter, evenly, with 20 still audible? At 0 the lesson plays in silence, its time runs and it can be completed. Does the slider keep its value after restarting the app?

## Volume and mute in the lesson player (after the volume setting): listen to it

Needs a running PulseAudio or PipeWire server and real speakers (on Windows, the sound check above first). Throw-away data folders:

```
scripts/isolated.sh -- go run ./cmd/radio-tutor
```

Make a parent account and a student, import `testdata/cassettes/sample.cassette`, log in as the student and open a lesson. Right after **Stop** there are a speaker button and a slider.

1. Press Play, and while the narration plays drag the slider down to about 30 and let go, then back up to 100. Does the narration get quieter and louder while you drag (within a fraction of a second), without a click, a gap or a restart of the narration? Does the number show above the thumb while you drag?
2. Press the speaker button while it plays: the sound stops at once (the lesson time keeps running and the captions go on) and the icon is crossed out. Press it again: the narration is heard again at the volume of the slider, in the middle of the sentence.
3. Mute, then move the slider: the sound comes back and the icon is not crossed out any more.
4. Set the volume to about 40 and leave the lesson with Back; open another lesson, and the exercise page with its speaker button: the sound is at 40 everywhere, and the slider starts at 40. Mute, close the app, start it again, open a lesson: it starts muted and the slider shows the volume you left.
5. Log out, log in as the parent, open Settings: the volume row shows the same value and the **Mute the sound** switch shows the muted state. Change both there, and open a lesson again: the controls show the new state. Also check with `pavucontrol` (or your desktop's mixer) that the "Radio Tutor" stream's own volume is not changed by the controls: the app scales the samples itself.
6. At 800x600, in Greek, with a lesson that has Skip and Next: is nothing cut off (the speaker and slider may wrap onto a second line)?

## Sound on Windows (the `waveout` sink): listen to it on a real Windows 10 or 11 PC

The `waveout` sink was tested against the real winmm only under Wine, in a sandbox with a device that discards the sound (nothing was heard), so what a person hears is unchecked. Needs a Windows 10 (1809) or 11 PC with speakers or headphones, a Windows build of Radio Tutor (`GOOS=windows`), and the Go toolchain for the first command. In PowerShell, in the repository:

```
go test -tags radiotutor_audio -run TestRealServerTone -count=3 -v ./internal/core/audio
```

1. You should hear a 1 s tone (440 Hz, quiet), with two gaps where the test pauses and resumes it (about 0.3 s at 0.3 s and 0.2 s at 0.7 s). Listen for a click at the pauses, a piece of the tone repeated or lost after a resume, or a delay at the end. The test passes when the position never goes backwards, each pause holds it, each resume succeeds and the clip ends by itself.
2. Open the app, a throw-away account and the sample cassette (`testdata/cassettes/sample.cassette`) and play a lesson: Pause and Play ten times quickly (the narration continues exactly where it stopped, with no click and no delay), press Stop in the middle, and let another play run to the end (the **Next** button appears at the end of the sound, not before or after it).
3. Volume and mute: while the narration plays drag the volume slider down and up (it changes within a fraction of a second), press the speaker button (the sound stops, the time goes on, the icon is crossed out) and press it again. Open the Windows volume mixer: Radio Tutor's own volume stays where it was, the app scales the samples itself.
4. Change the output device in Windows (for example plug in headphones and make them the default) and open another lesson: it plays through the new default device. Switching in the middle of a lesson may stop the sound; note what happens.
5. No device: disable every playback device (Settings, System, Sound, or Device Manager) and open a lesson: the notice "No sound device found. The lesson plays without sound." shows once, and the lesson plays (silently) to its end and is completed. Enable the devices again afterwards.

## W.1 Windows: what Wine cannot show

Needs a real Windows 10 (1809 or later) or 11 PC and a Windows build of the app (`scripts/build-windows.sh -release` gives `radio-tutor.exe` and `radio-tutor-cli.exe`; below, `radio-tutor-console.exe` means `radio-tutor-cli.exe`). Do not use an account whose `%LOCALAPPDATA%\radio-tutor` holds data you want to keep; a throwaway user is best. The unit tests of all of this pass under Wine (`scripts/test-windows.sh`); these are the parts a headless Wine cannot prove.

1. **Data folders.** Start the app, create the admin, close it. In Explorer (`%LOCALAPPDATA%\radio-tutor`): are there `config` (`settings.json`), `data` (`profiles.json`) and `state` (`radio-tutor.log`, `lock`)? Is there nothing under `%APPDATA%\radio-tutor`? Add an AI provider with a key, then in PowerShell: `icacls "$env:LOCALAPPDATA\radio-tutor\config\providers.json"`: only your own user may be listed (with full access), no `Users`, no `Everyone`, and no inherited entries.
2. **The lock with two instances.** Start the app, then start `radio-tutor-console.exe` again, or the GUI a second time: the second one must show the small window "Radio Tutor is already running" (GUI) or print it and exit with code 2 (console: `echo %ERRORLEVEL%`). Close the first one: the second start now works. Kill the first with Task Manager instead: the next start must work too (the OS drops the lock).
3. **Password reset in a real console.** In `cmd.exe` and in Windows Terminal, with the app closed: `radio-tutor-console.exe reset-password <username>`. The typed password must not show; Enter on each prompt moves to the next line; the console echoes again afterwards (type something at the prompt after the command ends); Ctrl+C at the prompt also leaves the echo on. Then log in with the new password. Also try it with the input piped: `echo newpassword| ...` is not expected to work in `cmd` (a trailing space is part of the password); use `(echo secret1& echo secret1)| radio-tutor-console.exe reset-password <username>`.
4. **The phone server and the firewall.** Log in as a student, open a worksheet and ask for a submission. Windows Defender Firewall asks whether to allow `radio-tutor.exe`: allow it on private networks. With a phone on the same Wi-Fi, scan the QR code: the page opens and the photos arrive. Also check what the QR shows when the PC is on Wi-Fi and has Hyper-V, WSL or a VPN installed: the address must be the Wi-Fi's, not `vEthernet` or a VPN's. Refuse the firewall prompt on a second try (remove the rule in Windows Defender Firewall first): the QR dialog must show the firewall hint after 60 seconds.
5. **Folders and files on a real NTFS.** Import a cassette, take a photo submission, rename nothing: then check that a cassette with a lesson id such as `con` is refused when imported, and that a student named `nul` cannot be created.
6. **Language.** On a PC whose Windows display language is Greek, a first run offers Greek (the locale is read from the system, since `LANG` is not set on Windows).
7. **Packaging.** SmartScreen warns about the unsigned `.exe` (More info, Run anyway); the README says so.

## W.2 Windows: the packaged app on a real PC

The Wine smoke test (`go test -tags radiotutor_wine ./cmd/radio-tutor`) proves that the two `.exe` files build, that the app opens, draws its first-run screen (in English and in Greek) and ends cleanly, and that the console exe validates and packs cassettes. Under Wine the app draws with Glutter's GDI fallback and there is no sound, no real taskbar and no firewall, so a person has to check, on a real Windows 10 (1809 or later) or 11 PC with a throwaway user, with the two `.exe` files made by `scripts/build-windows.sh -release` (copy `dist/windows` to the PC):

1. **Start.** Double-click `radio-tutor.exe`. SmartScreen warns (More info, Run anyway). The window opens with the first-run screen, in the language of Windows (English or Greek), without a console window.
2. **Icon.** The cassette icon shows in the title bar, the taskbar, Alt+Tab and the file in Explorer (right-click, Properties, Details: product name "Radio Tutor"). It looks sharp at 100% and 150% scale.
3. **Rendering.** The window draws smoothly (Direct3D 12) when you resize it and move it between monitors of different scales; if it does not, run with `GLUTTER_RENDERER=soft` and tell which one fails.
4. **A lesson with sound.** Create the parent account and a student, import `testdata/cassettes/sample.cassette`, play a lesson as the student: narration, captions and animation stay in step (see also the waveOut section above).
5. **The phone flow with the firewall prompt.** Open a worksheet, press Check my answers: Windows Defender Firewall asks; allow on private networks. The QR code opens the page on a phone on the same Wi-Fi, a photo arrives (see W.1, item 4 for the refused prompt).
6. **Grading.** With an AI provider set up, a photo of a solved exercise is graded and the next lesson unlocks.
7. **The commands.** In PowerShell: `.\radio-tutor-cli.exe validate sample.cassette` prints the result; `.\radio-tutor.exe version` prints the message that points to `radio-tutor-cli.exe` (in the same terminal, not a message box), and a double click on a shortcut with an argument shows the message box.

## 14.1 `radio-tutor setup-desktop` on a real desktop (Linux)

Needs a real GNOME or KDE session. It writes under your `~/.local/share` (five small files), so do it only when you want the entry; `--remove` undoes it.

1. Build or install the program: `go install ./cmd/radio-tutor` (it lands in `$(go env GOPATH)/bin`, usually `~/go/bin`).
2. `~/go/bin/radio-tutor setup-desktop` prints the five files it wrote (four icons and the `.desktop` file) and exits 0. Running it with `go run ./cmd/radio-tutor setup-desktop` instead must be refused with a message about `go install`, and write nothing.
3. Open the application menu or the Activities search and type "Radio": the entry "Radio Tutor" appears with the cassette icon (it can take a moment, or a log-out, on some desktops). Launch it from the menu: the window opens.
4. While it runs, look at the dock or task bar: the window is grouped under the entry's icon (this is `StartupWMClass`), not shown as a second, generic icon. Say which desktop and whether it is Wayland or X11.
5. With Greek as the session language the entry still reads "Radio Tutor" and its tooltip or description is the Greek comment.
6. `~/go/bin/radio-tutor setup-desktop --remove` prints the five files it deleted; the entry leaves the menu. Running it again says there is nothing to remove.

## W.3 `radio-tutor-cli.exe setup-desktop` on a real Windows PC

Needs a real Windows 10 (1809 or later) or 11 PC and `radio-tutor.exe` and `radio-tutor-cli.exe` in one folder (`scripts/build-windows.sh -release`). Do not use an account with data you care about.

1. In PowerShell, in that folder: `.\radio-tutor-cli.exe setup-desktop` prints the path of `Radio Tutor.lnk` (in `%APPDATA%\Microsoft\Windows\Start Menu\Programs`) and exits 0. Run it again: it replaces the shortcut, no second one appears.
2. Press the Windows key and type "Radio Tutor": the entry shows with the cassette icon. Start it from there: the application opens without a console window, and its data folders are the usual ones.
3. While it runs, the taskbar shows one icon for the window; pin it, close the app, start it from the pinned icon and from the Start menu entry: both are the same icon (this is the AppUserModelID `dev.radiotutor.app`, which Wine cannot check). Optionally, in PowerShell: `(New-Object -ComObject Shell.Application).NameSpace("$env:APPDATA\Microsoft\Windows\Start Menu\Programs").ParseName("Radio Tutor.lnk").ExtendedProperty("System.AppUserModel.ID")` prints `dev.radiotutor.app`.
4. Move the two files to another folder and run `setup-desktop` there: the shortcut now starts the program in the new folder. With `radio-tutor.exe` missing from the folder, the command fails with `GUIProgramMissing` and changes nothing.
5. `.\radio-tutor-cli.exe setup-desktop --remove` deletes the shortcut; the Start-menu entry disappears.
