// Package notifications contains the public, data-only contract for plugin
// notification publication.
//
// A Publisher is supplied only to a granted backend connection. It is not
// available to UI handlers, and the message has no source field: the host
// binds source identity to the connection that owns the Publisher.
package notifications

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxEnvelope is the maximum serialized callback publication envelope.
	MaxEnvelope = 16 << 10
	// MaxConcurrent is the per-connection publication concurrency bound.
	MaxConcurrent = 4
	// PublishTimeout is the callback deadline applied by the SDK transport.
	PublishTimeout = 5 * time.Second
)

// Message is the source payload accepted by the host notification service.
// Source identity is intentionally absent; the host owns it.
type Message struct {
	EventID     string       `json:"eventId"`
	Severity    string       `json:"severity"`
	Title       string       `json:"title"`
	Body        string       `json:"body,omitempty"`
	OccurredAt  time.Time    `json:"occurredAt"`
	Destination *Destination `json:"destination,omitempty"`
}

// Destination is a typed host destination. Plugin destinations identify a
// declared page and connection; external destinations use URL and must be
// absolute HTTPS links. Host-owned validation remains authoritative.
type Destination struct {
	Kind         string              `json:"kind"`
	Route        string              `json:"route,omitempty"`
	PluginID     string              `json:"pluginId,omitempty"`
	ConnectionID string              `json:"connectionId,omitempty"`
	Page         string              `json:"page,omitempty"`
	Params       map[string]string   `json:"params,omitempty"`
	Query        map[string][]string `json:"query,omitempty"`
	URL          string              `json:"url,omitempty"`
}

// Receipt identifies the durable host result. Created is false for an
// identical event replay.
type Receipt struct {
	ID      string `json:"id"`
	Created bool   `json:"created"`
}

// Publisher is the backend-only host callback. Implementations must not add
// source identity supplied by a child.
type Publisher interface {
	Publish(context.Context, Message) (Receipt, error)
}

// Error is a safe, stable publication failure. Its Error string is a code,
// never an upstream database or HTTP error.
type Error struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *Error) Error() string { return e.Code }

var errorMessages = map[string]string{
	"invalid_argument":   "Notification publication input is invalid.",
	"permission_denied":  "Notification publication is not granted.",
	"conflict":           "Notification publication conflicts with an existing event.",
	"resource_exhausted": "Notification publication capacity exceeded.",
	"deadline_exceeded":  "Notification publication timed out.",
	"unavailable":        "Notification publication is unavailable.",
}

// Fail returns a sanitized publication error. Unknown codes become
// unavailable so arbitrary producer errors cannot cross the boundary.
func Fail(code string) *Error {
	message, ok := errorMessages[code]
	if !ok {
		code, message = "unavailable", errorMessages["unavailable"]
	}
	return &Error{Code: code, Message: message, Retryable: code == "unavailable" || code == "deadline_exceeded" || code == "resource_exhausted"}
}

// Code returns the stable public failure code without exposing arbitrary
// transport or host error text.
func Code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return Fail(e.Code).Code
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	if errors.Is(err, context.Canceled) {
		return "unavailable"
	}
	return "unavailable"
}

// Validate checks the author-facing message shape. The host performs the
// authoritative source, timestamp and destination policy checks as well.
func (m Message) Validate() error {
	if len(m.EventID) > 128 || !validText(m.EventID, 128, true) || !validText(m.Title, 200, true) || !validText(m.Body, 4000, false) {
		return Fail("invalid_argument")
	}
	switch m.Severity {
	case "info", "success", "warning", "critical":
	default:
		return Fail("invalid_argument")
	}
	if m.OccurredAt.IsZero() {
		return Fail("invalid_argument")
	}
	if m.Destination != nil {
		if err := m.Destination.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (d Destination) Validate() error {
	if d.Kind == "" || !validToken(d.Kind, 32) || !validToken(d.Route, 512) || !validToken(d.PluginID, 128) || !validToken(d.ConnectionID, 128) || !validToken(d.Page, 128) || len(d.Params) > 32 || len(d.Query) > 32 {
		return Fail("invalid_argument")
	}
	for key, value := range d.Params {
		if !validToken(key, 64) || !validText(value, 512, true) {
			return Fail("invalid_argument")
		}
	}
	for key, values := range d.Query {
		if !validToken(key, 128) || len(values) > 32 {
			return Fail("invalid_argument")
		}
		for _, value := range values {
			if !validText(value, 512, true) {
				return Fail("invalid_argument")
			}
		}
	}
	if len(d.URL) > 2048 {
		return Fail("invalid_argument")
	}
	if d.URL != "" {
		if err := validateExternalURL(d.URL); err != nil {
			return err
		}
	}
	if d.Kind == "external" && d.URL == "" {
		return Fail("invalid_argument")
	}
	if d.Kind == "plugin" && (d.PluginID == "" || d.ConnectionID == "" || d.Page == "") {
		return Fail("invalid_argument")
	}
	return nil
}

func validToken(s string, max int) bool {
	if s == "" {
		return true
	}
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= max && strings.TrimSpace(s) != "" && !strings.ContainsAny(s, "\x00\r\n")
}

func validText(s string, max int, required bool) bool {
	if required && s == "" || utf8.RuneCountInString(s) > max || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r == '\x00' || (unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t') {
			return false
		}
	}
	return true
}

func validateExternalURL(raw string) error {
	if strings.ContainsAny(raw, "\x00\r\n") {
		return Fail("invalid_argument")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Opaque != "" {
		return Fail("invalid_argument")
	}
	return nil
}
