package phone

import (
	"html/template"
	"net/http/httptest"
	"reflect"
	"strings"
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

func TestPhoneTextUsesPhoneKeys(t *testing.T) {
	text := phoneText(i18n.EL)
	if text["phone.take"] == "" || text["phone.waiting"] == "" {
		t.Errorf("text = %v", text)
	}
	for key := range text {
		if !strings.HasPrefix(key, "phone.") {
			t.Errorf("key %q is not a phone key", key)
		}
	}
}
