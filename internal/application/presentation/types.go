// Package presentation defines instance appearance commands and durable resources.
package presentation

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/araihu/goshtoso/themes"
)

const (
	MaxFontBytes   = 10 << 20
	MaxThemes      = 128
	MaxFonts       = 256
	MaxFontStorage = 512 << 20
)

type Conflict struct {
	Kind            string `json:"kind"`
	ID              string `json:"id,omitempty"`
	CurrentRevision string `json:"currentRevision,omitempty"`
}
type Error struct {
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	FieldErrors map[string]string `json:"fieldErrors,omitempty"`
	Conflict    *Conflict         `json:"conflict,omitempty"`
	UsedBy      []string          `json:"usedBy,omitempty"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }
func Invalid() *Error {
	return &Error{Code: "invalid_request", Message: "Invalid presentation request."}
}
func Missing() *Error { return &Error{Code: "not_found", Message: "Resource not found."} }
func Stale(kind, id, revision string) *Error {
	return &Error{Code: "conflict", Message: "Saved state changed. Preserve your draft and inspect the saved version.", Conflict: &Conflict{kind, id, revision}}
}

type Selection struct {
	BrandLogoID             *string            `json:"brandLogoId,omitempty"`
	BrandIconID             string             `json:"brandIconId,omitempty"`
	Kind                    string             `json:"kind"`
	Key                     string             `json:"key,omitempty"`
	ID                      string             `json:"id,omitempty"`
	ExpectedCatalogRevision string             `json:"expectedCatalogRevision,omitempty"`
	ExpectedThemeRevision   string             `json:"expectedThemeRevision,omitempty"`
	Name                    string             `json:"name,omitempty"`
	Definition              *themes.Definition `json:"definition,omitempty"`
	Draft                   *Draft             `json:"draft,omitempty"`
}
type Draft struct {
	BrandLogoID *string           `json:"brandLogoId,omitempty"`
	BrandIconID *string           `json:"brandIconId,omitempty"`
	Name        string            `json:"name"`
	Definition  themes.Definition `json:"definition"`
}
type Settings struct {
	Revision  string    `json:"revision"`
	ColorMode string    `json:"colorMode"`
	Selection Selection `json:"selection"`
}
type State struct {
	BrandLogoID     string            `json:"brandLogoId,omitempty"`
	BrandIconID     string            `json:"brandIconId,omitempty"`
	Settings        Settings          `json:"settings"`
	CatalogRevision string            `json:"catalogRevision"`
	Definition      themes.Definition `json:"definition"`
}
type Save struct {
	CommandID                string    `json:"commandId"`
	ExpectedSettingsRevision string    `json:"expectedSettingsRevision"`
	ColorMode                string    `json:"colorMode"`
	Selection                Selection `json:"selection"`
}
type Receipt struct {
	CommandID     string   `json:"commandId"`
	Settings      Settings `json:"settings"`
	ThemeRevision string   `json:"themeRevision,omitempty"`
}
type Theme struct {
	BrandLogoID string            `json:"brandLogoId,omitempty"`
	BrandIconID string            `json:"brandIconId,omitempty"`
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Revision    string            `json:"revision"`
	Definition  themes.Definition `json:"definition"`
}
type Builtin struct {
	Key        string            `json:"key"`
	Label      string            `json:"label"`
	Definition themes.Definition `json:"definition"`
}
type Catalog struct {
	CatalogRevision string    `json:"catalogRevision"`
	Builtins        []Builtin `json:"builtins"`
	Custom          []Theme   `json:"custom"`
}
type CopyFrom struct {
	ID               string `json:"id"`
	ExpectedRevision string `json:"expectedRevision"`
}
type Create struct {
	BrandLogoID *string            `json:"brandLogoId,omitempty"`
	BrandIconID string             `json:"brandIconId,omitempty"`
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Definition  *themes.Definition `json:"definition,omitempty"`
	CopyFrom    *CopyFrom          `json:"copyFrom,omitempty"`
}
type Rename struct {
	Name             string `json:"name"`
	ExpectedRevision string `json:"expectedRevision"`
}
type Font struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Family     string   `json:"family"`
	Weight     string   `json:"weight"`
	Style      string   `json:"style"`
	Variable   bool     `json:"variable"`
	Format     string   `json:"format"`
	MIME       string   `json:"mime"`
	ByteLength int      `json:"byteLength"`
	Digest     string   `json:"digest"`
	Revision   string   `json:"revision"`
	UsedBy     []string `json:"usedBy"`
	Data       []byte   `json:"-"`
}
type Store interface {
	State(context.Context) (State, error)
	Catalog(context.Context) (Catalog, error)
	GetTheme(context.Context, string) (Theme, error)
	Save(context.Context, Save) (Receipt, error)
	CreateTheme(context.Context, Create) (Theme, bool, error)
	RenameTheme(context.Context, string, Rename) (Theme, error)
	DeleteTheme(context.Context, string, string) error
	Fonts(context.Context) ([]Font, error)
	Font(context.Context, string) (Font, error)
	CreateFont(context.Context, Font) (Font, bool, error)
	DeleteFont(context.Context, string, string) error
}

var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func ValidID(id, kind string) bool {
	return strings.HasPrefix(id, kind+":") && uuid.MatchString(strings.TrimPrefix(id, kind+":"))
}
func ValidCommand(id string) bool { return uuid.MatchString(id) }
func ValidRevision(r string) bool {
	n, e := strconv.ParseInt(r, 10, 64)
	return e == nil && n >= 0 && strconv.FormatInt(n, 10) == r
}
func ValidName(s string) bool {
	return s == strings.TrimSpace(s) && utf8.ValidString(s) && utf8.RuneCountInString(s) > 0 && utf8.RuneCountInString(s) <= 200 && !strings.ContainsAny(s, "\x00\r\n")
}
func Builtins() ([]Builtin, error) {
	out := []Builtin{}
	for _, t := range themes.BuiltIn() {
		d, e := themes.Preset(t.Key)
		if e != nil {
			return nil, e
		}
		out = append(out, Builtin{string(t.Key), t.Label, d})
	}
	return out, nil
}
