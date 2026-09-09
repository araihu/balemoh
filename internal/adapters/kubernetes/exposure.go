package kubernetes

import (
	"encoding/json"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/araihu/balemoh/internal/application/catalog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func backendIdentity(service corev1.Service) string {
	if len(service.Spec.Selector) == 0 {
		return ""
	}
	ports := make([]string, 0, len(service.Spec.Ports))
	for _, port := range service.Spec.Ports {
		target := port.TargetPort.String()
		if target == "0" || target == "" {
			target = strconv.Itoa(int(port.Port))
		}
		protocol := string(port.Protocol)
		if protocol == "" {
			protocol = "TCP"
		}
		ports = append(ports, protocol+"/"+target)
	}
	if len(ports) == 0 {
		return ""
	}
	sort.Strings(ports)
	raw, _ := json.Marshal(struct {
		Namespace string
		Selector  map[string]string
		Ports     []string
	}{service.Namespace, service.Spec.Selector, ports})
	return string(raw)
}

func routeExposures(route *unstructured.Unstructured) []catalog.Exposure {
	parents, _, _ := unstructured.NestedSlice(route.Object, "spec", "parentRefs")
	scopes := []string{}
	for _, raw := range parents {
		parent, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := parent["name"].(string)
		if name == "" {
			continue
		}
		namespace, _ := parent["namespace"].(string)
		if namespace == "" {
			namespace = route.GetNamespace()
		}
		group, _ := parent["group"].(string)
		if group == "" {
			group = "gateway.networking.k8s.io"
		}
		kind, _ := parent["kind"].(string)
		if kind == "" {
			kind = "Gateway"
		}
		section, _ := parent["sectionName"].(string)
		port, _ := parent["port"].(int64)
		raw, _ := json.Marshal([]any{group, kind, namespace, name, section, port})
		scopes = append(scopes, string(raw))
	}
	sort.Strings(scopes)
	if len(scopes) == 0 {
		return nil
	}
	// Treat different listener sets as different routing contexts.
	scopeBytes, _ := json.Marshal(scopes)
	scope := string(scopeBytes)
	hosts, _, _ := unstructured.NestedStringSlice(route.Object, "spec", "hostnames")
	rules, _, _ := unstructured.NestedSlice(route.Object, "spec", "rules")
	var result []catalog.Exposure
	for index, raw := range rules {
		rule, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		backends, err := httpRouteBackends(rule, route.GetNamespace(), index)
		if err != nil {
			continue
		}
		markUnknown := func() {
			for _, backend := range backends {
				if backend.service {
					result = append(result, catalog.Exposure{Service: backend.namespace + "/" + backend.name, Match: "Unsupported"})
				}
			}
		}
		matches, _, _ := unstructured.NestedSlice(rule, "matches")
		if len(matches) == 0 {
			matches = []interface{}{map[string]interface{}{}}
		}
		filters, _, _ := unstructured.NestedSlice(rule, "filters")
		redirectHost, redirectPath := "", ""
		redirect := false
		unsupported := false
		for _, raw := range filters {
			filter, ok := raw.(map[string]interface{})
			if !ok {
				unsupported = true
				break
			}
			if filter["type"] == "RequestHeaderModifier" || filter["type"] == "ResponseHeaderModifier" || filter["type"] == "URLRewrite" {
				continue
			}
			if filter["type"] != "RequestRedirect" {
				unsupported = true
				break
			}
			redirect = true
			redirectHost, _, _ = unstructured.NestedString(filter, "requestRedirect", "hostname")
			pathType, _, _ := unstructured.NestedString(filter, "requestRedirect", "path", "type")
			if pathType != "" && pathType != "ReplaceFullPath" {
				unsupported = true
				break
			}
			redirectPath, _, _ = unstructured.NestedString(filter, "requestRedirect", "path", "replaceFullPath")
			if port, found, _ := unstructured.NestedInt64(filter, "requestRedirect", "port"); found && port != 80 && port != 443 {
				unsupported = true
			}
		}
		if unsupported {
			markUnknown()
			continue
		}
		for _, raw := range matches {
			match, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			// Header/query/method routing does not prove one app identity.
			if len(match) > 1 || (len(match) == 1 && match["path"] == nil) {
				markUnknown()
				continue
			}
			path, _, _ := unstructured.NestedString(match, "path", "value")
			if path == "" {
				path = "/"
			}
			matchType, _, _ := unstructured.NestedString(match, "path", "type")
			if matchType == "" {
				matchType = "PathPrefix"
			}
			if matchType != "Exact" && matchType != "PathPrefix" {
				markUnknown()
				continue
			}
			for _, host := range hosts {
				if host == "" || strings.Contains(host, "*") {
					markUnknown()
					continue
				}
				base := catalog.Exposure{Scope: scope, Host: strings.ToLower(host), Path: path, Match: matchType}
				if redirect {
					base.RedirectHost = strings.ToLower(redirectHost)
					if base.RedirectHost == "" {
						base.RedirectHost = base.Host
					}
					base.RedirectPath = redirectPath
					if base.RedirectPath == "" {
						base.RedirectPath = path
					}
					result = append(result, base)
					continue
				}
				for _, backend := range backends {
					if backend.service {
						e := base
						e.Service = backend.namespace + "/" + backend.name
						e.Port = backend.port
						result = append(result, e)
					}
				}
			}
		}
	}
	return result
}

func externalServiceEndpoints(service corev1.Service) []catalog.Endpoint {
	hosts := append([]string{}, service.Spec.ExternalIPs...)
	for _, ingress := range service.Status.LoadBalancer.Ingress {
		if ingress.IP != "" {
			hosts = append(hosts, ingress.IP)
		} else if ingress.Hostname != "" {
			hosts = append(hosts, ingress.Hostname)
		}
	}
	if service.Spec.ExternalName != "" {
		hosts = append(hosts, service.Spec.ExternalName)
	}
	var endpoints []catalog.Endpoint
	for _, host := range hosts {
		for _, port := range service.Spec.Ports {
			// Preserve TCP/UDP instead of inventing an HTTP scheme from a port number.
			protocol := strings.ToLower(string(port.Protocol))
			if protocol == "" {
				protocol = "tcp"
			}
			endpoints = append(endpoints, catalog.Endpoint{Name: port.Name, URL: protocol + "://" + net.JoinHostPort(host, strconv.Itoa(int(port.Port))), Port: int(port.Port), Protocol: string(port.Protocol), Provenance: "kubernetes.service.external"})
		}
	}
	return endpoints
}

func nodeServiceEndpoints(service corev1.Service, nodes []corev1.Node) []catalog.Endpoint {
	var result []catalog.Endpoint
	for _, node := range nodes {
		host := ""
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeExternalIP {
				host = address.Address
				break
			}
			if address.Type == corev1.NodeInternalIP {
				host = address.Address
			}
		}
		if host == "" {
			continue
		}
		for _, port := range service.Spec.Ports {
			if port.NodePort > 0 {
				protocol := strings.ToLower(string(port.Protocol))
				if protocol == "" {
					protocol = "tcp"
				}
				result = append(result, catalog.Endpoint{Name: port.Name, URL: protocol + "://" + net.JoinHostPort(host, strconv.Itoa(int(port.NodePort))), Port: int(port.NodePort), Protocol: string(port.Protocol), Provenance: "kubernetes.service.nodeport"})
			}
		}
	}
	return result
}
