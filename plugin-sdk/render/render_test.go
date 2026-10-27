package render_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/araihu/balemoh/plugin-sdk/render"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

func TestBoundedTemplWriter(t *testing.T) {
	for _, n := range []int{ui.MaxHTML, ui.MaxHTML + 1} {
		c := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
			_, err := io.WriteString(w, strings.Repeat("x", n))
			return err
		})
		r, err := render.Component(t.Context(), "Fixture", c)
		if n > ui.MaxHTML && err == nil {
			t.Fatal("HTML limit bypassed through io.StringWriter")
		}
		if n == ui.MaxHTML && (err != nil || len(r.HTML) != n) {
			t.Fatalf("boundary rejected: %v", err)
		}
	}
}
