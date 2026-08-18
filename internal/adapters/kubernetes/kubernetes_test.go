package kubernetes

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/araihu/balemoh/internal/application/catalog"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestDiscovererDiscoversRoutesServicesPodsAndImages(t *testing.T) {
	typedClient := kubernetesfake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "grafana",
				Namespace: "apps",
				Labels:    map[string]string{"app": "grafana"},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Name: "web", Port: 3000, Protocol: corev1.ProtocolTCP}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "grafana-0",
				Namespace: "apps",
				Labels:    map[string]string{"app": "grafana"},
			},
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{{Name: "init", Image: "busybox:1.36"}},
				Containers: []corev1.Container{
					{
						Name:  "grafana",
						Image: "grafana/grafana:11",
						Ports: []corev1.ContainerPort{{Name: "web", ContainerPort: 3000, Protocol: corev1.ProtocolTCP}},
					},
					{Name: "sidecar", Image: "grafana/grafana:11"},
				},
			},
		},
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "grafana",
				Namespace: "apps",
				Labels:    map[string]string{"app": "grafana"},
			},
			Spec: networkingv1.IngressSpec{
				TLS: []networkingv1.IngressTLS{{Hosts: []string{"grafana.example.test"}}},
				Rules: []networkingv1.IngressRule{{
					Host: "grafana.example.test",
					IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path: "/",
							Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
								Name: "grafana",
								Port: networkingv1.ServiceBackendPort{Number: 3000},
							}},
						}},
					}},
				}},
			},
		},
	)
	route := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]interface{}{
			"name":      "grafana-route",
			"namespace": "apps",
			"labels":    map[string]interface{}{"app": "grafana"},
		},
		"spec": map[string]interface{}{
			"hostnames": []interface{}{"route.example.test"},
			"rules": []interface{}{
				map[string]interface{}{
					"matches": []interface{}{
						map[string]interface{}{
							"path": map[string]interface{}{"type": "PathPrefix", "value": "/grafana"},
						},
					},
					"backendRefs": []interface{}{
						map[string]interface{}{"name": "grafana", "port": int64(3000)},
					},
				},
			},
		},
	}}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), route)

	discoverer, err := NewDiscoverer(typedClient, dynamicClient, "cluster-1", "")
	if err != nil {
		t.Fatalf("NewDiscoverer() error = %v", err)
	}

	candidates, err := discoverer.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(candidates) != 4 {
		t.Fatalf("Discover() candidates = %d, want 4: %#v", len(candidates), candidates)
	}

	byKind := make(map[string]catalog.Candidate, len(candidates))
	for _, candidate := range candidates {
		byKind[candidate.Resource.Kind] = candidate
		if err := candidate.Validate(); err != nil {
			t.Fatalf("candidate %s invalid: %v", candidate.Resource.Kind, err)
		}
		if candidate.Source.Kind != sourceKind || candidate.Source.ID != "cluster-1" {
			t.Fatalf("candidate source = %#v, want kubernetes/cluster-1", candidate.Source)
		}
	}

	service := byKind["service"]
	if service.Metadata["label.app"] != "grafana" || len(service.Endpoints) != 1 || service.Endpoints[0].Port != 3000 || service.Endpoints[0].Provenance != "kubernetes.service" {
		t.Fatalf("service candidate = %#v, want label and port observation", service)
	}
	if len(service.Images) != 0 {
		t.Fatalf("service images = %#v, want empty", service.Images)
	}

	pod := byKind["pod"]
	if !reflect.DeepEqual(pod.Images, []string{"busybox:1.36", "grafana/grafana:11"}) {
		t.Fatalf("pod images = %#v, want init/container images without duplicates", pod.Images)
	}
	if len(pod.Endpoints) != 1 || pod.Endpoints[0].Port != 3000 || pod.Endpoints[0].Protocol != "TCP" {
		t.Fatalf("pod endpoints = %#v, want container port observation", pod.Endpoints)
	}

	ingress := byKind["ingress"]
	if len(ingress.Endpoints) != 1 || ingress.Endpoints[0].URL != "https://grafana.example.test/" || ingress.Endpoints[0].Port != 3000 {
		t.Fatalf("ingress endpoints = %#v, want exact TLS route", ingress.Endpoints)
	}

	routeCandidate := byKind["httproute"]
	if len(routeCandidate.Endpoints) != 1 || routeCandidate.Endpoints[0].URL != "//route.example.test/grafana" || routeCandidate.Endpoints[0].Port != 3000 {
		t.Fatalf("HTTPRoute endpoints = %#v, want exact scheme-relative host/path", routeCandidate.Endpoints)
	}
	if routeCandidate.Endpoints[0].Provenance != "kubernetes.httproute" {
		t.Fatalf("HTTPRoute provenance = %q, want kubernetes.httproute", routeCandidate.Endpoints[0].Provenance)
	}
}

func TestDiscovererHonorsNamespaceAndOptionalHTTPRouteCRD(t *testing.T) {
	typedClient := kubernetesfake.NewSimpleClientset(
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "inside", Namespace: "apps"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "outside", Namespace: "other"}},
	)
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		httpRouteGVR: "HTTPRouteList",
	})
	dynamicClient.PrependReactor("list", "httproutes", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apiNotFoundError()
	})

	discoverer, err := NewDiscoverer(typedClient, dynamicClient, "cluster-1", "apps")
	if err != nil {
		t.Fatalf("NewDiscoverer() error = %v", err)
	}
	candidates, err := discoverer.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].Resource.Kind != "service" || candidates[0].Resource.Name != "inside" {
		t.Fatalf("namespace-scoped candidates = %#v, want only apps/inside service", candidates)
	}
}

func TestDiscovererPropagatesHTTPRoutePermissionErrors(t *testing.T) {
	typedClient := kubernetesfake.NewSimpleClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		httpRouteGVR: "HTTPRouteList",
	})
	dynamicClient.PrependReactor("list", "httproutes", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden by test RBAC")
	})
	discoverer, err := NewDiscoverer(typedClient, dynamicClient, "cluster-1", "")
	if err != nil {
		t.Fatalf("NewDiscoverer() error = %v", err)
	}

	_, err = discoverer.Discover(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTPRoute") || !strings.Contains(err.Error(), "forbidden by test RBAC") {
		t.Fatalf("Discover() error = %v, want named HTTPRoute permission error", err)
	}
}

func TestHostIsTLSWildcardMatchesOneDNSLabel(t *testing.T) {
	tlsHosts := map[string]struct{}{"*.example.test": {}}
	for _, test := range []struct {
		host string
		want bool
	}{
		{host: "app.example.test", want: true},
		{host: "APP.EXAMPLE.TEST", want: true},
		{host: "app.internal.example.test", want: false},
		{host: "example.test", want: false},
	} {
		t.Run(test.host, func(t *testing.T) {
			if got := hostIsTLS(test.host, tlsHosts); got != test.want {
				t.Fatalf("hostIsTLS(%q) = %v, want %v", test.host, got, test.want)
			}
		})
	}
}

func TestNewDiscovererRejectsMissingDependencies(t *testing.T) {
	if _, err := NewDiscoverer(nil, nil, "cluster-1", ""); err == nil {
		t.Fatal("NewDiscoverer(nil, nil) error = nil, want dependency error")
	}
	if _, err := NewDiscoverer(kubernetesfake.NewSimpleClientset(), dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		httpRouteGVR: "HTTPRouteList",
	}), " ", ""); err == nil {
		t.Fatal("NewDiscoverer(blank source) error = nil, want source ID error")
	}
}

func apiNotFoundError() error {
	return apierrors.NewNotFound(schema.GroupResource{Group: httpRouteGVR.Group, Resource: httpRouteGVR.Resource}, "httproutes")
}
