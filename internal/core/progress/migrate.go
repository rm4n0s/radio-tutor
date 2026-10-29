package progress

import (
	"slices"

	"github.com/rm4n0s/errors"
	"github.com/rm4n0s/radio-tutor/internal/core/cassette"
)

// Migration says what Migrate changed.
type Migration struct {
	// OrphanedLessons are the ids of the lesson entries set aside, sorted.
	OrphanedLessons []string
	// OrphanedExercises are the ids of the exercise entries set aside, sorted.
	OrphanedExercises []string
	// AdjustedExercises are the ids of the kept exercise entries whose hints
	// or state changed because the new version has other hints or another
	// max_attempts, sorted.
	AdjustedExercises []string
	// LastItemCleared is true when last_item named an item the new version
	// does not have.
	LastItemCleared bool
}

// Migrate merges the save file with the cassette it now belongs to, when a
// newer version of the cassette was installed (SPEC §7.5, D18):
//   - entries of ids that still exist with the same type (lesson or exercise)
//     are kept, so a completed lesson stays completed;
//   - entries of ids that no longer exist, or that changed type, move to
//     p.Orphaned and are ignored from then on (an earlier orphaned entry of the
//     same id is replaced);
//   - the hints shown are limited to the hints the exercise now has, and a
//     kept exercise that is not passed gets the state its attempts and the new
//     max_attempts give (needs_review or none);
//   - last_item is cleared when the cassette no longer has that item;
//   - cassette_version becomes the new version. It reaches the file when p is
//     next saved.
//
// Submissions and overrides are history and stay as they are. Migrate is
// idempotent. It fails with the tag MigrateCassetteMismatch when p belongs to
// another cassette.
func Migrate(c *cassette.Cassette, p *Progress) (Migration, error) {
	if p.CassetteID != c.Manifest.ID {
		return Migration{}, errors.New("MigrateCassetteMismatch", "the save file belongs to another cassette",
			"cassette_id", c.Manifest.ID, "progress_cassette_id", p.CassetteID)
	}
	p.normalize()
	types := make(map[string]cassette.ItemType, len(c.Manifest.Items))
	for _, it := range c.Manifest.Items {
		types[it.ID] = it.Type
	}
	var m Migration

	for id, lp := range p.Lessons {
		if types[id] == cassette.ItemLesson {
			continue
		}
		p.orphaned().Lessons[id] = lp
		delete(p.Lessons, id)
		m.OrphanedLessons = append(m.OrphanedLessons, id)
	}
	for id, ep := range p.Exercises {
		ex := c.Exercises[id]
		if types[id] != cassette.ItemExercise || ex == nil {
			p.orphaned().Exercises[id] = ep
			delete(p.Exercises, id)
			m.OrphanedExercises = append(m.OrphanedExercises, id)
			continue
		}
		adjusted := ep
		adjusted.HintsShown = min(adjusted.HintsShown, len(ex.Hints))
		if ep.State != StatePassed {
			adjusted.State = derivedState(ex, ep)
		}
		if adjusted != ep {
			p.Exercises[id] = adjusted
			m.AdjustedExercises = append(m.AdjustedExercises, id)
		}
	}
	if p.LastItem != "" {
		if _, ok := types[p.LastItem]; !ok {
			p.LastItem = ""
			m.LastItemCleared = true
		}
	}
	p.CassetteVersion = c.Manifest.Version
	slices.Sort(m.OrphanedLessons)
	slices.Sort(m.OrphanedExercises)
	slices.Sort(m.AdjustedExercises)
	return m, nil
}

// orphaned returns p.Orphaned with both maps made.
func (p *Progress) orphaned() *Orphaned {
	if p.Orphaned == nil {
		p.Orphaned = &Orphaned{}
	}
	if p.Orphaned.Lessons == nil {
		p.Orphaned.Lessons = map[string]LessonProgress{}
	}
	if p.Orphaned.Exercises == nil {
		p.Orphaned.Exercises = map[string]ExerciseProgress{}
	}
	return p.Orphaned
}
