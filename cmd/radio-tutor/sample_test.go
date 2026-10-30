package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/problem"
)

// The sample cassette (step 7.1): its sources and the packed file are
// committed under testdata/cassettes and made by scripts/make-sample-cassette.sh.
const (
	sampleDir  = "../../testdata/cassettes/sample"
	samplePack = "../../testdata/cassettes/sample.cassette"
)

// TestSampleValidates validates the sample folder and the packed sample with
// the command, which draws every picture with Glutter's SVG renderer: no error
// and no warning.
func TestSampleValidates(t *testing.T) {
	for _, path := range []string{sampleDir, samplePack} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			code, stdout, stderr := runCmd(t, "validate", path)
			if code != 0 || stdout != "" {
				t.Errorf("validate %s: exit code %d, problems %q, stderr %q", path, code, stdout, stderr)
			}
		})
	}
}

// TestSamplePackIsCurrent packs the sample folder and compares the bytes with the
// committed sample.cassette: a changed source needs scripts/make-sample-cassette.sh
// to be run again.
func TestSamplePackIsCurrent(t *testing.T) {
	out := filepath.Join(t.TempDir(), "sample.cassette")
	if code, _, stderr := runCmd(t, "pack", sampleDir, out); code != 0 {
		t.Fatalf("pack: exit code %d, stderr %q", code, stderr)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(samplePack)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Error("testdata/cassettes/sample.cassette is not what the folder packs to; run scripts/make-sample-cassette.sh")
	}
}

// TestSampleContent checks what step 7.1 promises: an English cassette for ages
// 7 to 9 with two lessons, each followed by a worksheet of two exercises, with
// unique labels, hints, and captions that follow the narration.
func TestSampleContent(t *testing.T) {
	c, ps, err := cassette.Load(context.Background(), os.DirFS(sampleDir))
	if err != nil {
		t.Fatal(err)
	}
	if problem.HasErrors(ps) {
		t.Fatalf("problems: %v", ps)
	}
	m := c.Manifest
	if m.ID != "dev.radiotutor.sample.fractions" || m.Version.String() != "1.0.0" || m.Language != "en" ||
		m.Subject != "math" || len(m.Ages) != 2 || m.Ages[0] != 7 || m.Ages[1] != 9 {
		t.Errorf("manifest = %+v", m)
	}
	var kinds []string
	for _, it := range m.Items {
		kinds = append(kinds, string(it.Type))
	}
	if got, want := strings.Join(kinds, " "), "lesson exercise exercise lesson exercise exercise"; got != want {
		t.Errorf("items = %q, want %q", got, want)
	}
	if ws := c.Worksheets(); len(ws) != 2 || len(ws[0].ExerciseIDs) != 2 || len(ws[1].ExerciseIDs) != 2 {
		t.Errorf("worksheets = %+v", ws)
	}
	for id, le := range c.Lessons {
		if len(le.Tracks) < 5 {
			t.Errorf("lesson %s has %d tracks, want several actors", id, len(le.Tracks))
		}
		if n := len(le.Captions); n < 5 {
			t.Errorf("lesson %s has %d captions", id, n)
		}
		if last := le.Captions[len(le.Captions)-1]; last.End < 20 {
			t.Errorf("lesson %s speaks until %.1f s, want about 30 s", id, last.End)
		}
	}
	for id, ex := range c.Exercises {
		if len(ex.Hints) == 0 || ex.Grading == "" || ex.ReferenceSolution == "" {
			t.Errorf("exercise %s lacks hints, grading or a reference solution", id)
		}
	}
}
