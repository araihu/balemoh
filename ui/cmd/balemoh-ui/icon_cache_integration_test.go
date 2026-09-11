//go:build integration

package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/araihu/balemoh/client/iconassets"
	"github.com/araihu/balemoh/ui/iconpacks/selfhst"
	"github.com/araihu/balemoh/ui/internal/bff"
)

// This integration test downloads the actual locked archive into an empty
// cache. Run the compiled test binary separately to measure RSS without builds:
// go test -tags=integration -c ./cmd/balemoh-ui
// ./balemoh-ui.test -test.run=TestSelfhstColdCachePublishesVerifiedHTTP
func TestSelfhstColdCachePublishesVerifiedHTTP(t *testing.T) {
	directory := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Minute)
	defer cancel()
	release := make(chan struct{})
	prepared := make(chan error, 1)
	icons := startIconCache(ctx, directory, 30*time.Minute, func(ctx context.Context, dir string) (fs.FS, error) {
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		files, err := selfhst.Ensure(ctx, dir)
		prepared <- err
		return files, err
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	client, err := bff.NewAPIClient("http://127.0.0.1:1", http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := bff.NewWithIconHandler(client, time.Second, icons)
	if err != nil {
		t.Fatal(err)
	}
	var path, digest string
	for _, icon := range iconassets.Entries {
		if icon.Source == "selfhst" && len(icon.Variants) > 0 {
			path, digest = icon.Variants[0].Path, icon.Variants[0].SHA256
			break
		}
	}
	if path == "" {
		t.Fatal("selfhst catalog missing")
	}
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
	if w := request(); w.Code != 503 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cold response: %d", w.Code)
	}
	close(release)
	select {
	case err := <-prepared:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	deadline := time.Now().Add(time.Second)
	for {
		w := request()
		if w.Code == 200 {
			if fmt.Sprintf("%x", sha256.Sum256(w.Body.Bytes())) != digest {
				t.Fatal("served bytes differ from pinned catalog")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("prepared response: %d", w.Code)
		}
		time.Sleep(time.Millisecond)
	}
	// Ensure must reuse the verified revision after cold preparation.
	if _, err := selfhst.Ensure(ctx, directory); err != nil {
		t.Fatal(err)
	}
}
