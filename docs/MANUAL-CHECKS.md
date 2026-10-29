# Manual checks

Checks that need a person (a real display, real sound, a phone, a provider). Each item has the step id and the exact commands. When you have done one, tell the assistant the result; it is recorded in the step's entry in `docs/IMPLEMENTED.md` and the item is removed.

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

## 5.1 Real sound through the audio server

Needs a running PulseAudio or PipeWire (with `pipewire-pulse`) server on your desktop session.

```
go test -tags radiotutor_audio -run TestRealServerTone -count=1 -v ./internal/core/audio
```

You should hear a 1 s tone (440 Hz, quiet), with a gap of about 0.3 s in the middle where the test pauses and resumes it. The test passes when no server error occurs, the position never goes backwards, the pause holds the position, and the clip ends by itself after about 1.3 s of wall time. If no sound server answers, the test fails with the reason the app would show as "No sound device found".

Also look at the volume control of your desktop while it runs: the stream is named "Radio Tutor".
## 4.1 Phone server

A real phone on the same Wi-Fi as this computer. The firewall must allow port 8737 (`sudo firewall-cmd --add-port=8737/tcp --permanent && sudo firewall-cmd --reload`, then remove it again if you do not want it). The test prints the URL; it ends when the photos arrive or after five minutes, and keeps them in a temporary folder that `go test` removes at the end.

1. **The page on Android Chrome and iOS Safari.**
   `go test -tags radiotutor_phone ./internal/core/phone -run TestServeForRealPhone -v` (set `RADIO_TUTOR_LANG=el` for Greek, `RADIO_TUTOR_INTERFACE=<name>` to pick the Wi-Fi interface)
   Type the printed URL on the phone. Does the page show the title (with the odd characters as text), both exercises with their badges, and the big button? Does the button open the camera? Take 2 photos, remove one with the ✕, take another, press Send: does the progress bar move, and does the page then say "Sent! Look at the computer." while the test log says "received 2 photos"? Reload the page afterwards: it must say the link has expired.
