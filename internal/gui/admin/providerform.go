package admin

import (
	"context"
	"strconv"
	"strings"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// providerForm is the screen that adds or edits a provider (SPEC §11.1): name,
// base URL, model, API key, cloud or local, structured output mode and timeout.
// When the admin edits a provider and leaves the API key empty, the saved key is
// kept (providers.Store.Update replaces every field).
type providerForm struct {
	m.StatefulWidget
	original string // the name of the provider being edited; empty when adding
	initial  providers.Config
	preset   string
}

func newProviderForm(original string, initial providers.Config, preset string) *providerForm {
	return &providerForm{StatefulWidget: m.NewStatefulWidgetBase(), original: original, initial: initial, preset: preset}
}

// CreateState creates the state.
func (w *providerForm) CreateState() m.State { return &providerFormState{} }

type providerFormState struct {
	m.StateOf[*providerForm]
	name, url, model, key, timeout *m.TextEditingController
	kind                           providers.Kind
	mode                           grading.StructuredOutput
	busy                           bool
	err                            *i18n.Message
}

func (s *providerFormState) InitState() {
	c := s.Widget().initial
	s.name, s.url, s.model = m.NewTextEditingController(), m.NewTextEditingController(), m.NewTextEditingController()
	s.key, s.timeout = m.NewTextEditingController(), m.NewTextEditingController()
	s.name.SetText(c.Name)
	s.url.SetText(c.BaseURL)
	s.model.SetText(c.Model)
	timeout := c.TimeoutSeconds
	if timeout == 0 {
		timeout = providers.DefaultTimeoutSeconds
	}
	s.timeout.SetText(strconv.Itoa(timeout))
	s.kind, s.mode = c.Kind, c.StructuredOutput
	if s.mode == "" {
		s.mode = grading.StructuredJSONSchema
	}
}

func (s *providerFormState) Dispose() {
	for _, c := range []*m.TextEditingController{s.name, s.url, s.model, s.key, s.timeout} {
		c.Dispose()
	}
}

// save adds or updates the provider on a goroutine and closes the screen.
func (s *providerFormState) save() {
	if s.busy {
		return
	}
	scope := app.ScopeOf(s.Context())
	env := scope.Env()
	w := s.Widget()
	timeout, err := strconv.Atoi(strings.TrimSpace(s.timeout.Text()))
	if err != nil {
		s.SetState(func() { s.err = &i18n.Message{Key: "admin.providers.err_timeout"} })
		return
	}
	cfg := providers.Config{
		Name: s.name.Text(), BaseURL: s.url.Text(), Model: s.model.Text(), APIKey: s.key.Text(),
		Kind: s.kind, TimeoutSeconds: timeout, StructuredOutput: s.mode,
	}
	s.SetState(func() { s.busy, s.err = true, nil })
	s.Go(func(ctx context.Context) {
		var err error
		if w.original == "" {
			err = env.Providers.Add(ctx, cfg)
		} else {
			if strings.TrimSpace(cfg.APIKey) == "" {
				if old, ok := env.Providers.Get(w.original); ok {
					cfg.APIKey = old.APIKey
				}
			}
			err = env.Providers.Update(ctx, w.original, cfg)
		}
		if err != nil && ctx.Err() == nil {
			env.Log.Error("saving a provider failed", "name", cfg.Name, "err", err)
		}
		s.SetState(func() {
			s.busy = false
			if err != nil {
				msg := providerError(err)
				s.err = &msg
				return
			}
			app.Pop(s.Context())
		})
	})
}

func (s *providerFormState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	w := s.Widget()
	title := scope.T("admin.providers.add_title")
	keyHelper := scope.T("admin.providers.key_hint")
	if w.original != "" {
		title = scope.T("admin.providers.edit_title", "name", w.original)
		keyHelper = scope.T("admin.providers.key_hint_edit")
	}
	children := []m.Widget{headline(bc, title)}
	if w.preset != "" && w.preset != customPreset {
		children = append(children, gap(8), muted(bc, scope.T("admin.preset."+w.preset)))
	}
	children = append(children,
		gap(app.Gap),
		textField(KeyProviderName, s.name, scope.T("admin.providers.name"), "", false, nil), gap(app.Gap),
		textField(KeyProviderURL, s.url, scope.T("admin.providers.url"), scope.T("admin.providers.url_hint"), false, nil), gap(app.Gap),
		textField(KeyProviderModel, s.model, scope.T("admin.providers.model"), scope.T("admin.providers.model_hint"), false, nil), gap(app.Gap),
		textField(KeyProviderAPIKey, s.key, scope.T("admin.providers.key"), keyHelper, true, nil), gap(app.Gap),
		s.kindChoice(scope), gap(app.Gap),
		s.modeChoice(bc, scope), gap(app.Gap),
		textField(KeyProviderTimeout, s.timeout, scope.T("admin.providers.timeout"), scope.T("admin.providers.timeout_hint"), false, func(string) { s.save() }),
		gap(app.Gap),
	)
	if s.kind == providers.KindCloud {
		children = append(children, privacyNote(bc, scope), gap(app.Gap))
	}
	if s.err != nil {
		children = append(children, errorText(bc, scope.Catalog().Msg(*s.err)), gap(app.Gap))
	}
	if s.busy {
		children = append(children, m.NewCenter(m.NewCircularProgressIndicator()))
	} else {
		children = append(children, m.NewFilledButton(s.save, m.NewText(scope.T("admin.save")), m.FilledButtonOpts{Key: m.NewValueKey(app.KeySubmit)}))
	}
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: appBar(scope, bc, scope.T("admin.providers")),
		Body:   pageBody(pageWidth, children),
	})
}

// kindChoice is the choice between a cloud and a local provider; the progress
// screens show it next to the provider's name (SPEC §11.7).
func (s *providerFormState) kindChoice(scope *app.Scope) m.Widget {
	chip := func(key string, kind providers.Kind) m.Widget {
		return m.NewChoiceChip(m.NewText(scope.T("admin.provider.kind."+string(kind))), s.kind == kind, m.ChoiceChipOpts{
			Key:        m.NewValueKey(key),
			OnSelected: func(bool) { s.SetState(func() { s.kind = kind }) },
		})
	}
	return m.NewWrap([]m.Widget{
		m.NewText(scope.T("admin.providers.kind")),
		chip(KeyKindCloud, providers.KindCloud),
		chip(KeyKindLocal, providers.KindLocal),
	}, m.WrapOpts{Spacing: 12, RunSpacing: 8, CrossAxisAlignment: m.WrapCrossAlignmentCenter})
}

// modeChoice is the choice of the structured output mode, with a line that
// explains the chosen one.
func (s *providerFormState) modeChoice(bc m.BuildContext, scope *app.Scope) m.Widget {
	chips := []m.Widget{m.NewText(scope.T("admin.providers.mode"))}
	for _, mode := range []grading.StructuredOutput{grading.StructuredJSONSchema, grading.StructuredJSONObject, grading.StructuredNone} {
		chips = append(chips, m.NewChoiceChip(m.NewText(string(mode)), s.mode == mode, m.ChoiceChipOpts{
			Key:        m.NewValueKey(KeyModePrefix + string(mode)),
			OnSelected: func(bool) { s.SetState(func() { s.mode = mode }) },
		}))
	}
	return m.NewColumn([]m.Widget{
		m.NewWrap(chips, m.WrapOpts{Spacing: 12, RunSpacing: 8, CrossAxisAlignment: m.WrapCrossAlignmentCenter}),
		gap(4),
		muted(bc, scope.T("admin.providers.mode_"+string(s.mode))),
	}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart})
}
