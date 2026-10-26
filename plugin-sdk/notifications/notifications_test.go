package notifications_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/araihu/balemoh/plugin-sdk/notifications"
)

func TestMessageValidation(t *testing.T) {
	valid := notifications.Message{EventID: "event-1", Severity: "warning", Title: "Title", Body: "Body", OccurredAt: time.Now().UTC()}
	for name, message := range map[string]notifications.Message{
		"valid":             valid,
		"empty destination": withDestination(valid, &notifications.Destination{}),
		"core route":        withDestination(valid, &notifications.Destination{Kind: notifications.DestinationCore, Route: "/alerts"}),
		"external":          withDestination(valid, &notifications.Destination{Kind: "external", URL: "https://example.test/runbook"}),
		"plugin page":       withDestination(valid, &notifications.Destination{Kind: "plugin", PluginID: "example.alerts", ConnectionID: "00000000-0000-4000-8000-000000000001", Page: "detail", Params: map[string]string{"fingerprint": "abc"}}),
		"multiline text":    withBody(valid, "line 1\nline 2\t"),
	} {
		t.Run(name, func(t *testing.T) {
			if err := message.Validate(); err != nil {
				t.Fatalf("valid message rejected: %v", err)
			}
		})
	}
	for name, message := range map[string]notifications.Message{
		"missing event ID":               withEvent(valid, ""),
		"blank event ID":                 withEvent(valid, " \t"),
		"bad severity":                   withSeverity(valid, "debug"),
		"missing title":                  withTitle(valid, ""),
		"blank title":                    withTitle(valid, " \t"),
		"oversized body":                 withBody(valid, strings.Repeat("x", 4001)),
		"escaping-heavy body":            withBody(valid, strings.Repeat("\u2028", 3000)),
		"zero time":                      withOccurredAt(valid, time.Time{}),
		"bad external URL":               withDestination(valid, &notifications.Destination{Kind: "external", URL: "http://example.test"}),
		"URL userinfo":                   withDestination(valid, &notifications.Destination{Kind: "external", URL: "https://user:pass@example.test/path"}),
		"bad external port":              withDestination(valid, &notifications.Destination{Kind: "external", URL: "https://example.test:99999/path"}),
		"unknown kind":                   withDestination(valid, &notifications.Destination{Kind: "other", Route: "/alerts"}),
		"core plugin fields":             withDestination(valid, &notifications.Destination{Kind: "core", Route: "/alerts", PluginID: "example.alerts"}),
		"core parameter fields":          withDestination(valid, &notifications.Destination{Kind: "core", Route: "/alerts", Params: map[string]string{"tab": "summary"}}),
		"plugin mixed URL":               withDestination(valid, &notifications.Destination{Kind: "plugin", PluginID: "example.alerts", ConnectionID: "00000000-0000-4000-8000-000000000001", Page: "detail", URL: "https://example.test"}),
		"plugin noncanonical connection": withDestination(valid, &notifications.Destination{Kind: "plugin", PluginID: "example.alerts", ConnectionID: "00000000-0000-4000-8000-00000000000A", Page: "detail"}),
		"external typed fields":          withDestination(valid, &notifications.Destination{Kind: "external", URL: "https://example.test", Query: map[string][]string{"tab": {"summary"}}}),
	} {
		t.Run(name, func(t *testing.T) {
			if err := message.Validate(); err == nil || notifications.Code(err) != "invalid_argument" {
				t.Fatalf("invalid message accepted: %v", err)
			}
		})
	}
}

func TestMessageJSONUsesStableCamelCaseFields(t *testing.T) {
	message := notifications.Message{EventID: "event-1", Severity: "info", Title: "Title", OccurredAt: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC), Destination: &notifications.Destination{Kind: "plugin", PluginID: "example.alerts", ConnectionID: "00000000-0000-4000-8000-000000000001", Page: "current", Query: map[string][]string{"tab": {"summary"}}}}
	data, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"eventId"`, `"occurredAt"`, `"pluginId"`, `"connectionId"`} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("missing %s in %s", field, data)
		}
	}
	if strings.Contains(string(data), `"event_id"`) {
		t.Fatalf("snake case leaked into public JSON: %s", data)
	}
}

func TestCodeSanitizesUnknownErrors(t *testing.T) {
	if got := notifications.Code(errors.New("database password=secret")); got != "unavailable" {
		t.Fatalf("unexpected code: %s", got)
	}
	if got := notifications.Fail("not-a-code"); got.Code != "unavailable" || got.Message == "not-a-code" {
		t.Fatalf("unsanitized failure: %+v", got)
	}
}

func withDestination(message notifications.Message, destination *notifications.Destination) notifications.Message {
	message.Destination = destination
	return message
}
func withEvent(message notifications.Message, eventID string) notifications.Message {
	message.EventID = eventID
	return message
}
func withSeverity(message notifications.Message, severity string) notifications.Message {
	message.Severity = severity
	return message
}
func withTitle(message notifications.Message, title string) notifications.Message {
	message.Title = title
	return message
}
func withBody(message notifications.Message, body string) notifications.Message {
	message.Body = body
	return message
}
func withOccurredAt(message notifications.Message, occurredAt time.Time) notifications.Message {
	message.OccurredAt = occurredAt
	return message
}
