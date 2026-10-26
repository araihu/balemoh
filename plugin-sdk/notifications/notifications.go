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
	"strconv"
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

	// ErrorInfoDomain identifies SDK publication status details.
	ErrorInfoDomain = "balemoh.plugins"
	// ErrorInfoReasonBusy marks retryable per-connection callback contention.
	ErrorInfoReasonBusy = "NOTIFICATION_BUSY"
	// ErrorInfoReasonCapacity marks permanent inbox capacity exhaustion.
	ErrorInfoReasonCapacity = "NOTIFICATION_CAPACITY"
	// ErrorInfoReasonUnavailable marks an unavailable or unknown safe failure.
	ErrorInfoReasonUnavailable = "NOTIFICATION_UNAVAILABLE"

	SeverityInfo     = "info"
	SeveritySuccess  = "success"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"

	DestinationCore     = "core"
	DestinationPlugin   = "plugin"
	DestinationExternal = "external"
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
	"capacity_exceeded":  "Notification inbox capacity is full.",
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
	if !validText(m.EventID, "event ID", 128, false) || !validText(m.Title, "title", 0, false) || !validText(m.Body, "body", 0, true) || utf8.RuneCountInString(m.Title) > 200 || utf8.RuneCountInString(m.Body) > 4000 {
		return Fail("invalid_argument")
	}
	switch m.Severity {
	case SeverityInfo, SeveritySuccess, SeverityWarning, SeverityCritical:
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
	if d.IsZero() {
		return nil
	}
	if !validText(d.Kind, "destination kind", 32, false) || len(d.Params) > 32 || len(d.Query) > 32 {
		return Fail("invalid_argument")
	}
	for key, value := range d.Params {
		if !validText(key, "destination parameter key", 512, false) || !validText(value, "destination parameter value", 512, true) {
			return Fail("invalid_argument")
		}
	}
	for key, values := range d.Query {
		if !validText(key, "destination query key", 512, false) || len(values) > 32 {
			return Fail("invalid_argument")
		}
		for _, value := range values {
			if !validText(value, "destination query value", 512, true) {
				return Fail("invalid_argument")
			}
		}
	}
	switch d.Kind {
	case DestinationCore:
		if !validText(d.Route, "core destination route", 512, false) || d.PluginID != "" || d.ConnectionID != "" || d.Page != "" || d.URL != "" {
			return Fail("invalid_argument")
		}
	case DestinationPlugin:
		if !validText(d.PluginID, "plugin destination plugin ID", 256, false) || !validText(d.Page, "plugin destination page", 256, false) || !canonicalUUID(d.ConnectionID) || d.Route != "" || d.URL != "" {
			return Fail("invalid_argument")
		}
	case DestinationExternal:
		if len(d.URL) > 2048 || validateExternalURL(d.URL) != nil || d.Route != "" || d.PluginID != "" || d.ConnectionID != "" || d.Page != "" || len(d.Params) > 0 || len(d.Query) > 0 {
			return Fail("invalid_argument")
		}
	default:
		return Fail("invalid_argument")
	}
	return nil
}

func (d Destination) IsZero() bool {
	return d.Kind == "" && d.Route == "" && d.PluginID == "" && d.ConnectionID == "" && d.Page == "" && len(d.Params) == 0 && len(d.Query) == 0 && d.URL == ""
}

func validText(s, _ string, maxBytes int, allowEmpty bool) bool {
	if !allowEmpty && strings.TrimSpace(s) == "" || len(s) > maxBytes && maxBytes > 0 || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r == '\x00' || r == '\x7f' || (unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t') {
			return false
		}
	}
	return true
}

func validateExternalURL(raw string) error {
	if raw == "" || strings.ContainsAny(raw, "\x00\r\n\t") {
		return Fail("invalid_argument")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Opaque != "" {
		return Fail("invalid_argument")
	}
	if port := u.Port(); port != "" {
		if n, err := strconv.ParseUint(port, 10, 16); err != nil || n == 0 {
			return Fail("invalid_argument")
		}
	}
	return nil
}

func canonicalUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for i, r := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
