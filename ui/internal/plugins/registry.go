// Package plugins owns trusted UI children and per-invocation data grants.
package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"uuid"

	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	pluginruntime "github.com/araihu/balemoh/plugin-sdk/runtime"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

type Config struct {
	Installations []Installation `json:"installations"`
}
type Installation struct {
	ServiceIDs     []string    `json:"serviceIDs,omitempty"`
	Package        string      `json:"package"`
	ConnectionID   string      `json:"connectionId,omitempty"`
	Name           string      `json:"name,omitempty"`
	ConnectionKey  string      `json:"connectionKey,omitempty"`
	ManifestDigest string      `json:"manifestDigest,omitempty"`
	PackageDigest  string      `json:"packageDigest,omitempty"`
	Widgets        []Placement `json:"previewWidgets"`
}
type Placement struct {
	ID         string `json:"id"`
	Definition string `json:"definition"`
}
type uiProcess struct {
	mu      sync.RWMutex
	client  *pluginruntime.Client
	problem error
}
type Entry struct {
	Installation   Installation
	Package        *manifest.Package
	Problem        error
	Compatible     bool
	ConnectionName string
	process        *uiProcess
}

// Client returns a synchronized snapshot of the owned child, or nil while starting.
func (e *Entry) Client() *pluginruntime.Client {
	if e.process == nil {
		return nil
	}
	e.process.mu.RLock()
	defer e.process.mu.RUnlock()
	return e.process.client
}

type Registry struct {
	mu      sync.RWMutex
	entries []*Entry
	cache   map[string]cachedRender
	source  *Source
	cancel  context.CancelFunc
	ready   chan struct{}
	wg      sync.WaitGroup
	once    sync.Once
}
type Navigation struct {
	ID, Title, Href, Icon string
	Children              []Navigation
}
type Reference struct {
	PluginID       string `json:"pluginId"`
	ConnectionID   string `json:"connectionId"`
	ContributionID string `json:"contributionId"`
	PlacementID    string `json:"placementId"`
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Open reads explicit development-only placements; it never installs or pins.
func Open(ctx context.Context, configPath string) (*Registry, error) {
	return open(ctx, configPath, nil)
}

// OpenData resolves host-issued connection identities before mounting contributions.
func OpenData(ctx context.Context, configPath string, source *Source) (*Registry, error) {
	if source == nil {
		return nil, errors.New("plugin data source is required")
	}
	return open(ctx, configPath, source)
}
func open(ctx context.Context, configPath string, source *Source) (*Registry, error) {
	f, err := os.Open(configPath)
	if err != nil {
		return nil, errors.New("cannot read plugin configuration")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil || len(data) > 64<<10 {
		return nil, errors.New("plugin configuration exceeds limit")
	}
	var config Config
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if d.Decode(&config) != nil || d.Decode(new(any)) != io.EOF || config.Installations == nil || len(config.Installations) > 16 {
		return nil, errors.New("invalid plugin configuration")
	}
	lifetime, cancel := context.WithCancel(ctx)
	r := &Registry{source: source, cancel: cancel, ready: make(chan struct{})}
	r.wg.Add(1)
	go r.initialize(lifetime, configPath, config)
	return r, nil
}

// initialize performs optional discovery, pinning and child startup after Open
// returns. Core listeners never wait for plugin executables or the private API.
func (r *Registry) initialize(ctx context.Context, configPath string, config Config) {
	defer r.wg.Done()
	defer close(r.ready)
	var descriptors []api.PluginDescriptor
	var discoveryErr error
	if r.source != nil {
		for attempt := 0; attempt < 6; attempt++ {
			descriptors, discoveryErr = r.source.descriptors(ctx)
			if discoveryErr == nil {
				break
			}
			if attempt == 5 {
				break
			}
			timer := time.NewTimer(time.Second << attempt)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
	children := map[string]*Entry{}
	connections := map[string]bool{}
	placements := map[string]bool{}
	slots := make(chan struct{}, 4)
	var starts sync.WaitGroup
	publish := func(e *Entry) { r.mu.Lock(); r.entries = append(r.entries, e); r.mu.Unlock() }
	for _, installation := range config.Installations {
		if ctx.Err() != nil {
			break
		}
		e := &Entry{Installation: installation}
		if installation.Package == "" || len(installation.Widgets) > 16 || (r.source == nil && !uuidPattern.MatchString(installation.ConnectionID)) || (r.source != nil && (installation.ConnectionKey == "" || installation.ConnectionID != "")) {
			e.Problem = errors.New("invalid plugin connection configuration")
			publish(e)
			continue
		}
		if !filepath.IsAbs(installation.Package) {
			installation.Package = filepath.Join(filepath.Dir(configPath), installation.Package)
		}
		absolute, err := filepath.Abs(installation.Package)
		if err != nil {
			e.Problem = errors.New("invalid plugin package path")
			publish(e)
			continue
		}
		installation.Package = absolute
		e.Installation = installation
		if len(installation.ServiceIDs) > 0 && ui.ValidateServiceIDs(installation.ServiceIDs) != nil {
			e.Problem = errors.New("invalid service ID grants")
			publish(e)
			continue
		}
		p, err := manifest.Load(installation.Package)
		if err != nil {
			e.Problem = errors.New("invalid or incompatible plugin package")
			publish(e)
			continue
		}
		e.Package = p
		if pluginruntime.Supported(p.Manifest) != nil {
			e.Problem = errors.New("incompatible plugin capabilities")
			publish(e)
			continue
		}
		valid := true
		for _, w := range installation.Widgets {
			if !uuidPattern.MatchString(w.ID) || placements[w.ID] || len(placements) >= 128 {
				valid = false
				break
			}
			if _, ok := p.Manifest.Widget(w.Definition); !ok {
				valid = false
				break
			}
			placements[w.ID] = true
		}
		if !valid {
			e.Problem = errors.New("invalid plugin widget references")
			publish(e)
			continue
		}
		e.ConnectionName = installation.Name
		if e.ConnectionName == "" {
			e.ConnectionName = p.Manifest.Name
		}
		if r.source != nil {
			found := false
			if discoveryErr == nil {
				for _, descriptor := range descriptors {
					if descriptor.Id != p.Manifest.ID {
						continue
					}
					for _, connection := range descriptor.Connections {
						if connection.Key == installation.ConnectionKey && connection.Enabled && uuidPattern.MatchString(connection.Id) {
							installation.ConnectionID = connection.Id
							e.ConnectionName = connection.Name
							found = true
							if descriptor.ManifestDigest != p.Digest {
								e.Problem = errors.New("plugin API and UI manifests differ")
							}
						}
					}
				}
			}
			if !found {
				e.Problem = errors.New("plugin API connection missing, unavailable or disabled")
				publish(e)
				continue
			}
			e.Installation = installation
			digest, digestErr := p.ContentDigest()
			if digestErr != nil || installation.ManifestDigest != p.Digest || installation.PackageDigest != digest {
				e.Problem = errors.New("plugin UI package pin mismatch")
			}
		}
		key := p.Manifest.ID + "/" + installation.ConnectionID
		if connections[key] {
			e.Problem = errors.New("duplicate plugin connection")
			publish(e)
			continue
		}
		connections[key] = true
		// Validated metadata can mount host-owned placeholders even if its package pin
		// fails. No executable or rendered HTML from that entry is ever accepted.
		e.Compatible = true
		if e.Problem == nil {
			if prior := children[p.Manifest.ID]; prior != nil {
				if prior.Package.Root != p.Root || prior.Package.Digest != p.Digest {
					e.Problem = errors.New("conflicting plugin packages")
				} else {
					e.process = prior.process
				}
			} else {
				process := &uiProcess{}
				e.process = process
				children[p.Manifest.ID] = e
				starts.Add(1)
				r.wg.Add(1)
				go func() {
					defer r.wg.Done()
					var first sync.Once
					defer first.Do(starts.Done)
					pluginruntime.Supervise(ctx, func(lifetime context.Context) (*pluginruntime.Client, error) {
						select {
						case slots <- struct{}{}:
							defer func() { <-slots }()
						case <-lifetime.Done():
							return nil, lifetime.Err()
						}
						client, err := pluginruntime.Open(lifetime, p)
						if err != nil {
							process.mu.Lock()
							process.problem = errors.New("plugin UI startup failed")
							process.mu.Unlock()
							first.Do(starts.Done)
						}
						return client, err
					}, func(client *pluginruntime.Client) {
						process.mu.Lock()
						process.client = client
						process.problem = nil
						process.mu.Unlock()
						first.Do(starts.Done)
					}, func(client *pluginruntime.Client) {
						process.mu.Lock()
						process.client = nil
						process.problem = errors.New("plugin UI unavailable")
						process.mu.Unlock()
					}, pluginruntime.StateLogger(p.Manifest.ID, "ui"))
				}()
			}
		}
		publish(e)
	}
	starts.Wait()
}

// WaitReady is for explicit development/test coordination, never base rendering.
func (r *Registry) WaitReady(ctx context.Context) error {
	select {
	case <-r.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (r *Registry) Close() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		r.cancel()
		r.wg.Wait()
		seen := map[*pluginruntime.Client]bool{}
		for _, e := range r.Entries() {
			client := e.Client()
			if client != nil && !seen[client] {
				seen[client] = true
				client.Close()
			}
		}
	})
}
func (r *Registry) Problems() int {
	if r == nil {
		return 0
	}
	n := 0
	for _, e := range r.Entries() {
		if e.Problem != nil {
			n++
			continue
		}
		if e.process != nil {
			e.process.mu.RLock()
			if e.process.problem != nil || e.process.client != nil && e.process.client.Exited() {
				n++
			}
			e.process.mu.RUnlock()
		}
	}
	return n
}
func (r *Registry) Entries() []*Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]*Entry(nil), r.entries...)
}
func (r *Registry) find(plugin, connection string) *Entry {
	if r == nil {
		return nil
	}
	for _, e := range r.Entries() {
		if e.Compatible && e.Package != nil && e.Package.Manifest.ID == plugin && e.Installation.ConnectionID == connection {
			return e
		}
	}
	return nil
}

func (r *Registry) Navigation() []Navigation {
	var items []Navigation
	if r == nil {
		return items
	}
	entries := r.Entries()
	for _, e := range entries {
		if !e.Compatible || e.Package == nil {
			continue
		}
		var walk func([]manifest.Navigation) []Navigation
		walk = func(ns []manifest.Navigation) []Navigation {
			var out []Navigation
			for _, n := range ns {
				v := Navigation{ID: NavID(e.Package.Manifest.ID, e.Installation.ConnectionID, n.ID), Title: n.Title, Icon: n.Icon}
				if n.Page != "" {
					if !pluginruntime.Available(e.Package.Manifest, "page", n.Page) {
						continue
					}
					req := request(e, "page", n.Page, "")
					href, err := req.PageURL(e.Package.Manifest, n.Page, nil)
					if err != nil {
						continue
					}
					v.Href = href
				} else {
					v.Children = walk(n.Children)
					if len(v.Children) == 0 {
						continue
					}
				}
				out = append(out, v)
			}
			return out
		}
		group := walk(e.Package.Manifest.Navigation)
		if len(group) == 0 {
			continue
		}
		if len(entries) > 1 {
			items = append(items, Navigation{ID: NavID(e.Package.Manifest.ID, e.Installation.ConnectionID, "connection"), Title: e.ConnectionName, Children: group})
		} else {
			items = append(items, group...)
		}
	}
	if r.source != nil {
		items = append(items, Navigation{ID: "nav-plugin-widgets", Title: "Plugins/Widgets", Icon: "hi-16-solid-cog-6-tooth", Href: "/plugins/widgets"})
	}
	return items
}
func NavID(plugin, connection, local string) string {
	sum := sha256.Sum256([]byte(plugin + "\x00" + connection + "\x00" + local))
	return "nav-plugin-" + hex.EncodeToString(sum[:12])
}

func (r *Registry) Widgets() []api.DashboardWidget {
	items := []api.DashboardWidget{}
	if r == nil {
		return items
	}
	for _, e := range r.Entries() {
		if !e.Compatible || e.Package == nil {
			continue
		}
		for _, p := range e.Installation.Widgets {
			if !pluginruntime.Available(e.Package.Manifest, "widget", p.Definition) {
				continue
			}
			w, _ := e.Package.Manifest.Widget(p.Definition)
			raw, _ := json.Marshal(Reference{e.Package.Manifest.ID, e.Installation.ConnectionID, p.Definition, p.ID})
			data := map[string]any{}
			_ = json.Unmarshal(raw, &data)
			problem := ""
			if e.Problem != nil {
				problem = "Plugin unavailable"
			}
			items = append(items, api.DashboardWidget{Error: problem, Id: "plugin-" + p.ID, SourceKind: "plugin", SourceId: p.ID, Kind: "plugin-widget", Presentation: "plugin-ui", Title: w.Title, Data: data})
		}
	}
	return items
}

var ErrNotFound = errors.New("plugin contribution not found")
var ErrUnavailable = errors.New("plugin is unavailable")

// ResolvePage reads only installed metadata. It never invokes plugin code.
func (r *Registry) ResolvePage(plugin, connection, path string, q url.Values) (manifest.Page, ui.Request, string, error) {
	e := r.find(plugin, connection)
	if e == nil {
		return manifest.Page{}, ui.Request{}, "", ErrNotFound
	}
	for _, p := range e.Package.Manifest.Pages {
		if !pluginruntime.Available(e.Package.Manifest, "page", p.ID) {
			continue
		}
		params, ok := match(p.Path, path)
		if !ok {
			continue
		}
		req := request(e, "page", p.ID, "")
		req.Query = q
		req.PathParams = params
		active := ""
		var nav func([]manifest.Navigation)
		nav = func(ns []manifest.Navigation) {
			for _, n := range ns {
				if n.Page == p.ID {
					active = NavID(plugin, connection, n.ID)
				}
				nav(n.Children)
			}
		}
		nav(e.Package.Manifest.Navigation)
		return p, req, active, nil
	}
	return manifest.Page{}, ui.Request{}, "", ErrNotFound
}
func (r *Registry) Page(ctx context.Context, plugin, connection, path string, q url.Values) (ui.Response, error) {
	_, req, _, err := r.ResolvePage(plugin, connection, path, q)
	if err != nil {
		return ui.Response{}, err
	}
	e := r.find(plugin, connection)
	if e.Client() == nil {
		return ui.Response{}, ErrUnavailable
	}
	return r.handle(ctx, e, req, "")
}

// Placement resolves identity from host configuration, never from browser data.
func (r *Registry) Placement(id string) (Reference, bool) {
	if r == nil {
		return Reference{}, false
	}
	for _, e := range r.Entries() {
		if !e.Compatible {
			continue
		}
		for _, p := range e.Installation.Widgets {
			if p.ID == id && pluginruntime.Available(e.Package.Manifest, "widget", p.Definition) {
				return Reference{e.Package.Manifest.ID, e.Installation.ConnectionID, p.Definition, p.ID}, true
			}
		}
	}
	return Reference{}, false
}
func (r *Registry) Widget(ctx context.Context, ref Reference) (ui.Response, error) {
	e := r.find(ref.PluginID, ref.ConnectionID)
	if e == nil {
		return ui.Response{}, ErrNotFound
	}
	allowed := false
	for _, p := range e.Installation.Widgets {
		if !pluginruntime.Available(e.Package.Manifest, "widget", p.Definition) {
			continue
		}
		if p.ID == ref.PlacementID && p.Definition == ref.ContributionID {
			allowed = true
		}
	}
	if !allowed {
		return ui.Response{}, ErrNotFound
	}
	if e.Client() == nil {
		return ui.Response{}, ErrUnavailable
	}
	return r.handle(ctx, e, request(e, "widget", ref.ContributionID, ref.PlacementID), ref.PlacementID)
}
func (r *Registry) handle(ctx context.Context, e *Entry, req ui.Request, placement string) (ui.Response, error) {
	req = ui.ApplyDisplay(ctx, req)
	child := e.Client()
	if child == nil {
		return ui.Response{}, ErrUnavailable
	}
	if r.source == nil {
		return child.Handle(ctx, req)
	}
	grants, err := r.source.grants(ctx, e)
	if err != nil {
		return ui.Response{}, err
	}
	return child.HandleWithServices(ctx, req, scopedServices{source: r.source, entry: e, grants: grants, requestID: req.RequestID, placement: ui.Placement{ID: placement, ContributionID: req.ContributionID, Options: json.RawMessage(`{}`), Revision: "1"}})
}
func request(e *Entry, kind, id, placement string) ui.Request {
	return ui.Request{RequestID: uuid.NewV4().String(), PackageDigest: e.Installation.PackageDigest, ConnectionID: e.Installation.ConnectionID, PlacementID: placement, ContributionType: kind, ContributionID: id, Mode: kind, Method: "GET", Locale: "en", Timezone: "UTC", Theme: "goshtoso", ColorScheme: "system", DOMNamespace: e.Package.Manifest.ID + "/" + e.Installation.ConnectionID + "/" + kind + "/" + id + "/" + placement}
}
func match(pattern, value string) (map[string]string, bool) {
	if strings.ContainsAny(value, "%\\\x00") || strings.Contains(value, "..") || len(value) > 2048 {
		return nil, false
	}
	a, b := strings.Split(strings.TrimPrefix(pattern, "/"), "/"), strings.Split(strings.TrimPrefix(value, "/"), "/")
	if len(a) != len(b) {
		return nil, false
	}
	params := map[string]string{}
	for i, s := range a {
		if strings.HasPrefix(s, "{") {
			if b[i] == "" {
				return nil, false
			}
			params[strings.TrimSuffix(strings.TrimPrefix(s, "{"), "}")] = b[i]
		} else if s != b[i] {
			return nil, false
		}
	}
	return params, true
}

func (r Reference) Validate() error {
	if !uuidPattern.MatchString(r.ConnectionID) || !uuidPattern.MatchString(r.PlacementID) || r.PluginID == "" || r.ContributionID == "" {
		return fmt.Errorf("invalid plugin widget reference")
	}
	return nil
}

// AllowsServiceID consults installation metadata only. Retained notifications
// must not depend on child processes or current source observations.
func (r *Registry) AllowsServiceID(plugin, connection, id string) bool {
	if r == nil {
		return false
	}
	e := r.find(plugin, connection)
	return e != nil && e.allowsServiceID(id)
}
func (e *Entry) allowsServiceID(id string) bool {
	return e.Package != nil && e.Problem == nil && ui.ValidServiceID(id) && e.Package.Manifest.Declares(ui.ServiceCatalogCapability) && slices.Contains(e.Installation.ServiceIDs, id)
}
