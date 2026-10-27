// Package render converts trusted plugin-owned templ content into bounded HTML.
package render

import (
	"bytes"
	"context"
	"errors"

	"github.com/a-h/templ"
	"github.com/araihu/balemoh/plugin-sdk/ui"
)

type boundedBuffer struct{ buffer bytes.Buffer }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > ui.MaxHTML-b.buffer.Len() {
		return 0, errors.New("plugin HTML exceeds limit")
	}
	return b.buffer.Write(p)
}

func Component(ctx context.Context, title string, c templ.Component) (ui.Response, error) {
	var b boundedBuffer
	if err := c.Render(ctx, &b); err != nil {
		return ui.Response{}, err
	}
	return ui.Response{Kind: "content", Status: 200, Title: title, HTML: b.buffer.Bytes()}, nil
}
