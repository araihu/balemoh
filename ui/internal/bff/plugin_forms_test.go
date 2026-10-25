package bff

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/araihu/balemoh/ui/internal/view"
)

func TestPluginNumericOptionsRoundTripWithoutFloatConversion(t *testing.T) {
	for _, tc := range []struct{ kind, value string }{{"integer", "9007199254740993"}, {"number", "9007199254740993.125"}} {
		t.Run(tc.kind, func(t *testing.T) {
			original := []byte(`{"n":` + tc.value + `}`)
			values := view.PluginValues(original)
			form := view.PluginForm{Values: values}
			shown := form.Value("n")
			if shown != tc.value {
				t.Fatalf("form rounded %s to %s", tc.value, shown)
			}
			r := httptest.NewRequest("POST", "/", strings.NewReader(url.Values{"option.n": {shown}}.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			_ = r.ParseForm()
			raw, err := pluginFormOptions([]byte(`{"type":"object","properties":{"n":{"type":"`+tc.kind+`","minimum":0,"maximum":1000000000000000000}},"additionalProperties":false}`), r)
			if err != nil || string(raw) != string(original) {
				t.Fatalf("save rounded value: %s %v", raw, err)
			}
		})
	}
}
