package manifest

import "testing"

func TestFormSchemaSubset(t *testing.T) {
	for _, tt := range []struct {
		schema string
		valid  bool
	}{
		{`{"type":"object","properties":{"title":{"type":"string","maxLength":64},"count":{"type":"integer","minimum":1,"maximum":20},"enabled":{"type":"boolean"},"states":{"type":"array","items":{"type":"string","enum":["active","stale"]}}},"additionalProperties":false}`, true},
		{`{"type":"object","properties":{"nested":{"type":"object"}},"additionalProperties":false}`, false},
		{`{"type":"object","properties":{"url":{"type":"string","format":"uri"}},"additionalProperties":false}`, false},
		{`{"type":"object","additionalProperties":true}`, false},
	} {
		_, e := FormFields([]byte(tt.schema))
		if (e == nil) != tt.valid {
			t.Fatalf("valid %v: %v", tt.valid, e)
		}
	}
}
