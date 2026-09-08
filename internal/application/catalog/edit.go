package catalog

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

type Edit struct{ DisplayName, Description, Address string }

var ErrInvalidEdit = errors.New("invalid service edit")

func (e Edit) Normalize() Edit {
	return Edit{strings.TrimSpace(e.DisplayName), strings.TrimSpace(e.Description), strings.TrimSpace(e.Address)}
}
func (e Edit) Validate() error {
	if e.DisplayName == "" || utf8.RuneCountInString(e.DisplayName) > 200 || utf8.RuneCountInString(e.Description) > 2000 || len(e.Address) > 2048 {
		return ErrInvalidEdit
	}
	if e.Address != "" {
		u, err := url.Parse(e.Address)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
			return ErrInvalidEdit
		}
	}
	return nil
}
func (s *Service) Edit(ctx context.Context, id string, edit Edit) error {
	edit = edit.Normalize()
	if err := edit.Validate(); err != nil {
		return err
	}
	return s.store.SaveEdit(ctx, id, edit)
}
