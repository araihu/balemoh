package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicDemoRejectsPrivateUpstream(t *testing.T) {
	for _, synthetic := range []bool{false, true} {
		t.Run(map[bool]string{false: "private", true: "synthetic"}[synthetic], func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if synthetic {
					w.Header().Set("X-Balemoh-Demo", "synthetic-v1")
				}
				w.WriteHeader(200)
			}))
			defer upstream.Close()
			err := waitForDemoAPI(t.Context(), upstream.URL, upstream.Client())
			if (err == nil) != synthetic {
				t.Fatalf("synthetic=%v err=%v", synthetic, err)
			}
		})
	}
}
