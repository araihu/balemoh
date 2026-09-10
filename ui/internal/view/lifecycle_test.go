package view

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLifecycleTabsAndActions(t *testing.T) {
	data := PageData{Staging: true, Services: []Service{
		{ID: "a", DisplayName: "Live app", Status: "live"},
		{ID: "b", DisplayName: "Missing app", Status: "missing", Missing: true},
		{ID: "c", DisplayName: "Hidden app", Status: "hidden"},
	}}
	var out bytes.Buffer
	if err := StagingContent(data).Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	html := out.String()
	for _, label := range []string{"Live", "Missing", "Hidden"} {
		if !strings.Contains(html, label) {
			t.Fatalf("tab %s missing", label)
		}
	}
	if strings.Count(html, `name="pinned"`) != 1 {
		t.Fatal("pin checkboxes must only be offered for live rows")
	}
	for _, s := range data.Services {
		out.Reset()
		if err := ServiceLifecycle(s).Render(context.Background(), &out); err != nil {
			t.Fatal(err)
		}
		h := out.String()
		if strings.Contains(h, "Delete permanently") != s.Missing {
			t.Fatalf("wrong delete action for %s", s.Status)
		}
		if strings.Contains(h, "Show service") != (s.Status == "hidden") {
			t.Fatalf("wrong show action for %s", s.Status)
		}
	}
}
