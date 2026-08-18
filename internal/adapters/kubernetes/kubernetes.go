package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/araihu/balemoh/internal/application/catalog"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	kubernetesclient "k8s.io/client-go/kubernetes"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"
	networkingv1typed "k8s.io/client-go/kubernetes/typed/networking/v1"
	"k8s.io/client-go/rest"
)

const sourceKind = "kubernetes"

var httpRouteGVR = schema.GroupVersionResource{
	Group:    "gateway.networking.k8s.io",
	Version:  "v1",
	Resource: "httproutes",
}

// Discoverer reads Kubernetes resources without mutating the cluster. The
// sourceID must be stable for the lifetime of a cluster, preferably its UID.
type Discoverer struct {
	coreClient       corev1typed.CoreV1Interface
	networkingClient networkingv1typed.NetworkingV1Interface
	dynamicClient    dynamic.Interface
	sourceID         string
	namespace        string
}

func NewDiscoverer(client kubernetesclient.Interface, dynamicClient dynamic.Interface, sourceID, namespace string) (*Discoverer, error) {
	if client == nil {
		return nil, errors.New("kubernetes client must not be nil")
	}
	if dynamicClient == nil {
		return nil, errors.New("kubernetes dynamic client must not be nil")
	}
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return nil, errors.New("kubernetes source ID must not be empty")
	}
	if strings.ContainsRune(sourceID, '\x00') {
		return nil, errors.New("kubernetes source ID must not contain NUL")
	}
	return &Discoverer{
		coreClient:       client.CoreV1(),
		networkingClient: client.NetworkingV1(),
		dynamicClient:    dynamicClient,
		sourceID:         sourceID,
		namespace:        strings.TrimSpace(namespace),
	}, nil
}

func NewDiscovererFromConfig(config *rest.Config, sourceID, namespace string) (*Discoverer, error) {
	if config == nil {
		return nil, errors.New("kubernetes REST config must not be nil")
	}
	client, err := kubernetesclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes dynamic client: %w", err)
	}
	return NewDiscoverer(client, dynamicClient, sourceID, namespace)
}

func (d *Discoverer) Name() string {
	return sourceKind + "/" + d.sourceID
}

func (d *Discoverer) Discover(ctx context.Context) ([]catalog.Candidate, error) {
	observedAt := time.Now().UTC()
	candidates := make([]catalog.Candidate, 0)

	httpRoutes, err := d.discoverHTTPRoutes(ctx, observedAt)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, httpRoutes...)

	ingresses, err := d.discoverIngresses(ctx, observedAt)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, ingresses...)

	services, err := d.discoverServices(ctx, observedAt)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, services...)

	pods, err := d.discoverPods(ctx, observedAt)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, pods...)

	sortCandidates(candidates)
	return candidates, nil
}

func (d *Discoverer) discoverServices(ctx context.Context, observedAt time.Time) ([]catalog.Candidate, error) {
	list, err := d.coreClient.Services(d.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list Kubernetes Services: %w", err)
	}
	candidates := make([]catalog.Candidate, 0, len(list.Items))
	for _, service := range list.Items {
		candidate := d.newCandidate("service", service.Namespace, service.Name, "Kubernetes Service", service.Labels, observedAt)
		candidate.Metadata["service.type"] = string(service.Spec.Type)
		candidate.Endpoints = serviceEndpoints(service)
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func (d *Discoverer) discoverPods(ctx context.Context, observedAt time.Time) ([]catalog.Candidate, error) {
	list, err := d.coreClient.Pods(d.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list Kubernetes Pods: %w", err)
	}
	candidates := make([]catalog.Candidate, 0, len(list.Items))
	for _, pod := range list.Items {
		candidate := d.newCandidate("pod", pod.Namespace, pod.Name, "Kubernetes Pod", pod.Labels, observedAt)
		if pod.Status.Phase != "" {
			candidate.Metadata["pod.phase"] = string(pod.Status.Phase)
		}
		candidate.Images, candidate.Endpoints = podObservations(pod)
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func (d *Discoverer) discoverIngresses(ctx context.Context, observedAt time.Time) ([]catalog.Candidate, error) {
	list, err := d.networkingClient.Ingresses(d.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list Kubernetes Ingresses: %w", err)
	}
	candidates := make([]catalog.Candidate, 0, len(list.Items))
	for _, ingress := range list.Items {
		candidate := d.newCandidate("ingress", ingress.Namespace, ingress.Name, "Kubernetes Ingress", ingress.Labels, observedAt)
		candidate.Endpoints = ingressEndpoints(ingress)
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func (d *Discoverer) discoverHTTPRoutes(ctx context.Context, observedAt time.Time) ([]catalog.Candidate, error) {
	list, err := d.dynamicClient.Resource(httpRouteGVR).Namespace(d.namespace).List(ctx, metav1.ListOptions{})
	if apierrors.IsNotFound(err) {
		// Gateway API is optional. A cluster without the HTTPRoute CRD still
		// yields the core Kubernetes candidates.
		return []catalog.Candidate{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list Kubernetes HTTPRoutes: %w", err)
	}
	candidates := make([]catalog.Candidate, 0, len(list.Items))
	for index := range list.Items {
		route := &list.Items[index]
		candidate := d.newCandidate("httproute", route.GetNamespace(), route.GetName(), "Kubernetes HTTPRoute", route.GetLabels(), observedAt)
		endpoints, err := httpRouteEndpoints(route)
		if err != nil {
			return nil, fmt.Errorf("read Kubernetes HTTPRoute %s/%s: %w", route.GetNamespace(), route.GetName(), err)
		}
		candidate.Endpoints = endpoints
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func (d *Discoverer) newCandidate(kind, namespace, name, description string, labels map[string]string, observedAt time.Time) catalog.Candidate {
	candidate := catalog.NewCandidate(
		catalog.SourceRef{Kind: sourceKind, ID: d.sourceID},
		catalog.ResourceRef{Kind: kind, Namespace: namespace, Name: name},
		observedAt,
	)
	candidate.Description = description
	candidate.Metadata = resourceMetadata(kind, labels)
	return candidate
}

func resourceMetadata(kind string, labels map[string]string) map[string]string {
	metadata := map[string]string{"kubernetes.kind": kind}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		metadata["label."+key] = labels[key]
	}
	return metadata
}

func serviceEndpoints(service corev1.Service) []catalog.Endpoint {
	endpoints := make([]catalog.Endpoint, 0, len(service.Spec.Ports))
	for _, port := range service.Spec.Ports {
		name := strings.TrimSpace(port.Name)
		if name == "" {
			name = fmt.Sprintf("port-%d", port.Port)
		}
		endpoints = append(endpoints, catalog.Endpoint{
			Name:       name,
			Port:       int(port.Port),
			Protocol:   string(port.Protocol),
			Provenance: "kubernetes.service",
		})
	}
	return endpoints
}

func podObservations(pod corev1.Pod) ([]string, []catalog.Endpoint) {
	images := make([]string, 0)
	seenImages := make(map[string]struct{})
	endpoints := make([]catalog.Endpoint, 0)
	appendContainer := func(name, image string, ports []corev1.ContainerPort) {
		image = strings.TrimSpace(image)
		if image != "" {
			if _, exists := seenImages[image]; !exists {
				seenImages[image] = struct{}{}
				images = append(images, image)
			}
		}
		for _, port := range ports {
			endpointName := strings.TrimSpace(port.Name)
			if endpointName == "" {
				endpointName = strings.TrimSpace(name)
				if endpointName == "" {
					endpointName = fmt.Sprintf("port-%d", port.ContainerPort)
				}
			}
			endpoints = append(endpoints, catalog.Endpoint{
				Name:       endpointName,
				Port:       int(port.ContainerPort),
				Protocol:   string(port.Protocol),
				Provenance: "kubernetes.pod",
			})
		}
	}
	for _, container := range pod.Spec.InitContainers {
		appendContainer(container.Name, container.Image, container.Ports)
	}
	for _, container := range pod.Spec.Containers {
		appendContainer(container.Name, container.Image, container.Ports)
	}
	for _, container := range pod.Spec.EphemeralContainers {
		appendContainer(container.Name, container.Image, container.Ports)
	}
	return images, endpoints
}

func ingressEndpoints(ingress networkingv1.Ingress) []catalog.Endpoint {
	tlsHosts := make(map[string]struct{})
	tlsAnyHost := false
	for _, tls := range ingress.Spec.TLS {
		if len(tls.Hosts) == 0 {
			tlsAnyHost = true
		}
		for _, host := range tls.Hosts {
			tlsHosts[strings.ToLower(strings.TrimSpace(host))] = struct{}{}
		}
	}

	endpoints := make([]catalog.Endpoint, 0)
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		scheme := "http"
		if tlsAnyHost || hostIsTLS(rule.Host, tlsHosts) {
			scheme = "https"
		}
		for _, path := range rule.HTTP.Paths {
			name, port := ingressBackend(path.Backend)
			if name == "" {
				name = "route"
			}
			endpoints = append(endpoints, catalog.Endpoint{
				Name:       name,
				URL:        absoluteRouteURL(scheme, rule.Host, path.Path),
				Port:       port,
				Protocol:   scheme,
				Provenance: "kubernetes.ingress",
			})
		}
	}
	if ingress.Spec.DefaultBackend != nil {
		name, port := ingressBackend(*ingress.Spec.DefaultBackend)
		if name == "" {
			name = "default-backend"
		}
		endpoints = append(endpoints, catalog.Endpoint{
			Name:       name,
			Port:       port,
			Protocol:   "http",
			Provenance: "kubernetes.ingress",
		})
	}
	return endpoints
}

func ingressBackend(backend networkingv1.IngressBackend) (string, int) {
	if backend.Service != nil {
		return strings.TrimSpace(backend.Service.Name), int(backend.Service.Port.Number)
	}
	if backend.Resource != nil {
		return strings.TrimSpace(backend.Resource.Name), 0
	}
	return "", 0
}

func hostIsTLS(host string, tlsHosts map[string]struct{}) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if _, ok := tlsHosts[host]; ok {
		return true
	}
	for tlsHost := range tlsHosts {
		if strings.HasPrefix(tlsHost, "*.") && strings.HasSuffix(host, tlsHost[1:]) {
			return true
		}
	}
	return false
}

func httpRouteEndpoints(route *unstructured.Unstructured) ([]catalog.Endpoint, error) {
	hostnames, found, err := unstructured.NestedStringSlice(route.Object, "spec", "hostnames")
	if err != nil {
		return nil, fmt.Errorf("read hostnames: %w", err)
	}
	if !found || len(hostnames) == 0 {
		hostnames = []string{""}
	}
	rules, found, err := unstructured.NestedSlice(route.Object, "spec", "rules")
	if err != nil {
		return nil, fmt.Errorf("read rules: %w", err)
	}
	if !found {
		rules = nil
	}

	endpoints := make([]catalog.Endpoint, 0)
	for ruleIndex, rawRule := range rules {
		rule, ok := rawRule.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("rule %d has invalid shape", ruleIndex)
		}
		paths, err := httpRoutePaths(rule)
		if err != nil {
			return nil, fmt.Errorf("read rule %d paths: %w", ruleIndex, err)
		}
		backends, err := httpRouteBackends(rule, ruleIndex)
		if err != nil {
			return nil, fmt.Errorf("read rule %d backends: %w", ruleIndex, err)
		}
		for _, hostname := range hostnames {
			for _, path := range paths {
				for _, backend := range backends {
					endpoints = append(endpoints, catalog.Endpoint{
						Name:       backend.name,
						URL:        schemeRelativeRouteURL(hostname, path),
						Port:       backend.port,
						Protocol:   "http",
						Provenance: "kubernetes.httproute",
					})
				}
			}
		}
	}
	return endpoints, nil
}

func httpRoutePaths(rule map[string]interface{}) ([]string, error) {
	matches, found, err := unstructured.NestedSlice(rule, "matches")
	if err != nil {
		return nil, err
	}
	if !found || len(matches) == 0 {
		return []string{"/"}, nil
	}
	paths := make([]string, 0, len(matches))
	for index, rawMatch := range matches {
		match, ok := rawMatch.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("match %d has invalid shape", index)
		}
		path, found, err := unstructured.NestedString(match, "path", "value")
		if err != nil {
			return nil, err
		}
		if !found || strings.TrimSpace(path) == "" {
			path = "/"
		}
		paths = append(paths, normalizeRoutePath(path))
	}
	return paths, nil
}

type routeBackend struct {
	name string
	port int
}

func httpRouteBackends(rule map[string]interface{}, ruleIndex int) ([]routeBackend, error) {
	refs, found, err := unstructured.NestedSlice(rule, "backendRefs")
	if err != nil {
		return nil, err
	}
	if !found || len(refs) == 0 {
		return []routeBackend{{name: fmt.Sprintf("route-%d", ruleIndex)}}, nil
	}
	backends := make([]routeBackend, 0, len(refs))
	for index, rawRef := range refs {
		ref, ok := rawRef.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("backendRef %d has invalid shape", index)
		}
		name, _, err := unstructured.NestedString(ref, "name")
		if err != nil {
			return nil, err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = fmt.Sprintf("route-%d-%d", ruleIndex, index)
		}
		port, err := routeBackendPort(ref)
		if err != nil {
			return nil, fmt.Errorf("backendRef %d port: %w", index, err)
		}
		backends = append(backends, routeBackend{name: name, port: port})
	}
	return backends, nil
}

func routeBackendPort(ref map[string]interface{}) (int, error) {
	value, found := ref["port"]
	if !found {
		return 0, nil
	}
	var port int64
	switch value := value.(type) {
	case int:
		port = int64(value)
	case int32:
		port = int64(value)
	case int64:
		port = value
	case float64:
		if math.Trunc(value) != value {
			return 0, errors.New("must be an integer")
		}
		port = int64(value)
	case json.Number:
		parsed, err := strconv.ParseInt(string(value), 10, 64)
		if err != nil {
			return 0, err
		}
		port = parsed
	default:
		return 0, fmt.Errorf("has unsupported type %T", value)
	}
	if port < 0 || port > 65535 {
		return 0, fmt.Errorf("%d outside port range", port)
	}
	return int(port), nil
}

func normalizeRoutePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func absoluteRouteURL(scheme, host, path string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	return (&url.URL{Scheme: scheme, Host: host, Path: normalizeRoutePath(path)}).String()
}

func schemeRelativeRouteURL(host, path string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	return (&url.URL{Host: host, Path: normalizeRoutePath(path)}).String()
}

func sortCandidates(candidates []catalog.Candidate) {
	kindOrder := map[string]int{"httproute": 0, "ingress": 1, "service": 2, "pod": 3}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if kindOrder[left.Resource.Kind] != kindOrder[right.Resource.Kind] {
			return kindOrder[left.Resource.Kind] < kindOrder[right.Resource.Kind]
		}
		if left.Resource.Namespace != right.Resource.Namespace {
			return left.Resource.Namespace < right.Resource.Namespace
		}
		if left.Resource.Name != right.Resource.Name {
			return left.Resource.Name < right.Resource.Name
		}
		return left.ID < right.ID
	})
}

var _ catalog.Discoverer = (*Discoverer)(nil)
