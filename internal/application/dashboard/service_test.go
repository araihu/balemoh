package dashboard

import (
	"context"
	"errors"
	"testing"
)

type testProvider struct {
	kind    string
	items   []Widget
	err     error
	id      string
	enabled bool
}

func (p *testProvider) Kind() string                               { return p.kind }
func (p *testProvider) Selected(context.Context) ([]Widget, error) { return p.items, p.err }
func (p *testProvider) SetSelected(_ context.Context, id string, enabled bool) error {
	p.id = id
	p.enabled = enabled
	return p.err
}
func TestProvidersComposeAndFailIndependently(t *testing.T) {
	failed := &testProvider{kind: "broken", err: errors.New("secret upstream detail")}
	text := &testProvider{kind: "notes", items: []Widget{{SourceID: "1", Kind: "text", Presentation: "inline", Data: map[string]string{"text": "hello"}}}}
	service := New(failed, text)
	snapshot := service.Snapshot(context.Background())
	if len(snapshot.Items) != 1 || snapshot.Items[0].ID != "notes-1" || snapshot.Items[0].Presentation != "inline" || len(snapshot.Unavailable) != 1 || snapshot.Unavailable[0] != "broken" {
		t.Fatalf("snapshot %#v", snapshot)
	}
	if err := service.SetSelected(context.Background(), "notes", "1", true); err != nil || text.id != "1" || !text.enabled {
		t.Fatal("selection not dispatched")
	}
	if !errors.Is(service.SetSelected(context.Background(), "unknown", "1", true), ErrSource) {
		t.Fatal("unknown source accepted")
	}
}
