package view

import (
	"fmt"
	"time"

	"github.com/a-h/templ"
	"github.com/araihu/goshtoso/components/badge"
	"github.com/araihu/goshtoso/components/icon"
	"github.com/araihu/goshtoso/components/icon/heroicons"
)

type Notification struct {
	ID, Title, Body, Severity, Source              string
	SourceKind, SourceLabel                        string
	ProducerID, ProducerConnectionID, ProducerPage string
	ProducerIcon                                   icon.Symbol
	ReceivedAt                                     time.Time
	Read, HasDestination, External                 bool
}

type NotificationsPage struct {
	Items                     []Notification
	UnreadCount               int
	RetentionDays             int
	BulkError                 string
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

type NotificationGroup struct {
	Label string
	Items []Notification
}

func notificationGroups(items []Notification, now time.Time) []NotificationGroup {
	var groups []NotificationGroup
	today := now.UTC().Truncate(24 * time.Hour)
	var previous string
	for _, item := range items {
		day := item.ReceivedAt.UTC().Truncate(24 * time.Hour)
		key := day.Format("2006-01-02")
		if len(groups) == 0 || key != previous {
			label := day.Format("02 Jan 2006") + " · UTC"
			if day.Equal(today) {
				label = "Today · UTC"
			} else if day.Equal(today.AddDate(0, 0, -1)) {
				label = "Yesterday · UTC"
			}
			groups = append(groups, NotificationGroup{Label: label})
			previous = key
		}
		groups[len(groups)-1].Items = append(groups[len(groups)-1].Items, item)
	}
	return groups
}
func notificationRetentionLabel(days int) string {
	if days < 1 {
		return "Recent notifications"
	}
	if days == 1 {
		return "Notifications are kept for 1 day"
	}
	return fmt.Sprintf("Notifications are kept for %d days", days)
}

func notificationGroupPage(page NotificationsPage, items []Notification) NotificationsPage {
	page.Items = items
	return page
}
func notificationStatusRole(failed bool) string {
	if failed {
		return "alert"
	}
	return "status"
}
func notificationCompactClass(read bool) string {
	if read {
		return "balemoh-notification-tray-row"
	}
	return "balemoh-notification-tray-row balemoh-notification-tray-row-unread"
}
func notificationIconSymbol(item Notification) icon.Symbol {
	if item.ProducerIcon != "" {
		return item.ProducerIcon
	}
	return notificationSourceSymbol(item.SourceKind)
}

func notificationSourceSymbol(kind string) icon.Symbol {
	switch kind {
	case "plugin":
		return heroicons.Icon16SolidCube
	case "external":
		return heroicons.Icon16SolidArrowDownTray
	default:
		return heroicons.Icon16SolidBell
	}
}
func notificationReadTarget(id, returnTo string) string {
	if returnTo == "tray" || returnTo == "tray-unread" {
		return "#notification-tray-read-" + id
	}
	return "#notification-read-" + id
}

func notificationCompactSource(item Notification) string {
	if item.SourceLabel != "" {
		return item.SourceLabel
	}
	return item.Source
}
