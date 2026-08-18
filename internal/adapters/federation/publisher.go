package federation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/catalog"
)

const snapshotPath = "/api/v1/federation/snapshots"

// Publisher sends local discovery observations to a configured Balemoh gateway.
// It never sends local pin state.
type Publisher struct {
	endpoint *url.URL
	token    string
	client   *http.Client
}

func NewPublisher(gatewayURL, token string) (*Publisher, error) {
	gatewayURL = strings.TrimSpace(gatewayURL)
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("federation token must not be empty")
	}
	parsed, err := url.Parse(gatewayURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("federation gateway URL must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + snapshotPath
	return &Publisher{
		endpoint: parsed,
		token:    token,
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (p *Publisher) Publish(ctx context.Context, snapshot catalog.Snapshot) error {
	if p == nil || p.endpoint == nil || p.client == nil {
		return errors.New("federation publisher is not configured")
	}
	snapshot = snapshot.Normalize()
	if err := snapshot.Validate(); err != nil {
		return fmt.Errorf("validate federation snapshot: %w", err)
	}
	payload := federationSnapshot(snapshot)
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return fmt.Errorf("encode federation snapshot: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint.String(), &body)
	if err != nil {
		return fmt.Errorf("create federation request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+p.token)
	response, err := p.client.Do(request)
	if err != nil {
		return fmt.Errorf("send federation snapshot: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("federation gateway returned HTTP status %d", response.StatusCode)
	}
	return nil
}

func federationSnapshot(snapshot catalog.Snapshot) generated.FederationSnapshot {
	candidates := make([]generated.FederatedCandidate, 0, len(snapshot.Candidates))
	for _, candidate := range snapshot.Candidates {
		candidates = append(candidates, federatedCandidate(candidate))
	}
	return generated.FederationSnapshot{
		Source:     generated.SourceRef{Kind: snapshot.Source.Kind, Id: snapshot.Source.ID},
		Candidates: candidates,
		ObservedAt: snapshot.ObservedAt.UTC(),
	}
}

func federatedCandidate(candidate catalog.Candidate) generated.FederatedCandidate {
	metadata := make(map[string]string, len(candidate.Metadata))
	for key, value := range candidate.Metadata {
		metadata[key] = value
	}
	endpoints := make([]generated.ServiceEndpoint, 0, len(candidate.Endpoints))
	for _, endpoint := range candidate.Endpoints {
		endpoints = append(endpoints, generated.ServiceEndpoint{
			Name:       endpoint.Name,
			Url:        endpoint.URL,
			Port:       int32(endpoint.Port),
			Protocol:   endpoint.Protocol,
			Provenance: endpoint.Provenance,
		})
	}
	resource := generated.ResourceRef{Kind: candidate.Resource.Kind, Name: candidate.Resource.Name}
	if candidate.Resource.Namespace != "" {
		namespace := candidate.Resource.Namespace
		resource.Namespace = &namespace
	}
	return generated.FederatedCandidate{
		Id:          candidate.ID,
		Source:      generated.SourceRef{Kind: candidate.Source.Kind, Id: candidate.Source.ID},
		Resource:    resource,
		DisplayName: candidate.DisplayName,
		Description: candidate.Description,
		Metadata:    metadata,
		Endpoints:   endpoints,
		Images:      append([]string(nil), candidate.Images...),
		ObservedAt:  candidate.ObservedAt.UTC(),
	}
}

var _ catalog.SnapshotPublisher = (*Publisher)(nil)
