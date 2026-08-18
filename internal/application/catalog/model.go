package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var ErrNotFound = errors.New("catalog candidate not found")

type SourceRef struct {
	Kind string
	ID   string
}

type ResourceRef struct {
	Kind      string
	Namespace string
	Name      string
}

type Endpoint struct {
	Name       string
	URL        string
	Port       int
	Protocol   string
	Provenance string
}

type Candidate struct {
	ID          string
	Source      SourceRef
	Resource    ResourceRef
	DisplayName string
	Description string
	Metadata    map[string]string
	Endpoints   []Endpoint
	Images      []string
	ObservedAt  time.Time
	PinnedAt    *time.Time
}

func StableID(source SourceRef, resource ResourceRef) string {
	parts := []string{
		strings.TrimSpace(source.Kind),
		strings.TrimSpace(source.ID),
		strings.TrimSpace(resource.Kind),
		strings.TrimSpace(resource.Namespace),
		strings.TrimSpace(resource.Name),
	}
	var identity strings.Builder
	for _, part := range parts {
		_, _ = fmt.Fprintf(&identity, "%d:", len(part))
		identity.WriteString(part)
	}
	digest := sha256.Sum256([]byte(identity.String()))
	return hex.EncodeToString(digest[:])
}

func NewCandidate(source SourceRef, resource ResourceRef, observedAt time.Time) Candidate {
	source = SourceRef{
		Kind: strings.TrimSpace(source.Kind),
		ID:   strings.TrimSpace(source.ID),
	}
	resource = ResourceRef{
		Kind:      strings.TrimSpace(resource.Kind),
		Namespace: strings.TrimSpace(resource.Namespace),
		Name:      strings.TrimSpace(resource.Name),
	}
	return Candidate{
		ID:          StableID(source, resource),
		Source:      source,
		Resource:    resource,
		DisplayName: resource.Name,
		Metadata:    map[string]string{},
		Endpoints:   []Endpoint{},
		Images:      []string{},
		ObservedAt:  observedAt.UTC(),
	}
}

func (c Candidate) Normalize() Candidate {
	if c.Metadata == nil {
		c.Metadata = map[string]string{}
	}
	if c.Endpoints == nil {
		c.Endpoints = []Endpoint{}
	}
	if c.Images == nil {
		c.Images = []string{}
	}
	c.Source.Kind = strings.TrimSpace(c.Source.Kind)
	c.Source.ID = strings.TrimSpace(c.Source.ID)
	c.Resource.Kind = strings.TrimSpace(c.Resource.Kind)
	c.Resource.Namespace = strings.TrimSpace(c.Resource.Namespace)
	c.Resource.Name = strings.TrimSpace(c.Resource.Name)
	c.DisplayName = strings.TrimSpace(c.DisplayName)
	c.ObservedAt = c.ObservedAt.UTC()
	for index := range c.Endpoints {
		c.Endpoints[index].Name = strings.TrimSpace(c.Endpoints[index].Name)
		c.Endpoints[index].URL = strings.TrimSpace(c.Endpoints[index].URL)
		c.Endpoints[index].Protocol = strings.TrimSpace(c.Endpoints[index].Protocol)
		c.Endpoints[index].Provenance = strings.TrimSpace(c.Endpoints[index].Provenance)
	}
	for index := range c.Images {
		c.Images[index] = strings.TrimSpace(c.Images[index])
	}
	return c
}

func (c Candidate) Validate() error {
	c = c.Normalize()
	if c.Source.Kind == "" {
		return errors.New("source kind must not be empty")
	}
	if c.Source.ID == "" {
		return errors.New("source ID must not be empty")
	}
	if strings.ContainsRune(c.Source.Kind, '\x00') || strings.ContainsRune(c.Source.ID, '\x00') || strings.ContainsRune(c.Resource.Kind, '\x00') || strings.ContainsRune(c.Resource.Namespace, '\x00') || strings.ContainsRune(c.Resource.Name, '\x00') {
		return errors.New("source and resource identity must not contain NUL")
	}
	if c.Resource.Kind == "" {
		return errors.New("resource kind must not be empty")
	}
	if c.Resource.Name == "" {
		return errors.New("resource name must not be empty")
	}
	if c.ID == "" {
		return errors.New("candidate ID must not be empty")
	}
	if c.ID != StableID(c.Source, c.Resource) {
		return errors.New("candidate ID does not match source and resource identity")
	}
	if c.DisplayName == "" {
		return errors.New("display name must not be empty")
	}
	if c.ObservedAt.IsZero() {
		return errors.New("observed at must not be zero")
	}
	for key := range c.Metadata {
		if strings.TrimSpace(key) == "" {
			return errors.New("metadata key must not be empty")
		}
	}
	for index, image := range c.Images {
		if image == "" {
			return fmt.Errorf("image at index %d must not be empty", index)
		}
		if strings.ContainsRune(image, '\x00') {
			return fmt.Errorf("image at index %d must not contain NUL", index)
		}
	}
	for index, endpoint := range c.Endpoints {
		if endpoint.Port < 0 || endpoint.Port > 65535 {
			return fmt.Errorf("endpoint port at index %d must be between 0 and 65535", index)
		}
		if strings.TrimSpace(endpoint.URL) == "" {
			continue
		}
		parsed, err := url.Parse(endpoint.URL)
		if err != nil || parsed.Host == "" || (!parsed.IsAbs() && !strings.HasPrefix(endpoint.URL, "//")) {
			return fmt.Errorf("endpoint URL at index %d is invalid", index)
		}
	}
	return nil
}
