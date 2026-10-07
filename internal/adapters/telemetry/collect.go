// Package telemetry collects machine observations without changing discovery.
package telemetry

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	model "github.com/araihu/balemoh/internal/application/telemetry"
	docker "github.com/moby/moby/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type DockerClient interface {
	ContainerList(context.Context, docker.ContainerListOptions) (docker.ContainerListResult, error)
}
type Docker struct {
	Client             DockerClient
	SourceID, ProcPath string
	total, idle        uint64
	primed             bool
}

func NewDocker(host, source, proc string) (*Docker, error) {
	c, err := docker.NewClientWithOpts(docker.WithHost(host), docker.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Docker{Client: c, SourceID: strings.TrimSpace(source), ProcPath: proc}, nil
}
func (d *Docker) Collect(ctx context.Context) model.Batch {
	now := time.Now().UTC()
	h := model.Observation{Workloads: []model.Workload{}}
	// Read each metric independently; missing mounts must not invent zero usage.
	total, idle, err := cpuCounters(filepath.Join(d.ProcPath, "stat"))
	if err == nil {
		if d.primed && total > d.total && idle >= d.idle && idle-d.idle <= total-d.total {
			v := 100 * (1 - float64(idle-d.idle)/float64(total-d.total))
			h.CPUPercent = &v
		}
		d.total, d.idle, d.primed = total, idle, true
	} else {
		d.primed = false
	}
	h.MemoryUsedBytes, h.MemoryTotalBytes = memory(filepath.Join(d.ProcPath, "meminfo"))
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	containers, err := d.Client.ContainerList(requestCtx, docker.ContainerListOptions{All: true})
	cancel()
	if err == nil {
		h.InventoryAvailable = true
		for _, c := range containers.Items {
			if len(h.Workloads) >= 1000 {
				h.InventoryAvailable = false
				break
			}
			name := c.ID
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}
			h.Workloads = append(h.Workloads, model.Workload{Name: name, State: string(c.State)})
		}
	}
	return model.Batch{Source: model.Source{Kind: "container", ID: d.SourceID}, ObservedAt: now, Hosts: []model.Observation{h}}
}
func cpuCounters(path string) (uint64, uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, 0, fmt.Errorf("missing CPU counters")
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, fmt.Errorf("invalid CPU counters")
	}
	var total, idle uint64
	// Linux guest and guest_nice are included in user/nice; use only first 8 fields.
	for i := 1; i < len(fields) && i <= 8; i++ {
		value, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil || math.MaxUint64-total < value {
			return 0, 0, fmt.Errorf("invalid CPU counter")
		}
		total += value
		if i == 4 || i == 5 {
			idle += value
		}
	}
	return total, idle, nil
}
func memory(path string) (*int64, *int64) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	values := map[string]int64{}
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 || (fields[0] != "MemTotal:" && fields[0] != "MemAvailable:") {
			continue
		}
		n, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || n < 0 || n > math.MaxInt64/1024 || fields[2] != "kB" {
			return nil, nil
		}
		values[fields[0]] = n * 1024
	}
	total, ok := values["MemTotal:"]
	available, found := values["MemAvailable:"]
	if scanner.Err() != nil || !ok || !found || total <= 0 || available > total {
		return nil, nil
	}
	used := total - available
	return &used, &total
}

type Kubernetes struct {
	Core                v1.CoreV1Interface
	Dynamic             dynamic.Interface
	SourceID, Namespace string
}

func NewKubernetes(config *rest.Config, source, namespace string) (*Kubernetes, error) {
	core, err := v1.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Kubernetes{Core: core, Dynamic: dyn, SourceID: strings.TrimSpace(source), Namespace: namespace}, nil
}
func (k *Kubernetes) Collect(ctx context.Context) model.Batch {
	now := time.Now().UTC()
	root := model.Observation{Workloads: []model.Workload{}}
	batch := model.Batch{Source: model.Source{Kind: "kubernetes", ID: k.SourceID}, ObservedAt: now, Hosts: []model.Observation{root}}
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	nodes, err := k.Core.Nodes().List(requestCtx, metav1.ListOptions{})
	cancel()
	if err != nil || len(nodes.Items) > 127 {
		return batch
	}
	metrics := map[string]model.Metrics{}
	requestCtx, cancel = context.WithTimeout(ctx, 5*time.Second)
	observations, err := k.Dynamic.Resource(schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}).List(requestCtx, metav1.ListOptions{})
	cancel()
	if err == nil {
		for _, item := range observations.Items {
			stamp, _ := item.Object["timestamp"].(string)
			at, err := time.Parse(time.RFC3339Nano, stamp)
			if err != nil || now.Sub(at) > 2*time.Minute || at.After(now.Add(30*time.Second)) {
				continue
			}
			usage, _ := item.Object["usage"].(map[string]interface{})
			cpuRaw, _ := usage["cpu"].(string)
			memRaw, _ := usage["memory"].(string)
			cpu, cpuErr := resource.ParseQuantity(cpuRaw)
			mem, memErr := resource.ParseQuantity(memRaw)
			m := model.Metrics{}
			if cpuErr == nil && cpu.Sign() >= 0 {
				cores := cpu.AsApproximateFloat64()
				m.CPUPercent = &cores
			}
			if memErr == nil && mem.Sign() >= 0 {
				value := mem.Value()
				m.MemoryUsedBytes = &value
			}
			metrics[item.GetName()] = m
		}
	}
	requestCtx, cancel = context.WithTimeout(ctx, 5*time.Second)
	pods, podErr := k.Core.Pods(k.Namespace).List(requestCtx, metav1.ListOptions{})
	cancel()
	byNode := map[string][]model.Workload{}
	root.InventoryAvailable = podErr == nil
	if podErr == nil {
		for _, pod := range pods.Items {
			restarts := 0
			for _, c := range pod.Status.ContainerStatuses {
				restarts += int(c.RestartCount)
			}
			for _, c := range pod.Status.InitContainerStatuses {
				restarts += int(c.RestartCount)
			}
			workload := model.Workload{Name: pod.Name, Namespace: pod.Namespace, State: string(pod.Status.Phase), Restarts: restarts}
			if workload.State == "" {
				workload.State = "Unknown"
			}
			byNode[pod.Spec.NodeName] = append(byNode[pod.Spec.NodeName], workload)
			if len(root.Workloads) < 1000 {
				root.Workloads = append(root.Workloads, workload)
			} else {
				root.InventoryAvailable = false
			}
		}
	}
	allCPU, allMem := len(nodes.Items) > 0, len(nodes.Items) > 0
	var usedCores, totalCores float64
	var usedMem, totalMem int64
	for _, node := range nodes.Items {
		m := metrics[node.Name]
		capacity := node.Status.Capacity.Cpu().AsApproximateFloat64()
		capacityMem := node.Status.Capacity.Memory().Value()
		if m.CPUPercent != nil && capacity > 0 {
			usedCores += *m.CPUPercent
			percent := 100 * *m.CPUPercent / capacity
			if percent <= 100 {
				m.CPUPercent = &percent
			} else {
				m.CPUPercent = nil
			}
		} else {
			m.CPUPercent = nil
		}
		totalCores += capacity
		if capacityMem > 0 {
			m.MemoryTotalBytes = &capacityMem
		}
		if m.MemoryUsedBytes != nil && (capacityMem <= 0 || *m.MemoryUsedBytes > capacityMem) {
			m.MemoryUsedBytes = nil
		}
		if m.CPUPercent == nil {
			allCPU = false
		}
		if m.MemoryUsedBytes == nil || m.MemoryTotalBytes == nil {
			allMem = false
		} else {
			usedMem += *m.MemoryUsedBytes
			totalMem += capacityMem
		}
		workloads := byNode[node.Name]
		if workloads == nil {
			workloads = []model.Workload{}
		}
		inventoryAvailable := podErr == nil
		if len(workloads) > 1000 {
			workloads = workloads[:1000]
			inventoryAvailable = false
		}
		// Readiness is independent of measurement availability.
		ready := "Unknown"
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				ready = string(condition.Status)
			}
		}
		batch.Hosts = append(batch.Hosts, model.Observation{Node: node.Name, Metrics: m, Workloads: workloads, InventoryAvailable: inventoryAvailable, Ready: ready})
	}
	if allCPU && totalCores > 0 {
		value := 100 * usedCores / totalCores
		root.CPUPercent = &value
	}
	if allMem {
		root.MemoryUsedBytes = &usedMem
		root.MemoryTotalBytes = &totalMem
	}
	batch.Hosts[0] = root
	return batch
}
