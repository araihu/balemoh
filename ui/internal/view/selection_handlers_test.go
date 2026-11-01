package view

import (
	"bytes"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func TestSelectionCheckboxesKeepNativeSubmitHandlers(t *testing.T) {
	for _, test := range []struct {
		name      string
		component templ.Component
	}{
		{"service", ServiceSelection(Service{ID: "service", DisplayName: "Example"})},
		{"host", HostSelection("host", false, false, "1h", "", false, false)},
		{"plugin", PluginWidgetSelection(PluginWidgetRow{ID: "plugin", Title: "Example", SelectionURL: "/plugin-widgets/plugin/selection"}, false, false)},
	} {
		t.Run(test.name, func(t *testing.T) {
			var body bytes.Buffer
			if err := test.component.Render(t.Context(), &body); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(body.String(), `onchange="this.form.requestSubmit()"`) || strings.Contains(body.String(), templ.FailedSanitizationJS) {
				t.Fatal("checkbox change must submit its form through a trusted event handler")
			}
		})
	}
}
