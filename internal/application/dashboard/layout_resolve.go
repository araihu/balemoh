package dashboard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"slices"
	"strings"
)

func nameKey(s string) string { return cases.Fold().String(norm.NFKC.String(s)) }
func ordered(ids []string, o Ordering, names map[string]string) []string {
	ids = slices.Clone(ids)
	slices.SortFunc(ids, func(a, b string) int {
		if c := strings.Compare(nameKey(names[a]), nameKey(names[b])); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	if o.Mode != "custom" {
		return ids
	}
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	result := make([]string, 0, len(ids))
	for _, id := range o.IDs {
		if set[id] {
			result = append(result, id)
			delete(set, id)
		}
	}
	for _, id := range ids {
		if set[id] {
			result = append(result, id)
		}
	}
	return result
}
func groupFor(d LayoutDocument, w Widget) string {
	switch w.SourceKind {
	case "host":
		return d.SourceGroups.Host
	case "service":
		return d.SourceGroups.Service
	default:
		return d.SourceGroups.Other
	}
}
func uniqueLabels(names map[string]string, suffix func(string, []string) string) map[string]string {
	sets := map[string][]string{}
	out := map[string]string{}
	for id, name := range names {
		if name == "" {
			name = id
		}
		out[id] = name
		sets[nameKey(name)] = append(sets[nameKey(name)], id)
	}
	for _, ids := range sets {
		if len(ids) < 2 {
			continue
		}
		for _, id := range ids {
			out[id] += " · " + suffix(id, ids)
		}
	}
	return out
}
func groupLabels(names map[string]string) map[string]string {
	return uniqueLabels(names, func(id string, ids []string) string {
		if strings.HasPrefix(id, "builtin:") {
			return "default " + strings.TrimPrefix(id, "builtin:")
		}
		s := strings.ReplaceAll(id, "-", "")
		for n := 6; n <= len(s); n++ {
			unique := true
			for _, other := range ids {
				if other != id && strings.HasPrefix(strings.ReplaceAll(other, "-", ""), s[:n]) {
					unique = false
				}
			}
			if unique {
				return s[:n]
			}
		}
		return id
	})
}
func ResolveLayout(d LayoutDocument, active []Widget) ([]ResolvedGroup, map[string]string) {
	groups := map[string]LayoutGroup{}
	groupNames := map[string]string{}
	groupIDs := []string{}
	members := map[string]map[string]bool{}
	parents := map[string]string{}
	names := map[string]string{}
	widgets := map[string]Widget{}
	for _, w := range active {
		names[w.ID] = w.Title
		if names[w.ID] == "" {
			names[w.ID] = w.ID
		}
		widgets[w.ID] = w
	}
	for _, g := range d.Groups {
		groups[g.ID] = g
		groupNames[g.ID] = g.Title
		groupIDs = append(groupIDs, g.ID)
		members[g.ID] = map[string]bool{}
		for _, p := range g.Placements {
			parents[p.ItemID] = g.ID
			members[g.ID][p.ItemID] = true
			if _, ok := names[p.ItemID]; !ok {
				names[p.ItemID] = p.ItemID
			}
		}
	}
	for _, w := range active {
		parent := parents[w.ID]
		if parent == "" {
			parent = groupFor(d, w)
		}
		members[parent][w.ID] = true
	}
	labels := groupLabels(groupNames)
	itemLabels := uniqueLabels(names, func(id string, ids []string) string {
		suffix := id
		for n := 6; n <= len(id); n++ {
			tail := id[max(0, len(id)-n):]
			unique := true
			for _, other := range ids {
				if id != other && strings.HasSuffix(other, tail) {
					unique = false
				}
			}
			if unique {
				suffix = tail
				break
			}
		}
		kind := widgets[id].SourceKind
		if kind == "" {
			kind, _, _ = strings.Cut(id, "-")
		}
		return kind + " " + suffix
	})
	result := []ResolvedGroup{}
	tokens := map[string]string{}
	for _, id := range ordered(groupIDs, d.Homepage.Ordering, groupNames) {
		g := groups[id]
		ids := []string{}
		for member := range members[id] {
			ids = append(ids, member)
		}
		tokens[id] = MembershipToken(id, ids)
		r := ResolvedGroup{GroupID: id, Title: g.Title, DisplayLabel: labels[id], Members: []LayoutMember{}}
		for _, item := range ordered(ids, g.Ordering, names) {
			state := "dormant"
			if _, ok := widgets[item]; ok {
				state = "active"
			}
			r.Members = append(r.Members, LayoutMember{item, state, itemLabels[item]})
		}
		result = append(result, r)
	}
	return result, tokens
}
func MembershipToken(group string, ids []string) string {
	ids = slices.Clone(ids)
	if ids == nil {
		ids = []string{}
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode([]any{"layout-members-v1", group, ids})
	sum := sha256.Sum256(bytes.TrimSuffix(b.Bytes(), []byte("\n")))
	return "members-" + hex.EncodeToString(sum[:])
}
func ActiveLayoutGroups(groups []ResolvedGroup) []DashboardGroup {
	out := []DashboardGroup{}
	for _, g := range groups {
		v := DashboardGroup{GroupID: g.GroupID, Title: g.Title, DisplayLabel: g.DisplayLabel, ItemIDs: []string{}}
		for _, m := range g.Members {
			if m.State == "active" {
				v.ItemIDs = append(v.ItemIDs, m.ItemID)
			}
		}
		if len(v.ItemIDs) > 0 {
			out = append(out, v)
		}
	}
	return out
}
