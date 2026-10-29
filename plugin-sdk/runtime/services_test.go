package runtime_test

import (
	"context"
	pluginruntime "github.com/araihu/balemoh/plugin-sdk/runtime"
	plugintesting "github.com/araihu/balemoh/plugin-sdk/testing"
	"github.com/araihu/balemoh/plugin-sdk/ui"
	"net/url"
	"strings"
	"testing"
)

func TestRealChildServiceCatalogCallback(t *testing.T) {
	p := packageFixture(t)
	child, err := pluginruntime.Open(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	defer child.Close()
	id := strings.Repeat("a", 64)
	request := ui.Request{ContributionType: "page", ContributionID: "current", Mode: "page", Method: "GET", Query: url.Values{"service": {id}}}
	host := plugintesting.Services{ResolveServicesFunc: func(_ context.Context, ids []string) ([]ui.ServiceIdentity, error) {
		if len(ids) != 1 || ids[0] != id {
			t.Fatal(ids)
		}
		return []ui.ServiceIdentity{{ID: id, Name: "Pi-hole", IconURL: "/icons/pihole.png"}}, nil
	}}
	response, err := child.HandleWithServices(t.Context(), request, host)
	if err != nil || !strings.Contains(string(response.HTML), `"iconURL":"/icons/pihole.png"`) {
		t.Fatal(string(response.HTML), err)
	}
	if _, err = child.HandleWithServices(t.Context(), request, ui.NoDataServices{}); err == nil {
		t.Fatal("legacy host granted service read", err)
	}
	host.ResolveServicesFunc = func(context.Context, []string) ([]ui.ServiceIdentity, error) {
		return []ui.ServiceIdentity{{ID: id, Name: "Pi-hole", IconURL: "https://private.test/icon"}}, nil
	}
	if _, err = child.HandleWithServices(t.Context(), request, host); err == nil {
		t.Fatal("external URL accepted", err)
	}
}
