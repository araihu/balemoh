package view

import (
	"fmt"
	"github.com/a-h/templ"
	"time"

	"github.com/araihu/goshtoso/components/badge"
)

type Notification struct {
	ID, Title, Body, Severity, Source string
	ReceivedAt                        time.Time
	Read, HasDestination, External    bool
}

type NotificationsPage struct {
	Items                     []Notification
	UnreadCount               int
	NextCursor, Before, Error string
	UnreadOnly                bool
	ClearConfirmation         bool
	ClearError                string
}

func notificationTone(severity string) badge.Tone {
	switch severity {
	case "critical":
		return badge.ToneDanger
	case "warning":
		return badge.ToneWarning
	case "success":
		return badge.ToneSuccess
	default:
		return badge.ToneInfo
	}
}

func notificationCount(count int) string {
	if count > 99 {
		return "99+"
	}
	return fmt.Sprint(count)
}

func notificationAge(at time.Time) string {
	d := time.Since(at)
	if d < time.Minute {
		return "Just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return at.UTC().Format("02 Jan 2006")
}

func notificationClearAttrs(dialog bool) templ.Attributes {
	if !dialog {
		return nil
	}
	return templ.Attributes{"hx-post": "/notifications/clear", "hx-target": "#notification-clear-form", "hx-swap": "innerMorph", "hx-sync": "#balemoh-notifications:queue last", "hx-request": "timeout:4s", "hx-disable": "find button[type='submit']"}
}
func notificationClearCancelAttrs(dialog bool) templ.Attributes {
	if !dialog {
		return nil
	}
	return templ.Attributes{"x-on:click.prevent": "$dispatch('modal:close', {id:'clear-notifications'})"}
}
