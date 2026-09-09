package bff

import (
	"context"
	"fmt"
	"net/http"

	api "github.com/araihu/balemoh/client"
)

// Catalog is the small upstream surface required by the server-rendered UI.
// Keeping this interface here makes the BFF testable without coupling views to
// the generated SDK or to the API server's internal packages.
type Catalog interface {
	Lifecycle(context.Context, string, string) error
	Homepage(context.Context) ([]api.ServiceCandidate, error)
	Staging(context.Context) ([]api.ServiceCandidate, error)
	Edit(context.Context, string, api.ServiceEdit) error
	Pin(context.Context, string) error
	Unpin(context.Context, string) error
	Sync(context.Context) (api.DiscoverySyncResponse, error)
}

// APIClient adapts the generated OpenAPI client to the UI's narrow catalog
// interface. It intentionally returns safe operation errors rather than raw
// upstream response bodies.
type APIClient struct {
	client *api.ClientWithResponses
}

func NewAPIClient(baseURL string, httpClient *http.Client) (*APIClient, error) {
	options := make([]api.ClientOption, 0, 1)
	if httpClient != nil {
		options = append(options, api.WithHTTPClient(httpClient))
	}
	generated, err := api.NewClientWithResponses(baseURL, options...)
	if err != nil {
		return nil, fmt.Errorf("create API client: %w", err)
	}
	return &APIClient{client: generated}, nil
}

func (c *APIClient) Homepage(ctx context.Context) ([]api.ServiceCandidate, error) {
	response, err := c.client.GetHomepageServicesWithResponse(ctx)
	if err != nil {
		return nil, &upstreamError{operation: "homepage", cause: err}
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, &upstreamError{operation: "homepage", status: response.StatusCode()}
	}
	return response.JSON200.Services, nil
}

func (c *APIClient) Staging(ctx context.Context) ([]api.ServiceCandidate, error) {
	response, err := c.client.GetStagingServicesWithResponse(ctx)
	if err != nil {
		return nil, &upstreamError{operation: "staging", cause: err}
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, &upstreamError{operation: "staging", status: response.StatusCode()}
	}
	return response.JSON200.Services, nil
}

func (c *APIClient) Pin(ctx context.Context, id string) error {
	response, err := c.client.PinStagingServiceWithResponse(ctx, id)
	if err != nil {
		return &upstreamError{operation: "pin", cause: err}
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return &upstreamError{operation: "pin", status: response.StatusCode()}
	}
	return nil
}

func (c *APIClient) Unpin(ctx context.Context, id string) error {
	response, err := c.client.UnpinStagingServiceWithResponse(ctx, id)
	if err != nil {
		return &upstreamError{operation: "unpin", cause: err}
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return &upstreamError{operation: "unpin", status: response.StatusCode()}
	}
	return nil
}

func (c *APIClient) Sync(ctx context.Context) (api.DiscoverySyncResponse, error) {
	response, err := c.client.SyncDiscoveryWithResponse(ctx)
	if err != nil {
		return api.DiscoverySyncResponse{}, &upstreamError{operation: "sync", cause: err}
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return api.DiscoverySyncResponse{}, &upstreamError{operation: "sync", status: response.StatusCode()}
	}
	return *response.JSON200, nil
}

type upstreamError struct {
	operation string
	status    int
	cause     error
}

func (e *upstreamError) Error() string {
	if e.status > 0 {
		return fmt.Sprintf("upstream %s returned status %d", e.operation, e.status)
	}
	return fmt.Sprintf("upstream %s request failed", e.operation)
}

func (e *upstreamError) Unwrap() error { return e.cause }

func (c *APIClient) Edit(ctx context.Context, id string, edit api.ServiceEdit) error {
	response, err := c.client.EditStagingServiceWithResponse(ctx, id, edit)
	if err != nil {
		return &upstreamError{operation: "edit", cause: err}
	}
	if response.StatusCode() != http.StatusNoContent {
		return &upstreamError{operation: "edit", status: response.StatusCode()}
	}
	return nil
}

// IconCatalog exposes upload management without making bundled assets mutable.
type IconCatalog interface {
	Icons(context.Context) ([]api.UploadedIcon, error)
	SaveIcon(context.Context, string, api.IconUpload) (api.UploadedIcon, error)
	DeleteIcon(context.Context, string, string) error
	IconImage(context.Context, string) ([]byte, string, error)
}

func (c *APIClient) Icons(ctx context.Context) ([]api.UploadedIcon, error) {
	r, err := c.client.ListIconsWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if r.StatusCode() != 200 || r.JSON200 == nil {
		return nil, &upstreamError{operation: "icons", status: r.StatusCode()}
	}
	return *r.JSON200, nil
}
func (c *APIClient) SaveIcon(ctx context.Context, id string, v api.IconUpload) (api.UploadedIcon, error) {
	if id == "" {
		r, err := c.client.UploadIconWithResponse(ctx, v)
		if err != nil {
			return api.UploadedIcon{}, err
		}
		if r.StatusCode() != 201 || r.JSON201 == nil {
			return api.UploadedIcon{}, &upstreamError{operation: "upload icon", status: r.StatusCode()}
		}
		return *r.JSON201, nil
	}
	r, err := c.client.UpdateIconWithResponse(ctx, id, v)
	if err != nil {
		return api.UploadedIcon{}, err
	}
	if r.StatusCode() != 200 || r.JSON200 == nil {
		return api.UploadedIcon{}, &upstreamError{operation: "update icon", status: r.StatusCode()}
	}
	return *r.JSON200, nil
}
func (c *APIClient) DeleteIcon(ctx context.Context, id, revision string) error {
	r, err := c.client.DeleteIconWithResponse(ctx, id, &api.DeleteIconParams{Digest: revision})
	if err != nil {
		return err
	}
	if r.StatusCode() != 204 {
		return &upstreamError{operation: "delete icon", status: r.StatusCode()}
	}
	return nil
}
func (c *APIClient) IconImage(ctx context.Context, id string) ([]byte, string, error) {
	r, err := c.client.GetIconImageWithResponse(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if r.StatusCode() != 200 {
		return nil, "", &upstreamError{operation: "icon image", status: r.StatusCode()}
	}
	return r.Body, r.HTTPResponse.Header.Get("Content-Type"), nil
}

func (c *APIClient) Lifecycle(ctx context.Context, id, action string) error {
	r, err := c.client.ChangeServiceLifecycleWithResponse(ctx, id, api.ChangeServiceLifecycleJSONRequestBody{Action: api.ChangeServiceLifecycleJSONBodyAction(action)})
	if err != nil {
		return &upstreamError{operation: "lifecycle", cause: err}
	}
	if r.StatusCode() != http.StatusNoContent {
		return &upstreamError{operation: "lifecycle", status: r.StatusCode()}
	}
	return nil
}
