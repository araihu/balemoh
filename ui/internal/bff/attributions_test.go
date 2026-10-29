package bff

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh/ui/internal/view"
)

func TestAttributionsRemainAvailableWithoutCatalogOrIconCache(t *testing.T) {
	handler, err := New(&fakeCatalog{homepageErr: errors.New("offline"), stagingErr: errors.New("offline")}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	response := request(t, handler, http.MethodGet, "/attributions", "")
	if response.Code != http.StatusOK {
		t.Fatalf("attributions status = %d", response.Code)
	}
	html := response.Body.String()
	assertSocialMetadata(t, html, "https://balemoh.decastro.me/attributions")
	for _, expected := range []string{
		"<title>Attributions · Balemoh</title>", "Icons by selfh.st", "CC BY 4.0", "Devicon", "Lucide",
		"Balemoh artwork", "Goshtoso attributions page", "https://goshtoso.araihu.com/attributions",
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("attributions missing %q", expected)
		}
	}
	var content bytes.Buffer
	if err := view.AttributionsContent().Render(t.Context(), &content); err != nil {
		t.Fatal(err)
	}
	for _, inherited := range []string{"Alpine.js", "Tailwind CSS", "Heroicons", "a-h/templ"} {
		if strings.Contains(content.String(), inherited) {
			t.Errorf("duplicates Goshtoso credit %q", inherited)
		}
	}
	for _, tc := range []struct{ path, notice string }{
		{"/ui/icon-library/selfhst/LICENSES/selfhst.txt", "Creative Commons Attribution 4.0"},
		{"/ui/icon-licenses/docker-LICENSE.txt", "Copyright (c) 2015 konpa"},
		{"/ui/icon-licenses/kubernetes-LICENSE.txt", "Copyright (c) 2015 konpa"},
		{"/ui/icon-licenses/lucide-LICENSE.txt", "Lucide Icons and Contributors"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			license := request(t, handler, http.MethodGet, tc.path, "")
			if license.Code != http.StatusOK || !strings.HasPrefix(license.Header().Get("Content-Type"), "text/plain") || !strings.Contains(license.Body.String(), tc.notice) {
				t.Fatalf("license unavailable: status %d", license.Code)
			}
		})
	}
	missing := request(t, handler, http.MethodGet, "/ui/icon-licenses/missing.txt", "")
	if missing.Code != http.StatusNotFound {
		t.Errorf("unknown license status = %d", missing.Code)
	}
}
