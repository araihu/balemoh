package bff

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	api "github.com/araihu/balemoh/client"
)

type PresentationCatalog interface {
	Presentation(context.Context) (api.PresentationState, error)
	Themes(context.Context) (api.ThemeCatalog, error)
	Theme(context.Context, string) (api.CustomTheme, error)
	SavePresentation(context.Context, api.PresentationSave) (api.PresentationReceipt, error)
	CreateTheme(context.Context, api.ThemeCreate) (api.CustomTheme, error)
	RenameTheme(context.Context, string, api.ThemeRename) (api.CustomTheme, error)
	DeleteTheme(context.Context, string, string) error
	Fonts(context.Context) ([]api.UploadedFont, error)
	UploadFont(context.Context, string, io.Reader) (api.UploadedFont, error)
	DeleteFont(context.Context, string, string) error
	FontFile(context.Context, string, string) ([]byte, http.Header, int, error)
}
type presentationFailure struct {
	status    int
	detail    api.ErrorResponse
	uncertain bool
}

func (e *presentationFailure) Error() string { return e.detail.Message }
func presentationResult[T any](response *http.Response, err error) (T, error) {
	var value T
	if err != nil {
		return value, &presentationFailure{status: 503, detail: api.ErrorResponse{Code: "unavailable", Message: "Resultado não confirmado. Preserve o rascunho e tente novamente com o mesmo comando."}, uncertain: true}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		e := &presentationFailure{status: response.StatusCode, detail: api.ErrorResponse{Code: "unavailable", Message: "Não foi possível concluir a operação."}}
		_ = json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&e.detail)
		return value, e
	}
	if response.StatusCode == 204 {
		return value, nil
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&value); err != nil {
		return value, &presentationFailure{status: 503, detail: api.ErrorResponse{Code: "unavailable", Message: "Resposta não confirmada. Preserve o rascunho e consulte a versão salva."}, uncertain: true}
	}
	return value, nil
}
func (c *APIClient) Presentation(ctx context.Context) (api.PresentationState, error) {
	return presentationResult[api.PresentationState](c.client.GetPresentation(ctx))
}
func (c *APIClient) Themes(ctx context.Context) (api.ThemeCatalog, error) {
	return presentationResult[api.ThemeCatalog](c.client.ListThemes(ctx))
}
func (c *APIClient) Theme(ctx context.Context, id string) (api.CustomTheme, error) {
	return presentationResult[api.CustomTheme](c.client.GetTheme(ctx, id))
}
func (c *APIClient) SavePresentation(ctx context.Context, v api.PresentationSave) (api.PresentationReceipt, error) {
	return presentationResult[api.PresentationReceipt](c.client.SavePresentation(ctx, v))
}
func (c *APIClient) CreateTheme(ctx context.Context, v api.ThemeCreate) (api.CustomTheme, error) {
	return presentationResult[api.CustomTheme](c.client.CreateTheme(ctx, v))
}
func (c *APIClient) RenameTheme(ctx context.Context, id string, v api.ThemeRename) (api.CustomTheme, error) {
	return presentationResult[api.CustomTheme](c.client.RenameTheme(ctx, id, v))
}
func (c *APIClient) DeleteTheme(ctx context.Context, id, revision string) error {
	_, e := presentationResult[struct{}](c.client.DeleteTheme(ctx, id, &api.DeleteThemeParams{ExpectedRevision: revision}))
	return e
}
func (c *APIClient) Fonts(ctx context.Context) ([]api.UploadedFont, error) {
	return presentationResult[[]api.UploadedFont](c.client.ListFonts(ctx))
}
func (c *APIClient) UploadFont(ctx context.Context, contentType string, body io.Reader) (api.UploadedFont, error) {
	return presentationResult[api.UploadedFont](c.fontClient.UploadFontWithBody(ctx, contentType, body))
}
func (c *APIClient) DeleteFont(ctx context.Context, id, revision string) error {
	_, e := presentationResult[struct{}](c.client.DeleteFont(ctx, id, &api.DeleteFontParams{ExpectedRevision: revision}))
	return e
}
func (c *APIClient) FontFile(ctx context.Context, id, etag string) ([]byte, http.Header, int, error) {
	response, e := c.client.GetFontFile(ctx, id, func(_ context.Context, r *http.Request) error { r.Header.Set("If-None-Match", etag); return nil })
	if e != nil {
		return nil, nil, 503, e
	}
	if response.StatusCode != 200 && response.StatusCode != 304 {
		_, e = presentationResult[struct{}](response, nil)
		return nil, nil, 503, e
	}
	defer response.Body.Close()
	b, e := io.ReadAll(io.LimitReader(response.Body, (10<<20)+1))
	if len(b) > 10<<20 {
		e = errors.New("font too large")
	}
	return b, response.Header, response.StatusCode, e
}
