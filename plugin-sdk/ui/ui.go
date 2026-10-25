// Package ui defines the private author-facing UI boundary, not the data API.
package ui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/araihu/balemoh/plugin-sdk/manifest"
)

const MaxHTML = 1 << 20

type Request struct {
	RequestID        string
	ConnectionID     string
	PlacementID      string
	ContributionType string
	ContributionID   string
	Region           string
	Form             url.Values
	Body             []byte
	Mode             string
	Method           string
	PathParams       map[string]string
	Query            url.Values
	Locale           string
	Timezone         string
	Theme            string
	ColorScheme      string
	PackageDigest    string
	BasePath         string
	DOMNamespace     string
}
type Response struct {
	Kind        string
	Status      int
	Title       string
	Description string
	HTML        []byte
	Redirect    *Redirect
	AssetIDs    []string
}
type Redirect struct {
	Page   string
	Params map[string]string
	Query  url.Values
}
type QueryResult = backend.Result
type Placement struct {
	ID             string
	ContributionID string
	Options        json.RawMessage
	Revision       string
}
type HostServices interface {
	Query(context.Context, string, json.RawMessage) (QueryResult, error)
	Placement(context.Context) (Placement, error)
}
type Handler interface {
	Handle(context.Context, Request, HostServices) (Response, error)
}
type HandlerFunc func(context.Context, Request, HostServices) (Response, error)

func (f HandlerFunc) Handle(ctx context.Context, r Request, h HostServices) (Response, error) {
	return f(ctx, r, h)
}

var ErrUnavailable = errors.New("host service is unavailable in this capability")

// NoDataServices is the explicit P1 host boundary: no source-query grants.
type NoDataServices struct{ Current Placement }

func (NoDataServices) Query(context.Context, string, json.RawMessage) (QueryResult, error) {
	return QueryResult{}, ErrUnavailable
}
func (h NoDataServices) Placement(context.Context) (Placement, error) { return h.Current, nil }

func (r Request) DOMID(local string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{r.DOMNamespace, r.ConnectionID, r.PlacementID, r.ContributionType, r.ContributionID, local}, "\x00")))
	return "plugin-" + hex.EncodeToString(sum[:])
}

// PageURL resolves a declared page in this connection, including a base path.
func (r Request) PageURL(m manifest.Manifest, id string, params map[string]string) (string, error) {
	p, ok := m.Page(id)
	if !ok {
		return "", errors.New("unknown page")
	}
	parts := strings.Split(p.Path, "/")
	used := map[string]bool{}
	for i, s := range parts {
		if strings.HasPrefix(s, "{") {
			key := strings.TrimSuffix(strings.TrimPrefix(s, "{"), "}")
			v, ok := params[key]
			if !ok || v == "" || v == "." || v == ".." || strings.ContainsAny(v, "/\\%\x00") {
				return "", errors.New("invalid page parameter")
			}
			parts[i] = url.PathEscape(v)
			used[key] = true
		}
	}
	if len(used) != len(params) {
		return "", errors.New("unexpected page parameter")
	}
	return strings.TrimRight(r.BasePath, "/") + "/plugins/" + url.PathEscape(m.ID) + "/connections/" + url.PathEscape(r.ConnectionID) + "/pages" + strings.Join(parts, "/"), nil
}

func ValidateResponse(r Response, mode string) error {
	if len(r.HTML) > MaxHTML || !utf8.Valid(r.HTML) || !utf8.ValidString(r.Title) || !utf8.ValidString(r.Description) || len(r.Title) > 512 || len(r.Description) > 2048 || len(r.AssetIDs) > 32 {
		return errors.New("invalid UI response bounds")
	}
	switch r.Kind {
	case "content", "fragment":
		if r.Kind == "fragment" && mode != "fragment" || r.Kind == "content" && (mode != "page" && mode != "widget") {
			return errors.New("response mode mismatch")
		}
		if r.Status != 200 && (r.Status < 400 || r.Status > 499) && r.Status != 503 {
			return errors.New("invalid render status")
		}
		if r.Redirect != nil {
			return errors.New("unexpected redirect")
		}
	case "redirect":
		if r.Status != 303 || r.Redirect == nil || len(r.HTML) != 0 {
			return errors.New("invalid redirect")
		}
		if len(r.Redirect.Query.Encode()) > 64<<10 || len(r.Redirect.Params) > 32 {
			return errors.New("redirect exceeds bounds")
		}
	case "no-content":
		if r.Status != 204 || len(r.HTML) != 0 || r.Redirect != nil {
			return errors.New("invalid empty response")
		}
	default:
		return errors.New("unknown UI response kind")
	}
	return nil
}
func (r Request) FragmentURL(m manifest.Manifest, region string) (string, error) {
	if _, ok := m.Page(r.ContributionID); !ok || region == "" || strings.ContainsAny(region, "/%\\.\x00") {
		return "", errors.New("invalid fragment")
	}
	return strings.TrimRight(r.BasePath, "/") + "/plugins/" + url.PathEscape(m.ID) + "/connections/" + url.PathEscape(r.ConnectionID) + "/fragments/" + url.PathEscape(r.ContributionID) + "/" + url.PathEscape(region), nil
}
func (r Request) AssetURL(m manifest.Manifest, id string) (string, error) {
	for _, a := range m.Assets {
		if a.ID == id {
			return strings.TrimRight(r.BasePath, "/") + "/plugins/assets/" + url.PathEscape(m.ID) + "/" + r.PackageDigest + "/" + a.File, nil
		}
	}
	return "", errors.New("unknown asset")
}

func (r Request) WidgetURL() (string, error) {
	if r.PlacementID == "" || strings.ContainsAny(r.PlacementID, "/%\\\x00") {
		return "", errors.New("no scoped widget")
	}
	return strings.TrimRight(r.BasePath, "/") + "/plugin-widgets/" + url.PathEscape(r.PlacementID), nil
}
