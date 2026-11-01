package dashboard

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"slices"
)

func (s *Layouts) Get(ctx context.Context) (LayoutState, error) {
	v, _, e := s.Store.Read(ctx, false)
	return v, e
}
func (s *Layouts) Preview(ctx context.Context, r LayoutPreviewRequest) (LayoutPreview, error) {
	if _, e := uuid.Parse(r.RequestID); e != nil || r.DraftGeneration < 0 || r.DraftGeneration > 9007199254740991 || !ValidLayoutRevision(r.ExpectedRevision) {
		return LayoutPreview{}, LayoutFailure(400, "invalid_request", "Invalid preview identity, generation or revision.")
	}
	current, items, e := s.Store.Read(ctx, true)
	if e != nil {
		return LayoutPreview{}, e
	}
	d := CloneLayout(r.Document)
	if e = ValidateLayout(&d, current.Document, items); e != nil {
		if v, ok := e.(*LayoutError); ok && v.FieldErrors != nil {
			f := map[string]string{}
			for p, m := range v.FieldErrors {
				f["/document"+p] = m
			}
			v.FieldErrors = f
		}
		return LayoutPreview{}, e
	}
	groups, tokens := ResolveLayout(d, items)
	return LayoutPreview{r.RequestID, r.DraftGeneration, current.Revision, d, groups, tokens, current.Revision != r.ExpectedRevision}, nil
}
func (s *Layouts) change(ctx context.Context, revision string, fn func(LayoutDocument, []Widget) (LayoutDocument, error)) (LayoutState, error) {
	return s.Store.Update(ctx, revision, func(current LayoutState, items []Widget) (LayoutDocument, error) {
		d, e := fn(CloneLayout(current.Document), items)
		if e != nil {
			return d, e
		}
		e = ValidateLayout(&d, current.Document, items)
		return d, e
	})
}
func (s *Layouts) Replace(ctx context.Context, revision string, d LayoutDocument) (LayoutState, error) {
	return s.change(ctx, revision, func(LayoutDocument, []Widget) (LayoutDocument, error) { return CloneLayout(d), nil })
}
func completeOrder(o Ordering, ids []string) error {
	if o.Mode != "alphabetical" && o.Mode != "custom" || o.IDs == nil || o.Mode == "alphabetical" && len(o.IDs) > 0 {
		return LayoutFailure(422, "invalid_layout", "Invalid ordering rule.")
	}
	if o.Mode == "alphabetical" {
		return nil
	}
	seen := map[string]bool{}
	for _, id := range o.IDs {
		if seen[id] {
			return LayoutFailure(422, "invalid_layout", "Repeated ordering ID.")
		}
		seen[id] = true
	}
	if len(seen) != len(ids) {
		return LayoutFailure(409, "membership_changed", "Submit every current member exactly once.")
	}
	for _, id := range ids {
		if !seen[id] {
			return LayoutFailure(409, "membership_changed", "Submit every current member exactly once.")
		}
	}
	return nil
}
func (s *Layouts) OrderGroups(ctx context.Context, revision string, o Ordering) (LayoutState, error) {
	return s.change(ctx, revision, func(d LayoutDocument, _ []Widget) (LayoutDocument, error) {
		ids := []string{}
		for _, g := range d.Groups {
			ids = append(ids, g.ID)
		}
		if e := completeOrder(o, ids); e != nil {
			return d, e
		}
		d.Homepage.Ordering = o
		return d, nil
	})
}
func memberIDs(g ResolvedGroup) []string {
	ids := make([]string, 0, len(g.Members))
	for _, m := range g.Members {
		ids = append(ids, m.ItemID)
	}
	return ids
}
func (s *Layouts) OrderItems(ctx context.Context, revision, group, token string, o Ordering) (LayoutState, error) {
	return s.change(ctx, revision, func(d LayoutDocument, items []Widget) (LayoutDocument, error) {
		groups, tokens := ResolveLayout(d, items)
		for i, g := range d.Groups {
			if g.ID != group {
				continue
			}
			if tokens[group] != token {
				return d, LayoutFailure(409, "membership_changed", "Group membership changed. Refresh the preview.")
			}
			var ids []string
			for _, r := range groups {
				if r.GroupID == group {
					ids = memberIDs(r)
				}
			}
			if e := completeOrder(o, ids); e != nil {
				return d, e
			}
			if o.Mode == "custom" {
				d.Groups[i].Placements = []GroupPlacement{}
				for _, id := range ids {
					d.Groups[i].Placements = append(d.Groups[i].Placements, GroupPlacement{id})
				}
			}
			d.Groups[i].Ordering = o
			return d, nil
		}
		return d, LayoutFailure(404, "not_found", "Group not found.")
	})
}
func (s *Layouts) Move(ctx context.Context, revision, item string, m LayoutMove) (LayoutState, error) {
	return s.change(ctx, revision, func(d LayoutDocument, items []Widget) (LayoutDocument, error) {
		groups, _ := ResolveLayout(d, items)
		source := ""
		destination := -1
		var destIDs []string
		for _, g := range groups {
			for _, r := range g.Members {
				if r.ItemID == item {
					source = g.GroupID
				}
			}
			if g.GroupID == m.GroupID {
				destIDs = memberIDs(g)
			}
		}
		for i, g := range d.Groups {
			if g.ID == m.GroupID {
				destination = i
			}
		}
		if source == "" || destination < 0 {
			return d, LayoutFailure(404, "not_found", "Widget or destination group not found.")
		}
		if source == m.GroupID && len(m.BeforeItemID) == 0 {
			return d, nil
		}
		var before *string
		if len(m.BeforeItemID) > 0 {
			if e := json.Unmarshal(m.BeforeItemID, &before); e != nil {
				return d, LayoutFailure(400, "invalid_request", "beforeItemId must be a string or null.")
			}
		}
		destIDs = slices.DeleteFunc(destIDs, func(id string) bool { return id == item })
		at := len(destIDs)
		if before != nil {
			at = slices.Index(destIDs, *before)
			if at < 0 {
				return d, LayoutFailure(422, "invalid_layout", "beforeItemId must name another destination member.")
			}
		}
		for i := range d.Groups {
			d.Groups[i].Placements = slices.DeleteFunc(d.Groups[i].Placements, func(p GroupPlacement) bool { return p.ItemID == item })
			d.Groups[i].Ordering.IDs = slices.DeleteFunc(d.Groups[i].Ordering.IDs, func(id string) bool { return id == item })
		}
		if len(m.BeforeItemID) == 0 {
			d.Groups[destination].Placements = append(d.Groups[destination].Placements, GroupPlacement{item})
		} else {
			destIDs = slices.Insert(destIDs, at, item)
			d.Groups[destination].Placements = []GroupPlacement{}
			for _, id := range destIDs {
				d.Groups[destination].Placements = append(d.Groups[destination].Placements, GroupPlacement{id})
			}
			d.Groups[destination].Ordering = Ordering{"custom", destIDs}
		}
		return d, nil
	})
}
