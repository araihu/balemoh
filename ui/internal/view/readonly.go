package view

import (
	"context"
	_ "embed"
	"strings"
)

type readOnlyKey struct{}
type readOnlyValues struct{ publicURL string }

func WithReadOnly(ctx context.Context, publicURL string) context.Context {
	return context.WithValue(ctx, readOnlyKey{}, readOnlyValues{publicURL: strings.TrimRight(publicURL, "/")})
}
func ReadOnly(ctx context.Context) bool {
	_, ok := ctx.Value(readOnlyKey{}).(readOnlyValues)
	return ok
}
func PublicURL(ctx context.Context) string {
	value, _ := ctx.Value(readOnlyKey{}).(readOnlyValues)
	return value.publicURL
}

//go:embed static/demo-refresh.js
var demoRefreshJS []byte

func DemoRefreshJS() []byte { return demoRefreshJS }
