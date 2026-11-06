package dashboard

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestLayoutWireAndOrdering(t *testing.T) {
	raw, e := os.ReadFile("../../../docs/specs/spec-001-homepage-layouts/example-layout.json")
	if e != nil {
		t.Fatal(e)
	}
	var d LayoutDocument
	if e = DecodeLayoutJSON(raw, &d); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(string(raw), `"showHeading": false,`, "", 1), strings.Replace(string(raw), `"columns": 1`, `"columns": null`, 1), strings.Replace(string(raw), `"columns": 1`, `"columns": 1, "columns": 2`, 1), strings.Replace(string(raw), `"columns": 1`, `"columns": 1, "renderer": "row"`, 1), string(raw) + ` {}`} {
		var out LayoutDocument
		if DecodeLayoutJSON([]byte(bad), &out) == nil {
			t.Errorf("accepted invalid shape %s", bad)
		}
	}
	items := []Widget{{ID: "host-z", SourceKind: "host", Title: "Ｚulu"}, {ID: "host-a", SourceKind: "host", Title: "alpha"}, {ID: "host-b", SourceKind: "host", Title: "ALPHA"}}
	if e = ValidateLayout(&d, DefaultLayout(), items); e != nil {
		t.Fatal(e)
	}
	groups, tokens := ResolveLayout(d, items)
	if got := memberIDs(groups[0]); !slices.Equal(got, []string{"host-a", "host-b", "host-z"}) {
		t.Fatal(got)
	}
	if tokens["builtin:hosts"] != MembershipToken("builtin:hosts", []string{"host-z", "host-b", "host-a"}) {
		t.Fatal("membership token depends on order")
	}
	d.Groups[0].Placements = []GroupPlacement{{"host-z"}, {"host-gone"}}
	d.Groups[0].Ordering = Ordering{"custom", []string{"host-gone", "host-z"}}
	if e = ValidateLayout(&d, d, items); e != nil {
		t.Fatal(e)
	}
	groups, _ = ResolveLayout(d, items)
	if groups[0].Members[0].State != "dormant" || len(ActiveLayoutGroups(groups)[0].ItemIDs) != 3 {
		t.Fatal(groups)
	}
	before, _ := json.Marshal(d)
	items[0].Title = "Changed"
	ResolveLayout(d, items)
	after, _ := json.Marshal(d)
	if string(before) != string(after) {
		t.Fatal("resolver changed configuration")
	}
}
func TestLayoutValidationRetainsSubmittedPointersAndDeletedDormantMembers(t *testing.T) {
	old := DefaultLayout()
	old.Groups[0].Placements = []GroupPlacement{{"host-dormant"}}
	d := CloneLayout(old)
	d.Groups[2].Title = " "
	d.Groups[2].Layout.Columns = 99
	d.Groups[1].ID = d.Groups[0].ID
	e := ValidateLayout(&d, old, nil)
	var failure *LayoutError
	if !errors.As(e, &failure) || failure.Status != 422 {
		t.Fatal(e)
	}
	for _, key := range []string{"/groups/0/id", "/groups/1/id", "/groups/2/title", "/groups/2/layout/columns"} {
		if failure.FieldErrors[key] == "" {
			t.Errorf("missing submitted pointer %s: %+v", key, failure)
		}
	}
	d = CloneLayout(old)
	d.Groups = d.Groups[1:]
	d.Homepage.Placements = d.Homepage.Placements[1:]
	d.SourceGroups.Host = "builtin:services"
	if e = ValidateLayout(&d, old, nil); e == nil {
		t.Fatal("deleted dormant reference")
	}
	d.Groups[0].Placements = []GroupPlacement{{"host-dormant"}}
	if e = ValidateLayout(&d, old, nil); e != nil {
		t.Fatal(e)
	}
}

func TestRemovedBuiltinGroupCannotBeRecreated(t *testing.T) {
	previous := DefaultLayout()
	removed := CloneLayout(previous)
	removed.Groups = removed.Groups[1:]
	removed.Homepage.Placements = removed.Homepage.Placements[1:]
	removed.SourceGroups.Host = "builtin:services"
	if e := ValidateLayout(&removed, previous, nil); e != nil {
		t.Fatal(e)
	}
	recreated := DefaultLayout()
	if e := ValidateLayout(&recreated, removed, nil); e == nil {
		t.Fatal("removed builtin ID was accepted as a new group")
	}
}

func TestLayoutAccordionOptionalWireAndCanonicalDefaults(t *testing.T) {
	for _, collapsed := range []bool{false, true} {
		d := DefaultLayout()
		for i := range d.Groups {
			d.Groups[i].InitiallyCollapsed = collapsed
		}
		raw, _ := json.Marshal(d)
		var legacy LayoutDocument
		if err := DecodeLayoutJSON(raw, &legacy); err != nil {
			t.Fatal(err)
		}
		if err := ValidateLayout(&legacy, legacy, nil); err != nil {
			t.Fatal(err)
		}
		for i := range legacy.Groups {
			if legacy.Groups[i].Collapsible != nil {
				t.Fatal("legacy document gained an optional field")
			}
			legacy.Groups[i].Collapsible = new(collapsed)
		}
		if err := ValidateLayout(&legacy, legacy, nil); err != nil {
			t.Fatal(err)
		}
		normalized, _ := json.Marshal(legacy)
		if string(normalized) != string(raw) {
			t.Fatal("equivalent accordion flags changed canonical bytes")
		}
	}
	d := DefaultLayout()
	for _, g := range d.Groups {
		if g.ShowHeading || g.InitiallyCollapsed || g.Collapsible != nil {
			t.Fatal("default presentation must be opt-in")
		}
	}
	d.Groups[0].Collapsible = new(true)
	raw, _ := json.Marshal(d)
	var decoded LayoutDocument
	if err := DecodeLayoutJSON(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLayout(&decoded, decoded, nil); err != nil || decoded.Groups[0].Collapsible == nil || !*decoded.Groups[0].Collapsible {
		t.Fatal("open accordion was not retained", err)
	}
	for _, value := range []string{"null", `"true"`, "1", `true,"collapsible":false`} {
		bad := strings.Replace(string(raw), `"collapsible":true`, `"collapsible":`+value, 1)
		if DecodeLayoutJSON([]byte(bad), &decoded) == nil {
			t.Fatalf("accepted invalid collapsible: %s", value)
		}
	}
	d.Groups[0].Collapsible = new(false)
	d.Groups[0].InitiallyCollapsed = true
	var failure *LayoutError
	if err := ValidateLayout(&d, d, nil); !errors.As(err, &failure) || failure.FieldErrors["/groups/0/initiallyCollapsed"] == "" {
		t.Fatal("contradictory accordion flags accepted", err)
	}
}
