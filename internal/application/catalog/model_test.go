package catalog

import (
	"strings"
	"testing"
	"time"
)

func TestStableIDIsDeterministicAndIdentityBound(t *testing.T) {
	source := SourceRef{Kind: "docker", ID: "host-1"}
	resource := ResourceRef{Kind: "container", Namespace: "", Name: "whoami"}

	first := StableID(source, resource)
	if first == "" {
		t.Fatal("StableID() returned an empty ID")
	}
	if first != StableID(source, resource) {
		t.Fatal("StableID() is not deterministic")
	}
	if first == StableID(SourceRef{Kind: "docker", ID: "host-2"}, resource) {
		t.Fatal("StableID() did not change when source identity changed")
	}
	if first == StableID(source, ResourceRef{Kind: "container", Namespace: "prod", Name: "whoami"}) {
		t.Fatal("StableID() did not change when resource namespace changed")
	}
	if StableID(SourceRef{Kind: "a", ID: "b\x00c"}, resource) == StableID(SourceRef{Kind: "a\x00b", ID: "c"}, resource) {
		t.Fatal("StableID() collided for identity fields containing a NUL")
	}
}

func TestNewCandidateDefaultsDisplayNameAndCollections(t *testing.T) {
	observedAt := time.Date(2026, 8, 17, 12, 0, 0, 123, time.FixedZone("BRT", -3*60*60))
	candidate := NewCandidate(
		SourceRef{Kind: " docker ", ID: " host-1 "},
		ResourceRef{Kind: "container", Name: "whoami"},
		observedAt,
	)

	if candidate.ID != StableID(SourceRef{Kind: "docker", ID: "host-1"}, ResourceRef{Kind: "container", Name: "whoami"}) {
		t.Fatalf("ID = %q, want stable normalized identity", candidate.ID)
	}
	if candidate.DisplayName != "whoami" {
		t.Fatalf("DisplayName = %q, want %q", candidate.DisplayName, "whoami")
	}
	if candidate.Metadata == nil {
		t.Fatal("Metadata is nil, want initialized map")
	}
	if candidate.Endpoints == nil {
		t.Fatal("Endpoints is nil, want initialized slice")
	}
	if !candidate.ObservedAt.Equal(observedAt.UTC()) {
		t.Fatalf("ObservedAt = %v, want %v", candidate.ObservedAt, observedAt.UTC())
	}
}

func TestCandidateValidateRejectsInvalidValues(t *testing.T) {
	base := NewCandidate(
		SourceRef{Kind: "docker", ID: "host-1"},
		ResourceRef{Kind: "container", Name: "whoami"},
		time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC),
	)

	tests := map[string]struct {
		candidate Candidate
		want      string
	}{
		"missing source kind": {
			candidate: func() Candidate {
				candidate := base
				candidate.Source.Kind = " "
				return candidate
			}(),
			want: "source kind",
		},
		"missing resource name": {
			candidate: func() Candidate {
				candidate := base
				candidate.Resource.Name = ""
				return candidate
			}(),
			want: "resource name",
		},
		"mismatched id": {
			candidate: func() Candidate {
				candidate := base
				candidate.ID = "wrong"
				return candidate
			}(),
			want: "ID",
		},
		"missing display name": {
			candidate: func() Candidate {
				candidate := base
				candidate.DisplayName = "   "
				return candidate
			}(),
			want: "display name",
		},
		"zero observation time": {
			candidate: func() Candidate {
				candidate := base
				candidate.ObservedAt = time.Time{}
				return candidate
			}(),
			want: "observed at",
		},
		"invalid endpoint URL": {
			candidate: func() Candidate {
				candidate := base
				candidate.Endpoints = []Endpoint{{Name: "web", URL: "not a URL"}}
				return candidate
			}(),
			want: "endpoint URL",
		},
		"invalid endpoint port": {
			candidate: func() Candidate {
				candidate := base
				candidate.Endpoints = []Endpoint{{Name: "web", Port: 70000}}
				return candidate
			}(),
			want: "endpoint port",
		},
		"NUL in source identity": {
			candidate: func() Candidate {
				candidate := base
				candidate.Source.ID = "host\x00one"
				candidate.ID = StableID(candidate.Source, candidate.Resource)
				return candidate
			}(),
			want: "NUL",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := test.candidate.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.want)) {
				t.Fatalf("Validate() error = %q, want mention %q", err, test.want)
			}
		})
	}
}

func TestCandidateNormalizeTrimsEndpointObservation(t *testing.T) {
	candidate := NewCandidate(
		SourceRef{Kind: "docker", ID: "host-1"},
		ResourceRef{Kind: "container", Name: "whoami"},
		time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC),
	)
	candidate.Endpoints = []Endpoint{{
		Name:       " web ",
		URL:        "   ",
		Protocol:   " http ",
		Provenance: " plugin ",
	}}

	normalized := candidate.Normalize()
	if got := normalized.Endpoints[0]; got.Name != "web" || got.URL != "" || got.Protocol != "http" || got.Provenance != "plugin" {
		t.Fatalf("normalized endpoint = %#v, want trimmed fields and empty URL", got)
	}
}
