package phone

import (
	"html/template"
	"net/http/httptest"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/rm4n0s/radio-tutor/internal/core/i18n"
)

func TestPromptLines(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{" \n ", nil},
		{"one", []string{"one"}},
		{"one\ntwo", []string{"one", "two"}},
		{"one\r\ntwo\rthree", []string{"one", "two", "three"}},
		{"\none\n\ntwo\n", []string{"one", "", "two"}},
	}
	for _, tc := range tests {
		if got := promptLines(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("promptLines(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestReadStaticMissingAsset(t *testing.T) {
	_, err := readStatic(fstest.MapFS{})
	wantRoute(t, err, "readStatic.PhoneAssetMissing")
	if _, err := readStatic(assets); err != nil {
		t.Errorf("the embedded assets: %v", err)
	}
}

func TestRenderFailures(t *testing.T) {
	empty := template.New("empty")
	err := renderPage(empty, httptest.NewRecorder(), pageView{})
	wantRoute(t, err, "renderPage.PhonePageRenderFailed")
	err = renderMessage(empty, httptest.NewRecorder(), 404, i18n.EN, "phone.not_found.title", "phone.not_found.body")
	wantRoute(t, err, "renderMessage.PhoneMessageRenderFailed")
}

func TestPageViewUsesPhoneTexts(t *testing.T) {
	v := newPageView(PageData{Lang: i18n.EL, Title: "T"})
	if v.Lang != "el" || v.Text["phone.take"] == "" {
		t.Errorf("view = %+v", v)
	}
	for key := range v.Text {
		if len(key) < 6 || key[:6] != "phone." {
			t.Errorf("key %q is not a phone key", key)
		}
	}
}
