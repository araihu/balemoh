package bff

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	api "github.com/araihu/balemoh/client"
)

type liveHostCatalog struct {
	fakeCatalog
	mu     sync.Mutex
	detail api.HostDetail
	err    error
	window string
}

func (c *liveHostCatalog) Host(ctx context.Context, id, window string) (api.HostDetail, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.window = window
	if _, ok := ctx.Deadline(); !ok {
		return api.HostDetail{}, errors.New("upstream request has no deadline")
	}
	return c.detail, c.err
}
func TestHostEventStream(t *testing.T) {
	now := time.Now().UTC()
	value := 0.0
	catalog := &liveHostCatalog{detail: api.HostDetail{Host: api.Host{Id: "host", Name: "<script>bad</script>", Kind: "docker", CpuPercent: &value}, Samples: []api.HostSample{{At: now, CpuPercent: &value}, {At: now.Add(time.Minute)}}}}
	server := &server{catalog: catalog, timeout: time.Second}
	done := make(chan struct{}, 10)
	handler := http.NewServeMux()
	handler.HandleFunc("GET /hosts/{hostID}/events", func(w http.ResponseWriter, r *http.Request) {
		defer func() { done <- struct{}{} }()
		server.streamHost(w, r, 10*time.Millisecond)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(ts.URL + "/hosts/host/events?range=24h")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || response.Header.Get("Content-Type") != "text/event-stream" || response.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatal("missing stream headers")
	}
	reader := bufio.NewReader(response.Body)
	frame := func() string {
		t.Helper()
		var b strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			if line == "\n" {
				return b.String()
			}
			b.WriteString(line)
		}
	}
	first := frame()
	for _, want := range []string{`id="host-cpu-chart"`, `hx-swap-oob="innerMorph"`, `0.0%`, `Exact series values`, `Unavailable`} {
		if !strings.Contains(first, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(first, "<script>bad</script>") {
		t.Fatal("unescaped host data")
	}
	if next := frame(); next != ": keepalive\n" {
		t.Fatal("unchanged snapshot was resent")
	}
	catalog.mu.Lock()
	catalog.err = errors.New("private upstream error")
	catalog.mu.Unlock()
	for next := frame(); ; next = frame() {
		if strings.Contains(next, "Updates unavailable") {
			if strings.Contains(next, `id="host-cpu-chart"`) || strings.Contains(next, "private upstream") {
				t.Fatal("failure replaced chart or leaked error")
			}
			break
		}
	}
	catalog.mu.Lock()
	updated := 72.3
	catalog.detail.Host.CpuPercent = &updated
	catalog.detail.Samples = []api.HostSample{{At: now, CpuPercent: &updated}, {At: now.Add(time.Minute)}}
	catalog.err = nil
	catalog.mu.Unlock()
	for next := frame(); ; next = frame() {
		if strings.Contains(next, "72.3%") {
			if !strings.Contains(next, "Live updates connected") {
				t.Fatal("recovery status missing")
			}
			break
		}
	}
	response.Body.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("disconnected stream did not stop")
	}
	// Reconnect gets a complete snapshot even when nothing changed.
	response, err = client.Get(ts.URL + "/hosts/host/events?range=24h")
	if err != nil {
		t.Fatal(err)
	}
	reader = bufio.NewReader(response.Body)
	if !strings.Contains(frame(), "72.3%") {
		t.Fatal("reconnect missing snapshot")
	}
	response.Body.Close()
	<-done
	catalog.mu.Lock()
	window := catalog.window
	catalog.mu.Unlock()
	if window != "24h" {
		t.Fatalf("upstream range = %q", window)
	}
	for _, tc := range []struct {
		path   string
		err    error
		status int
	}{
		{"/hosts/host/events?range=7d", nil, 400},
		{"/hosts/missing/events", &upstreamError{status: 404}, 404},
		{"/hosts/host/events", errors.New("private error"), 503},
	} {
		catalog.mu.Lock()
		catalog.err = tc.err
		catalog.mu.Unlock()
		response, err = client.Get(ts.URL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != tc.status || strings.Contains(string(body), "private error") {
			t.Fatalf("%s: %d %s", tc.path, response.StatusCode, body)
		}
	}
}
