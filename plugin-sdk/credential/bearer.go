// Package credential defines the private service/source Bearer token contract.
package credential

import (
	"crypto/subtle"
	"errors"
	"io"
	"os"
	"strings"
)

// ValidBearer accepts RFC 6750 b64token characters, including trailing padding.
func ValidBearer(value string) bool {
	if value == "" || len(value) > 4096 {
		return false
	}
	padding := false
	for _, r := range value {
		if r == '=' {
			padding = true
			continue
		}
		if padding || !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-._~+/", r)) {
			return false
		}
	}
	return value[0] != '='
}
func MatchBearer(header, expected string) bool {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || !ValidBearer(expected) || !ValidBearer(parts[1]) {
		return false
	}
	return len(parts[1]) == len(expected) && subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expected)) == 1
}

// ReadBearerFile permits surrounding file whitespace, never whitespace/control in a token.
func ReadBearerFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("invalid credential file")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", errors.New("cannot read credential")
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("invalid credential file")
	}
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	value := strings.TrimSpace(string(data))
	if err != nil || len(data) > 4096 || !ValidBearer(value) {
		return "", errors.New("invalid credential file")
	}
	return value, nil
}
