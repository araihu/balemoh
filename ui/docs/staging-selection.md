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

| State | Request | Result | Retained selection | Pin effects |
|---|---|---|---|---|
| Ready | Known unpinned IDs | 303 to `/staging?notice=selection-pinned` | Cleared | Once per distinct ID |
| Already pinned | Repeat selection | Same redirect | Cleared | None |
| Empty or excessive | Zero or over 500 IDs | 400 in shell | None | None |
| Stale | Includes an unknown ID | 409 in shell before any pin | Available selected rows | None |
| Interrupted batch | Upstream pin fails | 503 in shell | Unfinished rows stay checked | Earlier successes remain pinned |
| Retry | Resubmit interrupted batch | Normal redirect on success | Cleared | Already pinned rows skipped |

Body size is limited to 64 KiB. IDs come only from POST form fields. Pinning
reuses the existing idempotent API; batches are sequential and not atomic.
Homepage unpin controls retain their existing routes.

## Verification

`GOWORK=off go test ./...`, `go vet ./...`, generation, and icon lock verification
pass. Handler tests cover the mutation contract, safe errors, and embedded assets.
Browser checks use a disposable local API fixture, not homelab pin data.
Verified checkbox keyboard operation, tooltip description, pin success,
failed pin with selection retention, empty state, and native PRG destination.
Inspected 390 px and 1440 px layouts in Goshtoso and Minimal, light and dark.
Table owns horizontal overflow; document stays within viewport width.
Checkbox outlines use the existing muted-text token for visible boundaries.

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
