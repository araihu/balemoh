package telemetry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	docker "github.com/moby/moby/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

type emptyDocker struct{}

func (emptyDocker) ContainerList(context.Context, docker.ContainerListOptions) (docker.ContainerListResult, error) {
	return docker.ContainerListResult{}, nil
}
func TestDockerHostCountersAndMissingData(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("stat", "cpu 100 0 100 800 0 0 0 0 50 0\n")
	write("meminfo", "MemTotal: 1000 kB\nMemAvailable: 400 kB\n")
	d := Docker{Client: emptyDocker{}, SourceID: "docker", ProcPath: dir}
	first := d.Collect(context.Background())
	if first.Hosts[0].CPUPercent != nil || *first.Hosts[0].MemoryUsedBytes != 600*1024 {
		t.Fatal("first CPU measurement must be unavailable; memory must use host counters")
	}
	write("stat", "cpu 150 0 100 850 0 0 0 0 50 0\n")
	second := d.Collect(context.Background())
	if second.Hosts[0].CPUPercent == nil || *second.Hosts[0].CPUPercent != 50 {
		t.Fatalf("CPU delta: %+v", second.Hosts[0].Metrics)
	}
	write("stat", "cpu 1 0 0 1 0\n")
	write("meminfo", "MemTotal: 100 kB\n")
	reset := d.Collect(context.Background())
	if reset.Hosts[0].CPUPercent != nil || reset.Hosts[0].MemoryUsedBytes != nil {
		t.Fatal("counter reset or missing available memory became zero")
	}
}
func TestKubernetesMetricsAndMissingNodeAggregation(t *testing.T) {
	node := func(name string) *corev1.Node {
		return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}, Status: corev1.NodeStatus{Capacity: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("4Gi")}}}
	}
	gvr := schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}
	metric := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "metrics.k8s.io/v1beta1", "kind": "NodeMetrics", "metadata": map[string]interface{}{"name": "a"}, "timestamp": time.Now().UTC().Format(time.RFC3339), "usage": map[string]interface{}{"cpu": "500m", "memory": "1Gi"}}}
	core := kubefake.NewClientset(node("a"), node("b"), &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "apps"}, Spec: corev1.PodSpec{NodeName: "a"}})
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "NodeMetricsList"})
	if _, err := dyn.Resource(gvr).Create(context.Background(), metric, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	k := Kubernetes{Core: core.CoreV1(), Dynamic: dyn, SourceID: "cluster", Namespace: "apps"}
	b := k.Collect(context.Background())
	if len(b.Hosts) != 3 || b.Hosts[0].CPUPercent != nil || !b.Hosts[0].InventoryAvailable {
		t.Fatal("partial cluster must not claim complete aggregate")
	}
	if b.Hosts[1].CPUPercent == nil || *b.Hosts[1].CPUPercent != 25 || len(b.Hosts[1].Workloads) != 1 {
		t.Fatalf("node metrics: %+v", b.Hosts[1])
	}
	if err := b.Validate(time.Now()); err != nil {
		t.Fatal(err)
	}
	second := metric.DeepCopy()
	second.SetName("b")
	second.Object["usage"] = map[string]interface{}{"cpu": "1000m", "memory": "2Gi"}
	if _, err := dyn.Resource(gvr).Create(context.Background(), second, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	complete := k.Collect(context.Background())
	if complete.Hosts[0].CPUPercent == nil || *complete.Hosts[0].CPUPercent != 37.5 || complete.Hosts[0].MemoryUsedBytes == nil || *complete.Hosts[0].MemoryUsedBytes != 3<<30 || *complete.Hosts[0].MemoryTotalBytes != 8<<30 {
		t.Fatal("complete cluster aggregation is incorrect")
	}
	dyn.PrependReactor("list", "nodes", func(action ktesting.Action) (bool, runtime.Object, error) { return true, nil, fmt.Errorf("forbidden") })
	denied := k.Collect(context.Background())
	if len(denied.Hosts) != 3 || denied.Hosts[1].CPUPercent != nil || !denied.Hosts[1].InventoryAvailable {
		t.Fatal("metrics failure erased inventory")
	}
}
