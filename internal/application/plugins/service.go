// Package plugins owns data-only plugin connections and query use cases.
package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type Failure struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func (e *Failure) Error() string { return e.Code }
func Fail(code string) *Failure {
	messages := map[string]string{"invalid_argument": "Invalid plugin input.", "permission_denied": "Plugin query is not granted.", "not_found": "Plugin data was not found.", "conflict": "Plugin state changed. Refresh to retry.", "precondition_failed": "Widget revision changed. Reload before saving.", "precondition_required": "Widget revision is required.", "resource_exhausted": "Plugin capacity exceeded.", "invalid_response": "Plugin response is invalid.", "deadline_exceeded": "Plugin request timed out.", "unavailable": "Plugin data is unavailable.", "collection_failed": "Source collection failed."}
	message, ok := messages[code]
	if !ok {
		code = "unavailable"
		message = messages[code]
	}
	return &Failure{code, message, code == "unavailable" || code == "deadline_exceeded" || code == "resource_exhausted" || code == "collection_failed"}
}
func ErrorCode(err error) string {
	var f *Failure
	if errors.As(err, &f) {
		return Fail(f.Code).Code
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	return "unavailable"
}

type Provision struct{ PluginID, Key, Name, Digest, ManifestDigest, Version string }
type Connection struct {
	ID             string `json:"id"`
	PluginID       string `json:"-"`
	Key            string `json:"key"`
	Name           string `json:"name"`
	Revision       string `json:"revision"`
	Enabled        bool   `json:"enabled"`
	Digest         string `json:"-"`
	ManifestDigest string `json:"-"`
	Version        string `json:"-"`
}
type Status struct {
	Ready                  bool       `json:"ready"`
	State                  string     `json:"state"`
	AppliedRevision        *string    `json:"appliedRevision"`
	ObservationRevision    *string    `json:"observationRevision"`
	ObservedAt             *time.Time `json:"observedAt"`
	LastSuccessAt          *time.Time `json:"lastSuccessAt"`
	LastAttemptAt          *time.Time `json:"lastAttemptAt"`
	SourceError            *Failure   `json:"sourceError,omitempty"`
	ConfigurationError     *Failure   `json:"configurationError,omitempty"`
	LastConfigureAttemptAt *time.Time `json:"lastConfigureAttemptAt"`
}
type ConnectionDescriptor struct {
	Connection
	Status
	Queries []string `json:"queries"`
}
type Descriptor struct {
	ID             string                 `json:"id"`
	Version        string                 `json:"version"`
	ManifestDigest string                 `json:"manifestDigest"`
	Connections    []ConnectionDescriptor `json:"connections"`
}
type InstallationProblem struct {
	InstallationID string   `json:"installationId"`
	PluginID       string   `json:"pluginId,omitempty"`
	ConnectionKey  string   `json:"connectionKey,omitempty"`
	Error          *Failure `json:"error"`
}
type Catalog struct {
	Plugins  []Descriptor          `json:"plugins"`
	Problems []InstallationProblem `json:"problems"`
}
type Result struct {
	Data                json.RawMessage `json:"data"`
	State               string          `json:"state"`
	ObservationRevision string          `json:"observationRevision"`
	ObservedAt          *time.Time      `json:"observedAt"`
	LastSuccessAt       *time.Time      `json:"lastSuccessAt"`
	SourceError         *Failure        `json:"sourceError,omitempty"`
}
type Store interface {
	Reconcile(context.Context, []Provision) ([]Connection, error)
	List(context.Context) ([]Connection, error)
	Get(context.Context, string, string) (Connection, error)
	Delete(context.Context, string) error
}
type Runtime interface {
	InstallationProblems() []InstallationProblem
	CachedStatus(string, string) (Status, []string)
	Query(context.Context, string, string, string, json.RawMessage) (Result, error)
}
type Service struct {
	store   Store
	runtime Runtime
}

func New(store Store, runtime Runtime) *Service { return &Service{store, runtime} }
func (s *Service) InstallationProblems() []InstallationProblem {
	return s.runtime.InstallationProblems()
}
func (s *Service) Reconcile(ctx context.Context, items []Provision) ([]Connection, error) {
	return s.store.Reconcile(ctx, items)
}
func (s *Service) Descriptors(ctx context.Context) ([]Descriptor, error) {
	rows, err := s.store.List(ctx)
	if err != nil {
		return nil, Fail("unavailable")
	}
	result := []Descriptor{}
	indices := map[string]int{}
	for _, c := range rows {
		i, ok := indices[c.PluginID]
		if !ok {
			i = len(result)
			indices[c.PluginID] = i
			result = append(result, Descriptor{ID: c.PluginID, Version: c.Version, ManifestDigest: c.ManifestDigest, Connections: []ConnectionDescriptor{}})
		}
		if c.Enabled {
			result[i].Version = c.Version
			result[i].ManifestDigest = c.ManifestDigest
		}
		state, queries := s.runtime.CachedStatus(c.PluginID, c.ID)
		if !c.Enabled {
			state = Status{State: "unavailable"}
			queries = []string{}
		}
		if !state.Ready {
			queries = []string{}
		}
		result[i].Connections = append(result[i].Connections, ConnectionDescriptor{c, state, queries})
	}
	return result, nil
}
func (s *Service) Query(ctx context.Context, plugin, connection, operation string, input json.RawMessage) (Result, error) {
	c, err := s.store.Get(ctx, plugin, connection)
	if err != nil {
		return Result{}, err
	}
	if !c.Enabled {
		return Result{}, Fail("permission_denied")
	}
	if len(input) > 64<<10 {
		return Result{}, Fail("invalid_argument")
	}
	return s.runtime.Query(ctx, plugin, connection, operation, input)
}

type requestIDKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}
func RequestID(ctx context.Context) string { id, _ := ctx.Value(requestIDKey{}).(string); return id }
