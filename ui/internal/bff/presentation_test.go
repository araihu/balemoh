package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/araihu/balemoh/ui/internal/view"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	pluginui "github.com/araihu/balemoh/plugin-sdk/ui"
	"github.com/araihu/goshtoso/themes"
)

func TestPresentationSDKSavePreservesCommandAndConflict(t *testing.T) {
	definition, _ := themes.Preset(themes.KeyGoshtoso)
	state := api.PresentationState{Settings: api.PresentationSettings{Revision: "7", ColorMode: api.PresentationSettingsColorModeSystem, Selection: api.PresentationSelection{Kind: "builtin", Key: stringPtr("goshtoso")}}, CatalogRevision: themes.CatalogRevision(), Definition: definition}
	var forwarded []api.PresentationSave
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/presentation":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(state)
				return
			}
			var command api.PresentationSave
			if e := json.NewDecoder(r.Body).Decode(&command); e != nil {
				t.Error(e)
			}
			forwarded = append(forwarded, command)
			w.WriteHeader(409)
			_, _ = io.WriteString(w, `{"code":"conflict","message":"Saved settings changed.","conflict":{"kind":"settings","currentRevision":"8"}}`)
		case "/api/v1/themes":
			_ = json.NewEncoder(w).Encode(api.ThemeCatalog{CatalogRevision: themes.CatalogRevision(), Builtins: []api.BuiltinTheme{{Key: "goshtoso", Label: "Goshtoso", Definition: definition}}, Custom: []api.CustomTheme{}})
		case "/api/v1/fonts", "/api/v1/icons":
			_, _ = io.WriteString(w, "[]")
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	client, e := NewAPIClient(upstream.URL, upstream.Client())
	if e != nil {
		t.Fatal(e)
	}
	handler, e := New(client, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest("GET", "/presentation?appearance=recovery", nil))
	if page.Code != 200 || !strings.Contains(page.Body.String(), "Temporary Goshtoso/System recovery mode") || !strings.Contains(page.Body.String(), `data-theme-selection=`) || strings.Contains(page.Body.String(), `data-theme-editor=`) {
		t.Fatal("recovery/theme selection unavailable", page.Code)
	}
	for _, path := range []string{"/presentation/themes/editor?appearance=recovery&themeID=builtin:goshtoso&new=1", "/presentation/themes/editor?themeID=builtin:goshtoso"} {
		editor := httptest.NewRecorder()
		handler.ServeHTTP(editor, httptest.NewRequest("GET", path, nil))
		if editor.Code != 200 || !strings.Contains(editor.Body.String(), `data-theme-editor=`) || !strings.Contains(editor.Body.String(), `id="presentation-confirm"`) || strings.Contains(editor.Body.String(), `data-theme-selection=`) {
			t.Fatal("dedicated editor unavailable", path, editor.Code)
		}
		if strings.Contains(path, "appearance=recovery") && !strings.Contains(editor.Body.String(), "Temporary Goshtoso/System recovery mode") {
			t.Fatal("editor lost recovery mode")
		}
		assertSocialMetadata(t, editor.Body.String(), "https://balemoh.decastro.me/presentation/themes/editor")
	}
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest("GET", "/presentation/themes/editor?themeID=missing", nil))
	if missing.Code != 404 {
		t.Fatal("unknown editor target", missing.Code)
	}
	preview := httptest.NewRecorder()
	handler.ServeHTTP(preview, httptest.NewRequest("GET", "/presentation/preview", nil))
	if preview.Code != 200 || preview.Header().Get("Content-Security-Policy") != "form-action 'none'; base-uri 'none'" {
		t.Fatal("preview response", preview.Code, preview.Header())
	}
	for _, want := range []string{"/ui/balemoh.css", "/ui/theme-preview.js", "Jellyfin", "Grafana", "nas-01", "24.6%", "42.8%", "balemoh-host-widget", "balemoh-service-card", "data-theme-preview", "Your workspace", "Add service", "More actions", "preview-workspace", "Service settings", "preview-service-group", "preview-notifications", "Activity and feedback", "Connection failed", `id="preview-dialog"`, `aria-invalid="true"`} {
		if !strings.Contains(preview.Body.String(), want) {
			t.Fatal("missing real preview card", want)
		}
	}
	if strings.Contains(preview.Body.String(), "hx-sse:connect") || strings.Contains(preview.Body.String(), "/dashboard/events") {
		t.Fatal("preview subscribes to live data")
	}
	for _, want := range []string{"Appearance · Balemoh", "/ui/presentation-fonts.js", "data-presentation-fonts"} {
		if !strings.Contains(page.Body.String(), want) {
			t.Fatal("appearance missing", want)
		}
	}
	body := `{"commandId":"11111111-1111-4111-8111-111111111111","expectedSettingsRevision":"7","colorMode":"light","selection":{"kind":"builtin","key":"goshtoso","expectedCatalogRevision":"` + themes.CatalogRevision() + `"}}`
	for i := 0; i < 2; i++ {
		response := httptest.NewRecorder()
		request := httptest.NewRequest("POST", "/presentation/save", strings.NewReader(body))
		request.Header.Set("Accept", "application/json")
		handler.ServeHTTP(response, request)
		if response.Code != 409 || !strings.Contains(response.Body.String(), `"currentRevision":"8"`) || response.Header().Get("Location") != "" {
			t.Fatal("conflict reloads or loses details", response.Code, response.Body)
		}
	}
	if len(forwarded) != 2 || forwarded[0].CommandId != forwarded[1].CommandId || forwarded[0].ExpectedSettingsRevision != "7" {
		t.Fatal("retry changed identity or revision", forwarded)
	}
}

type fontRoundTripper struct{ t *testing.T }

func (r fontRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	deadline, ok := request.Context().Deadline()
	if !ok || time.Until(deadline) < 25*time.Second {
		r.t.Error("font upload inherited normal request timeout")
	}
	return &http.Response{StatusCode: 201, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"font:11111111-1111-4111-8111-111111111111","name":"Example","family":"Go","weight":"400","style":"normal","variable":false,"format":"truetype","mime":"font/ttf","byteLength":1,"digest":"example","revision":"1","usedBy":[]}`))}, nil
}
func TestFontUploadUsesItsOwnThirtySecondClient(t *testing.T) {
	client, e := NewAPIClient("http://api.example", &http.Client{Transport: fontRoundTripper{t}, Timeout: time.Millisecond})
	if e != nil {
		t.Fatal(e)
	}
	_, e = client.UploadFont(context.Background(), "multipart/form-data; boundary=example", strings.NewReader("example"))
	if e != nil {
		t.Fatal(e)
	}
}

func TestAppearanceUsesIndependentLogoAndIcon(t *testing.T) {
	d, _ := themes.Preset(themes.KeyGoshtoso)
	state := api.PresentationState{BrandLogoId: stringPtr("upload:LOGO"), BrandIconId: stringPtr("upload:ICON"), Definition: d, Settings: api.PresentationSettings{ColorMode: "light", Selection: api.PresentationSelection{Kind: "custom"}}}
	appearance := appearanceFrom(state, themes.DefaultFonts(), false)
	if appearance.BrandLogoID != "upload:LOGO" || appearance.BrandIconID != "upload:ICON" {
		t.Fatal("branding mapping", appearance)
	}
	var logo, head bytes.Buffer
	if err := view.SiteLogo(appearance.BrandLogoID).Render(t.Context(), &logo); err != nil {
		t.Fatal(err)
	}
	if err := view.BrandHead(appearance).Render(t.Context(), &head); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logo.String(), "upload:LOGO") || strings.Contains(logo.String(), "upload:ICON") || !strings.Contains(head.String(), "upload:ICON") || strings.Contains(head.String(), "upload:LOGO") {
		t.Fatal("logo/icon mixed", logo.String(), head.String())
	}
	recovery := appearanceFrom(state, themes.DefaultFonts(), true)
	if recovery.BrandLogoID != "" || recovery.BrandIconID != "" {
		t.Fatal("recovery kept custom branding", recovery)
	}
}

func TestPluginContentUsesSavedGlobalAppearance(t *testing.T) {
	definition, _ := themes.Preset(themes.KeyMinimal)
	definition.Tokens["primary"], _ = themes.ParseColor("#165d9b")
	state := api.PresentationState{Settings: api.PresentationSettings{Revision: "1", ColorMode: api.PresentationSettingsColorModeDark, Selection: api.PresentationSelection{Kind: "custom", Id: stringPtr("theme:fixture")}}, Definition: definition}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/presentation" {
			_ = json.NewEncoder(w).Encode(state)
			return
		}
		if r.URL.Path == "/api/v1/fonts" {
			_, _ = io.WriteString(w, "[]")
			return
		}
		http.NotFound(w, r)
	}))
	defer upstream.Close()
	client, err := NewAPIClient(upstream.URL, upstream.Client())
	if err != nil {
		t.Fatal(err)
	}
	server := &server{presentation: client, timeout: time.Second, plugins: previewRegistry(t, manifest.UIProfile)}
	request := httptest.NewRequest("GET", "/plugins/example.fixture/connections/00000000-0000-4000-8000-000000000001/page-content/current?__native=1", nil)
	request.SetPathValue("pluginID", "example.fixture")
	request.SetPathValue("connectionID", "00000000-0000-4000-8000-000000000001")
	recorder := httptest.NewRecorder()
	server.withPluginAppearance(func(w http.ResponseWriter, r *http.Request) {
		display := pluginui.Display(r.Context())
		if display.Theme != "custom" || display.ColorScheme != "dark" {
			t.Fatal("plugin metadata lost saved appearance", display)
		}
		server.pluginContent(w, r, pluginui.Response{Kind: "content", Status: 200, HTML: []byte("<p>Plugin fixture</p>")}, nil)
	})(recorder, request)
	css, err := themes.RenderCSS(definition, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`data-theme="custom"`, `class="dark"`, `id="balemoh-appearance"`, css} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("native plugin lost saved appearance %s", want)
		}
	}
}
