package player

import (
	"context"
	"math"
	"strconv"
	"sync"
	"time"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/profile"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

const (
	// volumeStep is the step of the volume slider, in percent (the same as the
	// slider of the Settings screen).
	volumeStep = 5
	// volumeSliderWidth is the width of the volume slider, in logical pixels.
	volumeSliderWidth = 160.0
	// snackBarLift is how far above the bottom the error of a failed save floats:
	// over the controls it would hide the buttons.
	snackBarLift = 120.0
	// saveSoundTimeout bounds the saving of the volume, which is not cancelled
	// when the child leaves the screen right after changing it.
	saveSoundTimeout = 10 * time.Second
)

// soundPrefs is the volume and the mute of the lesson as the child sets them:
// the UI thread writes them, the goroutine that owns the audio reads them to
// save them, so a save always writes the latest of both.
type soundPrefs struct {
	mu     sync.Mutex
	volume int
	muted  bool
}

func (p *soundPrefs) set(volume int, muted bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume, p.muted = volume, muted
}

func (p *soundPrefs) get() (volume int, muted bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume, p.muted
}

// readSound takes the volume and the mute from the app-wide settings, which the
// parents' Settings screen shares (SPEC §8.1). UI thread.
func (s *lessonState) readSound(env *app.Env) {
	cur := env.Settings.Get()
	s.volume, s.muted = float64(cur.Volume), cur.Muted
	s.prefs.set(cur.Volume, cur.Muted)
}

// soundButton is the mute toggle: the speaker with sound on, the crossed one
// when muted.
func (s *lessonState) soundButton(scope *app.Scope) m.Widget {
	icon, label := m.Icons.VolumeUp(), scope.T("player.mute")
	if s.muted {
		icon, label = m.Icons.VolumeOff(), scope.T("player.unmute")
	}
	return m.NewIconButton(s.toggleMute, m.NewIcon(icon), m.IconButtonOpts{
		Key:     m.NewValueKey(KeyMute),
		Tooltip: label,
	})
}

// volumeSlider is the volume of the app, 0 to 100 in steps of 5, with its value
// above the thumb while it is dragged. It follows the drag in s.volume and the
// sound changes with it; the setting is saved when the drag ends.
func (s *lessonState) volumeSlider(scope *app.Scope) m.Widget {
	percent := strconv.Itoa(int(s.volume)) + "%" // the same in every language
	return m.NewSizedBox(m.NewSlider(s.volume, s.dragVolume, m.SliderOpts{
		Key:         m.NewValueKey(KeyVolume),
		Min:         0,
		Max:         m.Some(float64(settings.MaxVolume)),
		Divisions:   m.Some(settings.MaxVolume / volumeStep),
		Label:       m.Some(percent),
		OnChangeEnd: s.saveVolume,
		SemanticFormatterCallback: func(float64) string {
			return scope.T("player.volume") + " " + percent
		},
	}), m.SizedBoxOpts{Width: m.Some(volumeSliderWidth)})
}

// dragVolume follows the thumb: the sound changes at once, and moving the thumb
// while muted unmutes. UI thread.
func (s *lessonState) dragVolume(v float64) {
	volume := int(math.Round(v))
	s.SetState(func() {
		s.volume = float64(volume)
		s.muted = false
	})
	s.applySound(volume, false)
}

// saveVolume saves the volume when the drag ends. UI thread.
func (s *lessonState) saveVolume(v float64) {
	s.applySound(int(math.Round(v)), s.muted)
	s.requestSave()
}

// toggleMute mutes the sound or brings it back at the volume it had. UI thread.
func (s *lessonState) toggleMute() {
	if !s.loaded {
		return
	}
	muted := !s.muted
	s.SetState(func() { s.muted = muted })
	s.applySound(int(math.Round(s.volume)), muted)
	s.requestSave()
}

// applySound gives the audio player and the prefs the volume and the mute. The
// audio player's setters are safe from the UI thread and never block.
func (s *lessonState) applySound(volume int, muted bool) {
	s.prefs.set(volume, muted)
	if s.player != nil {
		s.player.SetVolume(volume)
		s.player.SetMuted(muted)
	}
}

// requestSave asks the audio goroutine to save the prefs; requests that arrive
// while one is waiting merge, because a save writes the latest values.
func (s *lessonState) requestSave() {
	select {
	case s.saveReq <- struct{}{}:
	default:
	}
}

// saveSound writes the volume and the mute to the app-wide settings. A failure
// is logged and shown in a snack bar; the sound stays as the child set it. It
// runs on the audio goroutine.
func (s *lessonState) saveSound(ctx context.Context, env *app.Env) {
	volume, muted := s.prefs.get()
	// Not cancelled when the child leaves right after changing the volume.
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), saveSoundTimeout)
	defer cancel()
	err := env.Settings.Update(saveCtx, func(st *settings.Settings) { st.Volume, st.Muted = volume, muted })
	if err == nil {
		return
	}
	env.Log.Error("saving the volume failed", "volume", volume, "muted", muted, "err", err)
	if ctx.Err() != nil {
		return
	}
	s.SetState(func() {
		scope := app.ScopeOf(s.Context())
		detailed := scope.Session() != nil && scope.Session().Role == profile.RoleAdmin
		// Floating above the controls, so that it does not cover the buttons.
		m.ScaffoldMessengerOf(s.Context()).ShowSnackBar(m.NewSnackBar(m.NewText(scope.Catalog().Msg(app.ErrorMessage(err, detailed))), m.SnackBarOpts{
			Behavior: m.SnackBarBehaviorFloating,
			Margin:   m.NewEdgeInsetsFromLTRB(app.PagePadding, 0, app.PagePadding, snackBarLift),
		}))
	})
}
