package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	api "github.com/araihu/balemoh/client"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh/ui/internal/view"
)

func TestIconUploadErrorKeepsDrawerDraft(t *testing.T) {
	r := httptest.NewRequest("POST", "/icons/upload", nil)
	r.Header.Set("HX-Request-Type", "partial")
	r.Header.Set("HX-Target", "div#icon-upload-body")
	w := httptest.NewRecorder()
	(&server{}).iconUploadError(w, r, view.LibraryIcon{Name: "My icon", Tags: "custom,home"}, "Reselect your file to retry.")
	for _, want := range []string{`value="My icon"`, `custom`, `home`, `data-tagslist`, `role="alert"`, "Reselect your file to retry.", `hx-post="/icons/upload"`, `hx-encoding="multipart/form-data"`, "Cancel"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("missing %q in recovery fragment", want)
		}
	}
	if strings.Contains(w.Body.String(), "<html") || strings.Contains(w.Body.String(), "icon-picker-results") {
		t.Fatal("upload error replaced drawer with full library")
	}
}

type modalUploadCatalog struct {
	IconCatalog
	failure bool
}

func (c modalUploadCatalog) SaveIcon(_ context.Context, _ string, upload api.IconUpload) (api.UploadedIcon, error) {
	if c.failure {
		return api.UploadedIcon{}, errors.New("invalid image")
	}
	if upload.Tags != "home,custom" {
		panic("tags not serialized")
	}
	return api.UploadedIcon{Id: "upload:test", Name: upload.Name}, nil
}
func TestServiceIconUploadModal(t *testing.T) {
	for _, failure := range []bool{false, true} {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		_ = form.WriteField("name", "My draft icon")
		_ = form.WriteField("tags[0]", "home")
		_ = form.WriteField("tags[1]", "custom")
		_ = form.Close()
		r := httptest.NewRequest("POST", "/icons/upload", &body)
		r.Header.Set("Content-Type", form.FormDataContentType())
		r.Header.Set("HX-Request-Type", "partial")
		r.Header.Set("HX-Target", "div#service-icon-upload-body")
		w := httptest.NewRecorder()
		(&server{icons: modalUploadCatalog{failure: failure}, timeout: time.Second}).uploadIcon(w, r)
		if w.Code != 200 || w.Header().Get("HX-Redirect") != "" {
			t.Fatal("modal upload must remain in editor")
		}
		if !strings.Contains(w.Body.String(), `hx-target="#service-icon-upload-body"`) {
			t.Fatal("modal form lost target")
		}
		if failure {
			for _, want := range []string{`value="My draft icon"`, `home`, `custom`, `data-tagslist`, `role="alert"`} {
				if !strings.Contains(w.Body.String(), want) {
					t.Errorf("missing %s", want)
				}
			}
			if w.Header().Get("HX-Trigger") != "" {
				t.Fatal("failed upload must not select or close")
			}
		} else {
			var events map[string]map[string]string
			if err := json.Unmarshal([]byte(w.Header().Get("HX-Trigger")), &events); err != nil {
				t.Fatal(err)
			}
			if events["icon-selected"]["id"] != "upload:test" || events["modal:close"]["id"] != "service-icon-upload" {
				t.Fatal(events)
			}
		}
	}
}
