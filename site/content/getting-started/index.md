---
title: Get started
description: Run the Balemoh API and UI locally, then connect a discovery source.
language: en
---

# Get started

Start with the API and UI on loopback. Connect a discovery source after both
processes are running. Go 1.27.3 or newer is required to build this checkout.

## Run from source

```sh
git clone https://github.com/araihu/balemoh.git
cd balemoh
BALEMOH_HTTP_ADDR=127.0.0.1:8080 go run ./cmd/balemoh
```

In a second terminal, from the same checkout:

```sh
cd ui
BALEMOH_UI_HTTP_ADDR=127.0.0.1:8081 GOWORK=off go run ./cmd/balemoh-ui
```

Open [the local UI](http://127.0.0.1:8081). The homepage starts empty.
Use Staging to review discovery results, then pin services to the homepage.

Check the API:

```sh
curl --fail http://127.0.0.1:8080/healthz
curl --fail http://127.0.0.1:8080/api/v1/staging/services
```

Startup creates the SQLite database and runs embedded migrations. Keep the
database on persistent storage. See [configuration](../reference/index.md)
for the generated API and UI environment references.

## Add discovery

Discovery has no sources enabled by default. Start with
[Docker or Podman](../guides/discovery/index.md#docker-and-podman), or deploy the API
inside Kubernetes with read-only RBAC.

Manual discovery refresh:

```sh
curl --fail -X POST http://127.0.0.1:8080/api/v1/discovery/sync
```

For several machines, use [federated agents](../guides/federation/index.md).

## Container deployment

The project publishes `ghcr.io/araihu/balemoh` for amd64 and arm64. Choose a
published immutable image digest rather than following a mutable tag.
The image starts `/balemoh`; use `/balemoh-ui` as the command for the UI process.

Mount persistent storage at `/data`. The image uses `/data/balemoh.db` for
SQLite and `/data/icons` for the UI icon cache. Point the UI at your API using
the configuration reference. Only the API needs discovery credentials or a
container socket.

The container socket grants substantial control over the host. Give it only to
the discovery agent and keep the API and UI behind your chosen access boundary.

## Use Balemoh

The [features and guides](../guides/index.md) explain the app's everyday workflows:

- [Review discovered services](../guides/discovery/index.md) and choose what appears on your homepage.
- [Search services](../guides/search/index.md) by name, description, or address.
- [Enable basic host telemetry](../guides/host-telemetry/index.md) and inspect CPU, memory, history, and workloads.
