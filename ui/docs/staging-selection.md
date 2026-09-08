# Staging selection

Staging uses Goshtoso Table cells containing Goshtoso Checkboxes and a native
POST form. Goshtoso v0.2.5 and v0.2.8 `ShowCheckbox` inputs have no form name,
value, or accessible label, so that option cannot carry this selection.
The consumer uses public component slots and leaves dependency markup alone.

The sync action uses a Heroicons sprite, an alternate-tone Button, and Tooltip.
The Kubernetes mark comes from the pinned Devicon source in `.iconpack.yaml`.
Regenerate it without changing trust:

```sh
GOWORK=off go run github.com/araihu/goshtoso/cmd/iconpack@v0.2.5 \
  -config .iconpack.yaml -out ./internal/appicons -package appicons \
  -const-prefix Icon -sprite-url /ui/icons/sprite.svg
```

## Mutation contract

Each Goshtoso checkbox submits only its row's form through HTMX. Checked sends
`pinned=true`; unchecked sends no pin field. The handler validates against the
current catalog, skips an already-saved state, and returns one Goshtoso TableRow
fragment. HTMX replaces the row without navigation and restores checkbox focus.
The checkbox is disabled while its request runs; requests for that row cannot
queue duplicate submissions.

Expected errors return a fresh row with a safe message and `X-Balemoh-Status`.
HTTP 200 allows HTMX's standard swap policy. If the catalog cannot confirm state,
the existing row stays, the checkbox is disabled, and a Refresh link appears.
Native form submission remains a full-page fallback, with a noscript Save button.

| Request | Result | Pin effects |
|---|---|---|
| Check or uncheck | Updated row fragment | Requested state saved |
| Repeat saved state | Same saved row | None |
| Invalid, duplicate, or oversized field | Error row, status header 400 | None |
| Unknown service | 409, retain row and offer refresh | None |
| Upstream failure | Error row with fresh state, or refresh recovery | May already have completed; retry is idempotent |

Body size is limited to 1 KiB. Pin state comes only from POST fields. Tests cover
pin, unpin, replay, invalid input, unknown IDs, failed writes, and recovery.
Goshtoso Link renders addresses in both the summary and resource details.

## Service groups

Catalog reads group Kubernetes observations by source, namespace, and Service
name using `kubernetes.services`. Routes and pods can appear in multiple groups;
shared resources never merge Services. Missing references stay standalone.
The group keeps the Service ID and its pin state. Previously pinned resource
rows remain visible until explicitly unpinned. No stored observations change.

`resources` is an optional API response field containing each member's resource,
endpoints, and images. Federation snapshots remain resource observations.
Homepage reads use the same aggregation, including unpinned member evidence.
The table composes native `details` in a Goshtoso Cell slot. This provides native
keyboard expansion without coupling to the table's internal Alpine state.
Column Width hooks keep identity and addresses readable when details open.

Checks cover shared backends, duplicate references/images, source and namespace
isolation, unresolved references, stable pins after pod replacement, and legacy
pins. Discovery's existing snapshot retention policy remains unchanged.
