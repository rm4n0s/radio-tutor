# Radio Tutor

Radio Tutor is a desktop tutoring app for children on Linux. It plays **cassettes**: files with short animated, narrated lessons and paper-and-pencil exercises for one subject (math, a language, history, anything a book teaches). An AI agent can write the cassettes for you from a book or a subject. A vision-capable AI model, in the cloud or on your own computer, checks the child's work from photos. Parents keep control: you create the profiles, choose the AI provider, see every photo and can overrule the AI.

A session of a child:

1. The child logs in with a name and a password and picks a cassette from the shelf.
2. The child plays a lesson: an animated picture, a narrator and captions.
3. The child solves the worksheet **on paper** and presses **Check my answers**.
4. The app shows a QR code. The child scans it with a phone, takes photos of the paper and sends them.
5. The AI reads the photos and tells the child, for each exercise, whether it is right and why. When every exercise of a lesson is passed, the next lesson unlocks.

Everything stays on your computer, except the photos and the exercise text, which go to the AI provider you choose (see [Choosing an AI provider](#choosing-an-ai-provider)).

## Requirements

- Linux with a desktop session (Wayland or X11) and a graphics driver (Vulkan or OpenGL ES, which any current desktop has).
- Go 1.27 to build the program. No C compiler is needed.
- PulseAudio, or PipeWire with `pipewire-pulse`, for the lesson sound. Without it the lessons play silently.
- A phone with a camera on the same Wi-Fi (or network) as the computer.
- A vision-capable AI provider: a cloud service with your own key, or a local server such as Ollama.
- To make your own cassettes: an AI agent that can run commands, `ffmpeg` and a text-to-speech program ([details](docs/CASSETTE-AUTHORING-PROMPT.md)).

## Install

```
go install github.com/rm4n0s/radio-tutor/cmd/radio-tutor@latest
```

This puts `radio-tutor` in `$(go env GOPATH)/bin` (usually `~/go/bin`); add that folder to your `PATH` if it is not there already.

Or build from a clone:

```
git clone https://github.com/rm4n0s/radio-tutor.git
cd radio-tutor
go build -o radio-tutor ./cmd/radio-tutor
./radio-tutor version
```

A build from a clone prints `(devel)` as its version; `go install` of a release prints the release.

## First run

Start the app:

```
radio-tutor
```

1. **Create the parent account.** On the first run the app asks for it: a username, a name to show, and a password of at least 8 characters. It becomes the first admin.
2. **Add the child.** Students, then Add: a username, a display name and a password of at least 4 characters. The child taps their name on the login screen and types the password.
3. **Set up the AI provider** (below) and press **Set active** on it. Until one is active, the exercises cannot be checked.
4. **Import a cassette.** Cassettes, then Import a cassette, and choose a `.cassette` file. To try the app, use `testdata/cassettes/sample.cassette` from this repository ("Halves and quarters", two lessons in English). Check any cassette first from a terminal:

   ```
   radio-tutor validate testdata/cassettes/sample.cassette
   ```

   It prints `... is valid (0 errors, 0 warnings)` when the file is fine.
5. Log out and log in as the child, or press **Play as me** on the parent's home screen to try it yourself.

In **Settings** you choose the language (English or Greek), whether students may import cassettes themselves, the volume of the lessons and exercise sounds (for every profile), and the phone's port and network interface. Only one copy of Radio Tutor runs at a time.

## Let the phone reach the computer

The computer serves a small web page to the phone on port **8737** (the default; change it in Settings and use your number below). A firewall blocks it until you open the port. The QR screen shows the address as text and, after a minute without a visit, reminds you of this.

On Fedora and other systems with firewalld:

```
sudo firewall-cmd --add-port=8737/tcp --permanent && sudo firewall-cmd --reload
```

To remove the rule later:

```
sudo firewall-cmd --remove-port=8737/tcp --permanent && sudo firewall-cmd --reload
```

On Ubuntu and other systems with ufw: `sudo ufw allow 8737/tcp`.

The address is `http://<this computer's address on your network>:8737/s/<one-time code>`. The link works for 30 minutes, until the photos are sent, or until the child presses Cancel. The page uses plain HTTP and is only meant for your home network.

## Choosing an AI provider

On the parent's home screen open **AI providers**, press Add a provider, pick a preset (it fills the address and an example model) and edit what you need. Any server with an OpenAI-compatible API works.

- **Cloud** (OpenAI, Gemini, Anthropic, OpenRouter): you need an API key and a model that can read images. The photos of the child's paper and the exercise text are sent to that company. The name of the child is never sent. Check its privacy rules before you use it.
- **Local** (Ollama, LM Studio, llama.cpp server): the photos stay on your computer or home network and no key is needed. Grading is slower and depends on how good the model is at reading handwriting.

Ollama with a vision model, offline:

```
ollama pull qwen2.5vl:7b
ollama serve
```

(Skip `ollama serve` if Ollama already runs as a service.) In Radio Tutor choose the **Ollama** preset; it uses `http://localhost:11434/v1` and the model `qwen2.5vl:7b`. Models in the presets are suggestions and can be out of date; use any vision-capable model you have.

Press **Test** on the provider: the app sends a small picture of "2 + 3 = 5" and shows whether a valid answer came back, the verdict (it should be "correct"), the time and the model. If the server does not support the chosen structured output, Test says which setting works. Fix the address, key or model until it says "It works", then press **Set active**.

API keys are kept in `providers.json`, readable only by you. If the file is readable by others the app refuses to use it and tells you to run `chmod 600` on it.

## Making cassettes

You do not write cassettes by hand. Give an AI agent that can run commands your book or subject and the prompt in [docs/CASSETTE-AUTHORING-PROMPT.md](docs/CASSETTE-AUTHORING-PROMPT.md); it writes the lessons, makes the narration and packs the file. The format it follows is in [docs/CASSETTE-FORMAT.md](docs/CASSETTE-FORMAT.md). To check or pack a cassette yourself:

```
radio-tutor validate my-cassette/            # a folder or a .cassette file; add --json for machines
radio-tutor pack my-cassette/ my.cassette    # validates first; --force replaces an existing file
```

`validate` prints one problem per line and exits with 0 when there are no errors (warnings are allowed), 1 when there are errors. Import the result with Cassettes, Import a cassette. Importing a newer version of an installed cassette keeps the child's progress. Watch a new cassette yourself before your child does.

## Where the data lives

Radio Tutor follows the XDG folders (`XDG_CONFIG_HOME`, `XDG_DATA_HOME` and `XDG_STATE_HOME`; the defaults are shown):

| Folder | Holds |
|---|---|
| `~/.config/radio-tutor/` | `settings.json`, `providers.json` (with API keys) |
| `~/.local/share/radio-tutor/` | `profiles.json` (with password hashes), `cassettes/`, `progress/<user>/` (save files), `photos/<user>/` (the children's photos) |
| `~/.local/state/radio-tutor/` | `radio-tutor.log` (add `RADIO_TUTOR_DEBUG=1` to the environment for more detail), the lock file |

Photos stay until you delete them (Progress, per submission or per student). A file that cannot be read is not overwritten: it is renamed to `<name>.broken-<time>` next to it and the app starts with an empty value.

## Backing up

Close Radio Tutor first, then copy the settings and the data folders (the state folder is only the log):

```
mkdir -m 700 ~/radio-tutor-backup
cp -a ~/.config/radio-tutor ~/radio-tutor-backup/config
cp -a ~/.local/share/radio-tutor ~/radio-tutor-backup/data
```

Keep the backup private: it holds API keys, password hashes and photos of the children's paper. To restore, close the app and copy the two folders back to the same places.

## Forgotten password

Close Radio Tutor (the command refuses to run while it is open), then in a terminal:

```
radio-tutor reset-password <username>
```

It asks twice for the new password without showing it, and saves it. The password must be at least 8 characters for a parent and 4 for a student. A parent can also reset a student's password in the app (Students, Reset password). If you lost the only parent's password, this command is the way back in.

## Troubleshooting

- **No sound.** The lesson shows "No sound device found. The lesson plays without sound." and plays silently. Check that PulseAudio or PipeWire is running in your desktop session (`pactl info`; with PipeWire the `pipewire-pulse` service must run), then start the app again.
- **The phone cannot open the page.** The phone must be on the same network as the computer (not a guest network that isolates devices). Type the address shown under the QR code. Open the port as described [above](#let-the-phone-reach-the-computer). If the computer has several networks, pick the right one in Settings, "Network interface for the phone page". "This computer is not connected to a network" means no private address was found. A page saying the link has expired needs a new Check my answers. If the port is taken by another program, choose another one in Settings.
- **"Radio Tutor is already running."** Only one copy can run. Find it with `pgrep -a radio-tutor`, close it and start again.
- **"The checker could not answer."** The AI provider did not give a usable answer. The child can press Try again (the same photos are checked again). A parent should open AI providers, press **Test** and fix the address, key or model; check that the provider is the active one and that a local server is running. The child sees the short message; a parent sees the details in the Progress screen and the log.
- **A file chooser does not open.** The import needs a desktop file dialog (xdg-desktop-portal with a backend for your desktop).

## License

BSD 3-Clause, see [LICENSE](LICENSE).

## Third-party code

`internal/core/pulse` is a copy of [github.com/jfreymuth/pulse](https://github.com/jfreymuth/pulse) v0.1.3 (Copyright (c) 2019 Johann Freymuth, MIT License), the PulseAudio client Radio Tutor plays sound with. It is changed for Radio Tutor (it no longer blocks when a stream is paused and resumed, and it uses this project's error handling). The license is in [internal/core/pulse/LICENSE](internal/core/pulse/LICENSE) and every copied file says where it comes from.

## For developers

Radio Tutor is Go on [Glutter](https://github.com/rm4n0s/glutter), without cgo. Start with [CLAUDE.md](CLAUDE.md) (rules and commands), [docs/SPEC.md](docs/SPEC.md) (the specification), [docs/CASSETTE-FORMAT.md](docs/CASSETTE-FORMAT.md), [docs/IMPLEMENTATION.md](docs/IMPLEMENTATION.md) (the plan) and [docs/IMPLEMENTED.md](docs/IMPLEMENTED.md) (what each step built). The checks that must pass:

```
gofmt -l cmd internal                  # prints nothing
go vet ./...
golangci-lint run
GOMEMLIMIT=3GiB go test -race -p 2 ./...
CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test ./...
scripts/error-tags.sh --untested       # prints nothing
```

Run the app on throw-away data folders with `scripts/isolated.sh -- go run ./cmd/radio-tutor`. Checks that need a person, a phone or a real provider are in [docs/MANUAL-CHECKS.md](docs/MANUAL-CHECKS.md).
