package submission

import (
	"context"
	"slices"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/progress"
)

// outcome is the answer of a grading call, ready to be applied.
type outcome struct {
	results    []progress.Result
	pageNotes  string
	provider   string
	model      string
	downgraded grading.StructuredOutput
}

// run grades the stored photos and applies the results. It runs on a goroutine
// of its own; ctx ends when the session is cancelled. A session that is no
// longer grading when the answer comes (it was cancelled) drops the answer.
func (s *Session) run(ctx context.Context) {
	s.mu.Lock()
	out := s.pending
	names := slices.Clone(s.photos)
	s.mu.Unlock()

	if out == nil {
		var providerName string
		var err error
		out, providerName, err = s.grade(ctx, names)
		if err != nil {
			s.failRun(err, providerName)
			return
		}
	}

	s.persistMu.Lock()
	defer s.persistMu.Unlock()
	s.mu.Lock()
	if s.state != stGrading {
		s.mu.Unlock()
		return
	}
	s.state = stApplying
	s.mu.Unlock()

	var changes progress.Changes
	err := s.persist(s.svc.base, func(p *progress.Progress, sub *progress.Submission) error {
		var err error
		changes, err = progress.ApplyResults(s.cass, p, out.results, s.svc.opts.Clock.Now())
		if err != nil {
			return err
		}
		sub.Photos = slices.Clone(names)
		sub.Status = progress.SubmissionGraded
		sub.Provider = out.provider
		sub.Model = out.model
		sub.PageNotes = out.pageNotes
		sub.Results = slices.Clone(out.results)
		sub.Error = ""
		return nil
	})
	if err != nil {
		// The answer is kept: Retry applies it again without asking the
		// provider a second time.
		s.persistFailure(s.svc.base, err, out.provider)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.pending = out
		s.state = stFailed
		s.emit(Failed{Err: err, CanRetry: true})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = nil
	s.state = stDone
	if out.downgraded != "" {
		s.svc.opts.Log.Warn("the provider used a weaker structured output mode than configured; the parent should change the setting",
			"provider", out.provider, "mode", string(out.downgraded))
	}
	s.emit(Graded{
		Results:    slices.Clone(out.results),
		Changes:    changes,
		PageNotes:  out.pageNotes,
		Provider:   out.provider,
		Model:      out.model,
		Downgraded: out.downgraded,
	})
	s.finish()
}

// failRun moves a grading session to failed, unless it was cancelled meanwhile.
func (s *Session) failRun(cause error, providerName string) {
	s.persistMu.Lock()
	defer s.persistMu.Unlock()
	s.mu.Lock()
	if s.state != stGrading {
		s.mu.Unlock()
		return
	}
	s.state = stFailed
	s.emit(Failed{Err: cause, CanRetry: true})
	s.mu.Unlock()
	s.persistFailure(s.svc.base, cause, providerName)
}

// grade asks the active provider to grade the stored photos. It returns the
// name of the provider when it got as far as knowing it. With no active
// provider it fails with the tag SubmissionNoProvider.
func (s *Session) grade(ctx context.Context, names []string) (*outcome, string, error) {
	opts := s.svc.opts
	cfg, ok := opts.Providers.Active()
	if !ok {
		return nil, "", errors.New("SubmissionNoProvider", "no grading provider is active", "id", s.ID)
	}
	provider, lenient, err := opts.BuildProvider(cfg, opts.Log)
	if err != nil {
		return nil, cfg.Name, err
	}
	photos, err := loadPhotos(photoDir(opts.Paths.PhotosDir(), s.Username, s.CassetteID, s.ID), names)
	if err != nil {
		return nil, cfg.Name, err
	}
	req, err := grading.NewRequest(s.cass, s.Exercises, photos)
	if err != nil {
		return nil, cfg.Name, err
	}
	grader := grading.Grader{Provider: provider, Lenient: lenient, Log: opts.Log, Now: opts.Clock.Now}
	res, err := grader.Grade(ctx, req)
	if err != nil {
		return nil, cfg.Name, err
	}
	return &outcome{
		results:    mapResults(res.Exercises),
		pageNotes:  res.PageNotes,
		provider:   res.Meta.Provider,
		model:      res.Meta.Model,
		downgraded: res.Meta.Downgraded,
	}, cfg.Name, nil
}

// mapResults turns the grader's results into the results of the save file. The
// grading parser already dropped the ids that were not asked, so a result for
// an exercise outside the session never gets here.
func mapResults(in []grading.ExerciseResult) []progress.Result {
	out := make([]progress.Result, 0, len(in))
	for _, r := range in {
		out = append(out, progress.Result{
			ExerciseID:       r.ExerciseID,
			Verdict:          progress.Verdict(r.Verdict),
			Transcription:    r.Transcription,
			FeedbackForChild: r.FeedbackForChild,
			NotesForParent:   r.NotesForParent,
		})
	}
	return out
}
