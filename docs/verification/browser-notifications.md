# Web Push notifications

Local work on `feat/browser-notifications`, based on `5682038`.

## Behavior and ownership

The bell tray and inbox opt-in control now creates a real Web Push subscription.
The API owns subscription storage, VAPID signing and a durable delivery queue.
New notification inserts queue one delivery per subscribed browser in the same
SQLite transaction. All producers use that insertion path. Publisher replay does
not add jobs. The browser worker handles push independently of any open tab.

Migration 17 adds subscription, configuration and delivery tables plus an insert
trigger. Jobs use leases, bounded retries and expiry. Read/cleared messages are
removed before claiming; already in-flight pushes cannot be recalled. HTTP
404/410 removes an expired endpoint. A stale response cannot remove a replacement
subscription. VAPID rotation invalidates old subscriptions. The client refreshes
its subscription lease when opened and uses an opaque per-browser token to
manage it. Only the token hash is stored in SQLite. No endpoint list is exposed.

The API configuration uses caarlos0/env and generated envdoc documentation.
`BALEMOH_WEB_PUSH_VAPID_KEY_FILE` and `BALEMOH_WEB_PUSH_SUBJECT` enable delivery;
the notification operator credential is also required. The key generator writes
a new 0600 file and refuses to overwrite one. No production key was generated.

The existing Goshtoso Button, Panel and ConsoleShell composition is retained.
The new app-owned code implements PushManager subscription, persistence and
worker events. A web app manifest supports Home Screen installation on iOS.
The previous page-polling delivery path was removed to avoid duplicate alerts.
Missing Web Push configuration is shown explicitly.

## Validation

Passed the final automated gates:

- `node --test ui/internal/view/static/tests/*.test.cjs`, 12 tests.
- Root and standalone UI `go test -race ./...` and `go vet ./...`.
- Standalone generated-client `go test ./...`.
- OpenAPI, SQLC, envdoc and consumer templ generation; `git diff --check`.


- Node tests cover explicit permission, subscription persistence and renewal,
  disable, denied/dismissed permission, missing configuration, unsupported
  browsers, storage errors, failed persistence, retryable cleanup, revoked
  permission, push with no clients, malformed pushes and safe click navigation.
  Native subscription JSON metadata is stripped before forwarding the typed API
  request. Existing inbox polling tests still pass.
- SQLite tests cover no inbox replay on subscription, atomic rollback, publisher
  deduplication, restart, exclusive leases, abandoned-lease recovery, backoff,
  expiry, read/clear, disable, ownership, VAPID rotation and stale 410 responses.
- HTTP/BFF tests cover operator/publisher separation, cross-origin mutation
  protection, ownership, malformed/oversized requests, no-store responses,
  private metadata exclusion, generated SDK authority and app manifest routes.
- Protocol tests send encrypted aes128gcm messages with VAPID through a controlled
  HTTP transport, check message bounds and reject private destinations,
  redirects and proxy bypasses. An integration test closes and reopens SQLite,
  cancels the publishing request, then sends the persisted job using a new worker
  without any UI process or browser tab. A second drain sends nothing.
- Local API and UI processes used an isolated temporary database and test keys.
  Publishing a synthetic message returned 201; the inbox and Web Push control
  rendered through the real BFF. No console warnings or errors in the preview.
  Narrow screenshots are saved in this chat's visualization directory under
  `browser-notifications/web-push-390.png`. The earlier theme matrix remains
  applicable to the unchanged component layout.

A live follow-up registered Chrome with FCM and confirmed the subscription in
SQLite. All Balemoh tabs in the test browser were closed before publication.
The worker received HTTP 201 from FCM for the third closed-tab test alert. This
verifies push-service acceptance, not OS display. The user did not observe the
initial alerts, and native popup display/click-through remain unverified.

Both macOS Chrome notification entries were initially disabled. With explicit
permission, each was enabled temporarily for testing and restored to disabled.
The test browser was unsubscribed, the server subscription count returned to
zero, and the localhost site permission was reset to its original ask state.
Temporary processes, keys and data were removed. The live Balemoh service was
not changed. Evidence includes `web-push-live-receipt.json`, enabled/disabled
browser screenshots and `web-push-os-settings-restored.png` in the visualization
directory. Delivery logs now record only attempt number and HTTP status, without
endpoint URLs, key material or message text. Focused race tests and vet passed
after that logging change. No production rollout occurred.

See the root README for enablement, limits and rollback. Web Push requires user
permission and a secure context. iPhone/iPad require Home Screen installation.
[MDN Push API](https://developer.mozilla.org/en-US/docs/Web/API/Push_API),
[WebKit Home Screen Web Push](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/).
