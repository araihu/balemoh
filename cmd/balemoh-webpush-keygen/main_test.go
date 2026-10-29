package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePrivateFileWithoutOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vapid.key")
	if err := generate([]string{path}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	key, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(string(contents)))
	if err != nil || len(key) != 32 {
		t.Fatal("invalid generated key")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("private key permissions too broad")
	}
	if err = generate([]string{path}); err == nil {
		t.Fatal("existing key overwritten")
	}
	again, _ := os.ReadFile(path)
	if string(contents) != string(again) {
		t.Fatal("existing key changed")
	}
}
