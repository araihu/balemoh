# Balemoh Kubernetes Discoverer and DevSpace Local Development

**Goal:** Connect the catalog to a concrete, read-only Kubernetes discoverer and provide a reproducible DevSpace workflow over KinD or vCluster in Docker (vind).

**Architecture:** `internal/adapters/kubernetes` owns typed `client-go` reads for Pods, Services, and Ingresses plus a dynamic Gateway API `HTTPRoute` read. The composition root enables it only when explicitly configured and uses `rest.InClusterConfig`; the catalog remains platform-agnostic. `devspace.yaml` deploys the API and UI with a namespace-scoped ServiceAccount for the API, syncs the repository into Go development containers, and provides a homelab-only HTTPRoute for the existing internal Gateway.

## Decisions

- `BALEMOH_KUBERNETES_ENABLED` defaults to false. Existing local `go run` remains database/API-only.
- Enabled discovery requires `BALEMOH_KUBERNETES_SOURCE_ID`; `BALEMOH_KUBERNETES_NAMESPACE` is optional and the source ID must be stable for the target cluster. DevSpace still supplies a namespace for its least-privilege local lane.
- The process does not read a host kubeconfig or mount a Docker socket. In-cluster credentials are supplied by Kubernetes ServiceAccount projection.
- HTTPRoute is queried through `gateway.networking.k8s.io/v1` with the dynamic client. A missing CRD is an optional empty source; permission and malformed-resource errors fail sync.
- HTTPRoute URL observations are scheme-relative (`//host/path`) because the route object does not identify the parent listener's HTTP/TLS scheme. Ingress TLS state produces absolute `http`/`https` URLs.
- Discovery follows `HTTPRoute/Ingress -> Service -> Pod`; when no route/Ingress resolves a Service it falls back to `Service -> Pod`. External or selector-less Services remain candidates, while unmatched standalone Pods are omitted.
- Pod images include init, regular, and ephemeral containers, deduplicated in declaration order.
- DevSpace uses the component chart and `golang:1.26-alpine` for repository sync and `go run`; it does not build or push an application image during local development.
- `devspace run-pipeline kind` provisions/connects KinD; `devspace run-pipeline vind` provisions/connects vCluster in Docker. Teardown is explicit through `hack/dev-k8s.sh`.
- `devspace run-pipeline homelab` reuses the existing homelab Gateway API and Pi-hole ExternalDNS setup, exposing the UI at `https://balemoh.decastro.me`; `homelab-deploy` performs the same resource deployment without starting DevSpace sync/port-forwarding. Both are separate from the `kind`/`vind` pipelines because those clusters do not provision the homelab Gateway.

## Local workflow

```sh
devspace run-pipeline kind
# or
devspace run-pipeline vind
```

The pipeline creates the `balemoh-dev` namespace, applies `devspace/rbac.yaml`, deploys the API and UI components, forwards `8080` and `8081`, and syncs the repository to `/workspace`. The `homelab` pipeline also applies `devspace/httproute.yaml` after both Services exist. Gateway API CRDs are not installed automatically in local clusters; the adapter remains useful when they are absent.

## Verification

- `go generate ./...` keeps API, sqlc, and environment documentation stable.
- Fake typed/dynamic clients cover route/Ingress-to-Service-to-Pod resolution, Service fallback, external Services, orphan-Pod filtering, Pod image extraction, namespace filtering, optional HTTPRoute CRD, and permission errors.
- Config tests cover disabled defaults and enabled cross-field requirements.
- `bash -n hack/dev-k8s.sh` and a YAML parser validate the local-dev artifacts without creating a cluster.
- `CGO_ENABLED=0 go test ./...`, `go vet ./...`, `go test -race ./...`, and a CGO-disabled build remain required.

No cluster is created, deleted, pushed, merged, or deployed as part of repository verification.
