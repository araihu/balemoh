package notifications

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type recordingStore struct {
	publishCalls int
	canonical    []byte
	message      Message
}

func (s *recordingStore) Publish(_ context.Context, _ Source, message Message, canonical []byte, _ time.Time) (Receipt, error) {
	s.publishCalls++
	s.canonical = append([]byte(nil), canonical...)
	s.message = message
	return Receipt{ID: "receipt", CreatedAt: time.Unix(10, 0).UTC(), Accepted: true}, nil
}

func (s *recordingStore) List(context.Context, ListOptions) (Page, error) { return Page{}, nil }
func (s *recordingStore) Get(context.Context, string) (Notification, error) {
	return Notification{}, ErrNotFound
}
func (s *recordingStore) MarkRead(context.Context, string, time.Time) error { return nil }

func (s *recordingStore) ClearAll(context.Context) error { return nil }

func TestServicePublishNormalizesAndCanonicalizes(t *testing.T) {
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	store := &recordingStore{}
	service := NewServiceWithClock(store, func() time.Time { return now })
	receipt, err := service.Publish(context.Background(), Source{Kind: " external ", ID: " agent "}, Message{
		EventID: "event-1", Severity: SeverityWarning, Title: "hostile <title>", Body: "line 1\nline 2", OccurredAt: now,
		Destination: Destination{Kind: SourceExternal, URL: "https://example.test/path?a=1"},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if !receipt.Accepted || store.publishCalls != 1 {
		t.Fatalf("receipt = %#v, calls = %d", receipt, store.publishCalls)
	}
	if store.message.Title != "hostile <title>" || store.message.Body != "line 1\nline 2" || store.message.EventID != "event-1" {
		t.Fatalf("normalized message = %#v", store.message)
	}
	if !strings.Contains(string(store.canonical), "hostile") {
		t.Fatalf("canonical payload lost title: %s", store.canonical)
	}
}

func TestServicePublishRequiresTitleAndEnforcesCoreBounds(t *testing.T) {
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	source := Source{Kind: SourceCore, ID: "core"}
	for _, title := range []string{"", " \t\n "} {
		store := &recordingStore{}
		service := NewServiceWithClock(store, func() time.Time { return now })
		message := Message{EventID: "title", Severity: SeverityInfo, Title: title, OccurredAt: now}
		if _, err := service.Publish(context.Background(), source, message); !errors.Is(err, ErrInvalid) {
			t.Errorf("title %q error = %v, want ErrInvalid", title, err)
		}
		if store.publishCalls != 0 {
			t.Errorf("title %q reached store", title)
		}
	}

	store := &recordingStore{}
	service := NewServiceWithClock(store, func() time.Time { return now })
	oversizedMultibyte := Message{
		EventID: "multibyte", Severity: SeverityInfo, Title: "title", Body: strings.Repeat("\u2028", 3000), OccurredAt: now,
	}
	if _, err := service.Publish(context.Background(), source, oversizedMultibyte); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("multibyte envelope error = %v, want ErrTooLarge", err)
	}

	oversizedQueryEntry := Message{
		EventID: "query-entry", Severity: SeverityInfo, Title: "title", OccurredAt: now,
		Destination: Destination{Kind: SourceCore, Route: "/notifications", Query: map[string][]string{"key": {strings.Repeat("x", MaxParameterBytes+1)}}},
	}
	if _, err := service.Publish(context.Background(), source, oversizedQueryEntry); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized query entry error = %v, want ErrInvalid", err)
	}

	queryValues := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		queryValues = append(queryValues, strings.Repeat("x", MaxParameterBytes))
	}
	oversizedQueryEnvelope := Message{
		EventID: "query-envelope", Severity: SeverityInfo, Title: "title", OccurredAt: now,
		Destination: Destination{Kind: SourcePlugin, PluginID: "plugin", ConnectionID: "550e8400-e29b-41d4-a716-446655440000", Page: "page", Query: map[string][]string{"q": queryValues}},
	}
	pluginSource := Source{Kind: SourcePlugin, ID: "plugin", ConnectionID: "550e8400-e29b-41d4-a716-446655440000"}
	if _, err := service.Publish(context.Background(), pluginSource, oversizedQueryEnvelope); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversized query envelope error = %v, want ErrTooLarge", err)
	}
}

func TestServicePublishEnvelopeBoundIgnoresSourceIdentity(t *testing.T) {
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	message := Message{
		EventID: "near-boundary", Severity: SeverityInfo, Title: "title", Body: strings.Repeat("\u2028", 2700), OccurredAt: now,
		Destination: Destination{Kind: SourceExternal, URL: "https://example.test/incident"},
	}
	size, err := EnvelopeBytes(message)
	if err != nil {
		t.Fatal(err)
	}
	if size > MaxEnvelopeBytes || MaxEnvelopeBytes-size > 256 {
		t.Fatalf("envelope size = %d, want within 256 bytes below %d", size, MaxEnvelopeBytes)
	}

	for _, source := range []Source{
		{Kind: SourceExternal, ID: "monitor"},
		{Kind: SourceExternal, ID: strings.Repeat("x", MaxSourceIDBytes)},
	} {
		store := &recordingStore{}
		service := NewServiceWithClock(store, func() time.Time { return now })
		receipt, err := service.Publish(context.Background(), source, message)
		if err != nil || !receipt.Accepted {
			t.Errorf("source ID length %d publish = %#v, %v; want accepted", len(source.ID), receipt, err)
		}
	}
}

func TestCanonicalMessageJSONUsesAuthorEnvelopeShape(t *testing.T) {
	message := Message{
		EventID: "event", Severity: SeverityInfo, Title: "title",
		OccurredAt: time.Date(2026, time.October, 25, 12, 0, 0, 0, time.FixedZone("offset", 2*60*60)),
	}
	canonical, err := CanonicalMessageJSON(message)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"eventId":"event","severity":"info","title":"title","occurredAt":"2026-10-25T10:00:00Z"}`
	if string(canonical) != want {
		t.Fatalf("canonical author envelope = %s, want %s", canonical, want)
	}
}

func TestServicePublishScopesPluginDestinationToSource(t *testing.T) {
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	source := Source{Kind: SourcePlugin, ID: "alertmanager", ConnectionID: "550e8400-e29b-41d4-a716-446655440000"}
	destination := Destination{Kind: SourcePlugin, PluginID: source.ID, ConnectionID: source.ConnectionID, Page: "alerts"}
	base := Message{EventID: "plugin-destination", Severity: SeverityWarning, Title: "alert", OccurredAt: now, Destination: destination}

	store := &recordingStore{}
	service := NewServiceWithClock(store, func() time.Time { return now })
	if _, err := service.Publish(context.Background(), source, base); err != nil {
		t.Fatalf("matching plugin destination rejected: %v", err)
	}

	for _, test := range []struct {
		name   string
		source Source
		mutate func(*Message)
	}{
		{name: "external source", source: Source{Kind: SourceExternal, ID: "monitor"}},
		{name: "core source", source: Source{Kind: SourceCore, ID: "core"}},
		{name: "different plugin", source: source, mutate: func(m *Message) { m.Destination.PluginID = "other-plugin" }},
		{name: "different connection", source: source, mutate: func(m *Message) {
			m.Destination.ConnectionID = "550e8400-e29b-41d4-a716-446655440001"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			message := base
			if test.mutate != nil {
				test.mutate(&message)
			}
			before := store.publishCalls
			if _, err := service.Publish(context.Background(), test.source, message); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Publish() error = %v, want ErrInvalid", err)
			}
			if store.publishCalls != before {
				t.Fatal("forged plugin destination reached store")
			}
		})
	}
}

func TestSourceValidationSeparatesConnectionIdentity(t *testing.T) {
	validPlugin := Source{Kind: SourcePlugin, ID: "alertmanager", ConnectionID: "550e8400-e29b-41d4-a716-446655440000"}
	if err := validPlugin.Validate(); err != nil {
		t.Fatalf("valid plugin source rejected: %v", err)
	}
	for _, source := range []Source{
		{Kind: "unknown", ID: "one"},
		{Kind: SourcePlugin, ID: "one"},
		{Kind: SourceCore, ID: "one", ConnectionID: "connection"},
		{Kind: SourceExternal, ID: "one\x00two"},
	} {
		if err := source.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("Source(%#v) error = %v, want ErrInvalid", source, err)
		}
	}
}

func TestMessageValidationRejectsUnsafeValuesAndHostileURLs(t *testing.T) {
	now := time.Now().UTC()
	base := Message{EventID: "event", Severity: SeverityInfo, Title: "title", OccurredAt: now}
	cases := []struct {
		name   string
		mutate func(*Message)
	}{
		{name: "future", mutate: func(m *Message) { m.OccurredAt = now.Add(5*time.Minute + time.Nanosecond) }},
		{name: "title too long", mutate: func(m *Message) { m.Title = strings.Repeat("x", MaxTitleCharacters+1) }},
		{name: "body too long", mutate: func(m *Message) { m.Body = strings.Repeat("x", MaxBodyCharacters+1) }},
		{name: "invalid severity", mutate: func(m *Message) { m.Severity = "fatal" }},
		{name: "userinfo URL", mutate: func(m *Message) {
			m.Destination = Destination{Kind: SourceExternal, URL: "https://user:password@example.test"}
		}},
		{name: "protocol relative URL", mutate: func(m *Message) { m.Destination = Destination{Kind: SourceExternal, URL: "//example.test/path"} }},
		{name: "invalid port URL", mutate: func(m *Message) {
			m.Destination = Destination{Kind: SourceExternal, URL: "https://example.test:99999/path"}
		}},
		{name: "unknown core route", mutate: func(m *Message) {
			m.Destination = Destination{Kind: SourceCore, Route: "/api/v1/notifications"}
		}},
		{name: "core route params", mutate: func(m *Message) {
			m.Destination = Destination{Kind: SourceCore, Route: "/notifications", Params: map[string]string{"id": "1"}}
		}},
		{name: "core route query", mutate: func(m *Message) {
			m.Destination = Destination{Kind: SourceCore, Route: "/notifications", Query: map[string][]string{"id": {"1"}}}
		}},
		{name: "plugin route missing UUID", mutate: func(m *Message) {
			m.Destination = Destination{Kind: SourcePlugin, PluginID: "p", ConnectionID: "not-a-uuid", Page: "page"}
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			message := base
			test.mutate(&message)
			if err := message.Validate(now); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Message.Validate() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestCanonicalPayloadSortsMaps(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	message := Message{EventID: "event", Severity: SeveritySuccess, Title: "title", OccurredAt: now, Destination: Destination{
		Kind: SourcePlugin, PluginID: "plugin", ConnectionID: "550e8400-e29b-41d4-a716-446655440000", Page: "page", Params: map[string]string{"z": "last", "a": "first"}, Query: map[string][]string{"q": {"one", "two"}},
	}}
	first, err := CanonicalPayload(Source{Kind: SourceCore, ID: "service"}, message)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalPayload(Source{Kind: SourceCore, ID: "service"}, Message{EventID: "event", Severity: SeveritySuccess, Title: "title", OccurredAt: now, Destination: Destination{
		Kind: SourcePlugin, PluginID: "plugin", ConnectionID: "550e8400-e29b-41d4-a716-446655440000", Page: "page", Params: map[string]string{"a": "first", "z": "last"}, Query: map[string][]string{"q": {"one", "two"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("canonical payload differs for map order: %s != %s", first, second)
	}
}
