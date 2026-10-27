package ui

import "context"

type DisplayContext struct{ Theme, ColorScheme string }
type displayKey struct{}

func WithDisplayContext(ctx context.Context, d DisplayContext) context.Context {
	return context.WithValue(ctx, displayKey{}, d)
}
func Display(ctx context.Context) DisplayContext {
	d, _ := ctx.Value(displayKey{}).(DisplayContext)
	if d.Theme == "" {
		d.Theme = "goshtoso"
	}
	if d.ColorScheme == "" {
		d.ColorScheme = "system"
	}
	return d
}
func ApplyDisplay(ctx context.Context, r Request) Request {
	d := Display(ctx)
	r.Theme = d.Theme
	r.ColorScheme = d.ColorScheme
	return r
}
