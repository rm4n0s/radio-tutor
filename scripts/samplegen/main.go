// Command samplegen makes the narration and the lesson timelines of the sample
// cassette from its sources. It is run by scripts/make-sample-cassette.sh.
//
// The sources (testdata/cassettes/sample-src) are:
//
//	script.json            the sentences to speak and the pauses between them
//	lessons/<id>.json      a lesson.json template: title, summary and tracks
//
// For every lesson (and every exercise prompt) listed in script.json it
// speaks each sentence with espeak-ng into its own WAV file, measures the files
// with ffprobe, joins them with ffmpeg (a lead silence, the sentences with
// their pauses, a tail silence) and encodes the result as Ogg Vorbis. The
// sentence times are known exactly from the measured lengths, so the captions
// are generated from them, and the key times of the template are resolved
// against them. The result is written into the cassette folder as
// lessons/<id>/narration.ogg and lessons/<id>/lesson.json (exercises:
// exercises/<id>/prompt.ogg).
//
// A key of a template gives its time either as "t" (seconds) or as "at", an
// anchor to the speech: "s3" is the start of sentence 3 (counting from 1), "e3"
// its end and "end" the end of the narration, each with an optional offset in
// seconds ("s3+0.5", "e2-0.2"). Anchors keep the animation in step with the
// narration when a sentence is reworded or the voice changes.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/timeline"
)

func main() {
	os.Exit(realMain())
}

func realMain() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err := run(ctx, os.Args[1:], os.Stdout)
	if err == nil {
		return 0
	}
	if appErr, ok := errors.FromError(err); ok {
		slog.Error("samplegen failed", "tag", appErr.Tag, "message", appErr.Message, "metadata", appErr.Metadata)
	} else {
		slog.Error("samplegen failed", "err", err)
	}
	return 1
}

// script is script.json.
type script struct {
	// Voice is the espeak-ng voice, e.g. "en-us".
	Voice string `json:"voice"`
	// Speed is espeak-ng's speed in words per minute.
	Speed int `json:"speed"`
	// Silence is the default pause after a sentence, in seconds.
	Silence float64 `json:"silence"`
	// Lead is the silence before the first sentence, in seconds.
	Lead float64 `json:"lead"`
	// Tail is the silence after the last sentence, in seconds.
	Tail float64 `json:"tail"`
	// Lessons are the lessons to make a narration and a timeline for.
	Lessons []item `json:"lessons"`
	// Exercises are the exercises that have a prompt.ogg.
	Exercises []item `json:"exercises"`
}

// item is the speech of one lesson or exercise.
type item struct {
	ID        string     `json:"id"`
	Sentences []sentence `json:"sentences"`
}

// sentence is one spoken sentence and its caption.
type sentence struct {
	Text string `json:"text"`
	// Pause is the silence after the sentence; nil means script.Silence.
	Pause *float64 `json:"pause,omitempty"`
}

// span is the time of one sentence in the narration.
type span struct{ Start, End float64 }

// tmplKey is a key of a template: a timeline key whose time is a number or an
// anchor.
type tmplKey struct {
	At      string   `json:"at,omitempty"`
	T       *float64 `json:"t,omitempty"`
	Opacity *float64 `json:"opacity,omitempty"`
	X       *float64 `json:"x,omitempty"`
	Y       *float64 `json:"y,omitempty"`
	Scale   *float64 `json:"scale,omitempty"`
	Rotate  *float64 `json:"rotate,omitempty"`
	Reveal  *float64 `json:"reveal,omitempty"`
	Ease    string   `json:"ease,omitempty"`
}

// tmplTrack is a track of a template.
type tmplTrack struct {
	Actor      string    `json:"actor"`
	Origin     []float64 `json:"origin,omitempty"`
	RevealBox  []float64 `json:"reveal_box,omitempty"`
	RevealFrom string    `json:"reveal_from,omitempty"`
	Keys       []tmplKey `json:"keys"`
}

// template is a lessons/<id>.json of the sources.
type template struct {
	Title   string      `json:"title"`
	Summary string      `json:"summary,omitempty"`
	Tracks  []tmplTrack `json:"tracks"`
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("samplegen", flag.ContinueOnError)
	src := fs.String("src", "testdata/cassettes/sample-src", "the folder with script.json and lessons/")
	out := fs.String("out", "testdata/cassettes/sample", "the cassette folder to write into")
	tmp := fs.String("tmp", "", "a work folder for the WAV files (default: a new temporary folder, deleted at the end)")
	if err := fs.Parse(args); err != nil {
		return errors.NewErr("SampleGenFlags", err)
	}
	work := *tmp
	if work == "" {
		dir, err := os.MkdirTemp("", "samplegen-")
		if err != nil {
			return errors.NewErr("SampleGenTempDir", err)
		}
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				slog.Warn("cannot remove the work folder", "dir", dir, "err", err)
			}
		}()
		work = dir
	}
	var sc script
	if err := readJSON(filepath.Join(*src, "script.json"), &sc); err != nil {
		return err
	}
	if sc.Voice == "" || sc.Speed <= 0 {
		return errors.New("SampleGenScriptInvalid", "script.json needs a voice and a speed")
	}
	for _, l := range sc.Lessons {
		spans, total, err := narrate(ctx, sc, l, work, filepath.Join(*out, "lessons", l.ID, "narration.ogg"))
		if err != nil {
			return err
		}
		var tm template
		if err := readJSON(filepath.Join(*src, "lessons", l.ID+".json"), &tm); err != nil {
			return err
		}
		data, err := lessonJSON(tm, l, spans, total)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(*out, "lessons", l.ID, "lesson.json"), data, 0o644); err != nil {
			return errors.NewErr("SampleGenWriteLesson", err).SetMetadata("lesson", l.ID)
		}
		if err := report(stdout, "lesson "+l.ID, l, spans, total); err != nil {
			return err
		}
	}
	for _, e := range sc.Exercises {
		spans, total, err := narrate(ctx, sc, e, work, filepath.Join(*out, "exercises", e.ID, "prompt.ogg"))
		if err != nil {
			return err
		}
		if err := report(stdout, "exercise "+e.ID, e, spans, total); err != nil {
			return err
		}
	}
	return nil
}

// report prints the time of every sentence, for the author to check the timeline
// against.
func report(w io.Writer, name string, it item, spans []span, total float64) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %.2f s\n", name, total)
	for i, s := range spans {
		fmt.Fprintf(&b, "  s%d %6.2f - %6.2f  %s\n", i+1, s.Start, s.End, it.Sentences[i].Text)
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return errors.NewErr("SampleGenReport", err)
	}
	return nil
}

func readJSON(path string, dst any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.NewErr("SampleGenReadFile", err).SetMetadata("path", path)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errors.NewErr("SampleGenParseJSON", err).SetMetadata("path", path)
	}
	return nil
}

// plan places the sentences in time: the lead silence, then each sentence
// followed by its pause (the last one by the tail). It returns the span of every
// sentence and the length of the narration.
func plan(sc script, sentences []sentence, durations []float64) (spans []span, total float64) {
	t := sc.Lead
	for i, d := range durations {
		spans = append(spans, span{Start: round(t), End: round(t + d)})
		t += d
		if i == len(durations)-1 {
			t += sc.Tail
			break
		}
		if p := sentences[i].Pause; p != nil {
			t += *p
		} else {
			t += sc.Silence
		}
	}
	return spans, round(t)
}

func round(v float64) float64 { return math.Round(v*1000) / 1000 }

// narrate speaks the sentences of it, joins and encodes them into outOgg and
// returns the sentence spans and the length of the narration.
func narrate(ctx context.Context, sc script, it item, work, outOgg string) ([]span, float64, error) {
	dir := filepath.Join(work, it.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, 0, errors.NewErr("SampleGenMkdir", err).SetMetadata("dir", dir)
	}
	if len(it.Sentences) == 0 {
		return nil, 0, errors.New("SampleGenNoSentences", "an item has no sentences", "id", it.ID)
	}
	var durations []float64
	var wavs []string
	for i, s := range it.Sentences {
		raw := filepath.Join(dir, fmt.Sprintf("raw-%02d.wav", i+1))
		wav := filepath.Join(dir, fmt.Sprintf("s-%02d.wav", i+1))
		if _, err := command(ctx, "espeak-ng", "-v", sc.Voice, "-s", strconv.Itoa(sc.Speed), "-w", raw, s.Text); err != nil {
			return nil, 0, err
		}
		// Every piece gets the same format, which the concat demuxer needs.
		if err := toWAV(ctx, raw, wav); err != nil {
			return nil, 0, err
		}
		d, err := duration(ctx, wav)
		if err != nil {
			return nil, 0, err
		}
		durations = append(durations, d)
		wavs = append(wavs, wav)
	}
	spans, total := plan(sc, it.Sentences, durations)

	// The join list: lead, sentence, pause, sentence, ..., tail.
	var list strings.Builder
	silence := func(seconds float64) error {
		name := filepath.Join(dir, fmt.Sprintf("silence-%d.wav", int(math.Round(seconds*1000))))
		if _, err := os.Stat(name); err != nil {
			_, err := command(ctx, "ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "anullsrc=r=22050:cl=mono",
				"-t", strconv.FormatFloat(seconds, 'f', 3, 64), "-c:a", "pcm_s16le", name)
			if err != nil {
				return err
			}
		}
		fmt.Fprintf(&list, "file '%s'\n", name)
		return nil
	}
	if err := silence(sc.Lead); err != nil {
		return nil, 0, err
	}
	for i, w := range wavs {
		fmt.Fprintf(&list, "file '%s'\n", w)
		gap := sc.Tail
		if i < len(wavs)-1 {
			gap = sc.Silence
			if p := it.Sentences[i].Pause; p != nil {
				gap = *p
			}
		}
		if err := silence(gap); err != nil {
			return nil, 0, err
		}
	}
	listFile := filepath.Join(dir, "list.txt")
	if err := os.WriteFile(listFile, []byte(list.String()), 0o644); err != nil {
		return nil, 0, errors.NewErr("SampleGenWriteList", err).SetMetadata("path", listFile)
	}
	if err := os.MkdirAll(filepath.Dir(outOgg), 0o755); err != nil {
		return nil, 0, errors.NewErr("SampleGenMkdir", err).SetMetadata("dir", filepath.Dir(outOgg))
	}
	// Mono, 22050 Hz and quality 2 keep the repository small; the bitexact flags
	// and no metadata make the same input give the same bytes.
	if _, err := command(ctx, "ffmpeg", "-v", "error", "-y", "-f", "concat", "-safe", "0", "-i", listFile,
		"-ac", "1", "-ar", "22050", "-map_metadata", "-1", "-fflags", "+bitexact", "-flags:a", "+bitexact",
		"-c:a", "libvorbis", "-q:a", "2", outOgg); err != nil {
		return nil, 0, err
	}
	got, err := duration(ctx, outOgg)
	if err != nil {
		return nil, 0, err
	}
	if got < total-0.05 {
		return nil, 0, errors.New("SampleGenShortAudio", "the encoded narration is shorter than planned",
			"id", it.ID, "planned", total, "got", got)
	}
	// The planned total is what the keys are checked against; the file is as
	// long as that or a little longer (Vorbis packets).
	return spans, total, nil
}

// toWAV converts in to mono 16-bit 22050 Hz WAV.
func toWAV(ctx context.Context, in, out string) error {
	_, err := command(ctx, "ffmpeg", "-v", "error", "-y", "-i", in, "-ac", "1", "-ar", "22050", "-c:a", "pcm_s16le", out)
	return err
}

// duration measures an audio file with ffprobe.
func duration(ctx context.Context, path string) (float64, error) {
	out, err := command(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(out), 64)
	if err != nil {
		return 0, errors.NewErr("SampleGenDuration", err).SetMetadata("path", path, "output", out)
	}
	return v, nil
}

// command runs a program and returns its standard output.
func command(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", errors.NewErr("SampleGenCommand", err).SetMetadata("command", name, "args", strings.Join(args, " "), "stderr", stderr.String())
	}
	return stdout.String(), nil
}

var anchorRe = regexp.MustCompile(`^(?:s(\d+)|e(\d+)|(end))([+-]\d+(?:\.\d+)?)?$`)

// resolve turns an anchor ("s3", "e2-0.2", "end-1") into a time.
func resolve(anchor string, spans []span, total float64) (float64, error) {
	m := anchorRe.FindStringSubmatch(anchor)
	if m == nil {
		return 0, errors.New("SampleGenAnchorInvalid", "an anchor is not s<N>, e<N> or end, with an optional +/- offset", "anchor", anchor)
	}
	var base float64
	switch {
	case m[3] != "":
		base = total
	default:
		digits, isStart := m[1], true
		if m[2] != "" {
			digits, isStart = m[2], false
		}
		n, err := strconv.Atoi(digits)
		if err != nil || n < 1 || n > len(spans) {
			return 0, errors.New("SampleGenAnchorSentence", "an anchor names a sentence that does not exist", "anchor", anchor, "sentences", len(spans))
		}
		if isStart {
			base = spans[n-1].Start
		} else {
			base = spans[n-1].End
		}
	}
	if m[4] != "" {
		off, err := strconv.ParseFloat(m[4], 64)
		if err != nil {
			return 0, errors.NewErr("SampleGenAnchorOffset", err).SetMetadata("anchor", anchor)
		}
		base += off
	}
	return round(base), nil
}

// lessonJSON resolves a template against the spans and formats the lesson.json:
// one track key per line, the captions from the sentences.
func lessonJSON(tm template, it item, spans []span, total float64) ([]byte, error) {
	var b bytes.Buffer
	enc := func(v any) (string, error) {
		data, err := json.Marshal(v)
		if err != nil {
			return "", errors.NewErr("SampleGenMarshal", err)
		}
		return string(data), nil
	}
	title, err := enc(tm.Title)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&b, "{\n  \"format\": 1,\n  \"title\": %s,\n", title)
	if tm.Summary != "" {
		summary, err := enc(tm.Summary)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "  \"summary\": %s,\n", summary)
	}
	b.WriteString("  \"tracks\": [")
	for i, tr := range tm.Tracks {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {\n")
		fmt.Fprintf(&b, "      \"actor\": %q,\n", tr.Actor)
		if len(tr.Origin) > 0 {
			s, err := enc(tr.Origin)
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(&b, "      \"origin\": %s,\n", s)
		}
		if len(tr.RevealBox) > 0 {
			s, err := enc(tr.RevealBox)
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(&b, "      \"reveal_box\": %s,\n", s)
		}
		if tr.RevealFrom != "" {
			fmt.Fprintf(&b, "      \"reveal_from\": %q,\n", tr.RevealFrom)
		}
		b.WriteString("      \"keys\": [")
		for j, k := range tr.Keys {
			t, err := keyTime(k, spans, total)
			if err != nil {
				return nil, err
			}
			if t > total {
				return nil, errors.New("SampleGenKeyAfterEnd", "a key is after the end of the narration", "actor", tr.Actor, "t", t, "total", total)
			}
			tk := timeline.Key{T: t, Opacity: k.Opacity, X: k.X, Y: k.Y, Scale: k.Scale, Rotate: k.Rotate, Reveal: k.Reveal, Ease: timeline.Ease(k.Ease)}
			s, err := enc(tk)
			if err != nil {
				return nil, err
			}
			if j > 0 {
				b.WriteString(",")
			}
			b.WriteString("\n        " + s)
		}
		b.WriteString("\n      ]\n    }")
	}
	b.WriteString("\n  ],\n  \"captions\": [")
	for i, s := range it.Sentences {
		c, err := enc(timeline.Caption{Start: spans[i].Start, End: spans[i].End, Text: s.Text})
		if err != nil {
			return nil, err
		}
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    " + c)
	}
	b.WriteString("\n  ]\n}\n")
	return b.Bytes(), nil
}

// keyTime is the time of a template key.
func keyTime(k tmplKey, spans []span, total float64) (float64, error) {
	switch {
	case k.At != "" && k.T != nil:
		return 0, errors.New("SampleGenKeyTime", "a key has both t and at", "at", k.At)
	case k.At != "":
		return resolve(k.At, spans, total)
	case k.T != nil:
		return *k.T, nil
	}
	return 0, errors.New("SampleGenKeyTime", "a key has neither t nor at")
}
