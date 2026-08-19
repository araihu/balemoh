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
				Selector: map[string]string{"app": "grafana"},
				Ports:    []corev1.ServicePort{{Name: "web", Port: 3000, Protocol: corev1.ProtocolTCP}},
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
	if service.Metadata["label.app"] != "grafana" || !hasEndpointURL(service, "https://grafana.example.test/") || !hasEndpointURL(service, "//route.example.test/grafana") || !hasEndpointProvenance(service, "kubernetes.service") {
		t.Fatalf("service candidate = %#v, want route URLs and service port observation", service)
	}
	if len(service.Images) != 0 {
		t.Fatalf("service images = %#v, want empty", service.Images)
	}

	pod := byKind["pod"]
	if !reflect.DeepEqual(pod.Images, []string{"busybox:1.36", "grafana/grafana:11"}) {
		t.Fatalf("pod images = %#v, want init/container images without duplicates", pod.Images)
	}
	if !hasEndpointURL(pod, "https://grafana.example.test/") || !hasEndpointURL(pod, "//route.example.test/grafana") || !hasEndpointProvenance(pod, "kubernetes.pod") {
		t.Fatalf("pod endpoints = %#v, want route URLs and container port observation", pod.Endpoints)
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
	if routeCandidate.Metadata["kubernetes.services"] != "apps/grafana" || ingress.Metadata["kubernetes.services"] != "apps/grafana" || pod.Metadata["kubernetes.services"] != "apps/grafana" {
		t.Fatalf("service cross references = route %q ingress %q pod %q, want apps/grafana", routeCandidate.Metadata["kubernetes.services"], ingress.Metadata["kubernetes.services"], pod.Metadata["kubernetes.services"])
	}
}

func TestDiscovererPrioritizesHTTPRouteBackendsAndKeepsExternalServices(t *testing.T) {
	typedClient := kubernetesfake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "routed", Namespace: "apps"},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": "routed"},
				Ports:    []corev1.ServicePort{{Name: "http", Port: 8080, Protocol: corev1.ProtocolTCP}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "unreferenced", Namespace: "apps"},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": "unreferenced"},
				Ports:    []corev1.ServicePort{{Name: "http", Port: 8081, Protocol: corev1.ProtocolTCP}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: "apps"},
			Spec: corev1.ServiceSpec{
				Type:         corev1.ServiceTypeExternalName,
				ExternalName: "outside.example.test",
				Ports:        []corev1.ServicePort{{Name: "https", Port: 443, Protocol: corev1.ProtocolTCP}},
			},
		},
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "routed-ingress", Namespace: "apps"},
			Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{
				Host: "routed.example.test",
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{
					Path:    "/",
					Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "routed", Port: networkingv1.ServiceBackendPort{Number: 8080}}},
				}}}},
			}}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "routed-0", Namespace: "apps", Labels: map[string]string{"app": "routed"}},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "example/routed:1"}}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "unreferenced-0", Namespace: "apps", Labels: map[string]string{"app": "unreferenced"}},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "example/unreferenced:1"}}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "orphan-0", Namespace: "apps", Labels: map[string]string{"app": "orphan"}},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "example/orphan:1"}}},
		},
	)
	route := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]interface{}{
			"name":      "routed-route",
			"namespace": "apps",
		},
		"spec": map[string]interface{}{
			"hostnames": []interface{}{"routed.example.test"},
			"rules": []interface{}{map[string]interface{}{
				"backendRefs": []interface{}{map[string]interface{}{"name": "routed", "port": int64(8080)}},
			}},
		},
	}}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), route)
	discoverer, err := NewDiscoverer(typedClient, dynamicClient, "cluster-1", "apps")
	if err != nil {
		t.Fatalf("NewDiscoverer() error = %v", err)
	}

	candidates, err := discoverer.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(candidates) != 5 {
		t.Fatalf("Discover() candidates = %d, want route/ingress + routed/external services + routed pod: %#v", len(candidates), candidates)
	}

	byIdentity := make(map[string]catalog.Candidate, len(candidates))
	for _, candidate := range candidates {
		byIdentity[candidate.Resource.Kind+"/"+candidate.Resource.Name] = candidate
	}
	for _, identity := range []string{"httproute/routed-route", "ingress/routed-ingress", "service/routed", "service/external", "pod/routed-0"} {
		if _, ok := byIdentity[identity]; !ok {
			t.Fatalf("missing staged candidate %q: %#v", identity, byIdentity)
		}
	}
	for _, identity := range []string{"service/unreferenced", "pod/unreferenced-0", "pod/orphan-0"} {
		if _, ok := byIdentity[identity]; ok {
			t.Fatalf("unexpected unreferenced candidate %q: %#v", identity, byIdentity[identity])
		}
	}
	if got := byIdentity["service/external"].Metadata["service.externalName"]; got != "outside.example.test" {
		t.Fatalf("external service name = %q, want outside.example.test", got)
	}
	for _, identity := range []string{"service/routed", "pod/routed-0"} {
		if !hasEndpointURL(byIdentity[identity], "//routed.example.test/") || !hasEndpointURL(byIdentity[identity], "http://routed.example.test/") {
			t.Fatalf("%s endpoints = %#v, want HTTPRoute and Ingress host observations", identity, byIdentity[identity].Endpoints)
		}
	}
}

func TestDiscovererFallsBackFromServicesToMatchingPods(t *testing.T) {
	typedClient := kubernetesfake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "apps"},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": "web"},
				Ports:    []corev1.ServicePort{{Port: 80, Protocol: corev1.ProtocolTCP}},
			},
		},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: "apps"}, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName, ExternalName: "outside.example.test"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-0", Namespace: "apps", Labels: map[string]string{"app": "web"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "example/web:1"}}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "orphan-0", Namespace: "apps", Labels: map[string]string{"app": "orphan"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "orphan", Image: "example/orphan:1"}}}},
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
	if len(candidates) != 3 {
		t.Fatalf("Discover() candidates = %d, want two services + matched pod: %#v", len(candidates), candidates)
	}
	for _, candidate := range candidates {
		if candidate.Resource.Kind == "pod" && candidate.Resource.Name == "orphan-0" {
			t.Fatalf("orphan Pod was staged: %#v", candidate)
		}
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

func hasEndpointURL(candidate catalog.Candidate, want string) bool {
	for _, endpoint := range candidate.Endpoints {
		if endpoint.URL == want {
			return true
		}
	}
	return false
}

func hasEndpointProvenance(candidate catalog.Candidate, want string) bool {
	for _, endpoint := range candidate.Endpoints {
		if endpoint.Provenance == want {
			return true
		}
	}
	return false
}
