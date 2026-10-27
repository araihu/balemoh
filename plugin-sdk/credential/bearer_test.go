package credential_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/araihu/balemoh/plugin-sdk/credential"
)

func TestFileAndAuthenticationShareTokenContract(t *testing.T) {
	for _, token := range []string{strings.Repeat("private", 6), "abc-._~+/123==", "contains space", "contains\ttab", "contains\x01control", "contains\x7fcontrol", "contains\u00a0space", "=padding", "a=b", ""} {
		t.Run(strings.ReplaceAll(token, "\x01", "control"), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "credential")
			if err := os.WriteFile(path, []byte(token+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			value, err := credential.ReadBearerFile(path)
			valid := credential.ValidBearer(token)
			if (err == nil) != valid {
				t.Fatal("file and lexical contract differ")
			}
			if valid && !credential.MatchBearer("Bearer "+value, value) {
				t.Fatal("accepted file can never authenticate")
			}
			if !valid && credential.MatchBearer("Bearer "+token, token) {
				t.Fatal("invalid token authenticated")
			}
		})
	}
}
