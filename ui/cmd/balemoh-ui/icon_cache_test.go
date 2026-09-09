package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestIconCacheDoesNotBlockAndPublishesWhenReady(t *testing.T) {
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handler := startIconCache(ctx, "test", time.Minute, func(ctx context.Context, _ string) (fs.FS, error) {
		select {
		case <-release:
			return fstest.MapFS{"icon.png": {Data: []byte("image")}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/icon.png", nil))
	if w.Code != 503 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("pending response: %d %v", w.Code, w.Header())
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/icon.png", nil))
		if w.Code == 200 {
			if w.Body.String() != "image" {
				t.Fatal(w.Body.String())
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("verified icons never became available")
}

type logMessages chan string

func (messages logMessages) Write(p []byte) (int, error) { messages <- string(p); return len(p), nil }

func TestIconCacheFailureIsLoggedWithoutStoppingHandler(t *testing.T) {
	messages := make(logMessages, 3)
	handler := startIconCache(context.Background(), "test", time.Second, func(context.Context, string) (fs.FS, error) { return nil, errors.New("download rejected") }, slog.New(slog.NewTextHandler(messages, nil)))
	deadline := time.After(time.Second)
	for {
		select {
		case message := <-messages:
			if !strings.Contains(message, "level=ERROR") {
				continue
			}
			if !strings.Contains(message, "download rejected") || !strings.Contains(message, "UI remains available") {
				t.Fatal(message)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("GET", "/icon.png", nil))
			if w.Code != 503 {
				t.Fatal(w.Code)
			}
			return
		case <-deadline:
			t.Fatal("failure not logged")
		}
	}
}

func TestIconCacheCancellationAndTimeout(t *testing.T) {
	for _, tc := range []struct {
		name, level, message string
		timeout              time.Duration
		cancel               bool
	}{
		{"shutdown", "level=INFO", "cancelled", time.Minute, true},
		{"timeout", "level=ERROR", "context deadline exceeded", 10 * time.Millisecond, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := make(chan struct{})
			finished := make(chan struct{})
			messages := make(logMessages, 4)
			handler := startIconCache(ctx, "test", tc.timeout, func(ctx context.Context, _ string) (fs.FS, error) {
				close(started)
				<-ctx.Done()
				close(finished)
				return nil, ctx.Err()
			}, slog.New(slog.NewTextHandler(messages, nil)))
			<-started
			if tc.cancel {
				cancel()
			}
			select {
			case <-finished:
			case <-time.After(time.Second):
				t.Fatal("preparation ignored cancellation")
			}
			deadline := time.After(time.Second)
			for {
				select {
				case message := <-messages:
					if !strings.Contains(message, tc.message) {
						continue
					}
					if !strings.Contains(message, tc.level) {
						t.Fatal(message)
					}
					w := httptest.NewRecorder()
					handler.ServeHTTP(w, httptest.NewRequest("GET", "/icon.png", nil))
					if w.Code != 503 || w.Header().Get("Cache-Control") != "no-store" {
						t.Fatalf("response = %d %v", w.Code, w.Header())
					}
					return
				case <-deadline:
					t.Fatal("expected lifecycle log missing")
				}
			}
		})
	}
}
