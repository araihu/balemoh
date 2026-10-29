package ui

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
)

const ServiceCatalogCapability = "catalog.services.read/v1"
const MaxServiceIDs = 64

// ServiceCatalog is an optional, invocation-scoped host capability. Hosts return
// only explicitly granted, visible services; unresolved IDs are omitted.
// It is separate from HostServices so existing authors and fixtures still work.
type ServiceCatalog interface {
	ResolveServices(context.Context, []string) ([]ServiceIdentity, error)
}

// ServiceIdentity contains presentation data only, never endpoints or credentials.
type ServiceIdentity struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IconURL       string `json:"iconURL,omitempty"`
	IconSpriteURL string `json:"iconSpriteURL,omitempty"`
	IconSymbol    string `json:"iconSymbol,omitempty"`
}

var serviceID = regexp.MustCompile(`^[a-f0-9]{64}$`)

func ValidServiceID(id string) bool { return serviceID.MatchString(id) }

func ValidateServiceIDs(ids []string) error {
	if len(ids) == 0 || len(ids) > MaxServiceIDs {
		return errors.New("invalid service IDs")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !ValidServiceID(id) || seen[id] {
			return errors.New("invalid service IDs")
		}
		seen[id] = true
	}
	return nil
}

func ValidateServiceIdentities(ids []string, services []ServiceIdentity) error {
	if ValidateServiceIDs(ids) != nil || len(services) > len(ids) {
		return errors.New("invalid service identities")
	}
	allowed := map[string]bool{}
	for _, id := range ids {
		allowed[id] = true
	}
	for _, service := range services {
		if !allowed[service.ID] || strings.TrimSpace(service.Name) == "" || len(service.Name) > 512 || len(service.IconSymbol) > 128 || !localAsset(service.IconURL) || !localAsset(service.IconSpriteURL) || (service.IconSymbol == "") != (service.IconSpriteURL == "") {
			return errors.New("invalid service identities")
		}
		delete(allowed, service.ID)
	}
	return nil
}

func localAsset(value string) bool {
	if value == "" {
		return true
	}
	u, err := url.Parse(value)
	return err == nil && len(value) <= 2048 && strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") && !strings.ContainsAny(value, "\\\r\n") && u.Host == "" && u.Scheme == "" && u.User == nil && u.Fragment == ""
}
