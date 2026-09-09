// Package selfhst prepares the pinned icon library in persistent storage.
package selfhst

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/araihu/balemoh/client/iconassets"
	"github.com/araihu/goshtoso/iconpack"
	"github.com/gofrs/flock"
)

//go:embed .iconpack.yaml
var config []byte

//go:embed .iconpack.lock.yaml
var lockfile []byte

// Ensure reuses a verified cache or downloads the locked pack and repairs it.
// The caller must give the cache directory persistent, private storage.
func Ensure(ctx context.Context, directory string) (fs.FS, error) {
	identity := sha256.Sum256(append(append([]byte{}, config...), lockfile...))
	directory = filepath.Join(directory, fmt.Sprintf("selfhst-%x", identity[:12]))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	guard := flock.New(filepath.Join(directory, ".startup.lock"))
	acquired, err := guard.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, fmt.Errorf("icon cache lock: %w", ctx.Err())
	}
	defer func() { _ = guard.Unlock(); _ = guard.Close() }()
	output := filepath.Join(directory, "library")
	if current, err := os.ReadFile(filepath.Join(directory, "CURRENT")); err == nil {
		revision := string(current)
		if filepath.Base(revision) != revision || !strings.HasPrefix(revision, "revision-") {
			return nil, fmt.Errorf("invalid icon cache revision")
		}
		output = filepath.Join(directory, revision, "library")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := verify(ctx, os.DirFS(output)); err == nil {
		return os.DirFS(output), nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	configPath := filepath.Join(directory, ".iconpack.yaml")
	if err := os.WriteFile(configPath, config, 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(directory, ".iconpack.lock.yaml"), lockfile, 0o600); err != nil {
		return nil, err
	}
	staging, err := os.MkdirTemp(directory, "revision-")
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(staging)
		}
	}()
	candidate := filepath.Join(staging, "library")
	// Trust stays false: runtime never establishes trust in new source content.
	if _, err := iconpack.Generate(ctx, iconpack.Options{Library: true, ConfigPath: configPath, OutputDir: candidate}); err != nil {
		return nil, err
	}
	if err := verify(ctx, os.DirFS(candidate)); err != nil {
		return nil, fmt.Errorf("verify downloaded icons: %w", err)
	}
	// Keep prior revisions available to already-running HTTP handlers.
	if err := publishRevision(directory, filepath.Base(staging)); err != nil {
		return nil, err
	}
	published = true
	return os.DirFS(candidate), nil

}

func verify(ctx context.Context, files fs.FS) error {
	for _, icon := range iconassets.Entries {
		if icon.Source != "selfhst" {
			continue
		}
		for _, variant := range icon.Variants {
			if err := ctx.Err(); err != nil {
				return err
			}
			name := strings.TrimPrefix(variant.Path, "/ui/icon-library/selfhst/")
			if err := verifyFile(files, name, variant.SHA256); err != nil {
				return err
			}
		}
	}
	return nil
}

func verifyFile(files fs.FS, name, digest string) error {
	file, err := files.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != digest {
		return fmt.Errorf("icon checksum mismatch: %s", name)
	}
	return nil
}

func publishRevision(directory, revision string) error {
	pointer, err := os.CreateTemp(directory, ".current-")
	if err != nil {
		return err
	}
	defer os.Remove(pointer.Name())
	if _, err := pointer.WriteString(revision); err != nil {
		_ = pointer.Close()
		return err
	}
	if err := pointer.Close(); err != nil {
		return err
	}
	return os.Rename(pointer.Name(), filepath.Join(directory, "CURRENT"))
}
