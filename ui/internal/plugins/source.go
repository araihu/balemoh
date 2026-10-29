package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/credential"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

// Source owns the private BFF credential. Only scoped data callbacks reach UI children.
type Source struct {
	client *api.Client
	// Services is configured once, before OpenData starts any children.
	Services ui.ServiceCatalog
}

func NewSource(baseURL, tokenPath string, timeout time.Duration) (*Source, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid plugin data API URL")
	}
	token, err := credential.ReadBearerFile(tokenPath)
	if err != nil || len(token) < 32 {
		return nil, errors.New("invalid plugin service credential")
	}

	if timeout <= 0 || timeout > 5*time.Second {
		timeout = 5 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	client, err := api.NewClient(baseURL, api.WithHTTPClient(&http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}), api.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
		r.Header.Set("Authorization", "Bearer "+token)
		return nil
	}))
	if err != nil {
		return nil, errors.New("cannot create plugin data API client")
	}
	return &Source{client: client}, nil
}
func readResponse(response *http.Response, target any) error {
	defer response.Body.Close()
	if response.StatusCode != 200 {
		code := map[int]string{400: "invalid_argument", 401: "permission_denied", 403: "permission_denied", 404: "not_found", 409: "conflict", 412: "conflict", 428: "conflict", 429: "resource_exhausted", 502: "invalid_response", 503: "unavailable", 504: "deadline_exceeded"}[response.StatusCode]
		return backend.Fail(code)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, backend.MaxResult+(64<<10)+1))
	if err != nil || len(data) > backend.MaxResult+(64<<10) {
		return backend.Fail("invalid_response")
	}
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.UseNumber()
	if d.Decode(target) != nil || d.Decode(new(any)) != io.EOF {
		return backend.Fail("invalid_response")
	}
	return nil
}
func (s *Source) descriptors(ctx context.Context) ([]api.PluginDescriptor, error) {
	response, err := s.client.GetPlugins(ctx)
	if err != nil {
		return nil, backend.Fail("unavailable")
	}
	var result api.PluginCatalog
	if err = readResponse(response, &result); err != nil {
		return nil, err
	}
	if result.Plugins == nil || result.Problems == nil || len(result.Plugins) > 16 {
		return nil, backend.Fail("invalid_response")
	}
	return result.Plugins, nil
}
func (s *Source) query(ctx context.Context, plugin, connection, operation string, input json.RawMessage, requestIDs ...string) (ui.QueryResult, error) {
	body, _ := json.Marshal(struct {
		Input json.RawMessage `json:"input"`
	}{input})
	response, err := s.client.QueryPluginWithBody(ctx, plugin, connection, operation, "application/json", strings.NewReader(string(body)), func(_ context.Context, r *http.Request) error {
		if len(requestIDs) > 0 {
			r.Header.Set("X-Request-ID", requestIDs[0])
		}
		return nil
	})
	if err != nil {
		return ui.QueryResult{}, backend.Fail(backend.Code(err))
	}
	var value api.PluginQueryResult
	if err = readResponse(response, &value); err != nil {
		return ui.QueryResult{}, err
	}
	data, err := json.Marshal(value.Data)
	if err != nil {
		return ui.QueryResult{}, backend.Fail("invalid_response")
	}
	var sourceError *backend.Error
	if value.SourceError != nil {
		sourceError = backend.Fail(value.SourceError.Code)
	}
	result := ui.QueryResult{Data: data, State: string(value.State), ObservationRevision: value.ObservationRevision, ObservedAt: value.ObservedAt, LastSuccessAt: value.LastSuccessAt, SourceError: sourceError}
	return result, result.Validate()
}

type scopedServices struct {
	source    *Source
	entry     *Entry
	placement ui.Placement
	grants    map[string]bool
	requestID string
}

func (h scopedServices) ResolveServices(ctx context.Context, ids []string) ([]ui.ServiceIdentity, error) {
	if ui.ValidateServiceIDs(ids) != nil {
		return nil, backend.Fail("invalid_argument")
	}
	if !h.entry.Package.Manifest.Declares(ui.ServiceCatalogCapability) || h.source.Services == nil {
		return nil, backend.Fail("permission_denied")
	}
	for _, id := range ids {
		if !h.entry.allowsServiceID(id) {
			return nil, backend.Fail("permission_denied")
		}
	}
	return h.source.Services.ResolveServices(ctx, ids)
}

func (h scopedServices) Query(ctx context.Context, operation string, input json.RawMessage) (ui.QueryResult, error) {
	q, ok := h.entry.Package.Manifest.Query(operation)
	if !ok || !h.grants[operation] {
		return ui.QueryResult{}, backend.Fail("permission_denied")
	}
	if !backend.Object(input, backend.MaxInput) || h.entry.Package.ValidateJSON(q.InputSchema, input) != nil {
		return ui.QueryResult{}, backend.Fail("invalid_argument")
	}
	result, err := h.source.query(ctx, h.entry.Package.Manifest.ID, h.entry.Installation.ConnectionID, operation, input, h.requestID)
	if err != nil {
		return ui.QueryResult{}, err
	}
	if h.entry.Package.ValidateJSON(q.OutputSchema, result.Data) != nil {
		return ui.QueryResult{}, backend.Fail("invalid_response")
	}
	return result, nil
}
func (h scopedServices) Placement(context.Context) (ui.Placement, error) {
	if h.placement.ID == "" {
		return ui.Placement{}, backend.Fail("not_found")
	}
	return h.placement, nil
}

// Resolve current grants only inside an isolated content request. Configure may
// still be starting when the BFF first discovers a connection UUID.
func (s *Source) grants(ctx context.Context, e *Entry) (map[string]bool, error) {
	grants, _, err := s.granted(ctx, e)
	return grants, err
}
func (s *Source) granted(ctx context.Context, e *Entry) (map[string]bool, api.PluginConnection, error) {
	result := map[string]bool{}
	descriptors, err := s.descriptors(ctx)
	if err != nil {
		return nil, api.PluginConnection{}, err
	}
	for _, descriptor := range descriptors {
		if descriptor.Id != e.Package.Manifest.ID || descriptor.ManifestDigest != e.Package.Digest {
			continue
		}
		for _, connection := range descriptor.Connections {
			if connection.Id != e.Installation.ConnectionID || !connection.Enabled || !connection.Ready || connection.AppliedRevision == nil || *connection.AppliedRevision != connection.Revision {
				continue
			}
			for _, query := range connection.Queries {
				result[query] = true
			}
			return result, connection, nil
		}
	}
	return nil, api.PluginConnection{}, backend.Fail("unavailable")
}
