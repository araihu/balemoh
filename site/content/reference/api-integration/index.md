---
title: API integration
description: Call your Balemoh API, create and pin a service, handle retries, and interpret JSON, text, and binary responses.
language: en
---

# API integration

The [OpenAPI reference](https://balemoh.araihu.com/openapi/catalogs/balemoh/documents/openapi/)
documents all 20 API operations, their request examples, field meanings, and
responses. Use the origin of your own API instance. This documentation website
serves static files and does not run the API.

## Create a homepage entry

For a local instance from [Get started](../../getting-started/index.md), check
readiness first:

```sh
curl --fail-with-body http://127.0.0.1:8080/healthz
```

A ready instance returns `{"status":"ok"}`. This checks the API and its configured
storage, not the health of discovered applications.

Create a manual service with a random 32-character hexadecimal retry key. The
following synthetic payload uses a fixed example key; use a new random key for
each new entry and keep that same key when retrying its creation:

```sh
curl --fail-with-body -i http://127.0.0.1:8080/api/v1/staging/services \
  -H 'Content-Type: application/json' \
  -d '{"requestId":"0123456789abcdef0123456789abcdef","displayName":"Grafana","description":"Metrics and dashboards","address":"https://grafana.example.test","icon":"selfhst:grafana"}'
```

Creation returns HTTP 201, a `Location` header, and a live, unpinned service under
`manual/local`. Replaying that key returns its current record without overwriting
later edits. Copy the returned `id`, then pin it:

```sh
SERVICE_ID='paste-the-returned-service-id'
curl --fail-with-body -X POST \
  "http://127.0.0.1:8080/api/v1/staging/services/${SERVICE_ID}/pin"
curl --fail-with-body http://127.0.0.1:8080/api/v1/homepage/services
```

The pin response has `pinned: true`. The homepage list includes only live, visible,
pinned services. Use `DELETE` on the same `/pin` path to unpin without deleting
the service. Repeated pin or unpin requests preserve the desired state.

For discovered apps, list `/api/v1/staging/services` and use its group IDs instead
of creating manual entries. Staging includes live, missing, and hidden entries;
it has no status filter or pagination query. See [Automatic service discovery](../../guides/discovery/index.md).

## Authentication and agent reports

Catalog, dashboard, host, and icon endpoints have no built-in authentication.
Keep your API behind your chosen network or reverse-proxy access boundary.

Only `/api/v1/federation/snapshots` and `/api/v1/federation/telemetry` use the
source-bound Bearer scheme. The credential must be registered for the exact
source kind and ID in the request body. It is not a general API login token.
Configure credentials through [Federated agents](../../guides/federation/index.md).

A discovery snapshot is a complete set, so an empty candidate list marks previous
observations missing. Never send an empty snapshot to represent a failed scan.
Telemetry reports update host measurements separately and do not change service
pins. The reference examples use fixed observation times; replace them with
current UTC before importing. Prefer the existing agent and generated client
over assembling source identities by hand.

## Handle responses and retries

Check the status and `Content-Type` before decoding a body:

| Response | Handling |
| --- | --- |
| Structured success | Decode JSON using the operation's response schema |
| 204 or 304 | Do not try to decode a body |
| Catalog or discovery error | Usually `ErrorResponse` JSON with `code` and `message` |
| Validation, host lookup, selection, or uploaded-icon error | Can be `text/plain`; check the operation's documented response |
| Uploaded icon image | Raw image bytes with the stored image media type and an ETag |

After a transport failure during manual creation, reuse the request key. After a
409 lifecycle conflict, refresh Staging before choosing another action. Icon
updates and deletion need the current digest from the uploaded-icon list; reload
that list after a revision conflict. Reuse an icon ETag in `If-None-Match` to obtain
a bodyless 304 when its image is unchanged.

Host telemetry uses null measurements for unavailable values, including gaps in
history. A null CPU reading is not zero CPU usage. The host detail example is an
excerpt; actual one-hour and 24-hour responses contain 60 and 1440 minute slots.
See [Basic host telemetry](../../guides/host-telemetry/index.md) for collection setup
and reporting status.
