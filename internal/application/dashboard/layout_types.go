package dashboard

import (
	"context"
	"encoding/json"
	"strconv"
)

type ContainerLayout struct {
	Type    string `json:"type"`
	Columns int    `json:"columns"`
	GapPx   int    `json:"gapPx"`
}
type Ordering struct {
	Mode string   `json:"mode"`
	IDs  []string `json:"ids"`
}
type GroupPlacement struct {
	ItemID string `json:"itemId"`
}
type HomepagePlacement struct {
	GroupID string `json:"groupId"`
	Span    string `json:"span"`
}
type LayoutGroup struct {
	ID                 string           `json:"id"`
	Title              string           `json:"title"`
	Layout             ContainerLayout  `json:"layout"`
	Ordering           Ordering         `json:"ordering"`
	ShowHeading        bool             `json:"showHeading"`
	InitiallyCollapsed bool             `json:"initiallyCollapsed"`
	Placements         []GroupPlacement `json:"placements"`
}
type HomepageLayout struct {
	Width      string              `json:"width"`
	Layout     ContainerLayout     `json:"layout"`
	Ordering   Ordering            `json:"ordering"`
	Placements []HomepagePlacement `json:"placements"`
}
type SourceGroups struct {
	Host    string `json:"host"`
	Service string `json:"service"`
	Other   string `json:"other"`
}
type LayoutDocument struct {
	SchemaVersion int            `json:"schemaVersion"`
	Homepage      HomepageLayout `json:"homepage"`
	Groups        []LayoutGroup  `json:"groups"`
	SourceGroups  SourceGroups   `json:"sourceGroups"`
}
type LayoutState struct {
	Revision string         `json:"revision"`
	Document LayoutDocument `json:"document"`
}
type LayoutMember struct {
	ItemID       string `json:"itemId"`
	State        string `json:"state"`
	DisplayLabel string `json:"displayLabel"`
}
type ResolvedGroup struct {
	GroupID      string         `json:"groupId"`
	Title        string         `json:"title"`
	DisplayLabel string         `json:"displayLabel"`
	Members      []LayoutMember `json:"members"`
}
type DashboardGroup struct {
	GroupID      string   `json:"groupId"`
	Title        string   `json:"title"`
	DisplayLabel string   `json:"displayLabel"`
	ItemIDs      []string `json:"itemIds"`
}
type LayoutPreviewRequest struct {
	RequestID        string         `json:"requestId"`
	DraftGeneration  int64          `json:"draftGeneration"`
	ExpectedRevision string         `json:"expectedRevision"`
	Document         LayoutDocument `json:"document"`
}
type LayoutPreview struct {
	RequestID        string            `json:"requestId"`
	DraftGeneration  int64             `json:"draftGeneration"`
	LayoutRevision   string            `json:"layoutRevision"`
	Document         LayoutDocument    `json:"document"`
	ResolvedGroups   []ResolvedGroup   `json:"resolvedGroups"`
	MembershipTokens map[string]string `json:"membershipTokens"`
	Stale            bool              `json:"stale"`
}
type LayoutMove struct {
	GroupID      string          `json:"groupId"`
	BeforeItemID json.RawMessage `json:"beforeItemId,omitempty"`
}

// LayoutStore reads local eligibility and changes configuration in the same transaction.
type LayoutStore interface {
	Read(context.Context, bool) (LayoutState, []Widget, error)
	ReadDefault(context.Context) ([]Widget, error)
	Update(context.Context, string, func(LayoutState, []Widget) (LayoutDocument, error)) (LayoutState, error)
}
type Layouts struct{ Store LayoutStore }
type LayoutError struct {
	Status      int               `json:"-"`
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	FieldErrors map[string]string `json:"fieldErrors,omitempty"`
}

func (e *LayoutError) Error() string { return e.Message }
func LayoutFailure(status int, code, message string) *LayoutError {
	return &LayoutError{Status: status, Code: code, Message: message}
}
func ValidLayoutRevision(s string) bool {
	n, e := strconv.ParseInt(s, 10, 64)
	return e == nil && n >= 0 && strconv.FormatInt(n, 10) == s
}
func DefaultLayout() LayoutDocument {
	d := LayoutDocument{SchemaVersion: 1, Homepage: HomepageLayout{Width: "full", Layout: ContainerLayout{"stack", 1, 16}, Ordering: Ordering{"alphabetical", []string{}}, Placements: []HomepagePlacement{}}, Groups: []LayoutGroup{}, SourceGroups: SourceGroups{"builtin:hosts", "builtin:services", "builtin:widgets"}}
	for _, v := range []struct{ id, title string }{{"builtin:hosts", "Hosts"}, {"builtin:services", "Services"}, {"builtin:widgets", "Widgets"}} {
		d.Groups = append(d.Groups, LayoutGroup{ID: v.id, Title: v.title, Layout: ContainerLayout{"masonry", 3, 16}, Ordering: Ordering{"alphabetical", []string{}}, ShowHeading: true, Placements: []GroupPlacement{}})
		d.Homepage.Placements = append(d.Homepage.Placements, HomepagePlacement{v.id, "auto"})
	}
	return d
}
func CloneLayout(d LayoutDocument) LayoutDocument {
	b, _ := json.Marshal(d)
	var out LayoutDocument
	_ = json.Unmarshal(b, &out)
	return out
}
