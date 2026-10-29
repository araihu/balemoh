package demo

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

const databaseMarker = "balemoh-synthetic-demo-v1\n"

// ClaimDatabase prevents accidentally publishing an existing private instance.
// The marker is an operator-error guard, not authorization against a filesystem
// owner who can replace both the database and marker.
func ClaimDatabase(path string) error {
	marker := path + ".synthetic-demo"
	body, markerErr := os.ReadFile(marker)
	if markerErr == nil {
		if bytes.Equal(body, []byte(databaseMarker)) {
			return nil
		}
		return fmt.Errorf("invalid synthetic database marker")
	}
	if !os.IsNotExist(markerErr) {
		return fmt.Errorf("read synthetic database marker: %w", markerErr)
	}
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return fmt.Errorf("demo refuses an existing database without its synthetic marker")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("claim synthetic database: %w", err)
	}
	_, writeErr := file.WriteString(databaseMarker)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
