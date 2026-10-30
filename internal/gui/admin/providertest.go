package admin

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/rm4n0s/errors"
	m "github.com/rm4n0s/glutter/material"
	"github.com/rm4n0s/radio-tutor/internal/core/grading"
	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
	"github.com/rm4n0s/radio-tutor/internal/core/providers"
	"github.com/rm4n0s/radio-tutor/internal/gui/app"
)

// testDialog runs providers.Test for a provider and shows what came back
// (SPEC §11.6): whether a valid reply came, its verdict, the time, the model, a
// structured output downgrade, or the error in plain words with its tag. The
// test runs on a goroutine tied to the dialog, so closing the dialog cancels it.
type testDialog struct {
	m.StatefulWidget
	cfg providers.Config
}

func newTestDialog(cfg providers.Config) *testDialog {
	return &testDialog{StatefulWidget: m.NewStatefulWidgetBase(), cfg: cfg}
}

// CreateState creates the state.
func (w *testDialog) CreateState() m.State { return &testDialogState{} }

type testDialogState struct {
	m.StateOf[*testDialog]
	started bool
	done    bool
	result  providers.TestResult
	err     error // the test could not be made
}

// DidChangeDependencies starts the test (the Scope cannot be read in InitState).
func (s *testDialogState) DidChangeDependencies() {
	if s.started {
		return
	}
	s.started = true
	cfg, env := s.Widget().cfg, app.ScopeOf(s.Context()).Env()
	log, version := env.Log, env.Version
	s.Go(func(ctx context.Context) {
		res, err := providers.Test(ctx, cfg, version)
		if ctx.Err() != nil {
			return
		}
		if err != nil || !res.OK {
			e := err
			if e == nil {
				e = res.Err
			}
			log.Error("testing a provider failed", "provider", cfg, "err", e)
		}
		s.SetState(func() { s.done, s.result, s.err = true, res, err })
	})
}

func (s *testDialogState) Build(bc m.BuildContext) m.Widget {
	scope := app.ScopeOf(bc)
	cfg := s.Widget().cfg
	var content m.Widget
	if !s.done {
		content = m.NewRow([]m.Widget{
			m.NewSizedBox(m.NewCircularProgressIndicator(), m.SizedBoxOpts{Width: m.Some(36.0), Height: m.Some(36.0)}),
			hgap(app.Gap),
			m.NewExpanded(body(bc, scope.T("admin.test.running", "name", cfg.Name))),
		}, m.RowOpts{CrossAxisAlignment: m.CrossAxisAlignmentCenter})
	} else {
		content = m.NewColumn(s.resultLines(bc, scope), m.ColumnOpts{CrossAxisAlignment: m.CrossAxisAlignmentStart, MainAxisSize: m.MainAxisSizeMin})
	}
	return m.NewAlertDialog(m.AlertDialogOpts{
		Title:      m.NewText(scope.T("admin.test.title", "name", cfg.Name)),
		Content:    m.NewSizedBox(content, m.SizedBoxOpts{Width: m.Some(460.0)}),
		Scrollable: true,
		Actions:    []m.Widget{dialogButton(KeyDialogOK, scope.T("admin.close"), true, true)},
	})
}

// resultLines are the lines of the result of the test.
func (s *testDialogState) resultLines(bc m.BuildContext, scope *app.Scope) []m.Widget {
	text := func(key, msg string) m.Widget {
		return m.NewText(msg, m.TextOpts{Key: m.NewValueKey(key), Style: m.ThemeOf(bc).TextTheme.BodyLarge})
	}
	if s.err != nil || !s.result.OK {
		err := s.err
		if err == nil {
			err = s.result.Err
		}
		return []m.Widget{
			m.NewText(scope.Catalog().Msg(testFailure(err)), m.TextOpts{
				Key:   m.NewValueKey(KeyTestResult),
				Style: m.ThemeOf(bc).TextTheme.BodyLarge.CopyWith(m.TextStyleCopy{Color: m.Some(m.ThemeOf(bc).ColorScheme.Error())}),
			}),
		}
	}
	r := s.result
	lines := []m.Widget{
		m.NewText(scope.T("admin.test.ok"), m.TextOpts{
			Key:   m.NewValueKey(KeyTestResult),
			Style: m.ThemeOf(bc).TextTheme.TitleMedium.CopyWith(m.TextStyleCopy{FontWeight: m.FontWeightW600}),
		}),
		gap(8),
		text("test-verdict", scope.T("admin.test.verdict", "verdict", scope.T("admin.verdict."+string(r.Verdict)))),
		gap(4),
		text("test-time", scope.T("admin.test.time", "seconds", seconds(r.Duration))),
		gap(4),
		text("test-model", scope.T("admin.test.model", "model", r.Model)),
	}
	if r.Verdict != grading.VerdictCorrect {
		lines = append(lines, gap(8), text("test-warning", scope.T("admin.test.not_correct")))
	}
	if r.Downgraded != "" {
		lines = append(lines, gap(8), text("test-downgraded", scope.T("admin.test.downgraded",
			"configured", string(s.Widget().cfg.StructuredOutput), "mode", string(r.Downgraded))))
	}
	return lines
}

// seconds formats a duration as seconds with one decimal.
func seconds(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', 1, 64)
}

// testFailure turns the error of a test into a message in plain words that ends
// with the tag, for example "The server refused the API key. Check the key.
// (OpenAIAuth)".
func testFailure(err error) i18n.Message {
	appErr, ok := errors.FromError(err)
	if !ok {
		return i18n.M("admin.test.failed", "message", err.Error(), "tag", "Unknown")
	}
	key := ""
	switch tag := appErr.Tag; {
	case tag == "OpenAIAuth":
		key = "admin.test.err_auth"
	case tag == "OpenAINotFound":
		key = "admin.test.err_not_found"
	case tag == "OpenAIRateLimited":
		key = "admin.test.err_rate"
	case tag == "OpenAIServer":
		key = "admin.test.err_server"
	case tag == "OpenAITimeout":
		key = "admin.test.err_timeout"
	case tag == "OpenAINetwork":
		key = "admin.test.err_network"
	case tag == "OpenAIStatus":
		key = "admin.test.err_status"
	case tag == "OpenAICanceled" || tag == "GradingCanceled":
		key = "admin.test.err_canceled"
	case strings.HasPrefix(tag, "GradingReply"), tag == "OpenAIBodyInvalid", tag == "OpenAINoChoices", tag == "OpenAIContentInvalid", tag == "OpenAIBodyTooLarge":
		key = "admin.test.err_reply"
	}
	if key == "" {
		return i18n.M("admin.test.failed", "message", appErr.Message, "tag", appErr.Tag)
	}
	return i18n.M(key, "tag", appErr.Tag)
}
