package admin

import (
	"context"
	"math"
	"strconv"
	"strings"

	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/phone"
	"github.com/rm4n0s/radio-tutor/internal/core/settings"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// The range of the phone port the screen accepts: ports below 1024 need
// privileges.
const (
	minPort = 1024
	maxPort = 65535
	// volumeStep is the step of the volume slider, in percent.
	volumeStep = 5
)

// Settings is the screen of settings.json (SPEC §12.4): language, whether
// students can import, the volume, the phone port and the network interface.
type Settings struct {
	m.StatefulWidget
	screens Screens
}

// NewSettings returns the settings screen; screens.Interfaces lists the network
// interfaces.
func NewSettings(screens Screens, opts ...m.WidgetOpts) *Settings {
	return &Settings{StatefulWidget: m.NewStatefulWidgetBase(opts...), screens: screens}
}

// CreateState creates the state.
func (w *Settings) CreateState() m.State { return &settingsState{} }

type settingsState struct {
	m.StateOf[*Settings]
	port       *m.TextEditingController
	started    bool
	volume     float64 // the position of the slider, in percent
	interfaces []phone.Interface
	err        *i18n.Message
}

func (s *settingsState) InitState() { s.port = m.NewTextEditingController() }

func (s *settingsState) Dispose() { s.port.Dispose() }

// DidChangeDependencies fills the port field and lists the network interfaces
// (the Scope cannot be read in InitState).
func (s *settingsState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	env := app.ScopeOf(s.Context()).Env()
	cur := env.Settings.Get()
	s.port.SetText(strconv.Itoa(cur.Phone.Port))
	s.volume = float64(cur.Volume)
	list := phone.ListInterfaces
	if f := s.Widget().screens.Interfaces; f != nil {
		list = f
	}
	ifaces, err := list()
	if err != nil {
		env.Log.Error("listing the network interfaces failed", "err", err)
	}
	s.interfaces = ifaces
}

// save changes the settings in the background and shows a failure.
func (s *settingsState) save(change func(*settings.Settings)) {
	env, scope := app.ScopeOf(s.Context()).Env(), app.ScopeOf(s.Context())
	s.Go(func(ctx context.Context) {
		err := env.Settings.Update(ctx, change)
		if err != nil && ctx.Err() == nil {
			env.Log.Error("saving the settings failed", "err", err)
		}
		s.SetState(func() {
			if err != nil {
				msg := errMessage(err)
				s.err = &msg
				return
			}
			s.err = nil
			showMessage(s.Context(), scope.T("admin.settings.saved"))
		})
	})
}

// volumeRow is the volume slider with its value. The slider keeps no value of
// its own: it follows the drag in s.volume, and the setting is saved when the
// drag ends.
func (s *settingsState) volumeRow(scope *app.Scope) m.Widget {
	percent := itoa(int(s.volume)) + "%" // the same in every language
	return m.NewRow([]m.Widget{
		m.NewExpanded(m.NewSlider(s.volume, func(v float64) {
			s.SetState(func() { s.volume = v })
		}, m.SliderOpts{
			Key:         m.NewValueKey(KeyVolume),
			Min:         0,
			Max:         m.Some(float64(settings.MaxVolume)),
			Divisions:   m.Some(settings.MaxVolume / volumeStep),
			Label:       m.Some(percent),
			OnChangeEnd: s.saveVolume,
		})),
		hgap(app.Gap),
		m.NewSizedBox(m.NewText(percent, m.TextOpts{Key: m.NewValueKey(KeyVolumeValue), TextAlign: m.TextAlignEnd}), m.SizedBoxOpts{Width: m.Some(64.0)}),
	}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})
}

// saveVolume saves the volume when the drag ends. Moving the slider while the
// sound is muted unmutes it, like the volume control of the lesson player.
func (s *settingsState) saveVolume(v float64) {
	volume := int(math.Round(v))
	s.save(func(st *settings.Settings) { st.Volume, st.Muted = volume, false })
}

// savePort checks the port and saves it.
func (s *settingsState) savePort() {
	port, err := strconv.Atoi(strings.TrimSpace(s.port.Text()))
	if err != nil || port < minPort || port > maxPort {
		s.SetState(func() {
			s.err = &i18n.Message{Key: "admin.settings.port_invalid", Args: []string{"min", itoa(minPort), "max", itoa(maxPort)}}
		})
		return
	}
	s.save(func(st *settings.Settings) { st.Phone.Port = port })
}

func (s *settingsState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	cur := scope.Env().Settings.Get()
	var langs []m.Widget
	for _, l := range i18n.Langs() {
		langs = append(langs, m.NewChoiceChip(m.NewText(l.Label()), l == scope.Catalog().Lang(), m.ChoiceChipOpts{
			Key:        m.NewValueKey(app.KeyLanguagePrefix + string(l)),
			OnSelected: func(bool) { scope.SetLanguage(l) },
		}))
	}
	children := []m.Widget{
		s.section(bc, scope.T("admin.settings.language")),
		m.NewWrap(langs, m.WrapOpts{Spacing: 12}),
		gap(app.Gap * 1.5),
		s.section(bc, scope.T("admin.settings.import")),
		m.NewSwitchListTile(cur.StudentsCanImport, func(v bool) {
			s.save(func(st *settings.Settings) { st.StudentsCanImport = v })
		}, m.SwitchListTileOpts{
			Key:            m.NewValueKey(KeyImportSwitch),
			Title:          m.NewText(scope.T("admin.settings.import_switch")),
			Subtitle:       m.NewText(scope.T("admin.settings.import_hint")),
			ContentPadding: m.NewEdgeInsetsAll(0),
		}),
		gap(app.Gap * 1.5),
		s.section(bc, scope.T("admin.settings.volume")),
		body(bc, scope.T("admin.settings.volume_hint")),
		s.volumeRow(scope),
		m.NewSwitchListTile(cur.Muted, func(v bool) {
			s.save(func(st *settings.Settings) { st.Muted = v })
		}, m.SwitchListTileOpts{
			Key:            m.NewValueKey(KeyMuteSwitch),
			Title:          m.NewText(scope.T("admin.settings.mute_switch")),
			Subtitle:       m.NewText(scope.T("admin.settings.mute_hint")),
			ContentPadding: m.NewEdgeInsetsAll(0),
		}),
		gap(app.Gap * 1.5),
		s.section(bc, scope.T("admin.settings.phone")),
		body(bc, scope.T("admin.settings.phone_hint")),
		gap(app.Gap),
		m.NewRow([]m.Widget{
			m.NewExpanded(textField(KeyPort, s.port, scope.T("admin.settings.port"),
				scope.T("admin.settings.port_hint", "min", itoa(minPort), "max", itoa(maxPort)), false, func(string) { s.savePort() })),
			hgap(app.Gap),
			m.NewFilledButton(s.savePort, m.NewText(scope.T("admin.save")), m.FilledButtonOpts{Key: m.NewValueKey(KeyPortSave)}),
		}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart}),
		gap(app.Gap),
		m.NewText(scope.T("admin.settings.interface")),
		gap(4),
		s.interfaceDropdown(scope, cur.Phone.Interface),
	}
	if s.err != nil {
		children = append(children, gap(app.Gap), errorText(bc, scope.Catalog().Msg(*s.err)))
	}
	return m.NewScaffold(m.ScaffoldOpts{
		AppBar: appBar(scope, bc, scope.T("admin.settings")),
		Body:   pageBody(pageWidth, children),
	})
}

// section is the title of a group of settings.
func (s *settingsState) section(bc m.BuildContext, text string) m.Widget {
	return m.NewPadding(m.NewEdgeInsetsSymmetric(6, 0), m.NewText(text, m.TextOpts{
		Style: m.ThemeOf(bc).TextTheme.TitleLarge.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600}),
	}))
}

// interfaceDropdown is the choice of the network interface: Automatic (the empty
// string) or one of the interfaces of this computer. An interface that was
// chosen and is not there now stays in the list.
func (s *settingsState) interfaceDropdown(scope *app.Scope, current string) m.Widget {
	items := []*m.DropdownMenuItem[string]{
		m.NewDropdownMenuItem(m.NewText(scope.T("admin.settings.interface_auto")), m.DropdownMenuItemOpts[string]{Key: m.NewValueKey(KeyInterfaceAuto), Value: m.Some("")}),
	}
	seen := map[string]bool{"": true}
	for _, ifc := range s.interfaces {
		if ifc.Loopback || seen[ifc.Name] {
			continue
		}
		seen[ifc.Name] = true
		items = append(items, m.NewDropdownMenuItem(m.NewText(interfaceLabel(ifc)), m.DropdownMenuItemOpts[string]{Key: m.NewValueKey(KeyInterfacePrefix + ifc.Name), Value: m.Some(ifc.Name)}))
	}
	if !seen[current] {
		items = append(items, m.NewDropdownMenuItem(m.NewText(current), m.DropdownMenuItemOpts[string]{Key: m.NewValueKey(KeyInterfacePrefix + current), Value: m.Some(current)}))
	}
	return m.NewDropdownButton(items, func(v m.Opt[string]) {
		name := v.OrElse("")
		s.save(func(st *settings.Settings) { st.Phone.Interface = name })
	}, m.DropdownButtonOpts[string]{
		Key:        m.NewValueKey(KeyInterface),
		Value:      m.Some(current),
		IsExpanded: true,
	})
}

// interfaceLabel is the name of the interface with its IPv4 addresses.
func interfaceLabel(ifc phone.Interface) string {
	var addrs []string
	for _, ip := range ifc.Addrs {
		if v4 := ip.To4(); v4 != nil {
			addrs = append(addrs, v4.String())
		}
	}
	if len(addrs) == 0 {
		return ifc.Name
	}
	return ifc.Name + " (" + strings.Join(addrs, ", ") + ")"
}
