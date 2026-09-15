package view

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestResourcePanelsPreserveEvidence(t *testing.T) {
	service := Service{Source: "kubernetes/test", Resources: []Service{
		{Namespace: "apps", Resource: "pod/api", Images: []string{"image@sha256:123"}, Endpoints: []Endpoint{
			{Name: "http", Protocol: "TCP", Port: 8080, URL: "https://app.example/", Provenance: "kubernetes.pod"},
			{Name: "metrics", Protocol: "TCP", Port: 8081},
			{Name: "unsafe", URL: "javascript:alert(1)"},
		}},
		{Resource: "container/<script>bad</script>"},
	}}
	var html bytes.Buffer
	if err := ServiceResources(service).Render(context.Background(), &html); err != nil {
		t.Fatal(err)
	}
	body := html.String()
	for _, want := range []string{"2 resources", "kubernetes/test", "Namespace: apps", "http · TCP · 8080", "metrics · TCP · 8081", "kubernetes.pod", `href="https://app.example/"`, "No public address", "No endpoints observed", "image@sha256:123", "&lt;script&gt;bad&lt;/script&gt;"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing resource evidence: %s", want)
		}
	}
	for _, bad := range []string{`href="javascript:`, "<script>bad</script>", "balemoh-resource-details"} {
		if strings.Contains(body, bad) {
			t.Errorf("unexpected resource markup: %s", bad)
		}
	}
}
