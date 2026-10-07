// Package telemetry defines host observations independently of service discovery.
package telemetry

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
)

var ErrHostLimit = errors.New("source exceeds 128 retained hosts")

type Source struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}
type Metrics struct {
	CPUPercent       *float64 `json:"cpuPercent"`
	MemoryUsedBytes  *int64   `json:"memoryUsedBytes"`
	MemoryTotalBytes *int64   `json:"memoryTotalBytes"`
}
type Workload struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	State     string `json:"state"`
	Restarts  int    `json:"restarts"`
}
type Observation struct {
	Node               string `json:"node"`
	InventoryAvailable bool   `json:"inventoryAvailable"`
	Ready              string `json:"ready"`
	Metrics
	Workloads []Workload `json:"workloads"`
}
type Batch struct {
	Source     Source        `json:"source"`
	ObservedAt time.Time     `json:"observedAt"`
	Hosts      []Observation `json:"hosts"`
}
type Host struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	Source     Source     `json:"source"`
	ParentID   string     `json:"parentId"`
	ObservedAt *time.Time `json:"observedAt"`
	ReceivedAt *time.Time `json:"receivedAt"`
	Status     string     `json:"status"`
	Observation
}
type Sample struct {
	At time.Time `json:"at"`
	Metrics
}
type Detail struct {
	Host     Host     `json:"host"`
	Children []Host   `json:"children"`
	Samples  []Sample `json:"samples"`
	Range    string   `json:"range"`
}

func ID(source Source, node string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(source.Kind+"\x00"+source.ID+"\x00"+node)))
}
func NewHost(source Source, node string) Host {
	h := Host{ID: ID(source, node), Name: source.ID, Source: source, Kind: "unknown", Status: "unavailable", Observation: Observation{Node: node, Workloads: []Workload{}}}
	switch source.Kind {
	case "container":
		h.Kind = "docker"
	case "kubernetes":
		h.Kind = "cluster"
	}
	if node != "" {
		h.Name = node
		h.Kind = "node"
		h.ParentID = ID(source, "")
	}
	return h
}
func (h *Host) SetStatus(now time.Time) {
	h.Status = "unavailable"
	if h.ReceivedAt == nil || h.ObservedAt == nil {
		return
	}
	h.Status = "fresh"
	if now.Sub(*h.ReceivedAt) > 3*time.Minute || now.Sub(*h.ObservedAt) > 3*time.Minute {
		h.Status = "stale"
		return
	}
	if h.CPUPercent == nil || h.MemoryUsedBytes == nil || h.MemoryTotalBytes == nil {
		h.Status = "partial"
	}
}
func validText(s string, max int, required bool) bool {
	return len(s) <= max && (!required || strings.TrimSpace(s) != "") && !strings.ContainsFunc(s, unicode.IsControl)
}
func (b Batch) Validate(now time.Time) error {
	if (b.Source.Kind != "container" && b.Source.Kind != "kubernetes") || !validText(b.Source.ID, 253, true) {
		return fmt.Errorf("invalid telemetry source")
	}
	if b.ObservedAt.IsZero() || b.ObservedAt.Before(now.Add(-24*time.Hour)) || b.ObservedAt.After(now.Add(30*time.Second)) {
		return fmt.Errorf("observation time outside accepted window")
	}
	if len(b.Hosts) == 0 || len(b.Hosts) > 128 || (b.Source.Kind == "container" && len(b.Hosts) != 1) {
		return fmt.Errorf("invalid host count")
	}
	seen := map[string]bool{}
	for _, h := range b.Hosts {
		if seen[h.Node] || !validText(h.Node, 253, false) || (b.Source.Kind == "container" && h.Node != "") {
			return fmt.Errorf("invalid or duplicate node")
		}
		seen[h.Node] = true
		if h.Ready != "" && h.Ready != "True" && h.Ready != "False" && h.Ready != "Unknown" {
			return fmt.Errorf("invalid readiness")
		}
		if err := h.Metrics.Validate(); err != nil {
			return err
		}
		if len(h.Workloads) > 1000 {
			return fmt.Errorf("too many workloads")
		}
		for _, w := range h.Workloads {
			if !validText(w.Name, 253, true) || !validText(w.Namespace, 253, false) || !validText(w.State, 64, true) || w.Restarts < 0 {
				return fmt.Errorf("invalid workload")
			}
		}
	}
	if !seen[""] {
		return fmt.Errorf("root host required")
	}
	return nil
}
func (m Metrics) Validate() error {
	if m.CPUPercent != nil && (math.IsNaN(*m.CPUPercent) || math.IsInf(*m.CPUPercent, 0) || *m.CPUPercent < 0 || *m.CPUPercent > 100) {
		return fmt.Errorf("invalid CPU percentage")
	}
	if (m.MemoryUsedBytes != nil && *m.MemoryUsedBytes < 0) || (m.MemoryTotalBytes != nil && *m.MemoryTotalBytes <= 0) || (m.MemoryUsedBytes != nil && m.MemoryTotalBytes == nil) || (m.MemoryUsedBytes != nil && *m.MemoryUsedBytes > *m.MemoryTotalBytes) {
		return fmt.Errorf("invalid memory measurement")
	}
	return nil
}
