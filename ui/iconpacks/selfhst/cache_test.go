package selfhst

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/araihu/balemoh/client/iconassets"
	"github.com/araihu/goshtoso/iconlibrary"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestVerifyFileRejectsCorruptionAndMissingFiles(t *testing.T) {
	good := []byte("verified image")
	digest := fmt.Sprintf("%x", sha256.Sum256(good))
	for _, tc := range []struct {
		name  string
		files fstest.MapFS
		valid bool
	}{
		{"valid", fstest.MapFS{"image.png": {Data: good}}, true},
		{"corrupted", fstest.MapFS{"image.png": {Data: []byte("changed")}}, false},
		{"missing", fstest.MapFS{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyFile(tc.files, "image.png", digest)
			if (err == nil) != tc.valid {
				t.Fatalf("verifyFile() = %v", err)
			}
		})
	}
}

func TestVerifyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := verify(ctx, fstest.MapFS{}); err != context.Canceled {
		t.Fatalf("verify() = %v", err)
	}
}

func TestEnsureReusesVerifiedCacheWithoutSourceFiles(t *testing.T) {
	original := iconassets.Entries
	t.Cleanup(func() { iconassets.Entries = original })
	data := []byte("cached image")
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	iconassets.Entries = []iconlibrary.Icon{{Source: "selfhst", Variants: []iconlibrary.Variant{{Path: "/ui/icon-library/selfhst/image.png", SHA256: digest}}}}
	directory := t.TempDir()
	identity := sha256.Sum256(append(append([]byte{}, config...), lockfile...))
	pack := filepath.Join(directory, fmt.Sprintf("selfhst-%x", identity[:12]))
	output := filepath.Join(pack, "library")
	if err := os.MkdirAll(output, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "image.png"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := Ensure(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyFile(files, "image.png", digest); err != nil {
		t.Fatal(err)
	}
	revision := filepath.Join(pack, "revision-test", "library")
	if err := os.MkdirAll(revision, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(revision, "image.png"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(revision, "new-revision"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := publishRevision(pack, "revision-test"); err != nil {
		t.Fatal(err)
	}
	next, err := Ensure(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	marker, err := next.Open("new-revision")
	if err != nil {
		t.Fatal(err)
	}
	_ = marker.Close()
	if err := verifyFile(files, "image.png", digest); err != nil {
		t.Fatalf("old reader lost its revision: %v", err)
	}

	if _, err := os.Stat(filepath.Join(pack, ".iconpack.yaml")); !os.IsNotExist(err) {
		t.Fatalf("warm cache unexpectedly prepared a source download: %v", err)
	}
}
