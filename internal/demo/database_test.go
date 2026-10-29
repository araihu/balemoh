package demo_test

import (
	"github.com/araihu/balemoh/internal/demo"
	"os"
	"testing"
)

func TestDemoRefusesUnmarkedExistingDatabase(t *testing.T) {
	path := t.TempDir() + "/private.db"
	original := []byte("private instance data")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := demo.ClaimDatabase(path); err == nil {
		t.Fatal("claimed private database")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Fatal("changed private data")
	}
	if _, err := os.Stat(path + ".synthetic-demo"); !os.IsNotExist(err) {
		t.Fatal("created marker for private data")
	}
}
func TestDemoClaimsFreshStorageAndAllowsItsOwnRestart(t *testing.T) {
	path := t.TempDir() + "/new/demo.db"
	if err := demo.ClaimDatabase(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("synthetic data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := demo.ClaimDatabase(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".synthetic-demo", []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := demo.ClaimDatabase(path); err == nil {
		t.Fatal("accepted invalid marker")
	}
}

func TestDemoRejectsSQLiteURIAndQueryAliases(t *testing.T) {
	for _, path := range []string{"file:/private.db", "private.db?mode=ro", "private.db#fragment", ":memory:", " "} {
		if err := demo.ClaimDatabase(path); err == nil {
			t.Fatalf("accepted ambiguous ownership path %q", path)
		}
	}
}
