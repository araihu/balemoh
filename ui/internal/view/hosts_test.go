package view

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestInteractiveHostHistoryPreservesMissingAndZero(t *testing.T) {
	zero, memory := 0.0, 42.5
	at := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	page := HostDetail{Samples: []HostSample{{At: at, CPU: &zero}, {At: at.Add(time.Minute), Memory: &memory}, {At: at.Add(2 * time.Minute), CPU: &zero, Memory: &memory}}}
	for _, cpu := range []bool{true, false} {
		cfg := historyChart(page, cpu)
		for i, sample := range page.Samples {
			value := sample.Memory
			if cpu {
				value = sample.CPU
			}
			point := cfg.Series[0].Data[i]
			if point.Missing != (value == nil) || value != nil && point.Value != *value {
				t.Fatalf("cpu=%t point %d lost missing/zero distinction", cpu, i)
			}
		}
		var output bytes.Buffer
		if err := HostChartContent(page, cpu).Render(context.Background(), &output); err != nil {
			t.Fatal(err)
		}
		body := output.String()
		for _, want := range []string{`goshtoso-charts-interactive`, `"value":null`, `"trigger":"axis"`} {
			if !strings.Contains(body, want) {
				t.Errorf("missing interactive output %s", want)
			}
		}
		if cpu && !strings.Contains(body, `"value":0`) {
			t.Error("zero reading missing")
		}
	}
}
