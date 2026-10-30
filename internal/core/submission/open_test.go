package submission

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
	"github.com/rm4n0s/radio-tutor/internal/core/phone"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
)

func TestOpenValidation(t *testing.T) {
	many := func(n int) []string {
		ids := make([]string, n)
		for i := range ids {
			ids[i] = "e1"
		}
		return ids
	}
	tests := []struct {
		name  string
		setup func(p *progress.Progress)
		ids   []string
		route string // "" means success
	}{
		{"one available exercise", nil, []string{"e1"}, ""},
		{"all of a worksheet", nil, []string{"e1", "e2", "e3"}, ""},
		{"none", nil, nil, "checkExercises.SubmissionExerciseCount"},
		{"eleven", nil, many(11), "checkExercises.SubmissionExerciseCount"},
		{"duplicate", nil, []string{"e1", "e2", "e1"}, "checkExercises.SubmissionExerciseDuplicate"},
		{"unknown", nil, []string{"e1", "zz"}, "checkExercises.SubmissionExerciseUnknown"},
		{"a lesson is not an exercise", nil, []string{"l1"}, "checkExercises.SubmissionExerciseUnknown"},
		{"locked by the unlock rule", nil, []string{"e4"}, "checkAvailable.SubmissionExerciseUnavailable"},
		{"passed", func(p *progress.Progress) {
			p.Exercises["e1"] = progress.ExerciseProgress{State: progress.StatePassed, PassedBy: progress.PassedByAI}
		}, []string{"e1", "e2"}, "checkAvailable.SubmissionExerciseUnavailable"},
		{"needs review", func(p *progress.Progress) {
			p.Exercises["e2"] = progress.ExerciseProgress{State: progress.StateNeedsReview, Attempts: 2}
		}, []string{"e2"}, "checkAvailable.SubmissionExerciseUnavailable"},
		{"locked, with free navigation", func(p *progress.Progress) { p.Settings.FreeNavigation = true }, []string{"e4"}, ""},
		{"passed, with free navigation", func(p *progress.Progress) {
			p.Settings.FreeNavigation = true
			p.Exercises["e1"] = progress.ExerciseProgress{State: progress.StatePassed, PassedBy: progress.PassedByAI}
		}, []string{"e1"}, ""},
		{"unlocked by a passed worksheet and a lesson", func(p *progress.Progress) {
			for _, id := range []string{"e1", "e2", "e3"} {
				p.Exercises[id] = progress.ExerciseProgress{State: progress.StatePassed, PassedBy: progress.PassedByAI}
			}
			p.Lessons["l1"] = progress.LessonProgress{CompletedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Plays: 1}
		}, []string{"e4"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			if tc.setup != nil {
				e.save("ann", tc.setup)
			}
			s, err := e.svc.Open(context.Background(), "ann", e.cass, tc.ids)
			if tc.route != "" {
				wantRoute(t, err, tc.route)
				// Nothing was stored, and no phone server was started for it.
				if p := e.load("ann"); len(p.Submissions) != 0 {
					t.Errorf("submissions %+v", p.Submissions)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := e.submission(s); len(got.ExerciseIDs) != len(tc.ids) {
				t.Errorf("record %+v", got)
			}
		})
	}
}

func TestOpenFailures(t *testing.T) {
	t.Run("cancelled context", func(t *testing.T) {
		e := newEnv(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := e.svc.Open(ctx, "ann", e.cass, []string{"e1"})
		wantRoute(t, err, "Service.Open.SubmissionOpenCancelled")
	})
	t.Run("unsafe username", func(t *testing.T) {
		e := newEnv(t)
		_, err := e.svc.Open(context.Background(), "../ann", e.cass, []string{"e1"})
		wantRoute(t, err, "ProgressUsernameInvalid")
	})
	t.Run("no network", func(t *testing.T) {
		e := newEnv(t)
		e.svc.opts.DetectLAN = func(context.Context, string) (phone.LANAddr, error) {
			return phone.LANAddr{}, errors.New("PhoneNoNetwork", "no network")
		}
		e.open2(t, "ann", "PhoneNoNetwork")
		if e.phone.Running() {
			t.Error("the phone server was started without a network")
		}
	})
	t.Run("the interface setting reaches the detection", func(t *testing.T) {
		e := newEnv(t)
		var got string
		e.svc.opts.DetectLAN = func(_ context.Context, preferred string) (phone.LANAddr, error) {
			got = preferred
			return phone.LANAddr{IP: net.ParseIP("127.0.0.1")}, nil
		}
		e.settings.update(func(s *settings.Settings) { s.Phone.Interface = "wlan7" })
		e.open("ann", "e1")
		if got != "wlan7" {
			t.Errorf("preferred = %q", got)
		}
	})
	t.Run("port in use", func(t *testing.T) {
		e := newEnv(t)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ln.Close() }()
		port := ln.Addr().(*net.TCPAddr).Port
		e.settings.update(func(s *settings.Settings) { s.Phone.Port = port })
		e.open2(t, "ann", "PhonePortInUse")
		if p := e.load("ann"); len(p.Submissions) != 0 {
			t.Errorf("a submission was stored: %+v", p.Submissions)
		}
	})
	t.Run("the random source fails", func(t *testing.T) {
		e := newEnv(t)
		e.svc.opts.Random = iotest.ErrReader(io.ErrClosedPipe)
		e.open2(t, "ann", "newID.SubmissionIDRandomFailed")
	})
	t.Run("every id is taken", func(t *testing.T) {
		e := newEnv(t)
		e.svc.opts.Random = bytes.NewReader(make([]byte, 64)) // zeros: the same id every time
		first := e.open("ann", "e1")
		if !strings.HasSuffix(first.ID, "-0000") {
			t.Fatalf("id %q", first.ID)
		}
		e.svc.opts.Random = bytes.NewReader(make([]byte, 64))
		e.open2(t, "ann", "newID.SubmissionIDTaken")
	})
	t.Run("the save file cannot be written", func(t *testing.T) {
		skipIfRoot(t)
		e := newEnv(t)
		e.save("ann", func(*progress.Progress) {})
		dir := filepath.Join(e.paths.ProgressDir(), "ann")
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chmod(dir, 0o700) }()
		_, err := e.svc.Open(context.Background(), "ann", e.cass, []string{"e1"})
		if err == nil {
			t.Fatal("expected an error")
		}
		if e.phone.Running() {
			// The token made for it was revoked: nothing is left to scan.
			if st, _ := get(t, "http://127.0.0.1:"+strconv.Itoa(e.phone.Port())+"/p/abc"); st != 410 {
				t.Errorf("status %d", st)
			}
		}
	})
}

// open2 opens a submission for ann that must fail with the route.
func (e *env) open2(t *testing.T, user, route string) {
	t.Helper()
	s, err := e.svc.Open(context.Background(), user, e.cass, []string{"e1"})
	if s != nil {
		t.Fatalf("Open succeeded: %+v", s)
	}
	wantRoute(t, err, route)
}

func TestOpenMigratesAnOlderSaveFile(t *testing.T) {
	e := newEnv(t)
	e.save("ann", func(p *progress.Progress) {
		p.CassetteVersion = cassette.Version{Major: 0, Minor: 9}
		p.Exercises["gone"] = progress.ExerciseProgress{State: progress.StatePassed}
		p.Exercises["e1"] = progress.ExerciseProgress{Attempts: 1, HintsShown: 5}
		p.LastItem = "gone"
	})
	e.open("ann", "e1")
	p := e.load("ann")
	if p.CassetteVersion != (cassette.Version{Major: 1}) {
		t.Errorf("cassette_version = %v", p.CassetteVersion)
	}
	if _, ok := p.Exercises["gone"]; ok || p.Orphaned == nil || p.Orphaned.Exercises["gone"].State != progress.StatePassed {
		t.Errorf("exercises %+v, orphaned %+v", p.Exercises, p.Orphaned)
	}
	if p.Exercises["e1"].HintsShown != 2 || p.LastItem != "" {
		t.Errorf("e1 = %+v, last item %q", p.Exercises["e1"], p.LastItem)
	}
}

func TestSaveFileOfANewerVersionIsNotMergedBackwards(t *testing.T) {
	e := newEnv(t)
	e.save("ann", func(p *progress.Progress) {
		p.CassetteVersion = cassette.Version{Major: 2}
		p.Exercises["newer"] = progress.ExerciseProgress{State: progress.StatePassed}
	})
	e.open("ann", "e1")
	p := e.load("ann")
	if p.CassetteVersion != (cassette.Version{Major: 2}) || p.Exercises["newer"].State != progress.StatePassed || p.Orphaned != nil {
		t.Errorf("version %v, exercises %+v, orphaned %+v", p.CassetteVersion, p.Exercises, p.Orphaned)
	}
}

func TestBrokenSaveFileIsSetAsideAndOpenGoesOn(t *testing.T) {
	e := newEnv(t)
	dir := filepath.Join(e.paths.ProgressDir(), "ann")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "org.example.sub-1.json")
	if err := os.WriteFile(file, []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := e.open("ann", "e1")
	if sub := e.submission(s); sub.Status != progress.SubmissionWaitingPhotos {
		t.Errorf("record %+v", sub)
	}
	matches, err := filepath.Glob(file + ".broken-*")
	if err != nil || len(matches) != 1 {
		t.Errorf("set-aside files %v, %v", matches, err)
	}
}

func TestPageDataFollowsTheLanguageAndTheCassette(t *testing.T) {
	e := newEnv(t)
	e.settings.update(func(s *settings.Settings) { s.Language = "el" })
	s := e.open("ann", "e1", "e3")
	status, page := get(t, s.URL)
	if status != 200 || !contains(page, "Fractions for all", "Title of e1", "Title of e3", `lang="el"`) {
		t.Errorf("status %d\n%s", status, page)
	}
}
