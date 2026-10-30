package admin

import (
	"context"

	"github.com/rm4n0s/errors"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// customPreset is the id of the "Other server" choice of the Add dialog; its
// key is KeyPresetCustom.
const customPreset = "custom"

// activeProvider is the active provider; false when there is none or the
// providers file cannot be used.
func activeProvider(env *app.Env) (providers.Config, bool) {
	if env.Providers == nil {
		return providers.Config{}, false
	}
	return env.Providers.Active()
}

// Providers is the screen of the AI providers (SPEC §11.6, §12): the list with
// the kind (cloud or local) and the active marker, Add from a preset, Edit,
// Delete, Set active, Test, and the privacy note of SPEC §11.7.
type Providers struct{ m.StatefulWidget }

// NewProviders returns the providers screen.
func NewProviders(opts ...m.WidgetOpts) *Providers {
	return &Providers{StatefulWidget: m.NewStatefulWidgetBase(opts...)}
}

// CreateState creates the state.
func (w *Providers) CreateState() m.State { return &providersState{} }

type providersState struct{ m.StateOf[*Providers] }

func (s *providersState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	env := scope.Env()
	children := []m.Widget{privacyNote(bc, scope), gap(app.Gap)}
	if env.Providers == nil {
		children = append(children, s.unusable(bc, scope, env))
		return m.NewScaffold(m.ScaffoldOpts{
			AppBar: appBar(scope, bc, scope.T("admin.providers")),
			Body:   pageBody(pageWidth, children),
		})
	}
	children = append(children,
		m.NewAlign(m.NewFilledButtonIcon(s.add, m.NewText(scope.T("admin.providers.add")), m.FilledButtonIconOpts{
			Icon: m.NewIcon(m.Icons.Add()),
			Key:  m.NewValueKey(KeyProviderAdd),
		}), m.AlignOpts{Alignment: m.AlignmentCenterLeft}),
		gap(app.Gap),
	)
	list := env.Providers.List()
	if len(list) == 0 {
		children = append(children, body(bc, scope.T("admin.providers.empty")))
	} else if _, ok := env.Providers.Active(); !ok {
		children = append(children, errorLine(bc, scope.T("admin.providers.none_active")), gap(app.Gap))
	}
	active := env.Providers.ActiveName()
	for _, c := range list {
		children = append(children, s.row(bc, scope, c, c.Name == active), gap(12))
	}
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: appBar(scope, bc, scope.T("admin.providers")),
		Body:   pageBody(pageWidth, children),
	})
}

// unusable explains why providers.json cannot be used (for example that it needs
// chmod 600).
func (s *providersState) unusable(bc m.BuildContext, scope *app.Scope, env *app.Env) m.Widget {
	message := scope.T("admin.providers.unusable")
	if appErr, ok := errors.FromError(env.ProvidersErr); ok {
		message = scope.T("admin.providers.unusable_detail", "message", appErr.Message, "tag", appErr.Tag)
	}
	return errorText(bc, message)
}

// privacyNote is the note of SPEC §11.7: a cloud provider receives the photos.
func privacyNote(bc m.BuildContext, scope *app.Scope) m.Widget {
	return m.NewCard(m.NewPadding(m.NewEdgeInsetsAll(16), m.NewRow([]m.Widget{
		m.NewIcon(m.Icons.Lock(), m.IconOpts{Size: m.Some(28.0)}),
		hgap(app.Gap),
		m.NewExpanded(m.NewText(scope.T("admin.providers.privacy"), m.TextOpts{
			Key: m.NewValueKey("providers-privacy"), Style: m.ThemeOf(bc).TextTheme.BodyMedium,
		})),
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart})), m.CardOpts{Margin: m.NewEdgeInsetsAll(0)})
}

// row is the card of one provider.
func (s *providersState) row(bc m.BuildContext, scope *app.Scope, c providers.Config, active bool) m.Widget {
	kindIcon := m.Icons.Cloud()
	if c.Kind == providers.KindLocal {
		kindIcon = m.Icons.Computer()
	}
	title := []m.Widget{
		m.NewFlexible(bold(bc, c.Name)), hgap(12),
		badge(scope.T("admin.provider.kind."+string(c.Kind)), kindIcon),
	}
	if active {
		title = append(title, hgap(8), badge(scope.T("admin.providers.active"), m.Icons.CheckCircle()))
	}
	buttons := []m.Widget{
		m.NewOutlinedButtonIcon(func() { s.test(scope, c.Name) }, m.NewText(scope.T("admin.providers.test")), m.OutlinedButtonIconOpts{
			Icon: m.NewIcon(m.Icons.Bolt()), Key: m.NewValueKey(KeyProviderTestPrefix + c.Name), Style: smallStyle(),
		}),
	}
	if !active {
		buttons = append(buttons, m.NewOutlinedButtonIcon(func() { s.setActive(scope, c.Name) }, m.NewText(scope.T("admin.providers.set_active")), m.OutlinedButtonIconOpts{
			Icon: m.NewIcon(m.Icons.Check()), Key: m.NewValueKey(KeyProviderActivePrefix + c.Name), Style: smallStyle(),
		}))
	}
	buttons = append(buttons,
		m.NewOutlinedButtonIcon(func() { s.edit(scope, c.Name) }, m.NewText(scope.T("admin.providers.edit")), m.OutlinedButtonIconOpts{
			Icon: m.NewIcon(m.Icons.Edit()), Key: m.NewValueKey(KeyProviderEditPrefix + c.Name), Style: smallStyle(),
		}),
		m.NewOutlinedButtonIcon(func() { s.delete(scope, c.Name) }, m.NewText(scope.T("admin.providers.delete")), m.OutlinedButtonIconOpts{
			Icon: m.NewIcon(m.Icons.Delete()), Key: m.NewValueKey(KeyProviderDeletePrefix + c.Name), Style: smallStyle(),
		}),
	)
	return m.NewCard(m.NewPadding(m.NewEdgeInsetsAll(16), m.NewColumn([]m.Widget{
		m.NewRow(title, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter, Key: m.NewValueKey(KeyProviderPrefix + c.Name)}),
		gap(4),
		muted(bc, c.Model+"  -  "+c.BaseURL),
		gap(12),
		m.NewWrap(buttons, m.WrapOpts{Spacing: 8, RunSpacing: 8}),
	}, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch})), m.CardOpts{Margin: m.NewEdgeInsetsAll(0)})
}

// smallStyle is the style of the buttons of a row, smaller than the touch size
// of the main buttons.
func smallStyle() *m.ButtonStyle {
	return m.OutlinedButtonStyleFrom(m.ButtonStyleFromOpts{
		MinimumSize: m.Some(m.Size{Width: 56, Height: 48}),
		Padding:     m.NewEdgeInsetsSymmetric(8, 14),
		Shape:       m.NewStadiumBorder(),
	})
}

// runStore calls the store function on a goroutine and shows its failure; done
// runs on the UI thread after a success.
func (s *providersState) runStore(scope *app.Scope, what string, do func(ctx context.Context) error, done func()) {
	env := scope.Env()
	s.Go(func(ctx context.Context) {
		err := do(ctx)
		if err != nil && ctx.Err() == nil {
			env.Log.Error("a provider change failed", "action", what, "err", err)
		}
		s.SetState(func() {
			switch {
			case err == nil:
				if done != nil {
					done()
				}
			case ctx.Err() == nil:
				showMessage(s.Context(), scope.Catalog().Msg(providerError(err)))
			}
		})
	})
}

// setActive makes the provider the active one.
func (s *providersState) setActive(scope *app.Scope, name string) {
	s.runStore(scope, "set active", func(ctx context.Context) error {
		return scope.Env().Providers.SetActive(ctx, name)
	}, nil)
}

// delete asks for confirmation, then removes the provider.
func (s *providersState) delete(scope *app.Scope, name string) {
	build := func(m.BuildContext) m.Widget {
		return confirmDialog(scope, scope.T("admin.providers.delete_title", "name", name),
			scope.T("admin.providers.delete_body"), scope.T("admin.providers.delete"))
	}
	showDialog(s, build, func(yes bool) {
		if yes {
			s.runStore(scope, "delete", func(ctx context.Context) error {
				return scope.Env().Providers.Delete(ctx, name)
			}, nil)
		}
	})
}

// test opens the Test dialog for the stored provider.
func (s *providersState) test(scope *app.Scope, name string) {
	cfg, ok := scope.Env().Providers.Get(name)
	if !ok {
		return
	}
	showDialog(s, func(m.BuildContext) m.Widget { return newTestDialog(cfg) }, func(bool) {})
}

// add asks which preset to start from, then opens the form.
func (s *providersState) add() {
	scope := app.ScopeOf(s.Context())
	showDialog(s, func(m.BuildContext) m.Widget { return presetDialog(scope) }, func(id string) {
		if id == "" {
			return
		}
		cfg := providers.Config{Kind: providers.KindLocal}
		if id != customPreset {
			for _, p := range providers.Presets() {
				if p.ID == id {
					cfg = p.Config()
				}
			}
		}
		s.openForm("", cfg, id)
	})
}

// edit opens the form for the stored provider.
func (s *providersState) edit(scope *app.Scope, name string) {
	cfg, ok := scope.Env().Providers.Get(name)
	if !ok {
		return
	}
	s.openForm(name, cfg, "")
}

// openForm pushes the provider form and shows the list again when it closes.
func (s *providersState) openForm(original string, cfg providers.Config, preset string) {
	ch := app.Push(s.Context(), func(m.BuildContext) m.Widget { return newProviderForm(original, cfg, preset) })
	s.Go(func(ctx context.Context) {
		select {
		case <-ch:
			s.SetState(func() {})
		case <-ctx.Done():
		}
	})
}

// presetDialog lists the presets of SPEC §11.1 and "Other server"; each answers
// its id.
func presetDialog(scope *app.Scope) m.Widget {
	entry := func(id, title, kind, text string) m.Widget {
		return m.NewBuilder(func(bc m.BuildContext) m.Widget {
			return m.NewListTile(m.ListTileOpts{
				Key:      m.NewValueKey(KeyPresetPrefix + id),
				Title:    m.NewText(title + "  (" + kind + ")"),
				Subtitle: m.NewText(text),
				OnTap:    func() { m.NavigatorPop(bc, id) },
			})
		})
	}
	var rows []m.Widget
	for _, p := range providers.Presets() {
		rows = append(rows, entry(p.ID, p.Name, scope.T("admin.provider.kind."+string(p.Kind)), scope.T("admin.preset."+p.ID)))
	}
	rows = append(rows, entry(customPreset, scope.T("admin.preset.custom_name"), scope.T("admin.provider.kind.local"), scope.T("admin.preset.custom")))
	return m.NewAlertDialog(m.AlertDialogOpts{
		Title:      m.NewText(scope.T("admin.providers.add_title")),
		Content:    m.NewSizedBox(m.NewColumn(rows, m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStretch, MainAxisSize: m.MainAxisSizeMin}), m.SizedBoxOpts{Width: m.Some(560.0)}),
		Scrollable: true,
		Actions:    []m.Widget{dialogButton(KeyDialogCancel, scope.T("admin.cancel"), "", false)},
	})
}

// providerError is the message for a failed change of the providers: the cases
// a parent can fix have their own text.
func providerError(err error) i18n.Message {
	if appErr, ok := errors.FromError(err); ok {
		switch appErr.Tag {
		case "ProviderNameDuplicate":
			return i18n.M("admin.providers.err_duplicate")
		case "ProviderNameInvalid":
			return i18n.M("admin.providers.err_name")
		case "ProviderTimeoutInvalid":
			return i18n.M("admin.providers.err_timeout")
		case "OpenAIConfigModel":
			return i18n.M("admin.providers.err_model")
		case "OpenAIConfigBaseURL", "OpenAIConfigBaseURLParse":
			return i18n.M("admin.providers.err_url")
		}
	}
	return errMessage(err)
}
