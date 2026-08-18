package container

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/araihu/balemoh/internal/application/catalog"
	dockertypes "github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

const sourceKind = "container"

const (
	composeProjectLabel       = "com.docker.compose.project"
	composeServiceLabel       = "com.docker.compose.service"
	composeWorkingDirLabel    = "com.docker.compose.project.working_dir"
	composeConfigFilesLabel   = "com.docker.compose.project.config_files"
	composeContainerNumberKey = "com.docker.compose.container-number"
	podmanProjectLabel        = "io.podman.compose.project"
	podmanServiceLabel        = "io.podman.compose.service"
	podmanWorkingDirLabel     = "io.podman.compose.project.working_dir"
	podmanConfigFilesLabel    = "io.podman.compose.project.config_files"
	podmanContainerNumberKey  = "io.podman.compose.container-number"
)

// Client is the read-only portion of the Docker-compatible container API used
// by the discoverer. Podman exposes the same API over its Docker-compatible
// socket.
type Client interface {
	ContainerList(context.Context, dockertypes.ListOptions) ([]dockertypes.Summary, error)
}

// Discoverer reads running Docker or Podman containers without mutating the
// runtime. The source ID must be stable for the lifetime of the host.
type Discoverer struct {
	client   Client
	sourceID string
}

type composeKey struct {
	project string
	service string
}

type composeAggregate struct {
	key         composeKey
	images      map[string]struct{}
	endpoints   map[catalog.Endpoint]struct{}
	hostIPs     map[string]struct{}
	containers  int
	workingDir  string
	configFiles string
}

func NewDiscoverer(client Client, sourceID string) (*Discoverer, error) {
	if client == nil {
		return nil, errors.New("container client must not be nil")
	}
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return nil, errors.New("container source ID must not be empty")
	}
	if strings.ContainsRune(sourceID, '\x00') {
		return nil, errors.New("container source ID must not contain NUL")
	}
	return &Discoverer{client: client, sourceID: sourceID}, nil
}

func NewDiscovererFromConfig(host, sourceID string) (*Discoverer, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, errors.New("container host must not be empty")
	}
	client, err := dockerclient.NewClientWithOpts(
		dockerclient.WithHost(host),
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create container client: %w", err)
	}
	return NewDiscoverer(client, sourceID)
}

func (d *Discoverer) Name() string {
	return sourceKind + "/" + d.sourceID
}

func (d *Discoverer) Source() catalog.SourceRef {
	return catalog.SourceRef{Kind: sourceKind, ID: d.sourceID}
}

func (d *Discoverer) Discover(ctx context.Context) ([]catalog.Candidate, error) {
	containers, err := d.client.ContainerList(ctx, dockertypes.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list running containers: %w", err)
	}

	observedAt := time.Now().UTC()
	candidates := make([]catalog.Candidate, 0, len(containers)*2)
	composes := make(map[composeKey]*composeAggregate)
	for _, summary := range containers {
		name := containerName(summary)
		if name == "" {
			continue
		}

		endpoints, hostIPs := publishedPorts(summary.Ports)
		candidate := d.newCandidate("container", "", name, "Container", observedAt)
		if id := strings.TrimSpace(summary.ID); id != "" {
			candidate.Metadata["container.id"] = id
		}
		if image := strings.TrimSpace(summary.Image); image != "" {
			candidate.Metadata["container.image"] = image
			candidate.Images = []string{image}
		}
		if state := strings.TrimSpace(string(summary.State)); state != "" {
			candidate.Metadata["container.state"] = state
		}
		setPublishedIPs(candidate.Metadata, hostIPs)
		addComposeMetadata(candidate.Metadata, summary.Labels)
		candidate.Endpoints = endpoints
		candidates = append(candidates, candidate)

		key, ok := composeIdentity(summary.Labels)
		if !ok {
			continue
		}
		aggregate := composes[key]
		if aggregate == nil {
			aggregate = &composeAggregate{
				key:       key,
				images:    make(map[string]struct{}),
				endpoints: make(map[catalog.Endpoint]struct{}),
				hostIPs:   make(map[string]struct{}),
			}
			composes[key] = aggregate
		}
		aggregate.add(summary, endpoints, hostIPs)
	}

	for _, aggregate := range composes {
		candidate := d.newCandidate("compose-service", aggregate.key.project, aggregate.key.service, "Compose Service", observedAt)
		candidate.Metadata["compose.project"] = aggregate.key.project
		candidate.Metadata["compose.service"] = aggregate.key.service
		candidate.Metadata["compose.containers"] = strconv.Itoa(aggregate.containers)
		if aggregate.workingDir != "" {
			candidate.Metadata["compose.workingDir"] = aggregate.workingDir
		}
		if aggregate.configFiles != "" {
			candidate.Metadata["compose.configFiles"] = aggregate.configFiles
		}
		candidate.Images = sortedSet(aggregate.images)
		candidate.Endpoints = sortedEndpoints(aggregate.endpoints)
		setPublishedIPs(candidate.Metadata, sortedSet(aggregate.hostIPs))
		candidates = append(candidates, candidate)
	}

	sortCandidates(candidates)
	return candidates, nil
}

func (d *Discoverer) newCandidate(kind, namespace, name, description string, observedAt time.Time) catalog.Candidate {
	candidate := catalog.NewCandidate(
		catalog.SourceRef{Kind: sourceKind, ID: d.sourceID},
		catalog.ResourceRef{Kind: kind, Namespace: namespace, Name: name},
		observedAt,
	)
	candidate.Description = description
	candidate.Metadata = map[string]string{"container.kind": kind}
	return candidate
}

func (a *composeAggregate) add(summary dockertypes.Summary, endpoints []catalog.Endpoint, hostIPs []string) {
	a.containers++
	if image := strings.TrimSpace(summary.Image); image != "" {
		a.images[image] = struct{}{}
	}
	for _, endpoint := range endpoints {
		a.endpoints[endpoint] = struct{}{}
	}
	for _, hostIP := range hostIPs {
		a.hostIPs[hostIP] = struct{}{}
	}
	if a.workingDir == "" {
		a.workingDir = composeLabel(summary.Labels, composeWorkingDirLabel, podmanWorkingDirLabel)
	}
	if a.configFiles == "" {
		a.configFiles = composeLabel(summary.Labels, composeConfigFilesLabel, podmanConfigFilesLabel)
	}
}

func composeIdentity(labels map[string]string) (composeKey, bool) {
	project := composeLabel(labels, composeProjectLabel, podmanProjectLabel)
	service := composeLabel(labels, composeServiceLabel, podmanServiceLabel)
	if project == "" || service == "" {
		return composeKey{}, false
	}
	return composeKey{project: project, service: service}, true
}

func addComposeMetadata(metadata map[string]string, labels map[string]string) {
	if project := composeLabel(labels, composeProjectLabel, podmanProjectLabel); project != "" {
		metadata["compose.project"] = project
	}
	if service := composeLabel(labels, composeServiceLabel, podmanServiceLabel); service != "" {
		metadata["compose.service"] = service
	}
	if workingDir := composeLabel(labels, composeWorkingDirLabel, podmanWorkingDirLabel); workingDir != "" {
		metadata["compose.workingDir"] = workingDir
	}
	if configFiles := composeLabel(labels, composeConfigFilesLabel, podmanConfigFilesLabel); configFiles != "" {
		metadata["compose.configFiles"] = configFiles
	}
	if containerNumber := composeLabel(labels, composeContainerNumberKey, podmanContainerNumberKey); containerNumber != "" {
		metadata["compose.containerNumber"] = containerNumber
	}
}

func composeLabel(labels map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(labels[key]); value != "" {
			return value
		}
	}
	return ""
}

func containerName(summary dockertypes.Summary) string {
	for _, rawName := range summary.Names {
		if name := strings.Trim(strings.TrimSpace(rawName), "/"); name != "" {
			return name
		}
	}
	return strings.TrimSpace(summary.ID)
}

func publishedPorts(ports []dockertypes.Port) ([]catalog.Endpoint, []string) {
	endpoints := make(map[catalog.Endpoint]struct{})
	hostIPs := make(map[string]struct{})
	for _, port := range ports {
		if port.PublicPort == 0 {
			continue
		}
		protocol := strings.ToLower(strings.TrimSpace(port.Type))
		if protocol == "" {
			protocol = "tcp"
		}
		hostIP := normalizeHostIP(port.IP)
		hostIPs[hostIP] = struct{}{}
		endpoints[catalog.Endpoint{
			Name:       publishedPortName(protocol, port),
			URL:        protocol + "://" + net.JoinHostPort(hostIP, strconv.Itoa(int(port.PublicPort))),
			Port:       int(port.PublicPort),
			Protocol:   strings.ToUpper(protocol),
			Provenance: "container.port",
		}] = struct{}{}
	}
	return sortedEndpoints(endpoints), sortedSet(hostIPs)
}

func publishedPortName(protocol string, port dockertypes.Port) string {
	if port.PrivatePort > 0 && port.PrivatePort != port.PublicPort {
		return fmt.Sprintf("%s/%d->%d", protocol, port.PublicPort, port.PrivatePort)
	}
	return fmt.Sprintf("%s/%d", protocol, port.PublicPort)
}

func normalizeHostIP(ip string) string {
	ip = strings.TrimSpace(ip)
	ip = strings.TrimPrefix(ip, "[")
	ip = strings.TrimSuffix(ip, "]")
	if ip == "" {
		return "0.0.0.0"
	}
	return ip
}

func setPublishedIPs(metadata map[string]string, hostIPs []string) {
	if len(hostIPs) > 0 {
		metadata["container.publishedIPs"] = strings.Join(hostIPs, ",")
	}
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sortedEndpoints(values map[catalog.Endpoint]struct{}) []catalog.Endpoint {
	result := make([]catalog.Endpoint, 0, len(values))
	for endpoint := range values {
		result = append(result, endpoint)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].URL != result[j].URL {
			return result[i].URL < result[j].URL
		}
		if result[i].Port != result[j].Port {
			return result[i].Port < result[j].Port
		}
		if result[i].Protocol != result[j].Protocol {
			return result[i].Protocol < result[j].Protocol
		}
		return result[i].Name < result[j].Name
	})
	return result
}

func sortCandidates(candidates []catalog.Candidate) {
	sort.Slice(candidates, func(i, j int) bool {
		left := candidates[i].Resource
		right := candidates[j].Resource
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Namespace != right.Namespace {
			return left.Namespace < right.Namespace
		}
		return left.Name < right.Name
	})
}
