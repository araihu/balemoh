package dashboard

import (
	"cmp"
	"fmt"
	"github.com/google/uuid"
	"slices"
	"strings"
	"unicode/utf8"
)

func ValidateLayout(d *LayoutDocument, previous LayoutDocument, active []Widget) error {
	if d.SchemaVersion != 1 {
		return LayoutFailure(400, "invalid_request", "Unsupported layout schema version.")
	}
	if len(d.Groups) < 1 || len(d.Groups) > 32 {
		return LayoutFailure(409, "limit_exceeded", "A layout needs between 1 and 32 groups.")
	}
	fields := map[string]string{}
	add := func(p, m string) { fields[p] = m }
	geometry := func(v ContainerLayout, p string) {
		if !slices.Contains([]string{"stack", "grid", "masonry"}, v.Type) {
			add(p+"/type", "Choose Stack, Grid or Masonry.")
		}
		if v.Columns < 1 || v.Columns > 4 {
			add(p+"/columns", "Choose 1 through 4 columns.")
		}
		if !slices.Contains([]int{12, 16, 24}, v.GapPx) {
			add(p+"/gapPx", "Choose 12, 16 or 24 pixels.")
		}
	}
	order := func(o Ordering, members map[string]bool, p string) {
		if o.IDs == nil {
			add(p+"/ids", "Use an array, including for an empty order.")
		}
		if o.Mode != "alphabetical" && o.Mode != "custom" {
			add(p+"/mode", "Choose Alphabetical or Custom.")
		}
		if o.Mode == "alphabetical" && len(o.IDs) > 0 {
			add(p+"/ids", "Alphabetical order has no ranked IDs.")
		}
		seen := map[string]int{}
		for i, id := range o.IDs {
			at := fmt.Sprintf("%s/ids/%d", p, i)
			if old, ok := seen[id]; ok {
				add(at, "Repeated rank.")
				add(fmt.Sprintf("%s/ids/%d", p, old), "Repeated rank.")
			}
			seen[id] = i
			if !members[id] {
				add(at, "Rank must belong to this parent.")
			}
		}
	}
	groups := map[string]bool{}
	firstGroup := map[string]int{}
	oldRefs := map[string]bool{}
	eligible := map[string]bool{}
	for _, g := range previous.Groups {
		for _, p := range g.Placements {
			oldRefs[p.ItemID] = true
		}
	}
	for _, w := range active {
		eligible[w.ID] = true
	}
	refs := map[string]string{}
	refPointers := map[string]string{}
	unknown := ""
	count := 0
	for i := range d.Groups {
		g := &d.Groups[i]
		p := fmt.Sprintf("/groups/%d", i)
		g.Title = strings.TrimSpace(g.Title)
		if old, ok := firstGroup[g.ID]; ok {
			add(p+"/id", "Repeated group ID.")
			add(fmt.Sprintf("/groups/%d/id", old), "Repeated group ID.")
		}
		firstGroup[g.ID] = i
		groups[g.ID] = true
		if !slices.Contains([]string{"builtin:hosts", "builtin:services", "builtin:widgets"}, g.ID) {
			id, e := uuid.Parse(g.ID)
			if e != nil || id.String() != g.ID {
				add(p+"/id", "Use a canonical UUID for a user group.")
			}
		}
		if g.Title == "" || utf8.RuneCountInString(g.Title) > 80 {
			add(p+"/title", "Enter a title of 1 through 80 characters.")
		}
		geometry(g.Layout, p+"/layout")
		members := map[string]bool{}
		if g.Placements == nil {
			add(p+"/placements", "Use an array.")
		}
		for j, item := range g.Placements {
			count++
			at := fmt.Sprintf("%s/placements/%d/itemId", p, j)
			id := item.ItemID
			if id == "" || len(id) > 256 {
				add(at, "Use an existing widget ID of at most 256 bytes.")
			}
			if old, ok := refPointers[id]; ok {
				add(old, "Widget belongs to exactly one group.")
				add(at, "Widget belongs to exactly one group.")
			}
			refPointers[id] = at
			refs[id] = g.ID
			members[id] = true
			if !oldRefs[id] && !eligible[id] {
				unknown = id
			}
		}
		order(g.Ordering, members, p+"/ordering")
	}
	if count > 512 {
		return LayoutFailure(409, "limit_exceeded", "At most 512 explicit widget references are allowed.")
	}
	if d.Homepage.Width != "full" && d.Homepage.Width != "contained" {
		add("/homepage/width", "Choose Full or Contained.")
	}
	geometry(d.Homepage.Layout, "/homepage/layout")
	order(d.Homepage.Ordering, groups, "/homepage/ordering")
	placed := map[string]int{}
	for i, p := range d.Homepage.Placements {
		at := fmt.Sprintf("/homepage/placements/%d", i)
		if old, ok := placed[p.GroupID]; ok {
			add(at+"/groupId", "Repeated group placement.")
			add(fmt.Sprintf("/homepage/placements/%d/groupId", old), "Repeated group placement.")
		}
		placed[p.GroupID] = i
		if !groups[p.GroupID] {
			add(at+"/groupId", "Group does not exist.")
		}
		if p.Span != "auto" && p.Span != "full" {
			add(at+"/span", "Choose Auto or Full.")
		}
	}
	for id := range groups {
		if _, ok := placed[id]; !ok {
			add("/homepage/placements", "Place every configured group exactly once.")
		}
	}
	for k, id := range map[string]string{"host": d.SourceGroups.Host, "service": d.SourceGroups.Service, "other": d.SourceGroups.Other} {
		if !groups[id] {
			add("/sourceGroups/"+k, "Choose an existing group.")
		}
	}
	for _, g := range previous.Groups {
		if !groups[g.ID] {
			for _, p := range g.Placements {
				if _, ok := refs[p.ItemID]; !ok {
					add("/groups", "Move every retained widget reference before deleting its group.")
				}
			}
		}
	}
	if len(fields) > 0 {
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		message := "Layout needs corrections."
		if len(keys) > 32 {
			message += " Additional field errors were omitted."
			keys = keys[:32]
		}
		bounded := map[string]string{}
		for _, k := range keys {
			bounded[k] = fields[k]
		}
		return &LayoutError{Status: 422, Code: "invalid_layout", Message: message, FieldErrors: bounded}
	}
	if unknown != "" {
		return LayoutFailure(404, "not_found", "A newly referenced widget is not selected.")
	}
	slices.SortFunc(d.Groups, func(a, b LayoutGroup) int { return cmp.Compare(a.ID, b.ID) })
	slices.SortFunc(d.Homepage.Placements, func(a, b HomepagePlacement) int { return cmp.Compare(a.GroupID, b.GroupID) })
	for i := range d.Groups {
		slices.SortFunc(d.Groups[i].Placements, func(a, b GroupPlacement) int { return cmp.Compare(a.ItemID, b.ItemID) })
	}
	return nil
}
