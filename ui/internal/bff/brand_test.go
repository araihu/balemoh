package bff

import (
	"bytes"
	"image/png"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBrandAssetsAreServedWithoutCatalogDependency(t *testing.T) {
	handler, err := New(&fakeCatalog{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, contentType string }{
		{"logo-light.svg", "image/svg+xml"},
		{"logo-dark.svg", "image/svg+xml"},
		{"favicon.svg", "image/svg+xml"},
		{"favicon-32.png", "image/png"},
		{"apple-touch-icon-180.png", "image/png"},
		{"icon-maskable-512.png", "image/png"},
		{"manifest-icons.json", "application/json"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				response := request(t, handler, method, "/ui/brand/"+tc.path, "")
				if response.Code != http.StatusOK {
					t.Fatalf("%s: status %d", method, response.Code)
				}
				if !strings.HasPrefix(response.Header().Get("Content-Type"), tc.contentType) {
					t.Errorf("%s: content type %s", method, response.Header().Get("Content-Type"))
				}
				if method == http.MethodHead && response.Body.Len() != 0 {
					t.Error("HEAD returned asset body")
				}
			}
		})
	}
	response := request(t, handler, http.MethodGet, "/ui/brand/missing.svg", "")
	if response.Code != http.StatusNotFound {
		t.Errorf("unknown asset status %d, want 404", response.Code)
	}
}

func TestPagesExposeBrandAndUpdatedSocialPreview(t *testing.T) {
	handler, err := New(&fakeCatalog{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/staging", "/hosts", "/attributions"} {
		t.Run(path, func(t *testing.T) {
			response := request(t, handler, http.MethodGet, path, "")
			assertSocialMetadata(t, response.Body.String(), "https://balemoh.decastro.me"+path)
			for _, expected := range []string{"/ui/brand/logo-light.svg", "/ui/brand/logo-dark.svg", "/ui/brand/favicon.svg", "/ui/brand/apple-touch-icon-180.png", "/ui/brand/manifest-icons.json", "https://balemoh.decastro.me/ui/social-v2.png"} {
				if !strings.Contains(response.Body.String(), expected) {
					t.Errorf("missing %s", expected)
				}
			}
		})
	}
	response := request(t, handler, http.MethodGet, "/ui/social-v2.png", "")
	info, err := png.DecodeConfig(bytes.NewReader(response.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 1280 || info.Height != 640 {
		t.Errorf("social preview = %dx%d, want 1280x640", info.Width, info.Height)
	}
	if response.Body.Len() >= 1<<20 || !strings.HasPrefix(response.Header().Get("Content-Type"), "image/png") {
		t.Fatal("invalid social preview delivery")
	}
	legacy := request(t, handler, http.MethodGet, "/ui/social-v1.png", "")
	if legacy.Code != http.StatusOK {
		t.Errorf("previously shared social preview status = %d, want 200", legacy.Code)
	}
}

func assertSocialMetadata(t *testing.T, html, canonical string) {
	t.Helper()
	for _, marker := range []string{`<title>`, `name="description"`, `rel="canonical"`, `property="og:url"`, `property="og:type"`, `property="og:title"`, `property="og:description"`, `property="og:site_name"`, `property="og:image"`, `property="og:image:type"`, `property="og:image:width"`, `property="og:image:height"`, `property="og:image:alt"`, `name="twitter:card"`, `name="twitter:title"`, `name="twitter:description"`, `name="twitter:image"`, `name="twitter:image:alt"`} {
		if strings.Count(html, marker) != 1 {
			t.Errorf("metadata %s must appear once", marker)
		}
	}
	if !strings.Contains(html, `href="`+canonical+`"`) || !strings.Contains(html, `property="og:url" content="`+canonical+`"`) {
		t.Error("canonical and Open Graph URL mismatch", canonical)
	}
}
