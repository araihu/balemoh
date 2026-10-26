package notifications

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"uuid"
)

const (
	MaxEventIDBytes    = 128
	MaxTitleCharacters = 200
	MaxBodyCharacters  = 4000
	MaxSourceIDBytes   = 256
	MaxRouteBytes      = 512
	MaxPageBytes       = 256
	MaxParameterBytes  = 512
	MaxEnvelopeBytes   = 16 << 10
	MaxInboxSize       = 10000
	MaxPageSize        = 50
)

var (
	ErrInvalid     = errors.New("invalid notification")
	ErrConflict    = errors.New("notification identity conflict")
	ErrCapacity    = errors.New("notification inbox capacity reached")
	ErrNotFound    = errors.New("notification not found")
	ErrUnavailable = errors.New("notification storage unavailable")
	ErrTooLarge    = errors.New("notification envelope exceeds limit")
)

// Source identifies the producer of a notification. ConnectionID is required
// only for plugin sources and is empty for core and external sources.
type Source struct {
	Kind         string `json:"kind"`
	ID           string `json:"id"`
	ConnectionID string `json:"connectionId,omitempty"`
}

const (
	SourceCore     = "core"
	SourcePlugin   = "plugin"
	SourceExternal = "external"
)

// Severity is intentionally independent of alert severity.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeveritySuccess  Severity = "success"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Destination is a typed reference. Exactly one destination kind may be set.
// External destinations are links only; the server never fetches URL.
type Destination struct {
	Kind         string              `json:"kind,omitempty"`
	Route        string              `json:"route,omitempty"`
	PluginID     string              `json:"pluginId,omitempty"`
	ConnectionID string              `json:"connectionId,omitempty"`
	Page         string              `json:"page,omitempty"`
	Params       map[string]string   `json:"params,omitempty"`
	Query        map[string][]string `json:"query,omitempty"`
	URL          string              `json:"url,omitempty"`
}

// Message is the producer-supplied notification content.
type Message struct {
	EventID     string      `json:"eventId"`
	Severity    Severity    `json:"severity"`
	Title       string      `json:"title"`
	Body        string      `json:"body,omitempty"`
	OccurredAt  time.Time   `json:"occurredAt"`
	Destination Destination `json:"destination,omitempty"`
}

// Notification is the durable inbox projection.
type Notification struct {
	ID          string      `json:"id"`
	Source      Source      `json:"source"`
	EventID     string      `json:"eventId"`
	Severity    Severity    `json:"severity"`
	Title       string      `json:"title"`
	Body        string      `json:"body,omitempty"`
	OccurredAt  time.Time   `json:"occurredAt"`
	ReceivedAt  time.Time   `json:"receivedAt"`
	Destination Destination `json:"destination,omitempty"`
	ReadAt      *time.Time  `json:"readAt,omitempty"`
}

// Receipt acknowledges a durable publish. Accepted is false for an identical
// replay and true only when a new row was committed.
type Receipt struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	Accepted  bool      `json:"accepted"`
}

// ListOptions describes a stable sequence page. Before is an internal host
// sequence, not an HTTP cursor.
type ListOptions struct {
	Before     uint64
	UnreadOnly bool
	Limit      int
}

// Page contains one database snapshot of rows and unread count.
type Page struct {
	Notifications []Notification `json:"notifications"`
	UnreadCount   int            `json:"unreadCount"`
	NextBefore    uint64         `json:"-"`
}

func (s Source) Normalize() Source {
	s.Kind = strings.TrimSpace(s.Kind)
	s.ID = strings.TrimSpace(s.ID)
	s.ConnectionID = strings.TrimSpace(s.ConnectionID)
	if s.Kind == SourcePlugin && s.ConnectionID != "" {
		if parsed, err := uuid.Parse(s.ConnectionID); err == nil {
			s.ConnectionID = parsed.String()
		}
	}
	return s
}

func (m Message) Normalize() Message {
	m.OccurredAt = m.OccurredAt.UTC()
	m.Destination = m.Destination.Normalize()
	return m
}

func (d Destination) Normalize() Destination {
	if d.Kind == SourcePlugin && d.ConnectionID != "" {
		if parsed, err := uuid.Parse(d.ConnectionID); err == nil {
			d.ConnectionID = parsed.String()
		}
	}
	return d
}

func (s Source) Validate() error {
	s = s.Normalize()
	if s.Kind != SourceCore && s.Kind != SourcePlugin && s.Kind != SourceExternal {
		return fmt.Errorf("%w: source kind must be core, plugin, or external", ErrInvalid)
	}
	if err := validateText(s.ID, "source ID", MaxSourceIDBytes, false); err != nil {
		return err
	}
	if s.Kind == SourcePlugin {
		parsed, err := uuid.Parse(s.ConnectionID)
		if err != nil {
			return fmt.Errorf("%w: plugin source connection ID must be a UUID", ErrInvalid)
		}
		if parsed.String() != s.ConnectionID {
			return fmt.Errorf("%w: plugin source connection ID must be canonical", ErrInvalid)
		}
	} else if s.ConnectionID != "" {
		return fmt.Errorf("%w: %s source connection ID must be empty", ErrInvalid, s.Kind)
	}
	return nil
}

func (m Message) Validate(now time.Time) error {
	m = m.Normalize()
	if err := validateText(m.EventID, "event ID", MaxEventIDBytes, false); err != nil {
		return err
	}
	if !validSeverity(m.Severity) {
		return fmt.Errorf("%w: severity must be info, success, warning, or critical", ErrInvalid)
	}
	if err := validateText(m.Title, "title", 0, false); err != nil {
		return err
	}
	if utf8.RuneCountInString(m.Title) > MaxTitleCharacters {
		return fmt.Errorf("%w: title exceeds %d characters", ErrInvalid, MaxTitleCharacters)
	}
	if err := validateText(m.Body, "body", 0, true); err != nil {
		return err
	}
	if utf8.RuneCountInString(m.Body) > MaxBodyCharacters {
		return fmt.Errorf("%w: body exceeds %d characters", ErrInvalid, MaxBodyCharacters)
	}
	if m.OccurredAt.IsZero() {
		return fmt.Errorf("%w: occurred at must not be zero", ErrInvalid)
	}
	if m.OccurredAt.After(now.UTC().Add(5 * time.Minute)) {
		return fmt.Errorf("%w: occurred at is too far in the future", ErrInvalid)
	}
	if err := m.Destination.Validate(); err != nil {
		return err
	}
	return nil
}

func (d Destination) IsZero() bool {
	return d.Kind == "" && d.Route == "" && d.PluginID == "" && d.ConnectionID == "" && d.Page == "" && len(d.Params) == 0 && len(d.Query) == 0 && d.URL == ""
}

func (d Destination) Validate() error {
	d = d.Normalize()
	if d.IsZero() {
		return nil
	}
	if d.Kind != SourceCore && d.Kind != SourcePlugin && d.Kind != SourceExternal {
		return fmt.Errorf("%w: destination kind is invalid", ErrInvalid)
	}
	if err := validateMap(d.Params, "destination parameter"); err != nil {
		return err
	}
	if err := validateQuery(d.Query); err != nil {
		return err
	}
	switch d.Kind {
	case SourceCore:
		if err := validateText(d.Route, "core destination route", MaxRouteBytes, false); err != nil {
			return err
		}
		if _, ok := coreDestinationRoutes[d.Route]; !ok {
			return fmt.Errorf("%w: core destination route is not host-owned", ErrInvalid)
		}
		if d.PluginID != "" || d.ConnectionID != "" || d.Page != "" || d.URL != "" || len(d.Params) > 0 || len(d.Query) > 0 {
			return fmt.Errorf("%w: core destination has plugin or URL fields", ErrInvalid)
		}
	case SourcePlugin:
		if err := validateText(d.PluginID, "plugin destination plugin ID", MaxSourceIDBytes, false); err != nil {
			return err
		}
		if err := validateText(d.Page, "plugin destination page", MaxPageBytes, false); err != nil {
			return err
		}
		parsed, err := uuid.Parse(d.ConnectionID)
		if err != nil || parsed.String() != d.ConnectionID {
			return fmt.Errorf("%w: plugin destination connection ID must be a canonical UUID", ErrInvalid)
		}
		if d.Route != "" || d.URL != "" {
			return fmt.Errorf("%w: plugin destination has core or URL fields", ErrInvalid)
		}
	case SourceExternal:
		if d.URL == "" {
			return fmt.Errorf("%w: external destination URL must not be empty", ErrInvalid)
		}
		parsed, err := url.Parse(d.URL)
		portValid := true
		if err == nil && parsed.Port() != "" {
			port, portErr := strconv.ParseUint(parsed.Port(), 10, 16)
			portValid = portErr == nil && port > 0
		}
		if err != nil || !portValid || parsed.Scheme != "https" || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" || strings.ContainsAny(d.URL, "\x00\r\n\t") {
			return fmt.Errorf("%w: external destination URL must be an absolute HTTPS URL without credentials", ErrInvalid)
		}
		if d.Route != "" || d.PluginID != "" || d.ConnectionID != "" || d.Page != "" || len(d.Params) > 0 || len(d.Query) > 0 {
			return fmt.Errorf("%w: external destination has typed reference fields", ErrInvalid)
		}
	}
	return nil
}

var coreDestinationRoutes = map[string]struct{}{
	"/":              {},
	"/hosts":         {},
	"/staging":       {},
	"/notifications": {},
}

func validSeverity(severity Severity) bool {
	switch severity {
	case SeverityInfo, SeveritySuccess, SeverityWarning, SeverityCritical:
		return true
	default:
		return false
	}
}

func validateMap(values map[string]string, label string) error {
	for key, value := range values {
		if err := validateText(key, label+" key", MaxParameterBytes, false); err != nil {
			return err
		}
		if err := validateText(value, label+" value", MaxParameterBytes, true); err != nil {
			return err
		}
	}
	return nil
}

func validateQuery(values map[string][]string) error {
	for key, entries := range values {
		if err := validateText(key, "destination query key", MaxParameterBytes, false); err != nil {
			return err
		}
		for _, value := range entries {
			if err := validateText(value, "destination query value", MaxParameterBytes, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateText(value, label string, maxBytes int, allowEmpty bool) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%w: %s must be valid UTF-8", ErrInvalid, label)
	}
	if !allowEmpty && strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s must not be empty", ErrInvalid, label)
	}
	if maxBytes > 0 && len(value) > maxBytes {
		return fmt.Errorf("%w: %s exceeds %d bytes", ErrInvalid, label, maxBytes)
	}
	for _, r := range value {
		if r == '\x00' || (r < 0x20 && r != '\n' && r != '\r' && r != '\t') || r == 0x7f {
			return fmt.Errorf("%w: %s contains control characters", ErrInvalid, label)
		}
	}
	return nil
}

// CanonicalMessageJSON returns the stable author envelope used for the
// publication size bound. It intentionally excludes host-bound source
// identity, normalizes the timestamp to UTC and omits empty optional fields.
func CanonicalMessageJSON(message Message) ([]byte, error) {
	message = message.Normalize()
	var destination *Destination
	if !message.Destination.IsZero() {
		normalized := message.Destination.Normalize()
		destination = &normalized
	}
	canonicalMessage := struct {
		EventID     string       `json:"eventId"`
		Severity    Severity     `json:"severity"`
		Title       string       `json:"title"`
		Body        string       `json:"body,omitempty"`
		OccurredAt  time.Time    `json:"occurredAt"`
		Destination *Destination `json:"destination,omitempty"`
	}{
		EventID: message.EventID, Severity: message.Severity, Title: message.Title,
		Body: message.Body, OccurredAt: message.OccurredAt.UTC(), Destination: destination,
	}
	return json.Marshal(canonicalMessage)
}

// EnvelopeBytes reports the serialized author envelope size used by Publish.
func EnvelopeBytes(message Message) (int, error) {
	payload, err := CanonicalMessageJSON(message)
	if err != nil {
		return 0, err
	}
	return len(payload), nil
}

// CanonicalPayload returns the immutable source-bound payload bytes used for
// replay and conflict detection. encoding/json sorts map keys, making this
// stable. The source is deliberately excluded from EnvelopeBytes.
func CanonicalPayload(source Source, message Message) ([]byte, error) {
	canonicalMessage, err := CanonicalMessageJSON(message)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Source  Source          `json:"source"`
		Message json.RawMessage `json:"message"`
	}{Source: source.Normalize(), Message: canonicalMessage})
}
