package view

import (
	"context"
	_ "embed"
)

type readOnlyKey struct{}

func WithReadOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, readOnlyKey{}, true)
}
func ReadOnly(ctx context.Context) bool { value, _ := ctx.Value(readOnlyKey{}).(bool); return value }

//go:embed static/demo-refresh.js
var demoRefreshJS []byte

func DemoRefreshJS() []byte { return demoRefreshJS }
