package main

import (
	"context"
	"database/sql"
	"strings"

	"github.com/araihu/balemoh/internal/adapters/sqlite"
	sender "github.com/araihu/balemoh/internal/adapters/webpush"
	push "github.com/araihu/balemoh/internal/application/webpush"
	"github.com/araihu/balemoh/internal/config"
)

func configureWebPush(ctx context.Context, options config.Options, db *sql.DB) (*push.Service, error) {
	store := sqlite.NewWebPushStore(db)
	var transport *sender.Sender
	var publicKey string
	if options.WebPushVAPIDKeyFile != "" {
		contents, err := readNotificationFile(options.WebPushVAPIDKeyFile, 4096, "Web Push key")
		if err != nil {
			return nil, err
		}
		transport, err = sender.New(strings.TrimSpace(string(contents)), options.WebPushSubject)
		if err != nil {
			return nil, err
		}
		publicKey = transport.PublicKey()
	}
	if err := store.Configure(ctx, publicKey, transport != nil); err != nil {
		return nil, err
	}
	if transport == nil {
		return push.New(store, nil, ""), nil
	}
	return push.New(store, transport, publicKey), nil
}
func startWebPush(ctx context.Context, service *push.Service) func() {
	worker, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); service.Run(worker) }()
	return func() { cancel(); <-done }
}
