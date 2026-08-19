package federation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
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
	return NewPublisherWithOptions(gatewayURL, token, false)
}

// NewPublisherWithOptions constructs a publisher. Plain HTTP is accepted only
// when explicitly enabled for a loopback development gateway.
func NewPublisherWithOptions(gatewayURL, token string, allowInsecureHTTP bool) (*Publisher, error) {
	gatewayURL = strings.TrimSpace(gatewayURL)
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("federation token must not be empty")
	}
	parsed, err := url.Parse(gatewayURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("federation gateway URL must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	if parsed.Scheme == "http" {
		if !allowInsecureHTTP {
			return nil, errors.New("federation gateway URL must use HTTPS unless insecure HTTP is explicitly enabled")
		}
		if !isLoopbackHost(parsed.Hostname()) {
			return nil, errors.New("insecure federation HTTP is restricted to a loopback gateway")
		}
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

func isLoopbackHost(hostname string) bool {
	if strings.EqualFold(strings.TrimSpace(hostname), "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(hostname, "[]"))
	return ip != nil && ip.IsLoopback()
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
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("federation gateway returned HTTP status %d", response.StatusCode)
	}
	var result generated.DiscoverySyncResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&result); err != nil {
		return errors.New("federation gateway returned an invalid synchronization response")
	}
	if result.Sources != 1 || result.Candidates != int32(len(snapshot.Candidates)) {
		return fmt.Errorf("federation gateway acknowledged sources=%d candidates=%d, want sources=1 candidates=%d", result.Sources, result.Candidates, len(snapshot.Candidates))
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
