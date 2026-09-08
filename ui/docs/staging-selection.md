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

Each Goshtoso checkbox submits its own native POST form on change. Checked sends
`pinned=true`; unchecked sends no pin field. The handler validates the body and
service against the current catalog, skips an already-saved state, then pins or
unpins. Success redirects to the same checkbox in staging. Native navigation
keeps the browser's transport errors visible; no optimistic saved state is kept.
Without JavaScript, a noscript Save button submits the same form.

| Request | Result | Pin effects |
|---|---|---|
| Check or uncheck | 303 back to the checkbox | Requested state saved |
| Repeat saved state | Same redirect | None |
| Invalid, duplicate, or oversized field | 400 in shell | None |
| Unknown service | 409 in shell | None |
| Upstream failure | 503 in shell with fresh catalog state | May already have completed; retry is idempotent |

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
