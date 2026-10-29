package ui

import (
	"strings"
	"testing"
)

func TestServiceCatalogBoundsAndPresentation(t *testing.T) {
	id := strings.Repeat("a", 64)
	for _, ids := range [][]string{nil, {"pihole"}, {id, id}, make([]string, 65)} {
		if ValidateServiceIDs(ids) == nil {
			t.Fatal("accepted invalid batch", ids)
		}
	}
	base := ServiceIdentity{ID: id, Name: "Pi-hole", IconURL: "/icons/upload%3Aone/image"}
	if ValidateServiceIdentities([]string{id}, []ServiceIdentity{base}) != nil {
		t.Fatal("valid local icon rejected")
	}
	for _, path := range []string{"https://example.test/icon.png", "//example.test/icon.png", "/\\example.test/icon.png", "javascript:alert(1)"} {
		bad := base
		bad.IconURL = path
		if ValidateServiceIdentities([]string{id}, []ServiceIdentity{bad}) == nil {
			t.Fatal("accepted nonlocal image", path)
		}
	}
	if ValidateServiceIdentities([]string{id}, []ServiceIdentity{base, base}) == nil {
		t.Fatal("duplicate result accepted")
	}
	base.ID = strings.Repeat("b", 64)
	if ValidateServiceIdentities([]string{id}, []ServiceIdentity{base}) == nil {
		t.Fatal("unexpected ID accepted")
	}
}
