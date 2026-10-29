package bff

import (
	"context"
	api "github.com/araihu/balemoh/client"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

// ServiceCatalog uses the same effective names and icons as Home. Only
// live services are disclosed, including unpinned entries. Grants live in the
// plugin registry, not in user-controlled alert annotations.
type ServiceCatalog struct {
	Catalog interface {
		Staging(context.Context) ([]api.ServiceCandidate, error)
	}
}

func (c ServiceCatalog) ResolveServices(ctx context.Context, ids []string) ([]ui.ServiceIdentity, error) {
	if ui.ValidateServiceIDs(ids) != nil {
		return nil, backend.Fail("invalid_argument")
	}
	services, err := c.Catalog.Staging(ctx)
	if err != nil {
		return nil, backend.Fail("unavailable")
	}
	byID := map[string]api.ServiceCandidate{}
	for _, s := range services {
		if s.Status == "live" && !s.Missing {
			byID[s.Id] = s
		}
	}
	result := []ui.ServiceIdentity{}
	for _, id := range ids {
		service, ok := byID[id]
		if !ok {
			continue
		}
		mapped := mapService(service)
		result = append(result, ui.ServiceIdentity{ID: id, Name: mapped.DisplayName, IconURL: mapped.Icon.URL, IconSpriteURL: mapped.Icon.SpriteURL, IconSymbol: mapped.Icon.Symbol})
	}
	return result, nil
}
