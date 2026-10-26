// Package backend defines the data-only, author-facing plugin contract.
package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/notifications"
)

const MaxInput = 64 << 10
const MaxResult = 1 << 20

type Config struct {
	ConnectionID  string                  `json:"connectionId"`
	Revision      string                  `json:"revision"`
	Config        json.RawMessage         `json:"config"`
	Secrets       map[string]string       `json:"-"`
	Notifications notifications.Publisher `json:"-"`
}
type Status struct {
	ConnectionID        string     `json:"connectionId"`
	AppliedRevision     string     `json:"appliedRevision"`
	State               string     `json:"state"`
	ObservationRevision string     `json:"observationRevision"`
	ObservedAt          *time.Time `json:"observedAt"`
	LastAttemptAt       *time.Time `json:"lastAttemptAt"`
	LastSuccessAt       *time.Time `json:"lastSuccessAt"`
	SourceError         *Error     `json:"sourceError,omitempty"`
}
type Request struct {
	ConnectionID, OperationID, RequestID string
	Input                                json.RawMessage
}
type Result struct {
	Data                json.RawMessage `json:"data"`
	State               string          `json:"state"`
	ObservationRevision string          `json:"observationRevision"`
	ObservedAt          *time.Time      `json:"observedAt"`
	LastSuccessAt       *time.Time      `json:"lastSuccessAt"`
	SourceError         *Error          `json:"sourceError,omitempty"`
}
type Handler interface {
	Configure(context.Context, Config) (Status, error)
	Status(context.Context, string) (Status, error)
	Query(context.Context, Request) (Result, error)
	CloseConnection(context.Context, string) error
}
type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func (e *Error) Error() string { return e.Code }
func Fail(code string) *Error {
	messages := map[string]string{"invalid_argument": "Invalid plugin input.", "permission_denied": "Plugin query is not granted.", "not_found": "Plugin data was not found.", "conflict": "Observation changed. Refresh to retry.", "resource_exhausted": "Plugin capacity exceeded.", "invalid_response": "Plugin response is invalid.", "deadline_exceeded": "Plugin request timed out.", "unavailable": "Plugin data is unavailable.", "collection_failed": "Source collection failed."}
	message, ok := messages[code]
	if !ok {
		code = "unavailable"
		message = messages[code]
	}
	return &Error{code, message, code == "unavailable" || code == "deadline_exceeded" || code == "resource_exhausted" || code == "collection_failed"}
}
func Code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return Fail(e.Code).Code
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	return "unavailable"
}
func Object(data []byte, max int) bool {
	if len(data) > max {
		return false
	}
	var value map[string]json.RawMessage
	return json.Unmarshal(data, &value) == nil && value != nil && bytes.HasPrefix(bytes.TrimSpace(data), []byte("{"))
}
func Revision(s string) bool {
	n, err := strconv.ParseUint(s, 10, 64)
	return err == nil && n > 0 && strconv.FormatUint(n, 10) == s
}
func State(s string) bool {
	return s == "fresh" || s == "stale" || s == "partial" || s == "unavailable"
}
func (r Result) Validate() error {
	if !Object(r.Data, MaxResult) || !State(r.State) || !Revision(r.ObservationRevision) {
		return Fail("invalid_response")
	}
	if r.State != "unavailable" && (r.ObservedAt == nil || r.LastSuccessAt == nil) {
		return Fail("invalid_response")
	}
	return nil
}
