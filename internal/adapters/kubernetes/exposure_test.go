package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	"testing"
)

func TestBackendIdentityAndExposedAddresses(t *testing.T) {
	service := corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "apps"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "immich"}, Ports: []corev1.ServicePort{{Name: "http", Port: 2283, TargetPort: intstr.FromString("http"), NodePort: 32283, Protocol: corev1.ProtocolTCP}}}}
	alias := service.DeepCopy()
	alias.Spec.Ports[0].Port = 80
	if backendIdentity(service) != backendIdentity(*alias) {
		t.Fatal("front port prevented target equivalence")
	}
	alias.Spec.Ports[0].TargetPort = intstr.FromInt32(9999)
	if backendIdentity(service) == backendIdentity(*alias) {
		t.Fatal("different targets equivalent")
	}
	service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "192.0.2.2"}}
	endpoints := externalServiceEndpoints(service)
	if len(endpoints) != 1 || endpoints[0].URL != "tcp://192.0.2.2:2283" {
		t.Fatalf("LB address=%#v", endpoints)
	}
	endpoints = nodeServiceEndpoints(service, []corev1.Node{{Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "2001:db8::1"}}}}})
	if len(endpoints) != 1 || endpoints[0].URL != "tcp://[2001:db8::1]:32283" {
		t.Fatalf("node address=%#v", endpoints)
	}
}
func TestRouteEvidencePreservesRedirectAndListener(t *testing.T) {
	route := &unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{"namespace": "apps"}, "spec": map[string]interface{}{
		"parentRefs": []interface{}{map[string]interface{}{"name": "gateway", "sectionName": "https"}}, "hostnames": []interface{}{"old.test"},
		"rules": []interface{}{map[string]interface{}{"filters": []interface{}{map[string]interface{}{"type": "RequestRedirect", "requestRedirect": map[string]interface{}{"hostname": "app.test", "path": map[string]interface{}{"type": "ReplaceFullPath", "replaceFullPath": "/"}}}}}},
	}}}
	evidence := routeExposures(route)
	if len(evidence) != 1 || evidence[0].RedirectHost != "app.test" || evidence[0].RedirectPath != "/" {
		t.Fatalf("redirect evidence=%#v", evidence)
	}
	other := route.DeepCopy()
	_ = unstructured.SetNestedSlice(other.Object, []interface{}{map[string]interface{}{"name": "gateway", "sectionName": "http"}}, "spec", "parentRefs")
	if routeExposures(other)[0].Scope == evidence[0].Scope {
		t.Fatal("listeners collapsed")
	}
}
