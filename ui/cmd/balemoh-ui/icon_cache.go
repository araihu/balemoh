package main

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

func startIconCache(ctx context.Context, directory string, timeout time.Duration, prepare func(context.Context, string) (fs.FS, error), logger *slog.Logger) http.Handler {
	var ready atomic.Pointer[http.Handler]
	go func() {
		started := time.Now()
		logger.Info("icon cache preparation started", "cache_dir", directory)
		prepareCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		files, err := prepare(prepareCtx, directory)
		if err != nil {
			if ctx.Err() != nil {
				logger.Info("icon cache preparation cancelled", "cache_dir", directory, "elapsed", time.Since(started))
			} else {
				logger.Error("icon cache preparation failed; UI remains available", "cache_dir", directory, "elapsed", time.Since(started), "error", err)
			}
			return
		}
		handler := http.FileServer(http.FS(files))
		ready.Store(&handler)
		logger.Info("icon cache ready", "cache_dir", directory, "elapsed", time.Since(started))
	}()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		handler := ready.Load()
		if handler == nil {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "Icon library unavailable", http.StatusServiceUnavailable)
			return
		}
		(*handler).ServeHTTP(w, r)
	})
}
