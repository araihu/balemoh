package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/a-h/templ"
	"github.com/araihu/balemoh/plugin-sdk/backend"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"github.com/araihu/balemoh/plugin-sdk/render"
	"github.com/araihu/balemoh/plugin-sdk/runtime"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

func main() {
	p, err := manifest.Load(".")
	if err != nil {
		os.Exit(1)
	}
	if _, err := os.Stat("mismatch"); err == nil {
		p.Manifest.Version = "9.0.0"
	}
	runtime.Serve(p, ui.HandlerFunc(func(ctx context.Context, r ui.Request, h ui.HostServices) (ui.Response, error) {
		if _, err := os.Stat("slow"); err == nil {
			_ = os.WriteFile("called-"+r.ContributionType+"-"+r.PlacementID, nil, 0600)
			<-ctx.Done()
			return ui.Response{}, ctx.Err()
		}
		if r.Query.Get("wait") == "true" {
			<-ctx.Done()
			return ui.Response{}, ctx.Err()
		}
		if r.Query.Get("oversized") == "true" {
			return ui.Response{Kind: "content", Status: 200, HTML: make([]byte, ui.MaxHTML+1)}, nil
		}
		if operation := r.Query.Get("data"); operation != "" {
			result, err := h.Query(ctx, operation, json.RawMessage(`{}`))
			if err != nil {
				return ui.Response{}, err
			}
			return ui.Response{Kind: "content", Status: 200, HTML: result.Data}, nil
		}
		if _, err := h.Query(ctx, "not-granted", json.RawMessage(`{}`)); !errors.Is(err, ui.ErrUnavailable) && backend.Code(err) != "permission_denied" {
			return ui.Response{}, errors.New("unexpected query grant")
		}
		return render.Component(ctx, "Fixture plugin", templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
			_, err := io.WriteString(w, "<p id=\""+r.DOMID("fixture")+"\">Synthetic plugin content</p>"+os.Getenv("BALEMOH_TEST_SECRET"))
			return err
		}))
	}))
}
