package view

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/araihu/goshtoso/components/badge"
)

type HostWorkload struct {
	Name, Namespace, State, Node, Reason string
	Ready                                *bool
	Images                               []string
	Restarts                             int
	Docker                               bool
}

type HostApplication struct {
	Service      Service
	Workloads    []HostWorkload
	Missing      bool
	Unassociated bool
	ObservedAt   time.Time
}

func sameApplications(current HostDetail, previous *HostDetail) bool {
	if previous == nil || len(current.Applications) != len(previous.Applications) {
		return false
	}
	for i, app := range current.Applications {
		if app.Service.ID != previous.Applications[i].Service.ID {
			return false
		}
	}
	return true
}

func applicationChanged(current HostApplication, previous HostApplication) bool {
	return !reflect.DeepEqual(current, previous)
}

func (w HostWorkload) Attention() bool {
	switch strings.ToLower(w.State) {
	case "succeeded":
		return false
	case "running":
		return w.Ready != nil && !*w.Ready || w.Reason != ""
	default:
		return true
	}
}

func (w HostWorkload) Readiness() string {
	if w.Docker {
		return "Not reported"
	}
	if w.Ready == nil {
		return "Unknown"
	}
	if *w.Ready {
		return "Ready"
	}
	return "Not ready"
}

func (a HostApplication) Attention() bool {
	if a.Missing {
		return false
	}
	for _, w := range a.Workloads {
		if w.Attention() {
			return true
		}
	}
	return false
}

func (a HostApplication) Summary() string {
	if a.Missing {
		return "Absent from discovery"
	}
	if len(a.Workloads) == 0 {
		return "No associated workloads reported"
	}
	ready, running, completed := 0, 0, 0
	unknown := false
	for _, w := range a.Workloads {
		if strings.EqualFold(w.State, "succeeded") {
			completed++
			continue
		}
		if w.Ready != nil && *w.Ready {
			ready++
		}
		if strings.EqualFold(w.State, "running") {
			running++
		}
		unknown = unknown || w.Ready == nil
	}
	if a.Workloads[0].Docker {
		return fmt.Sprintf("%d/%d containers running", running, len(a.Workloads))
	}
	if completed == len(a.Workloads) {
		return fmt.Sprintf("%d completed", completed)
	}
	active := len(a.Workloads) - completed
	if completed > 0 {
		if unknown {
			return fmt.Sprintf("%d active · readiness unknown · %d completed", active, completed)
		}
		return fmt.Sprintf("%d/%d active workloads ready · %d completed", ready, active, completed)
	}
	if unknown {
		if len(a.Workloads) == 1 {
			return "1 workload · readiness unknown"
		}
		return fmt.Sprintf("%d workloads · readiness unknown", len(a.Workloads))
	}
	return fmt.Sprintf("%d/%d workloads ready", ready, len(a.Workloads))
}

func (a HostApplication) Tone() badge.Tone {
	if a.Missing || len(a.Workloads) == 0 {
		return badge.ToneDefault
	}
	if a.Attention() {
		return badge.ToneWarning
	}
	for _, w := range a.Workloads {
		if !w.Docker && w.Ready == nil && !strings.EqualFold(w.State, "succeeded") {
			return badge.ToneDefault
		}
	}
	return badge.ToneSuccess
}

func (a HostApplication) SearchText() string {
	parts := []string{a.Service.DisplayName, a.Service.Namespace, a.Service.Description, a.Service.Resource}
	for _, w := range a.Workloads {
		parts = append(parts, w.Name, w.Namespace, w.Node, w.State, w.Reason)
		parts = append(parts, w.Images...)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func (a HostApplication) Locations() string {
	seen := map[string]bool{}
	parts := []string{}
	for _, w := range a.Workloads {
		if w.Node != "" && !seen[w.Node] {
			parts = append(parts, w.Node)
			seen[w.Node] = true
		}
	}
	return strings.Join(parts, ", ")
}

func (a HostApplication) Scope() string {
	if a.Missing {
		return "missing"
	}
	return "current"
}

func applicationCounts(page HostDetail) string {
	apps, attention, missing := 0, 0, 0
	for _, app := range page.Applications {
		if app.Missing {
			missing++
		} else {
			if !app.Unassociated {
				apps++
			}
			if app.Attention() {
				attention++
			}
		}
	}
	attentionText := fmt.Sprintf("%d groups need attention", attention)
	if attention == 1 {
		attentionText = "1 group needs attention"
	}
	return fmt.Sprintf("%d applications · %d workloads · %s · %d absent", apps, len(page.Workloads), attentionText, missing)
}
