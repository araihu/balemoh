package bff

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/araihu/balemoh/ui/internal/view"
)

func TestIconUploadErrorKeepsDrawerDraft(t *testing.T) {
	r := httptest.NewRequest("POST", "/icons/upload", nil)
	r.Header.Set("HX-Request", "true")
	r.Header.Set("HX-Target", "icon-upload-body")
	w := httptest.NewRecorder()
	(&server{}).iconUploadError(w, r, view.LibraryIcon{Name: "My icon", Tags: "custom,home"}, "Reselect your file to retry.")
	for _, want := range []string{`value="My icon"`, `value="custom,home"`, `role="alert"`, "Reselect your file to retry.", `hx-post="/icons/upload"`, `hx-encoding="multipart/form-data"`, "Cancel"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("missing %q in recovery fragment", want)
		}
	}
	if strings.Contains(w.Body.String(), "<html") || strings.Contains(w.Body.String(), "icon-picker-results") {
		t.Fatal("upload error replaced drawer with full library")
	}
}
