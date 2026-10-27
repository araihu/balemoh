package http_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adapter "github.com/araihu/balemoh/internal/adapters/http"
	"github.com/araihu/balemoh/internal/adapters/sqlite"
	"github.com/araihu/balemoh/internal/api/generated"
	p "github.com/araihu/balemoh/internal/application/presentation"
	"github.com/araihu/goshtoso/themes"
	"golang.org/x/image/font/gofont/goregular"
)

func presentationHandler(t *testing.T) http.Handler {
	t.Helper()
	db, e := sqlite.Open(t.Context(), t.TempDir()+"/db.sqlite")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = db.Close() })
	if e = sqlite.RunMigrations(db); e != nil {
		t.Fatal(e)
	}
	h := adapter.NewHandler(fakeChecker{}, &fakeCatalog{}).WithPresentation(sqlite.NewPresentationStore(db))
	return generated.HandlerWithOptions(h, generated.StdHTTPServerOptions{BaseRouter: http.NewServeMux(), ErrorHandlerFunc: adapter.RequestError})
}
func requestJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest(method, path, bytes.NewReader(b)))
	return r
}
func TestPresentationHTTPAtomicSaveAndEnvelope(t *testing.T) {
	h := presentationHandler(t)
	state := requestJSON(t, h, "GET", "/api/v1/presentation", nil)
	if state.Code != 200 {
		t.Fatal(state.Code, state.Body)
	}
	var initial p.State
	if json.Unmarshal(state.Body.Bytes(), &initial) != nil {
		t.Fatal("invalid state")
	}
	d, _ := themes.Preset(themes.KeyDracula)
	cmd := p.Save{CommandID: "11111111-1111-4111-8111-111111111111", ExpectedSettingsRevision: "0", ColorMode: "light", Selection: p.Selection{Kind: "newCustom", ID: "theme:11111111-1111-4111-8111-111111111111", Name: "Dracula copy", Definition: &d, ExpectedCatalogRevision: initial.CatalogRevision}}
	saved := requestJSON(t, h, "PUT", "/api/v1/presentation", cmd)
	if saved.Code != 200 {
		t.Fatal(saved.Code, saved.Body)
	}
	blocked := requestJSON(t, h, "DELETE", "/api/v1/themes/"+cmd.Selection.ID+"?expectedRevision=1", nil)
	if blocked.Code != 409 || !strings.Contains(blocked.Body.String(), `"code":"in_use"`) {
		t.Fatal("active delete", blocked.Code, blocked.Body)
	}
	same := requestJSON(t, h, "PUT", "/api/v1/presentation", cmd)
	if same.Code != 200 || same.Body.String() != saved.Body.String() {
		t.Fatal("replay", same.Code, same.Body)
	}
	cmd.CommandID = "22222222-2222-4222-8222-222222222222"
	conflict := requestJSON(t, h, "PUT", "/api/v1/presentation", cmd)
	if conflict.Code != 409 || !strings.Contains(conflict.Body.String(), `"kind":"settings"`) {
		t.Fatal("stale save", conflict.Code, conflict.Body)
	}
	missing := requestJSON(t, h, "DELETE", "/api/v1/themes/"+cmd.Selection.ID, nil)
	if missing.Code != 400 || !strings.Contains(missing.Body.String(), `"code":"invalid_request"`) {
		t.Fatal("missing revision envelope", missing.Code, missing.Body)
	}
	raw := httptest.NewRequest("PUT", "/api/v1/presentation", strings.NewReader(`{"unexpected":true}`))
	invalid := httptest.NewRecorder()
	h.ServeHTTP(invalid, raw)
	if invalid.Code != 400 {
		t.Fatal(invalid.Code)
	}
	definition, _ := json.Marshal(d)
	definition = bytes.Replace(definition, []byte(`"channels":[`), []byte(`"channels":[0,`), 1)
	if _, e := themes.Decode(definition); e == nil {
		t.Fatal("extra channel accepted")
	}
}
func TestFontHTTPUploadReplayCacheAndRead(t *testing.T) {
	h := presentationHandler(t)
	id := "font:11111111-1111-4111-8111-111111111111"
	upload := func(data []byte) *httptest.ResponseRecorder {
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		_ = w.WriteField("id", id)
		_ = w.WriteField("name", "My Go font")
		part, _ := w.CreateFormFile("file", "untrusted-extension.otf")
		_, _ = part.Write(data)
		_ = w.Close()
		r := httptest.NewRequest("POST", "/api/v1/fonts", &body)
		r.Header.Set("Content-Type", w.FormDataContentType())
		result := httptest.NewRecorder()
		h.ServeHTTP(result, r)
		return result
	}
	first := upload(goregular.TTF)
	if first.Code != 201 {
		t.Fatal(first.Code, first.Body)
	}
	again := upload(goregular.TTF)
	if again.Code != 200 {
		t.Fatal(again.Code, again.Body)
	}
	var font p.Font
	_ = json.Unmarshal(first.Body.Bytes(), &font)
	if font.Format != "truetype" || font.Weight != "400" {
		t.Fatal(font)
	}
	r := httptest.NewRequest("GET", "/api/v1/fonts/"+id+"/file", nil)
	result := httptest.NewRecorder()
	h.ServeHTTP(result, r)
	bytes_, _ := io.ReadAll(result.Result().Body)
	if result.Code != 200 || !bytes.Equal(bytes_, goregular.TTF) || result.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("font delivery", result.Code)
	}
	r = httptest.NewRequest("GET", "/api/v1/fonts/"+id+"/file", nil)
	r.Header.Set("If-None-Match", result.Header().Get("ETag"))
	cached := httptest.NewRecorder()
	h.ServeHTTP(cached, r)
	if cached.Code != 304 {
		t.Fatal("font cache", cached.Code)
	}
	d, _ := themes.Preset(themes.KeyGoshtoso)
	d.Typography.Title = themes.FontRef{Source: "upload", ID: id}
	saved := requestJSON(t, h, "PUT", "/api/v1/presentation", p.Save{CommandID: "11111111-1111-4111-8111-111111111111", ExpectedSettingsRevision: "0", ColorMode: "system", Selection: p.Selection{Kind: "newCustom", ID: "theme:11111111-1111-4111-8111-111111111111", Name: "Referenced font", Definition: &d, ExpectedCatalogRevision: themes.CatalogRevision()}})
	if saved.Code != 200 {
		t.Fatal(saved.Code, saved.Body)
	}
	blocked := requestJSON(t, h, "DELETE", "/api/v1/fonts/"+id+"?expectedRevision=1", nil)
	if blocked.Code != 409 || !strings.Contains(blocked.Body.String(), `"usedBy":["Referenced font [theme:11111111-1111-4111-8111-111111111111]"`) {
		t.Fatal("referenced font delete", blocked.Code, blocked.Body)
	}
	h = presentationHandler(t)
	padded := make([]byte, p.MaxFontBytes)
	copy(padded, goregular.TTF)
	boundary := upload(padded)
	if boundary.Code != 201 {
		t.Fatal("exact 10 MiB boundary", boundary.Code, boundary.Body)
	}
	oversized := upload(append(padded, 0))
	if oversized.Code != 413 {
		t.Fatal("10 MiB plus one", oversized.Code, oversized.Body)
	}
	bad := upload([]byte("ttcf0000"))
	if bad.Code != 415 {
		t.Fatal("unsupported", bad.Code, bad.Body)
	}
}
