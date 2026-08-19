# Balemoh UI

`ui` is a separate Go module for Balemoh's private, server-rendered frontend
and BFF. It uses `github.com/araihu/goshtoso` components and the
`consoleshell` app shell, while importing the API only through the generated
client in the sibling [`client`](../client) module.

The UI deliberately does not import the root module's `internal/` packages.
This keeps the browser-facing composition replaceable when a Balemoh instance
later becomes a gateway for other clusters and Docker hosts.

## Local run

Start the API, then run the UI from this directory:

```sh
GOWORK=off go generate ./...
BALEMOH_UI_API_BASE_URL=http://127.0.0.1:8080 GOWORK=off go run ./cmd/balemoh-ui
```

Configuration is documented in
[`internal/config/environment.md`](internal/config/environment.md):

- `BALEMOH_UI_HTTP_ADDR` defaults to `:8081`.
- `BALEMOH_UI_API_BASE_URL` defaults to `http://127.0.0.1:8080`.
- request and shutdown timeouts default to `5s`.

## Pin selection contract

| State | Request | Expected result | Effect |
|---|---|---|---|
| Candidates selected | `POST /staging/pin` with one or more `service_id` values | `303 /staging?notice=pinned` | Each unique candidate is pinned once |
| No candidates selected | `POST /staging/pin` without IDs | `303 /staging?notice=none-selected` | No mutation |
| Repeated candidate ID | Same POST with duplicate IDs | Same success redirect | Duplicate values are deduplicated |
| Upstream pin failure | Same POST with a failing candidate | In-shell mutation error | The failure is not silently swallowed |

The BFF exposes document routes for `/` and `/staging`, and native HTML form
actions for staging sync, bulk pin, and unpin. The staging table uses
Goshtoso's checkbox selection and Alpine to serialize selected candidate IDs
before the native pin POST. Mutations use POST/Redirect/GET; HTMX is
intentionally disabled for this first slice.

## Design brief

The primary task is an operator scanning discovered candidates and deciding
which services belong on the homepage. The layout is a compact operations
list inside a responsive Goshtoso console shell, with this information order:

1. pinned homepage services;
2. staging candidates;
3. source/resource provenance, endpoints, and observed images.

The visual direction is restrained and operational: semantic Goshtoso states,
neutral panels and dividers, no metric-card gallery, and reduced motion. Empty,
success, and upstream-error states are rendered explicitly.
