# Balemoh Kubernetes Discoverer and DevSpace Local Development

**Goal:** Connect the catalog to a concrete, read-only Kubernetes discoverer and provide a reproducible DevSpace workflow over KinD or vCluster in Docker (vind).

**Architecture:** `internal/adapters/kubernetes` owns typed `client-go` reads for Pods, Services, and Ingresses plus a dynamic Gateway API `HTTPRoute` read. The composition root enables it only when explicitly configured and uses `rest.InClusterConfig`; the catalog remains platform-agnostic. `devspace.yaml` deploys the app with a namespace-scoped ServiceAccount and syncs the repository into a Go development container.

## Decisions

- `BALEMOH_KUBERNETES_ENABLED` defaults to false. Existing local `go run` remains database/API-only.
- Enabled discovery requires `BALEMOH_KUBERNETES_SOURCE_ID` and `BALEMOH_KUBERNETES_NAMESPACE`; the source ID must be stable for the target cluster.
- The process does not read a host kubeconfig or mount a Docker socket. In-cluster credentials are supplied by Kubernetes ServiceAccount projection.
- HTTPRoute is queried through `gateway.networking.k8s.io/v1` with the dynamic client. A missing CRD is an optional empty source; permission and malformed-resource errors fail sync.
- HTTPRoute URL observations are scheme-relative (`//host/path`) because the route object does not identify the parent listener's HTTP/TLS scheme. Ingress TLS state produces absolute `http`/`https` URLs.
- Pod images include init, regular, and ephemeral containers, deduplicated in declaration order.
- DevSpace uses the component chart and `golang:1.26-alpine` for repository sync and `go run`; it does not build or push an application image during local development.
- `devspace run-pipeline kind` provisions/connects KinD; `devspace run-pipeline vind` provisions/connects vCluster in Docker. Teardown is explicit through `hack/dev-k8s.sh`.

## Local workflow

```sh
devspace run-pipeline kind
# or
devspace run-pipeline vind
```

The pipeline creates the `balemoh-dev` namespace, applies `devspace/rbac.yaml`, deploys the component, forwards `8080`, and syncs the repository to `/workspace`. Gateway API CRDs are not installed automatically; the adapter remains useful when they are absent.

## Verification

- `go generate ./...` keeps API, sqlc, and environment documentation stable.
- Fake typed/dynamic clients cover all four resource kinds, Pod image extraction, namespace filtering, optional HTTPRoute CRD, and permission errors.
- Config tests cover disabled defaults and enabled cross-field requirements.
- `bash -n hack/dev-k8s.sh` and a YAML parser validate the local-dev artifacts without creating a cluster.
- `CGO_ENABLED=0 go test ./...`, `go vet ./...`, `go test -race ./...`, and a CGO-disabled build remain required.

No cluster is created, deleted, pushed, merged, or deployed as part of repository verification.
