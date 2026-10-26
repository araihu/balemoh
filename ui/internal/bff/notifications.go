package bff

import (
	"bytes"
	"context"
	"errors"
	"github.com/a-h/templ"
	pluginruntime "github.com/araihu/balemoh/plugin-sdk/runtime"
	pluginui "github.com/araihu/balemoh/plugin-sdk/ui"
	"github.com/araihu/balemoh/ui/internal/view"
	"net/http"
	"net/url"
	"strings"
	"time"
	"uuid"
)

// NotificationCatalog never invokes plugin discovery or rendering.
type NotificationCatalog interface {
	Notifications(context.Context, string, bool, int) (view.NotificationsPage, error)
	Notification(context.Context, string) (NotificationRecord, error)
	MarkNotificationRead(context.Context, string) error
}
type NotificationRecord struct {
	Item   view.Notification
	Target *NotificationTarget
}
type NotificationTarget struct {
	Kind, Route, PluginID, ConnectionID, Page, URL string
	Params                                         map[string]string
	Query                                          map[string][]string
}

const notificationUnavailable = "Notifications unavailable. Try refreshing."

func (s *server) notificationPage(w http.ResponseWriter, r *http.Request) {
	page, status := s.notificationList(r, 50)
	s.render(w, r, view.PageData{Title: "Notifications", Description: "Messages from Balemoh, plugins and integrations.", Path: "/notifications", NotificationsPage: &page}, status)
}
func (s *server) notificationList(r *http.Request, limit int) (view.NotificationsPage, int) {
	page := view.NotificationsPage{Before: r.URL.Query().Get("before"), UnreadOnly: r.URL.Query().Get("unread") == "true"}
	if len(page.Before) > 512 || (r.URL.Query().Get("unread") != "" && r.URL.Query().Get("unread") != "true" && r.URL.Query().Get("unread") != "false") {
		page.Error = "Invalid notification filter."
		return page, 400
	}
	client, ok := s.catalog.(NotificationCatalog)
	if !ok {
		page.Error = notificationUnavailable
		return page, 503
	}
	ctx, cancel := context.WithTimeout(r.Context(), min(s.timeout, 3*time.Second))
	defer cancel()
	result, err := client.Notifications(ctx, page.Before, page.UnreadOnly, limit)
	if err != nil {
		page.Error = notificationUnavailable
		return page, 503
	}
	result.Before, result.UnreadOnly = page.Before, page.UnreadOnly
	return result, 200
}
func (s *server) notificationSummary(w http.ResponseWriter, r *http.Request) {
	page, status := s.notificationList(r, 1)
	component := templ.Component(view.NotificationSummary(page.UnreadCount, status == 200))
	if r.URL.Query().Get("inbox") == "true" {
		component = templ.Join(component, view.NotificationPageCount(page.UnreadCount, status == 200, true))
	}
	renderNotificationFragment(w, r, status, component)
}
func (s *server) notificationTray(w http.ResponseWriter, r *http.Request) {
	page, status := s.notificationList(r, 5)
	component := templ.Join(view.NotificationList(page, true), view.NotificationSummaryOOB(page.UnreadCount, status == 200))
	if r.URL.Query().Get("inbox") == "true" {
		component = templ.Join(component, view.NotificationPageCount(page.UnreadCount, status == 200, true))
	}
	renderNotificationFragment(w, r, status, component)
}
func renderNotificationFragment(w http.ResponseWriter, r *http.Request, status int, component templ.Component) {
	var b bytes.Buffer
	if err := component.Render(r.Context(), &b); err != nil {
		http.Error(w, notificationUnavailable, 503)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(b.Bytes())
}
func (s *server) notificationRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("notificationID")
	if _, err := uuid.Parse(id); err != nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid notification action.", 400)
		return
	}
	returnTo := notificationReturnTo(r.Form.Get("returnTo"))
	client, ok := s.catalog.(NotificationCatalog)
	ctx, cancel := context.WithTimeout(r.Context(), min(s.timeout, 3*time.Second))
	defer cancel()
	var err error
	if !ok {
		err = errors.New("notification client unavailable")
	} else {
		err = client.MarkNotificationRead(ctx, id)
	}
	if r.Header.Get("HX-Request-Type") != "partial" {
		if err == nil {
			redirect(w, r, notificationRedirect(returnTo))
			return
		}
		page := view.NotificationsPage{Error: "Could not mark notification read. Retry to confirm read state."}
		s.render(w, r, view.PageData{Title: "Notifications", Description: "Notification update failed.", Path: "/notifications", NotificationsPage: &page}, 503)
		return
	}
	if err != nil {
		renderNotificationFragment(w, r, 503, view.NotificationReadControl(id, false, "Could not save read state. Retry.", returnTo))
		return
	}
	// An acknowledged write owns read state; a subsequent count failure is independent.
	listRequest := r.Clone(r.Context())
	if returnTo != "tray" {
		u, _ := url.Parse(returnTo)
		listURL := *r.URL
		listURL.RawQuery = u.RawQuery
		listRequest.URL = &listURL
	}
	limit := 1
	if listRequest.URL.Query().Get("unread") == "true" {
		limit = 50
	}
	page, status := s.notificationList(listRequest, limit)
	content := templ.Component(view.NotificationReadControl(id, true, "", returnTo))
	if limit == 50 && status == 200 {
		w.Header().Set("HX-Retarget", "#notification-page-list")
		content = view.NotificationList(page, false)
	}
	component := templ.Join(content, view.NotificationSummaryOOB(page.UnreadCount, status == 200))
	if returnTo != "tray" {
		component = templ.Join(component, view.NotificationPageCount(page.UnreadCount, status == 200, true))
	}
	renderNotificationFragment(w, r, 200, component)
}
func notificationReturnTo(value string) string {
	if value == "tray" {
		return value
	}
	u, err := url.Parse(value)
	if err != nil || u.IsAbs() || u.Host != "" || u.Path != "/notifications" || len(u.RawQuery) > 600 {
		return "/notifications"
	}
	q := url.Values{}
	if before := u.Query().Get("before"); before != "" && len(before) <= 512 {
		q.Set("before", before)
	}
	if u.Query().Get("unread") == "true" {
		q.Set("unread", "true")
	}
	u.RawQuery = q.Encode()
	u.Fragment = ""
	return u.String()
}
func notificationRedirect(value string) string {
	if value == "tray" {
		return "/notifications"
	}
	return value
}
func (s *server) notificationOpen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("notificationID")
	if _, err := uuid.Parse(id); err != nil {
		http.NotFound(w, r)
		return
	}
	client, ok := s.catalog.(NotificationCatalog)
	ctx, cancel := context.WithTimeout(r.Context(), min(s.timeout, 3*time.Second))
	defer cancel()
	if ok {
		record, err := client.Notification(ctx, id)
		if err == nil && record.Target != nil {
			if target := s.notificationDestination(*record.Target); target != "" {
				http.Redirect(w, r, target, http.StatusFound)
				return
			}
		}
	}
	page := view.NotificationsPage{Error: "This notification destination is unavailable. Its message remains in the inbox."}
	s.render(w, r, view.PageData{Title: "Notification destination unavailable", Description: "The notification source is unavailable.", Path: "/notifications", NotificationsPage: &page}, 404)
}
func (s *server) notificationDestination(target NotificationTarget) string {
	switch target.Kind {
	case "external":
		u, err := url.Parse(target.URL)
		if err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && !strings.ContainsAny(target.URL, "\r\n\x00") {
			return u.String()
		}
	case "core":
		switch target.Route {
		case "/", "/hosts", "/staging", "/notifications":
			return target.Route
		}
	case "plugin":
		if s.plugins == nil {
			return ""
		}
		entry := s.plugins.Connection(target.PluginID, target.ConnectionID)
		if entry == nil || !pluginruntime.Available(entry.Package.Manifest, "page", target.Page) {
			return ""
		}
		request := pluginui.Request{ConnectionID: target.ConnectionID}
		path, err := request.PageURL(entry.Package.Manifest, target.Page, target.Params)
		if err != nil {
			return ""
		}
		if len(target.Query) > 0 {
			path += "?" + url.Values(target.Query).Encode()
		}
		return path
	}
	return ""
}
