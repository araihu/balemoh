# Host telemetry

Hosts links to dedicated pages for Docker machines, Kubernetes clusters and nodes. Each page shows CPU, memory, one-minute history for 1h or 24h, reporting timestamps and workload inventory. Root pages also link to catalog services.

Telemetry is opt-in and independent of discovery. Enable it on each agent:

```dotenv
BALEMOH_TELEMETRY_ENABLED=true
```

Keep the existing source identity, gateway URL and source-bound federation token. The gateway accepts telemetry using the same registered source credential as discovery, through a separate endpoint. Upgrade gateway API and UI before enabling agents. No existing service settings change.

## Docker

Run the agent on the Linux machine it observes. The Docker socket and proc mount must belong to that same host. Telemetry requires a local Unix Docker-compatible socket. Add this read-only bind mount to the existing agent container:

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock:ro
  - /proc:/host/proc:ro
environment:
  BALEMOH_TELEMETRY_ENABLED: 'true'
  BALEMOH_HOST_PROC_PATH: '/host/proc'
```

The read-only socket mount does not make the Docker API read-only. Keep the existing socket access controls. The collector only lists containers and reads host CPU/memory counters. It does not need privileged mode or host PID namespace. A remote Docker API endpoint cannot supply machine CPU/RAM through this collector; deploy an agent with the local socket on that machine.

CPU needs two readings. Memory uses MemTotal minus MemAvailable. Missing proc files produce unavailable values. Docker Desktop reports its Linux VM when that VM's proc filesystem is mounted, not the macOS host.

## Kubernetes

Add these read-only rules to the agent's existing ClusterRole:

```yaml
- apiGroups: ['']
  resources: ['nodes', 'pods']
  verbs: ['list']
- apiGroups: ['metrics.k8s.io']
  resources: ['nodes']
  verbs: ['list']
```

CPU and memory come from the Node Metrics API. Without Metrics Server or with denied access, inventory and heartbeat still work. Pod inventory respects BALEMOH_KUBERNETES_NAMESPACE. Node metrics and cluster totals cover the whole node, including other namespaces. A cluster aggregate is unavailable until every listed node has that measurement.

## Data meaning

- Reporting means receipt and observation are no more than three minutes old.
- Partial data means the report arrived but at least one CPU/RAM measurement is missing.
- Stale means the last receipt or observation is over three minutes old. It does not prove the machine is offline.
- No telemetry means no observation has arrived. Discovery-only agents still appear through the catalog.

Unavailable workload inventory is distinguished from an empty inventory. Kubernetes node readiness is displayed separately. Docker restart counts and per-workload CPU/RAM are not collected.

Each source is limited to 128 retained hosts, with up to 1,000 workloads per host and 2 MiB per remote batch. Missing node inventory persists for 24 hours, then expires. Truncated workload lists are marked incomplete. Samples expire after 24 hours; registered roots remain. Repeated or old batches cannot refresh heartbeat. Range charts keep missing minutes as gaps and expose exact values in an expandable table. Host detail pages update through one SSE connection using Goshtoso HTMX. The UI server checks for changes every five seconds; agents still collect once per minute. Charts, latest values, reporting status and inventory update automatically, including expanded charts. Missing values remain gaps. A failed update keeps the last data visible and shows retry status. Interrupted streams reconnect; background tabs pause and resume. Initial connection failures offer Refresh. Catalog links refresh on navigation. Reverse proxies must allow streaming responses without buffering and an idle timeout above the five-second keepalive interval.

## Development checks

```sh
GOWORK=off go test ./...
(cd client && GOWORK=off go test ./...)
(cd ui && GOWORK=off go test ./...)
```

Migration 000008 adds telemetry tables only. Disable collection to stop new writes. For a schema rollback, stop the new binaries first; the down migration deletes telemetry history and host registrations, leaving the service catalog intact.
