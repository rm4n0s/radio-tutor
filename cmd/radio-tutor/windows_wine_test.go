//go:build radiotutor_wine && linux

package main

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// This file is the smoke test of the real Windows app under Wine (SPEC §16). It is a host-side
// test: it builds radio-tutor.exe and radio-tutor-cli.exe with scripts/build-windows.sh, runs them
// through scripts/wine-exec.sh (a headless X server, a private prefix, the audio drivers off, Mesa
// software drivers only, never the GPU), and checks what they do:
//
//	go test -tags radiotutor_wine -p 1 ./cmd/radio-tutor     (through scripts/heavy.sh)
//
// The prefix is .wine-test/smoke-prefix of the repository (git-ignored), and the user data of the
// app inside it is deleted before and after each run, so every run is a first run and nothing of
// the real user is read or written. The app saves its first frame with Glutter's self-capture
// (GLUTTER_SCREENSHOT) and ends by itself.

// wineEnv is what a test of this file needs to start Wine.
type wineEnv struct {
	root   string // the repository
	prefix string // the private Wine prefix of the smoke test
	bin    string // the folder of the built .exe files
	work   string // the folder of the captured frames
}

// repoRoot finds the repository: the folder with go.mod above the working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the working directory")
		}
		dir = parent
	}
}

// command runs a command in its own process group, so that a timeout ends it and everything it
// started, and returns its combined output and exit code (-1 when it could not run).
func (e wineEnv) command(t *testing.T, timeout time.Duration, env []string, name string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = e.root
	cmd.Env = append(os.Environ(), env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 10 * time.Second
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return out.String(), exit.ExitCode()
		}
		t.Fatalf("%s: %v\n%s", name, err, out.String())
	}
	return out.String(), 0
}

// wine runs a Windows program through scripts/wine-exec.sh with the prefix of this test.
func (e wineEnv) wine(t *testing.T, env []string, exe string, args ...string) (string, int) {
	t.Helper()
	env = append(env, "RADIO_TUTOR_WINEPREFIX="+e.prefix, "RADIO_TUTOR_WINE_TIMEOUT=150")
	out, code := e.command(t, 4*time.Minute, env, filepath.Join(e.root, "scripts", "wine-exec.sh"), append([]string{exe}, args...)...)
	if code == 69 {
		t.Skip("Wine or Xvfb is not installed (sudo dnf install wine-core xorg-x11-server-Xvfb)")
	}
	return out, code
}

// appData is the folders of the app's data inside the prefix (one per Windows user of the prefix).
func (e wineEnv) appData(t *testing.T) []string {
	t.Helper()
	dirs, err := filepath.Glob(filepath.Join(e.prefix, "drive_c", "users", "*", "AppData", "Local", "radio-tutor"))
	if err != nil {
		t.Fatal(err)
	}
	return dirs
}

// resetAppData deletes the data of the app in the prefix, so the next run is a first run.
func (e wineEnv) resetAppData(t *testing.T) {
	t.Helper()
	for _, dir := range e.appData(t) {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
	}
}

// appLog returns the log file of the app in the prefix ("" when there is none).
func (e wineEnv) appLog(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, dir := range e.appData(t) {
		data, err := os.ReadFile(filepath.Join(dir, "state", "radio-tutor.log"))
		if err == nil {
			b.Write(data)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	return b.String()
}

// winPath is the path of a host file as a Windows program sees it under Wine (the host's root is Z:).
func winPath(host string) string { return "Z:" + strings.ReplaceAll(host, "/", `\`) }

// build builds the two .exe files with scripts/build-windows.sh into e.bin.
func (e wineEnv) build(t *testing.T) {
	t.Helper()
	out, code := e.command(t, 8*time.Minute, []string{"RADIO_TUTOR_NO_HEAVY=1"},
		filepath.Join(e.root, "scripts", "build-windows.sh"), "-o", e.bin)
	if code != 0 {
		t.Fatalf("scripts/build-windows.sh exited %d:\n%s", code, out)
	}
}

// leftovers lists the processes of this prefix that are still alive (wineserver, wine programs).
func leftovers(t *testing.T, prefix string) []string {
	t.Helper()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for _, ent := range entries {
		if !ent.IsDir() || ent.Name()[0] < '0' || ent.Name()[0] > '9' {
			continue
		}
		env, err := os.ReadFile(filepath.Join("/proc", ent.Name(), "environ"))
		if err != nil {
			continue // not ours, or gone
		}
		if !bytes.Contains(env, []byte("\x00WINEPREFIX="+prefix+"\x00")) && !bytes.HasPrefix(env, []byte("WINEPREFIX="+prefix+"\x00")) {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join("/proc", ent.Name(), "cmdline"))
		if err != nil {
			continue
		}
		left = append(left, ent.Name()+" "+strings.ReplaceAll(string(cmdline), "\x00", " "))
	}
	return left
}

// frameStats decodes the PNG at path and returns its size, the number of distinct colors and the
// share (0 to 1) of the pixels that have the most common color.
func frameStats(t *testing.T, path string) (image.Point, int, float64) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the app saved no frame: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("the saved frame is not a PNG: %v", err)
	}
	b := img.Bounds()
	counts := map[[4]uint32]int{}
	most := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			k := [4]uint32{r, g, bl, a}
			counts[k]++
			if counts[k] > most {
				most = counts[k]
			}
		}
	}
	return b.Size(), len(counts), float64(most) / float64(b.Dx()*b.Dy())
}

// differentPixels counts the pixels at which two PNG files differ (they must have the same size).
func differentPixels(t *testing.T, a, b string) int {
	t.Helper()
	load := func(path string) image.Image {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	ia, ib := load(a), load(b)
	if ia.Bounds() != ib.Bounds() {
		t.Fatalf("frames have different sizes: %v and %v", ia.Bounds(), ib.Bounds())
	}
	n := 0
	bd := ia.Bounds()
	for y := bd.Min.Y; y < bd.Max.Y; y++ {
		for x := bd.Min.X; x < bd.Max.X; x++ {
			ra, ga, ba, aa := ia.At(x, y).RGBA()
			rb, gb, bb, ab := ib.At(x, y).RGBA()
			if ra != rb || ga != gb || ba != bb || aa != ab {
				n++
			}
		}
	}
	return n
}

func TestWineSmoke(t *testing.T) {
	root := repoRoot(t)
	e := wineEnv{
		root:   root,
		prefix: filepath.Join(root, ".wine-test", "smoke-prefix"),
		bin:    filepath.Join(root, ".wine-test", "smoke-bin"),
		work:   filepath.Join(root, ".wine-test", "smoke-frames"),
	}
	if err := os.MkdirAll(e.work, 0o755); err != nil {
		t.Fatal(err)
	}
	e.build(t)
	gui := filepath.Join(e.bin, "radio-tutor.exe")
	cli := filepath.Join(e.bin, "radio-tutor-cli.exe")
	for _, f := range []string{gui, cli} {
		if st, err := os.Stat(f); err != nil || st.Size() < 1<<20 {
			t.Fatalf("%s was not built: %v", f, err)
		}
	}
	e.resetAppData(t)
	t.Cleanup(func() {
		e.resetAppData(t)
		if left := leftovers(t, e.prefix); len(left) > 0 {
			t.Errorf("processes of the Wine prefix are left running:\n%s", strings.Join(left, "\n"))
		}
	})

	// The first run in English and in Greek: the app opens its window, presents a first frame, saves
	// it and ends with status 0. LANG is passed through to the Windows process (Windows has no LANG, and
	// the app reads it first, SPEC §14).
	frames := map[string]string{}
	runs := []struct {
		name string
		lang string
	}{
		{"english", "en_US.UTF-8"},
		{"greek", "el_GR.UTF-8"},
	}
	for _, tt := range runs {
		t.Run("first run "+tt.name, func(t *testing.T) {
			e.resetAppData(t)
			frame := filepath.Join(e.work, tt.name+".png")
			if err := os.Remove(frame); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			frames[tt.name] = frame
			out, code := e.wine(t, []string{"LANG=" + tt.lang, "GLUTTER_SCREENSHOT=" + winPath(frame)}, gui)
			log := e.appLog(t)
			if code != 0 {
				t.Fatalf("exit status %d, want 0\noutput:\n%s\nlog:\n%s", code, out, log)
			}
			for _, bad := range []string{"panic:", "fatal error:"} {
				if strings.Contains(out, bad) || strings.Contains(log, bad) {
					t.Errorf("%q in the output or the log\noutput:\n%s\nlog:\n%s", bad, out, log)
				}
			}
			if !strings.Contains(log, "first frame presented") {
				t.Errorf("no first frame was presented; log:\n%s", log)
			}
			size, colors, mostShare := frameStats(t, frame)
			if size.X < app.MinWindowWidth || size.Y < app.MinWindowHeight {
				t.Errorf("the frame is %v, smaller than the window's minimum", size)
			}
			// The first-run screen has a title, fields and a button: many colors (the
			// anti-aliased text) and no single color for the whole frame.
			if colors < 12 || mostShare > 0.9 {
				t.Errorf("the frame looks blank: %d colors, the most common fills %.0f%%", colors, mostShare*100)
			}
		})
	}
	t.Run("language changes the frame", func(t *testing.T) {
		en, el := frames["english"], frames["greek"]
		if en == "" || el == "" {
			t.Skip("a first run failed")
		}
		// All the text differs between the two languages: a large part of the text area changes.
		if n := differentPixels(t, en, el); n < 2000 {
			t.Errorf("only %d pixels differ between the English and the Greek first run", n)
		}
	})

	t.Run("gui exe given a command points to the cli exe", func(t *testing.T) {
		out, code := e.wine(t, nil, gui, "version")
		if code != 2 || !strings.Contains(out, "radio-tutor-cli.exe") {
			t.Errorf("exit %d, output %q; want exit 2 and a mention of radio-tutor-cli.exe", code, out)
		}
	})
	t.Run("cli exe validates the sample cassette", func(t *testing.T) {
		out, code := e.wine(t, nil, cli, "validate", filepath.Join(root, "testdata", "cassettes", "sample.cassette"))
		if code != 0 || !strings.Contains(out, "is valid (0 errors") {
			t.Errorf("exit %d, output:\n%s", code, out)
		}
	})
	t.Run("cli exe packs a folder and validates the result", func(t *testing.T) {
		packed := filepath.Join(e.work, "packed.cassette")
		if err := os.Remove(packed); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		out, code := e.wine(t, nil, cli, "pack", filepath.Join(root, "testdata", "cassettes", "sample"), packed)
		if code != 0 {
			t.Fatalf("pack exit %d, output:\n%s", code, out)
		}
		out, code = e.wine(t, nil, cli, "validate", packed)
		if code != 0 || !strings.Contains(out, "is valid") {
			t.Errorf("validate exit %d, output:\n%s", code, out)
		}
	})
	t.Run("cli exe reports an invalid cassette", func(t *testing.T) {
		out, code := e.wine(t, nil, cli, "validate", filepath.Join(root, "testdata", "no-such.cassette"))
		if code == 0 || strings.Contains(out, "panic:") {
			t.Errorf("exit %d, output:\n%s", code, out)
		}
	})
	t.Run("cli exe adds and removes the Start-menu shortcut", func(t *testing.T) {
		// The shortcut goes into the Start menu of the private prefix, never the user's.
		shortcuts := func() []string {
			found, err := filepath.Glob(filepath.Join(e.prefix, "drive_c", "users", "*", "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs", "Radio Tutor.lnk"))
			if err != nil {
				t.Fatal(err)
			}
			return found
		}
		for _, link := range shortcuts() {
			if err := os.Remove(link); err != nil {
				t.Fatal(err)
			}
		}
		t.Cleanup(func() {
			for _, link := range shortcuts() {
				if err := os.Remove(link); err != nil {
					t.Error(err)
				}
			}
		})
		out, code := e.wine(t, nil, cli, "setup-desktop")
		if code != 0 || !strings.Contains(out, "Radio Tutor.lnk") {
			t.Fatalf("setup-desktop exit %d, output:\n%s", code, out)
		}
		links := shortcuts()
		if len(links) != 1 {
			t.Fatalf("the Start menu of the prefix holds %v, want one Radio Tutor.lnk", links)
		}
		data, err := os.ReadFile(links[0])
		if err != nil {
			t.Fatal(err)
		}
		// The file stores the target path as plain (ANSI) text as well as Unicode.
		if !bytes.Contains(data, []byte("radio-tutor.exe")) {
			t.Errorf("the shortcut does not name radio-tutor.exe")
		}
		out, code = e.wine(t, nil, cli, "setup-desktop", "--remove")
		if code != 0 || !strings.Contains(out, "Radio Tutor.lnk") {
			t.Fatalf("setup-desktop --remove exit %d, output:\n%s", code, out)
		}
		if left := shortcuts(); len(left) != 0 {
			t.Errorf("the shortcut is still there after --remove: %v", left)
		}
	})
	t.Run("cli exe prints its version", func(t *testing.T) {
		out, code := e.wine(t, nil, cli, "version")
		if code != 0 || strings.TrimSpace(out) == "" {
			t.Errorf("exit %d, output %q", code, out)
		}
	})
}
