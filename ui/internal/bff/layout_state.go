package bff

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	api "github.com/araihu/balemoh/client"
	"github.com/google/uuid"
)

func unconfirmedLayout() *layoutFailure {
	return &layoutFailure{status: 503, detail: api.ErrorResponse{Code: "unavailable", Message: "Layout result was not confirmed. Your exact draft is retained."}, uncertain: true}
}

// Generated structs do not enforce required JSON fields when decoding responses.
func completeLayoutJSON(raw []byte, t reflect.Type) bool {
	if !utf8.Valid(raw) || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false
	}
	switch t.Kind() {
	case reflect.Struct:
		var object map[string]json.RawMessage
		if json.Unmarshal(raw, &object) != nil || len(object) != t.NumField() {
			return false
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			value, ok := object[strings.Split(f.Tag.Get("json"), ",")[0]]
			if !ok || !completeLayoutJSON(value, f.Type) {
				return false
			}
		}
	case reflect.Slice:
		var values []json.RawMessage
		if json.Unmarshal(raw, &values) != nil || values == nil {
			return false
		}
		for _, v := range values {
			if !completeLayoutJSON(v, t.Elem()) {
				return false
			}
		}
	}
	return true
}

func validLayoutState(s api.LayoutState) bool {
	n, e := strconv.ParseInt(s.Revision, 10, 64)
	if e != nil || n < 0 || strconv.FormatInt(n, 10) != s.Revision {
		return false
	}
	d := s.Document
	if d.SchemaVersion != 1 || len(d.Groups) < 1 || len(d.Groups) > 32 || !d.Homepage.Width.Valid() || !validLayoutGeometry(d.Homepage.Layout) {
		return false
	}
	groups, refs := map[string]bool{}, map[string]bool{}
	for _, g := range d.Groups {
		if groups[g.Id] || g.Title == "" || g.Title != strings.TrimSpace(g.Title) || utf8.RuneCountInString(g.Title) > 80 || !validLayoutGeometry(g.Layout) || g.Placements == nil {
			return false
		}
		if !slices.Contains([]string{"builtin:hosts", "builtin:services", "builtin:widgets"}, g.Id) {
			id, e := uuid.Parse(g.Id)
			if e != nil || id.String() != g.Id {
				return false
			}
		}
		groups[g.Id] = true
		members := map[string]bool{}
		for _, p := range g.Placements {
			if p.ItemId == "" || len(p.ItemId) > 256 || refs[p.ItemId] {
				return false
			}
			refs[p.ItemId] = true
			members[p.ItemId] = true
		}
		if !validLayoutOrder(g.Ordering, members) {
			return false
		}
	}
	if len(refs) > 512 || !validLayoutOrder(d.Homepage.Ordering, groups) || len(d.Homepage.Placements) != len(groups) {
		return false
	}
	placed := map[string]bool{}
	for _, p := range d.Homepage.Placements {
		if !groups[p.GroupId] || placed[p.GroupId] || !p.Span.Valid() {
			return false
		}
		placed[p.GroupId] = true
	}
	return groups[d.SourceGroups.Host] && groups[d.SourceGroups.Service] && groups[d.SourceGroups.Other]
}
func validLayoutGeometry(g api.LayoutContainer) bool {
	return g.Type.Valid() && g.Columns >= 1 && g.Columns <= 4 && g.GapPx.Valid()
}
func validLayoutOrder(o api.LayoutOrdering, members map[string]bool) bool {
	if !o.Mode.Valid() || o.Ids == nil || o.Mode == "alphabetical" && len(o.Ids) > 0 {
		return false
	}
	seen := map[string]bool{}
	for _, id := range o.Ids {
		if seen[id] || !members[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
func sameLayoutDocument(a, b api.LayoutDocument) bool {
	normalize := func(d api.LayoutDocument) []byte {
		// Copy before canonicalizing so confirmation cannot mutate the submitted draft.
		raw, _ := json.Marshal(d)
		var copied api.LayoutDocument
		_ = json.Unmarshal(raw, &copied)
		d = copied
		slices.SortFunc(d.Groups, func(a, b api.LayoutGroup) int { return strings.Compare(a.Id, b.Id) })
		slices.SortFunc(d.Homepage.Placements, func(a, b api.LayoutHomepagePlacement) int { return strings.Compare(a.GroupId, b.GroupId) })
		for i := range d.Groups {
			g := &d.Groups[i]
			g.Title = strings.TrimSpace(g.Title)
			slices.SortFunc(g.Placements, func(a, b api.LayoutGroupPlacement) int { return strings.Compare(a.ItemId, b.ItemId) })
		}
		raw, _ = json.Marshal(d)
		return raw
	}
	return bytes.Equal(normalize(a), normalize(b))
}

func validLayoutPreview(p api.LayoutPreview) bool {
	if !validLayoutState(api.LayoutState{Document: p.Document, Revision: p.LayoutRevision}) || p.DraftGeneration < 0 || p.DraftGeneration > 9007199254740991 || len(p.ResolvedGroups) != len(p.Document.Groups) || len(p.MembershipTokens) != len(p.Document.Groups) {
		return false
	}
	groups := map[string]api.LayoutGroup{}
	for _, g := range p.Document.Groups {
		groups[g.Id] = g
	}
	seenGroups, seenItems := map[string]bool{}, map[string]bool{}
	for _, g := range p.ResolvedGroups {
		config, ok := groups[g.GroupId]
		if !ok || seenGroups[g.GroupId] || g.Title != config.Title || g.DisplayLabel == "" || g.Members == nil {
			return false
		}
		seenGroups[g.GroupId] = true
		ids := []string{}
		members := map[string]bool{}
		for _, m := range g.Members {
			if m.ItemId == "" || len(m.ItemId) > 256 || m.DisplayLabel == "" || !m.State.Valid() || seenItems[m.ItemId] {
				return false
			}
			seenItems[m.ItemId] = true
			members[m.ItemId] = true
			ids = append(ids, m.ItemId)
		}
		for _, placement := range config.Placements {
			if !members[placement.ItemId] {
				return false
			}
		}
		slices.Sort(ids)
		var raw bytes.Buffer
		encoder := json.NewEncoder(&raw)
		encoder.SetEscapeHTML(false)
		_ = encoder.Encode([]any{"layout-members-v1", g.GroupId, ids})
		sum := sha256.Sum256(bytes.TrimSuffix(raw.Bytes(), []byte("\n")))
		if p.MembershipTokens[g.GroupId] != fmt.Sprintf("members-%x", sum) {
			return false
		}
	}
	return true
}

func documentedLayoutFailure(status int, code string) bool {
	switch status {
	case 400:
		return code == "invalid_request"
	case 403:
		return code == "read_only"
	case 404:
		return code == "not_found"
	case 409:
		return code == "limit_exceeded" || code == "membership_changed"
	case 412:
		return code == "revision_conflict"
	case 413:
		return code == "request_too_large"
	case 422:
		return code == "invalid_layout"
	case 428:
		return code == "precondition_required"
	case 503:
		// Unavailable may follow a COMMIT error; only unreadable configuration proves no write.
		return code == "layout_unreadable"
	default:
		return false
	}
}

func uniqueLayoutResponseJSON(raw []byte) bool {
	if !utf8.Valid(raw) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value func() bool
	value = func() bool {
		token, e := decoder.Token()
		if e != nil {
			return false
		}
		delimiter, container := token.(json.Delim)
		if !container {
			return true
		}
		switch delimiter {
		case '{':
			keys := map[string]bool{}
			for decoder.More() {
				key, e := decoder.Token()
				if e != nil {
					return false
				}
				name, ok := key.(string)
				if !ok || keys[name] {
					return false
				}
				keys[name] = true
				if !value() {
					return false
				}
			}
		case '[':
			for decoder.More() {
				if !value() {
					return false
				}
			}
		default:
			return false
		}
		end, e := decoder.Token()
		return e == nil && (delimiter == '{' && end == json.Delim('}') || delimiter == '[' && end == json.Delim(']'))
	}
	if !value() {
		return false
	}
	_, e := decoder.Token()
	return e == io.EOF
}
