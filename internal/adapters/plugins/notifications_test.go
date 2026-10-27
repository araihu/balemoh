package plugins

import (
	"context"
	"encoding/json"
	storage "github.com/araihu/balemoh/internal/adapters/sqlite"
	core "github.com/araihu/balemoh/internal/application/notifications"
	app "github.com/araihu/balemoh/internal/application/plugins"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	sdk "github.com/araihu/balemoh/plugin-sdk/notifications"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type notificationSink struct {
	calls            atomic.Int32
	entered, release chan struct{}
	err              error
	source           core.Source
}

func (s *notificationSink) Publish(ctx context.Context, source core.Source, _ core.Message) (core.Receipt, error) {
	s.calls.Add(1)
	s.source = source
	if s.entered != nil {
		s.entered <- struct{}{}
		select {
		case <-s.release:
		case <-ctx.Done():
			return core.Receipt{}, ctx.Err()
		}
	}
	return core.Receipt{ID: "00000000-0000-4000-8000-000000000001", Accepted: true}, s.err
}
func publisherFixture(sink *notificationSink) (*connectionPublisher, sdk.Message) {
	p := &manifest.Package{Manifest: manifest.Manifest{ManifestVersion: 1, Protocol: manifest.Protocol, UIProfile: manifest.UIProfile, Version: "0.0.1", Name: "Fixture", Targets: map[string]manifest.Target{"linux/amd64": {UI: "ui"}}, ConnectionSchema: "schema.json", Secrets: []manifest.Secret{}, Queries: []manifest.Query{}, Navigation: []manifest.Navigation{}, Widgets: []manifest.Widget{}, Assets: []manifest.Asset{}, ID: "example.fixture", Pages: []manifest.Page{{ID: "detail", Title: "Details", Path: "/alerts/{fingerprint}", Methods: []string{"GET"}, Capabilities: []string{}}}, Capabilities: manifest.Capabilities{Required: []string{"page.content/v1"}, Optional: []string{}}}}
	publisher := newConnectionPublisher(context.Background(), sink, p, "00000000-0000-4000-8000-000000000002")
	return publisher, sdk.Message{EventID: "one", Title: "Alert", Severity: "warning", OccurredAt: time.Now()}
}
func TestPublisherRevocationLinearizesAgainstCommit(t *testing.T) {
	sink := &notificationSink{entered: make(chan struct{}, 1), release: make(chan struct{})}
	publisher, message := publisherFixture(sink)
	published := make(chan error, 1)
	go func() { _, err := publisher.Publish(t.Context(), message); published <- err }()
	<-sink.entered
	revoked := make(chan struct{})
	go func() { publisher.revoke(); close(revoked) }()
	select {
	case <-revoked:
		t.Fatal("revoked before pending publication finished")
	case <-time.After(20 * time.Millisecond):
	}
	close(sink.release)
	if err := <-published; err != nil {
		t.Fatal(err)
	}
	<-revoked
	if _, err := publisher.Publish(t.Context(), message); sdk.Code(err) != "permission_denied" {
		t.Fatalf("revoked callback accepted: %v", err)
	}
	if sink.calls.Load() != 1 || sink.source.ID != "example.fixture" || sink.source.ConnectionID != publisher.source.ConnectionID {
		t.Fatal("source scope or revoked call changed")
	}
}

func TestPublisherRejectsForeignOrUndeclaredDestinations(t *testing.T) {
	sink := &notificationSink{}
	publisher, message := publisherFixture(sink)
	message.Destination = &sdk.Destination{Kind: "plugin", PluginID: "other.fixture", ConnectionID: publisher.source.ConnectionID, Page: "detail", Params: map[string]string{"fingerprint": "abc"}}
	if _, err := publisher.Publish(t.Context(), message); sdk.Code(err) != "permission_denied" {
		t.Fatal("foreign target accepted")
	}
	message.Destination.PluginID = publisher.source.ID
	message.Destination.Page = "unknown"
	if _, err := publisher.Publish(t.Context(), message); sdk.Code(err) != "invalid_argument" {
		t.Fatal("undeclared page accepted")
	}
	message.Destination.Page = "detail"
	message.Destination.Params["fingerprint"] = "../bad"
	if _, err := publisher.Publish(t.Context(), message); sdk.Code(err) != "invalid_argument" {
		t.Fatal("unsafe params accepted")
	}
	if sink.calls.Load() != 0 {
		t.Fatal("invalid target reached core")
	}
	message.Destination.Params["fingerprint"] = "abc"
	if _, err := publisher.Publish(t.Context(), message); err != nil {
		t.Fatal(err)
	}
	sink.err = core.ErrCapacity
	if _, err := publisher.Publish(t.Context(), message); sdk.Code(err) != "capacity_exceeded" {
		t.Fatal("capacity semantics lost")
	}
}
func TestNotificationCapabilityGrantIsExplicitAndAffectsRevision(t *testing.T) {
	path, file := deploymentFixture(t)
	installation := &file.Installations[0]
	manifestPath := installation.Package + "/plugin.json"
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var m manifest.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	m.Capabilities.Optional = append(m.Capabilities.Optional, notificationPublishCapability)
	data, _ = json.Marshal(m)
	if err := os.WriteFile(manifestPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	pkg, err := manifest.Load(installation.Package)
	if err != nil {
		t.Fatal(err)
	}
	installation.ManifestDigest = pkg.Digest
	installation.PackageDigest, _ = pkg.ContentDigest()
	saveDeployment(t, path, file)
	deployment, err := Load(path)
	if err != nil || len(deployment.Problems()) != 0 {
		t.Fatal(err)
	}
	first := deployment.installations[0].connections[0]
	if first.canPublishNotifications() {
		t.Fatal("declaration implicitly granted publishing")
	}
	installation.Connections[0].CapabilityGrants = []string{notificationPublishCapability}
	saveDeployment(t, path, file)
	deployment, err = Load(path)
	if err != nil || len(deployment.Problems()) != 0 {
		t.Fatal(err)
	}
	second := deployment.installations[0].connections[0]
	if !second.canPublishNotifications() || first.record.Digest == second.record.Digest {
		t.Fatal("grant did not affect authority/revision")
	}
	installation.Connections[0].CapabilityGrants = []string{"future.invalid/v1"}
	saveDeployment(t, path, file)
	deployment, err = Load(path)
	if err != nil || len(deployment.Problems()) != 1 {
		t.Fatal("unknown capability grant accepted")
	}
}

func TestOversizedPublicationFailureIsPermanent(t *testing.T) {
	err := notificationFailure(core.ErrTooLarge)
	typed, ok := err.(*sdk.Error)
	if !ok || typed.Code != "invalid_argument" || typed.Retryable {
		t.Fatal("oversized publication must not be retried", err)
	}
}

func notificationOnlyDeployment(t *testing.T) (string, File) {
	t.Helper()
	path, file := deploymentFixture(t)
	installation := &file.Installations[0]
	manifestPath := filepath.Join(installation.Package, "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var m manifest.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	m.Queries = []manifest.Query{}
	m.Capabilities.Required = []string{"page.content/v1"}
	m.Capabilities.Optional = []string{notificationPublishCapability}
	m.Pages[0].Capabilities = []string{}
	data, err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	pkg, err := manifest.Load(installation.Package)
	if err != nil {
		t.Fatal(err)
	}
	installation.ManifestDigest = pkg.Digest
	installation.PackageDigest, err = pkg.ContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	installation.Connections[0].Grants = []string{}
	installation.Connections[0].CapabilityGrants = []string{notificationPublishCapability}
	return path, file
}

func TestNotificationOnlyBackendReportsActualConfigurationAndExit(t *testing.T) {
	for _, reject := range []bool{true, false} {
		t.Run(map[bool]string{true: "rejected", false: "configured-and-exited"}[reject], func(t *testing.T) {
			path, file := notificationOnlyDeployment(t)
			if reject {
				if err := os.WriteFile(filepath.Join(filepath.Dir(path), "token"), []byte("wrong-token"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			saveDeployment(t, path, file)
			deployment, err := Load(path)
			if err != nil || len(deployment.Problems()) != 0 {
				t.Fatal("invalid fixture", err, deployment.Problems())
			}
			id := "00000000-0000-4000-8000-000000000002"
			r := NewRuntime(t.Context(), deployment, []app.Connection{{ID: id, PluginID: "example.fixture", Key: "one", Revision: "7", Enabled: true}}, &notificationSink{})
			t.Cleanup(r.Close)
			deadline := time.Now().Add(10 * time.Second)
			for {
				state, grants := r.CachedStatus("example.fixture", id)
				if len(grants) != 0 {
					t.Fatal("notification-only backend acquired query grants")
				}
				if reject {
					if state.Ready || state.AppliedRevision != nil {
						t.Fatal("rejected backend fabricated readiness", state)
					}
					if state.ConfigurationError != nil {
						break
					}
				} else if state.Ready {
					if state.AppliedRevision == nil || *state.AppliedRevision != "7" {
						t.Fatal("configured revision missing", state)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("backend did not report actual Configure result", state)
				}
				time.Sleep(5 * time.Millisecond)
			}
			r.Close()
			state, grants := r.CachedStatus("example.fixture", id)
			if state.Ready || state.State != "unavailable" || len(grants) != 0 {
				t.Fatal("stopped backend fabricated readiness", state)
			}
		})
	}
}

func TestSDKEnvelopeContractReachesDurableCore(t *testing.T) {
	db, err := storage.Open(t.Context(), filepath.Join(t.TempDir(), "inbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := storage.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	service := core.NewService(storage.NewNotificationStore(db))
	publisher, message := publisherFixture(&notificationSink{})
	publisher.publisher = service
	for _, tc := range []struct{ name, body string }{
		{"unicode-fits", strings.Repeat("\u2028", 2000)},
		{"unicode-too-large", strings.Repeat("\u2028", 3000)},
		{"html-fits", strings.Repeat("<>&", 600)},
		{"html-too-large", strings.Repeat("<>&", 1000)},
		{"emoji", strings.Repeat("😀", 3900)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := message
			m.EventID = tc.name
			m.Body = tc.body
			m.OccurredAt = m.OccurredAt.UTC()
			encoded, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			coreMessage := core.Message{EventID: m.EventID, Severity: core.Severity(m.Severity), Title: m.Title, Body: m.Body, OccurredAt: m.OccurredAt}
			size, err := core.EnvelopeBytes(coreMessage)
			if err != nil || size != len(encoded) {
				t.Fatal("SDK/core JSON shapes differ", size, len(encoded), err)
			}
			valid := size <= core.MaxEnvelopeBytes
			if (m.Validate() == nil) != valid {
				t.Fatal("SDK disagrees with the core envelope limit", size)
			}
			receipt, err := publisher.Publish(t.Context(), m)
			if !valid {
				if sdk.Code(err) != "invalid_argument" {
					t.Fatal("oversize rejection changed semantics", err)
				}
				return
			}
			if err != nil || !receipt.Created {
				t.Fatal("SDK-valid message rejected by core", size, err)
			}
			stored, err := service.Get(t.Context(), receipt.ID)
			if err != nil || stored.Body != m.Body {
				t.Fatal("message lost or changed at persistence", err)
			}
		})
	}
}
