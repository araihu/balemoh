package view

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/araihu/goshtoso/themes"
)

func TestPresentationDraftAndModalRuntime(t *testing.T) {
	node, e := exec.LookPath("node")
	if e != nil {
		t.Skip("Node unavailable")
	}
	cmd := exec.CommandContext(t.Context(), node, "--test", "static/presentation.test.mjs", "static/presentation-fonts.test.mjs", "static/theme-selection.test.mjs", "static/appearance.test.mjs", "static/theme-preview.test.mjs")
	if output, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("presentation runtime: %v\n%s", e, output)
	}
}

func TestAppearanceHeadEmitsValidatedCSS(t *testing.T) {
	d, _ := themes.Preset(themes.KeyGoshtoso)
	css, e := themes.RenderCSS(d, nil)
	if e != nil {
		t.Fatal(e)
	}
	var output bytes.Buffer
	if e = AppearanceHead(Appearance{CSS: css}).Render(t.Context(), &output); e != nil {
		t.Fatal(e)
	}
	html := output.String()
	if !strings.Contains(html, css) || strings.Contains(html, "@templ.Raw(") {
		t.Fatal("appearance stylesheet was not rendered")
	}
}

func TestGlobalModeUsesThreeNativeIconRadios(t *testing.T) {
	d, _ := themes.Preset(themes.KeyGoshtoso)
	page := ThemesPage{Available: true, Initial: d, Fonts: themes.DefaultFonts(), ColorMode: "dark", SelectedID: "builtin:goshtoso", Items: []ThemeItem{{ID: "builtin:goshtoso", Name: "Goshtoso", Definition: d}}}
	var output bytes.Buffer
	if e := ThemesContent(page).Render(t.Context(), &output); e != nil {
		t.Fatal(e)
	}
	html := output.String()
	if strings.Count(html, `name="colorMode"`) != 3 || strings.Contains(html, `presentation-color-mode-trigger`) {
		t.Fatal("global mode must be a three-way native radio group")
	}
	for _, value := range []string{"system", "light", "dark"} {
		if !strings.Contains(html, `id="presentation-mode-`+value+`"`) {
			t.Fatal("missing mode", value)
		}
	}
}

func TestThemeSelectionAndEditorAreSeparateSurfaces(t *testing.T) {
	d, _ := themes.Preset(themes.KeyGoshtoso)
	page := ThemesPage{Available: true, Initial: d, Fonts: themes.DefaultFonts(), SelectedID: "builtin:goshtoso", ColorMode: "system", Creating: true, Items: []ThemeItem{{ID: "builtin:goshtoso", Name: "Goshtoso", Definition: d}}}
	var output bytes.Buffer
	if err := ThemesContent(page).Render(t.Context(), &output); err != nil {
		t.Fatal(err)
	}
	html := output.String()

	assertRequiresJavaScript := func(marker string) {
		t.Helper()
		button := regexp.MustCompile(`<button[^>]*` + marker + `[^>]*>`).FindString(html)
		if button == "" || !strings.Contains(button, " disabled") || !strings.Contains(html, "<noscript>") || !strings.Contains(html, `tabindex="-1"`) {
			t.Fatal("theme saving must start disabled with no-script explanation and focusable feedback", marker)
		}
	}
	assertRequiresJavaScript("data-theme-apply-button")
	for _, forbidden := range []string{"data-theme-editor=", "data-presentation-upload", "data-presentation-name", "Reset draft", "Manage custom themes"} {
		if strings.Contains(html, forbidden) {
			t.Fatal("editor control on theme selection", forbidden)
		}
	}
	output.Reset()
	if err := ThemeEditorContent(page).Render(t.Context(), &output); err != nil {
		t.Fatal(err)
	}
	html = output.String()
	assertRequiresJavaScript("data-presentation-save")
	for _, wanted := range []string{"Create theme", "Base theme", "data-theme-editor=", "Back to Themes"} {
		if !strings.Contains(html, wanted) {
			t.Fatal("editor missing", wanted)
		}
	}
	if strings.Contains(html, "data-presentation-upload") || strings.Contains(html, "upload-font-file") || strings.Contains(html, "data-theme-selection=") || strings.Contains(html, "Apply theme</button>") {
		t.Fatal("selection surface leaked into editor")
	}
}

func TestCustomBrandHeadAndFixedProductFooter(t *testing.T) {
	var output bytes.Buffer
	a := Appearance{BrandIconID: "upload:MYLOGO"}
	if err := BrandHead(a).Render(t.Context(), &output); err != nil {
		t.Fatal(err)
	}
	html := output.String()
	if !strings.Contains(html, IconImageURL(a.BrandIconID)) || strings.Contains(html, "/ui/brand/favicon-32.png") || strings.Contains(html, "/ui/brand/manifest-icons.json") {
		t.Fatal("default icon overrides custom branding", html)
	}
	output.Reset()
	if err := ProductFooter().Render(t.Context(), &output); err != nil {
		t.Fatal(err)
	}
	html = output.String()
	for _, expected := range []string{"/ui/brand/logo-light.svg", "/ui/brand/logo-dark.svg", "is made by", "https://araihu.com/", "Arai Hû", "with", "https://goshtoso.araihu.com/", "Goshtoso", "selfh.st/icons"} {
		if !strings.Contains(html, expected) {
			t.Fatal("fixed product credit missing", expected)
		}
	}
	if strings.Contains(html, a.BrandIconID) {
		t.Fatal("theme replaces product credit")
	}
}

func TestThemeActionGroupsAndApplyOrder(t *testing.T) {
	d, _ := themes.Preset(themes.KeyGoshtoso)
	page := ThemesPage{Available: true, Initial: d, SelectedID: "theme:one", Fonts: themes.DefaultFonts(), Items: []ThemeItem{{ID: "builtin:goshtoso", Name: "Goshtoso", Definition: d}, {ID: "theme:one", Name: "Custom", Revision: "1", Definition: d}}}
	var output bytes.Buffer
	if err := ThemesContent(page).Render(t.Context(), &output); err != nil {
		t.Fatal(err)
	}
	html := output.String()
	for _, marker := range []string{"data-goshtoso-action-group", `id="theme-primary-edit"`, `id="theme-primary-create"`, `id="theme-secondary-create"`} {
		if !strings.Contains(html, marker) {
			t.Fatal("missing theme action", marker)
		}
	}
	if strings.LastIndex(html, "data-theme-apply-button") < strings.LastIndex(html, "data-theme-selection-reload") || strings.LastIndex(html, "data-theme-apply-button") < strings.LastIndex(html, "data-theme-playground") {
		t.Fatal("Apply must follow preview, feedback and reload")
	}
}

func TestFontsAreManagedInTheirOwnAppearanceTab(t *testing.T) {
	d := ThemesPage{Available: true, Uploads: []FontItem{{ID: "font:one", Name: "Local font", Weight: "400", Style: "normal", Revision: "1", UsedBy: []string{"Ocean"}}}}
	var output bytes.Buffer
	if err := PresentationContent(IconPage{}, d).Render(t.Context(), &output); err != nil {
		t.Fatal(err)
	}
	html := output.String()
	for _, want := range []string{"Appearance", `id="tabpanelpresentation-tabsfonts"`, "data-presentation-fonts", `name="file"`, "data-font-delete", "Used by Ocean", "presentation-font-confirm"} {
		if !strings.Contains(html, want) {
			t.Fatal("Fonts tab missing", want)
		}
	}
}

func TestThemeDeletionOnlyAppearsForSavedEditorThemes(t *testing.T) {
	d, _ := themes.Preset(themes.KeyGoshtoso)
	for _, tc := range []struct {
		name, id                        string
		creating, available, wantDelete bool
	}{
		{"saved", "theme:one", false, true, true}, {"new copy", "theme:one", true, true, false}, {"preset", "builtin:goshtoso", true, true, false}, {"unavailable", "theme:one", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := ThemesPage{Available: tc.available, Creating: tc.creating, SelectedID: tc.id, Initial: d, Fonts: themes.DefaultFonts(), Items: []ThemeItem{{ID: "theme:one", Name: "Saved", Revision: "4", Definition: d}, {ID: "builtin:goshtoso", Name: "Goshtoso", Definition: d}}}
			var output bytes.Buffer
			if err := ThemeEditorContent(page).Render(t.Context(), &output); err != nil {
				t.Fatal(err)
			}
			html := output.String()
			if strings.Contains(html, `id="theme-primary-delete"`) != tc.wantDelete {
				t.Fatal("wrong delete action visibility")
			}
			if strings.Contains(html, "Manage custom themes") || strings.Contains(html, "data-presentation-management") {
				t.Fatal("duplicate theme management remains")
			}
			if tc.wantDelete && strings.Index(html, `id="theme-primary-delete"`) > strings.Index(html, `id="presentation-save"`) {
				t.Fatal("delete action must be in the header")
			}
		})
	}
}
