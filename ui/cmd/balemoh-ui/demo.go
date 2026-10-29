package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// waitForDemoAPI prevents a public demo BFF from pointing at a private instance.
func waitForDemoAPI(ctx context.Context, base string, client *http.Client) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	probeClient := *client
	probeClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/healthz", nil)
		if err != nil {
			return err
		}
		response, err := probeClient.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode >= 300 && response.StatusCode < 400 {
				return fmt.Errorf("synthetic API health endpoint must not redirect")
			}
			if response.StatusCode == http.StatusOK {
				if response.Header.Get("X-Balemoh-Demo") != "synthetic-v1" {
					return fmt.Errorf("public demo UI requires a synthetic Balemoh API")
				}
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("synthetic API did not become ready: %w", ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}
