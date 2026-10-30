package worksheet

import (
	"context"
	"io/fs"
	"sync"

	"github.com/rm4n0s/radio-tutor/internal/core/audio"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// promptAudio plays the prompt.ogg of the exercises of a worksheet, one at a
// time. The sink is opened on the first play and closed with the screen. All its
// methods may be called from any goroutine; none touches a widget, so the owner
// passes the news ("this exercise stopped playing") through a callback that
// hands it to the UI thread.
type promptAudio struct {
	env  *app.Env
	fsys fs.FS

	mu      sync.Mutex
	sink    audio.Sink
	player  *audio.Player
	current string // the exercise that plays; empty when none
	changed func(playing string)
	watch   context.CancelFunc
	closed  bool
}

// newPromptAudio returns an audio player for the prompts in fsys.
func newPromptAudio(env *app.Env, fsys fs.FS) *promptAudio {
	return &promptAudio{env: env, fsys: fsys}
}

// Toggle plays the prompt audio of the exercise from the start, or stops it
// when it is the one that plays. It returns the exercise that plays afterwards
// ("" for none) and fails when the file cannot be read or decoded or the sink
// cannot start. changed is called, on a goroutine of its own, when a clip plays
// to its end.
func (a *promptAudio) Toggle(ctx context.Context, id string, changed func(playing string)) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return "", nil
	}
	a.changed = changed
	if a.current == id && a.player != nil {
		a.player.Stop()
		a.current = ""
		return "", nil
	}
	clip, err := a.decode(ctx, id)
	if err != nil {
		return a.current, err
	}
	if a.player == nil {
		sink, err := a.env.SinkFactory(ctx)
		if err != nil {
			return a.current, err
		}
		a.sink = sink
		a.player = audio.NewPlayer(sink, nil)
		watchCtx, cancel := context.WithCancel(context.Background())
		a.watch = cancel
		go a.watchEnd(watchCtx, a.player)
	}
	if err := a.player.Load(clip); err != nil {
		return a.current, err
	}
	cur := a.env.Settings.Get() // the app-wide volume and mute, as they are now
	a.player.SetVolume(cur.Volume)
	a.player.SetMuted(cur.Muted)
	if err := a.player.Play(ctx); err != nil {
		a.current = ""
		return "", err
	}
	a.current = id
	return id, nil
}

// decode reads and decodes the prompt audio of the exercise.
func (a *promptAudio) decode(ctx context.Context, id string) (*audio.Clip, error) {
	f, err := a.fsys.Open(promptFile(id, "prompt.ogg"))
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			a.env.Log.Warn("closing a prompt audio file failed", "exercise", id, "err", cerr)
		}
	}()
	return audio.Decode(ctx, f)
}

// watchEnd tells the owner when the clip plays to its end. The player has one
// Ended channel, so there is one watcher for the life of the player.
func (a *promptAudio) watchEnd(ctx context.Context, p *audio.Player) {
	for {
		select {
		case <-p.Ended():
			a.mu.Lock()
			notify := a.changed
			was := a.current
			a.current = ""
			a.mu.Unlock()
			if was != "" && notify != nil {
				notify("")
			}
		case <-ctx.Done():
			return
		}
	}
}

// Stop stops what plays, without blocking the caller.
func (a *promptAudio) Stop() {
	go func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.player != nil && a.current != "" {
			a.player.Stop()
		}
		a.current = ""
	}()
}

// Close stops the sound and releases the sink, without blocking the caller.
func (a *promptAudio) Close() {
	go func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.closed = true
		if a.watch != nil {
			a.watch()
		}
		if a.player != nil {
			a.player.Close()
		}
		if a.sink != nil {
			a.sink.Close()
		}
		a.player, a.sink, a.current = nil, nil, ""
	}()
}
