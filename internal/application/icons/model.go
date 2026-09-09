package icons

import (
	"errors"
	"github.com/araihu/goshtoso/iconlibrary"
	"strings"
	"unicode/utf8"
)

var ErrInvalid = errors.New("invalid icon")
var ErrNotFound = errors.New("icon not found")
var ErrConflict = errors.New("icon changed or is in use")

type Icon struct {
	ID, Name, Tags, MIME, Digest string
	UsedBy                       []string
	Data                         []byte
}

func Validate(name, tags string, data []byte) (string, error) {
	if strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 200 || utf8.RuneCountInString(tags) > 1000 {
		return "", ErrInvalid
	}
	mime, _, _, err := iconlibrary.ValidateImage(data)
	if err != nil {
		return "", err
	}
	return mime, nil
}
