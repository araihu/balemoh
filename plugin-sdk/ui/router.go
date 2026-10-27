package ui

import (
	"context"
	"errors"
	"regexp"
)

// Router registers plugin-local handlers. The host still owns contribution scope.
type Router struct {
	pages     map[string]Handler
	fragments map[string]Handler
}

func NewRouter() *Router {
	return &Router{pages: map[string]Handler{}, fragments: map[string]Handler{}}
}
func (r *Router) Page(id string, h Handler) { r.pages[id] = h }
func (r *Router) Fragment(page, region string, h Handler) {
	if !regexp.MustCompile(`^[a-z0-9-]{1,64}$`).MatchString(region) {
		panic("invalid fragment region")
	}
	r.fragments[page+"/"+region] = h
}
func (r *Router) Handle(ctx context.Context, q Request, h HostServices) (Response, error) {
	handler := r.pages[q.ContributionID]
	if q.Mode == "fragment" {
		handler = r.fragments[q.ContributionID+"/"+q.Region]
	}
	if handler == nil {
		return Response{}, errors.New("unregistered local handler")
	}
	return handler.Handle(ctx, q, h)
}
