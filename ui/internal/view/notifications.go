package view

import (
	"fmt"
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
