# Unified notifications implementation

Status: N1–N5 implemented and verified in the homelab on 2026-10-26.
Contract: [unified notifications](../specs/2026-10-25-notifications-design.md).
Release stage: trusted single-instance pre-v0.1.0.

## N1. Core inbox and API

Add `internal/application/notifications` with validated messages, bound producer
identity, Publish/List/MarkRead ports and typed failures. Add an additive SQLite
migration and repository with unique event identity, transactional unread totals,
stable sequence pagination and idempotent read writes. Extend OpenAPI and regenerate
server/client code. Add separately scoped external publisher authentication.

Use four non-null identity columns and a unique index. Core/external producers
normalize absent connection IDs to the empty string; plugin producers require
a connection UUID. Test both identity shapes. Resolve authenticated existing
events before new-message capacity limits; enforce new-event uniqueness
and capacity in one transaction. Prove retry at capacity and conflict at capacity.

Prove durable retry, conflicting replay, capacity, pagination during arrival,
read state after reopen, auth/source isolation and bounded request bodies.
The notification service starts independently of installed plugins. Reject a
publisher credential on GET/PUT and an operator/BFF credential on external
POST. Test escaped hostile title/body/source text, invalid destinations,
external-link attributes and the absence of server-side destination fetching.
Exercise concurrency/capacity 429 reasons and retry metadata without promising
a time-window rate limit.

## N2. Shared bell and inbox UI

Add the bell to `consoleshell.Config.HeaderActions`, beside the color-mode control.
Use Goshtoso popover, list/table primitives, badges and native destination links.
Add independently loaded BFF summary/tray fragments and `/notifications` with
cursor navigation and per-item read forms. No plugin registry call is needed
to render the bell, count, tray or historical messages.

Verify keyboard/escape/focus, 390/1440 px, supported appearance overrides, loading,
empty, unavailable, unread arrival and failed read writes. Refresh preserves
the open menu, scroll, other header controls and existing telemetry SSE.

Reject forged cross-origin read forms and prove read state is unchanged. Render
hostile producer text as escaped content. With deterministic timers and visibility
signals, prove one in-flight refresh, hidden-to-visible resumption, 15/30/60-second
failure/recovery scheduling and no hidden-document polling. Use an instrumented
hung/stopped plugin runtime to assert zero child invocations for the shell and
notification fragments, with local fixture responses completing within one
second. Test visual `99+` against the full accessible count and ensure unavailable
never becomes a rendered zero.

Block the notification API/count client before requesting an ordinary core page.
With other page dependencies healthy, assert the initial HTML returns within
the one-second local fixture budget, includes only the loading target, and
makes zero notification client calls. Then request the targeted summary/tray
fragment: only that request may call the blocked client. Verify its bounded
timeout renders unavailable in that target while the browser's original shell,
theme control and navigation remain mounted and interactive. Repeat with the
notification API and storage unavailable. These checks are separate from the
hung-plugin proof above; the shell must also be independent of notifications.

## N3. SDK publisher and host callback

Add public notification types and an optional connection-scoped backend Publisher.
Extend the manifest/deployment capability contract and additive broker RPCs.
Require backend targets for publication capability, including publishers with
no queries. Bind source identity and grants in the API adapter; close callbacks on replacement,
disable, child exit and shutdown. Update scaffold and an independent fixture plugin.

Prove old plugins still run, UI calls cannot publish, ungranted/revoked sessions
fail, receipt follows commit, callback bounds apply, and retry preserves identity.
Do not expose the private API credential or raw broker details to plugin authors.

## N4. Alertmanager producer

Keep a silent first-observation baseline and firing occurrence identities in
memory for each plugin backend session. Attempt delivery of new active critical/warning transitions through the
SDK callback. Preserve source detail links and source-independent notification
text. Plugin polling/rendering must not flood the inbox or block collection.

Use isolated fixtures for duplicate polls, new occurrences, suppression,
restart, source outage and ambiguous publication timeout. Do not turn existing
homelab alerts into a synthetic notification backlog during deployment.
Prove restart establishes another silent baseline. Document that transitions
during downtime and unpublished events at crash can be missed. Test the bounded
publication queue under pending responses and show collection remains independent.
Cover full-queue repeated polls, partial observations, removal/reappearance, and
crash immediately before/after queue admission. Assert one admission attempt
per continuously observed active occurrence, one warning on its drop, no later
poll admission retries while it remains active, and bounded collection latency.
Durable plugin state/outbox delivery is a separate milestone requiring a
namespaced state port, migration/backup lifecycle and atomic outbox ordering.

## N5. Independent review and rollout

Freeze Git-native targets for independent architecture and code-quality reviews.
Resolve current blockers before publishing reviewed images. Retain rollback
pins and backup evidence, update GitOps through a scoped worktree, validate
migration compatibility and inventory, then verify Argo and the real browser.
Real validation is read-only for notifications, connections, widgets and pins.
Keep Balemoh PR 42 draft unless separately authorized for merge.

### Rollout receipt, 2026-10-26

Independent architecture and quality reviews accepted host `4bfa57c`, plugin
`f039fab`, and the GitOps rollout. Plugin [PR 8](https://github.com/araihu/balemoh-plugin-alertmanager/pull/8)
merged. GitOps [PR 637](https://github.com/guilycst/home-lab/pull/637) and the
CSI mount correction [PR 642](https://github.com/guilycst/home-lab/pull/642) merged.
Balemoh [PR 42](https://github.com/araihu/balemoh/pull/42) remains draft.
Its separate integration conflicts with newer main work remain outside this rollout.
Matching-host-SHA consumer [CI](https://github.com/araihu/balemoh-plugin-alertmanager/actions/runs/40270820405)
passed, including generated contracts and real-process plugin integration.

Deployed immutable images:

- Host: `ghcr.io/araihu/balemoh@sha256:2308a695822de68b6b4da2bd59dc0028be47240ca33b03d0282bc69647d2c6f0`.
- Plugin: `ghcr.io/araihu/balemoh-plugin-alertmanager@sha256:b78afa8e81553e0346d8d72d20a859ce783f2413d95796a875cd2e5b114dde15`.

Argo reported Synced, Healthy and Succeeded at GitOps revision
`7e54c28c61c89ab211928d2f6ac1437d504f538f`. Both API and UI were ready with
zero restarts. The PostSync Job confirmed `/data` mounted `ro`, clean schema12,
integrity OK and a clean schema11 rollback baseline with SHA-256
`e115de86e97a750cfd860a6ca7522f3e969ca67d52c486772b772d2d35de6c71`.

Read-only before/after reconciliation preserved the connection identity, both
complete widget placements and their options/revisions, 31 pinned services and
36 dashboard items. The connection advanced from revision8 to revision9 and
reported ready/fresh. Operator inbox GET succeeded; anonymous and plugin service
credentials were rejected. Inbox contained zero notifications. No production
publication, read-state mutation, alert/silence change or forced source transition
was used. No new qualifying warning/critical transition occurred during live
validation; delivery transitions, retry, deduplication and persistence were tested
with isolated fixtures instead.

The real browser verified the asynchronous bell/tray, full inbox, unread filter,
Escape focus return, Home live updates and the real Alertmanager page. Light/dark
mode persisted across reload; the original dark preference was restored.

## Before a supported long-lived release

Add capacity visibility and an operator retention/export-delete policy before
v0.1.0. The initial bounded inbox rejects new publication at capacity and has
no automatic deletion. Do not describe it as an indefinitely running inbox.
