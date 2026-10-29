# Balemoh UI

`ui` is a separate Go module for Balemoh's private, server-rendered frontend
and BFF. It uses `github.com/araihu/goshtoso` components and the
`consoleshell` app shell, while importing the API only through the generated
client in the sibling [`client`](../client) module.

The UI deliberately does not import the root module's `internal/` packages.
This keeps the browser-facing composition replaceable when a Balemoh instance
later becomes a gateway for other clusters and Docker hosts.

The Hosts sidebar item opens `/hosts`, listing each discovered Kubernetes cluster
and Docker host once by source kind and ID. It uses the full catalog, including
pinned, hidden, and missing entries. Sources without catalog observations do not
appear. The page is read-only and uses the existing console shell and table.
Rows show Kubernetes or Docker avatars. Unrecognized source kinds appear as
"Unknown host" with a Heroicons server avatar.
Expand a host row to see its type, catalog service count, and latest observation
time. Counts include pinned, hidden, and missing services; timestamps do not
indicate current host health.

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

The UI pins Goshtoso v0.4.5 with HTMX 4.0.0 and Alpine.js 3.17.2. Runtime
assets are served locally through Goshtoso and the compatible App Shells module.
Navigation and service editing use native links and POST/Redirect/GET. Pin
checkboxes, icon search, details, and uploads use HTMX fragments identified by
`HX-Request-Type: partial` and `HX-Target: div#<id>`. Error fragments retain
their HTTP status; unconfirmed pin state returns `HX-Reswap: none` to preserve
the row and show recovery.

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

## Notification producer icons

A plugin may declare a producer logo with manifest `icon`, referencing a bundled
`image/png` asset of kind `image`. The logo takes priority in the bell and center,
including saved notifications and messages without a destination. The BFF matches
only authenticated source plugin/connection metadata. URLs are minted through the
existing content-addressed local asset route; no remote image URLs or plugin calls
are involved. PNG dimensions are bounded to 2048px and MIME/header checked at load.

Without a logo, the destination navigation's bundled Heroicon remains the fallback.
Detail pages inherit the first valid navigation icon in manifest order. Unknown
symbols keep the source-kind fallback. `document` assets use `text/plain` for
bundling licenses and attribution alongside the icon in immutable plugin packages.
The new optional manifest field requires a host with this SDK version; deploy the
host and plugin pins together while the unpublished manifest remains alpha.

## Notification center presentation

Browser notifications use Web Push, enabled through the bell tray or notification
center. Click **Enable browser notifications**, then allow the browser permission
prompt. Delivery continues after closing the tab. HTTPS or localhost, service
workers and browser push support are required. iPhone and iPad users must add
Balemoh to their Home Screen first; the UI serves an installable app manifest.

Configure the API server with a stable private VAPID key file and a contact
subject, as described in the root README. The UI obtains only the public key
through the notification operator credential. Missing backend configuration is
shown explicitly; it does not silently fall back to open-tab-only delivery.

Opt-in stores a private browser management token locally and saves the
push subscription and its encryption keys in the API database. Existing inbox messages are not replayed.
Opening Balemoh renews the subscription's 90-day lease. Disabling unsubscribes the
browser and removes its server subscription and pending jobs. If server cleanup
fails, the UI offers a retry after local delivery has stopped. Browser storage is
required to retain subscription management across visits.

New notifications queue delivery atomically with publication. Delivery retries
transient failures and removes endpoints rejected with 404 or 410. Push-service
acceptance does not guarantee immediate OS display; offline devices, browser/OS
settings and expired subscriptions can prevent delivery. Message previews may
appear on the lock screen. Clicking opens the inbox without marking read. The
worker does not intercept or cache requests.

The inbox and bell share notification rows, producer icons, unread indicators,
source/time metadata and read controls. The center keeps full message text in a
58rem reading column. Its Goshtoso panel, toggle and icon actions use the existing
semantic tokens. App CSS only owns reading width, row layout and control spacing.

| State/action | Result | Preserved context | Writes |
|---|---|---|---|
| Filter or refresh | Replace the center only | URL, filter, keyboard focus; shell and bell remain | None |
| Filter/refresh failure | Show retry feedback and unavailable state | Current route and focused control | None |
| Mark read | Update the row or filtered list | Filter and cursor; focus goes to the response target | Existing idempotent read endpoint |
| Mark all read | Update inbox/count and announce completion | Filter and cursor; focus stays in inbox | Existing atomic read-all endpoint |
| Clear action | Open separate confirmation; Escape restores trigger focus | Inbox unchanged until confirmed | None before confirmation |

Clear confirmation and native POST fallbacks retain the existing contracts.
