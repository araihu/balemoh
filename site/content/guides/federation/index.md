---
title: Federated agents
description: Collect discovery snapshots from remote clusters and container hosts in one Balemoh gateway.
language: en
---

# Federated agents

A Balemoh instance can be an agent, a gateway, or both. Agents discover local
resources and publish snapshots to the gateway. The gateway does not need a
remote Docker socket or remote Kubernetes credentials.

## Register sources at the gateway

Register each allowed source by its kind and stable ID. Assign an independent
Bearer credential to each source. Load real credentials through your deployment's
secret mechanism, never a committed environment file.

Configure `BALEMOH_FEDERATION_ALLOWED_SOURCES` and
`BALEMOH_FEDERATION_SOURCE_TOKENS` using the formats in the
[API configuration reference](../../reference/api-environment/index.md).

## Connect each agent

Enable the local discovery adapter and give it a stable source ID. Set
`BALEMOH_FEDERATION_GATEWAY_URL` to the gateway HTTPS URL and
`BALEMOH_FEDERATION_TOKEN` to that source's credential.

Source identity must match the gateway registration. Local and remote sources
cannot claim the same identity. Production federation uses HTTPS. Insecure HTTP
requires an explicit opt-in and is limited to loopback development.

## What synchronization preserves

Snapshots retain resources, images, endpoints, and provenance. Pins stay local
to each gateway. Import reconciles the source, removes absent remote candidates
that are not pinned, and preserves local pins.

Automatic discovery synchronization runs at startup and on the configured
interval; manual sync remains available. Host telemetry is optional and runs
independently from discovery.

See the [OpenAPI reference](https://balemoh.araihu.com/openapi/catalogs/balemoh/documents/openapi/)
for federation snapshot and telemetry endpoints.
