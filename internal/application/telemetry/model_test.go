package telemetry

import (
	"math"
	"testing"
	"time"
)

func TestTelemetryValidationAndFreshness(t *testing.T) {
	now := time.Now().UTC()
	valid := Batch{Source: Source{Kind: "container", ID: "raspi"}, ObservedAt: now, Hosts: []Observation{{}}}
	for _, tc := range []struct {
		name   string
		change func(*Batch)
	}{
		{"future", func(b *Batch) { b.ObservedAt = now.Add(time.Minute) }}, {"expired", func(b *Batch) { b.ObservedAt = now.Add(-25 * time.Hour) }},
		{"spoof node", func(b *Batch) { b.Hosts[0].Node = "node" }}, {"nan", func(b *Batch) { v := math.NaN(); b.Hosts[0].CPUPercent = &v }},
		{"memory", func(b *Batch) { b.Hosts[0].MemoryUsedBytes = new(int64(1)) }}, {"missing root", func(b *Batch) { b.Hosts = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := valid
			b.Hosts = append([]Observation(nil), valid.Hosts...)
			tc.change(&b)
			if b.Validate(now) == nil {
				t.Fatal("invalid telemetry accepted")
			}
		})
	}
	h := NewHost(valid.Source, "")
	h.SetStatus(now)
	if h.Status != "unavailable" {
		t.Fatal(h.Status)
	}
	h.ObservedAt = &now
	h.ReceivedAt = &now
	h.SetStatus(now)
	if h.Status != "partial" {
		t.Fatal(h.Status)
	}
	h.SetStatus(now.Add(4 * time.Minute))
	if h.Status != "stale" {
		t.Fatal(h.Status)
	}
	if ID(Source{Kind: "container", ID: "same"}, "") == ID(Source{Kind: "kubernetes", ID: "same"}, "") {
		t.Fatal("source kinds share identity")
	}
}
