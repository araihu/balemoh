package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/ui/internal/view"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

func layoutValues(raw []byte) (map[string]string, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value any
	if e := d.Decode(&value); e != nil {
		return nil, e
	}
	out := map[string]string{}
	var walk func(any, string)
	walk = func(value any, path string) {
		switch v := value.(type) {
		case map[string]any:
			if len(v) == 0 {
				out[path] = "None"
			}
			for k, child := range v {
				walk(child, path+"/"+strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1"))
			}
		case []any:
			if len(v) == 0 {
				out[path] = "None"
			}
			seen := map[string]bool{}
			for i, child := range v {
				segment := strconv.Itoa(i + 1)
				if obj, ok := child.(map[string]any); ok {
					for _, name := range []string{"id", "groupId", "itemId"} {
						if id, ok := obj[name].(string); ok && id != "" && !seen[id] {
							segment = id
							seen[id] = true
							break
						}
					}
				}
				walk(child, path+"/"+strings.ReplaceAll(strings.ReplaceAll(segment, "~", "~0"), "/", "~1"))
			}
		case string:
			out[path] = v
			if v == "" {
				out[path] = "(empty)"
			}
		case bool:
			if v {
				out[path] = "Yes"
			} else {
				out[path] = "No"
			}
		case nil:
			out[path] = "Null"
		default:
			out[path] = fmt.Sprint(v)
		}
	}
	walk(value, "")
	return out, nil
}
func layoutSettingLabel(path string) string {
	labels := map[string]string{"schemaVersion": "Schema version", "homepage": "Page", "groups": "Groups", "sourceGroups": "Default groups", "layout": "Arrangement", "type": "Type", "columns": "Maximum columns", "gapPx": "Spacing", "width": "Width", "ordering": "Order", "mode": "Mode", "ids": "Ranked IDs", "placements": "Memberships", "groupId": "Group ID", "itemId": "Item ID", "id": "ID", "title": "Title", "showHeading": "Show heading", "initiallyCollapsed": "Start collapsed", "span": "Span", "host": "Host items", "service": "Service items", "other": "Other items"}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, p := range parts {
		if label, ok := labels[p]; ok {
			parts[i] = label
		} else {
			parts[i] = strings.ReplaceAll(strings.ReplaceAll(p, "~1", "/"), "~0", "~")
		}
	}
	return strings.Join(parts, " / ")
}
func (s *server) layoutCompare(w http.ResponseWriter, r *http.Request) {
	c, ok := s.layoutClient(w)
	if !ok {
		return
	}
	var body struct {
		Document json.RawMessage `json:"document"`
	}
	if !decodeLayoutBFF(w, r, &body) {
		return
	}
	draft, e := layoutValues(body.Document)
	if e != nil {
		writeLayoutBFFError(w, &layoutFailure{status: 400, detail: api.ErrorResponse{Code: "invalid_request", Message: "Invalid draft for comparison."}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	saved, e := c.DashboardLayout(ctx)
	if e != nil {
		writeLayoutBFFError(w, e)
		return
	}
	raw, _ := json.Marshal(saved.Document)
	values, e := layoutValues(raw)
	if e != nil {
		writeLayoutBFFError(w, e)
		return
	}
	keys := []string{}
	for k := range draft {
		keys = append(keys, k)
	}
	for k := range values {
		if _, ok := draft[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	rows := []view.LayoutComparisonRow{}
	for _, k := range keys {
		left, ok := draft[k]
		if !ok {
			left = "Not present"
		}
		right, ok := values[k]
		if !ok {
			right = "Not present"
		}
		rows = append(rows, view.LayoutComparisonRow{ID: k, Setting: layoutSettingLabel(k), Draft: left, Saved: right, Changed: left != right})
	}
	var html bytes.Buffer
	if e = view.LayoutComparison(rows).Render(r.Context(), &html); e != nil {
		writeLayoutBFFError(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"revision": saved.Revision, "html": html.String()})
}
