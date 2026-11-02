# Homepage layout UI and acceptance ledger

This is a testable design contract. Rows are not claims that runtime tests passed.
Use the actual Goshtoso consumer, not the plain-HTML research page, for acceptance.

## Surface brief

Primary task: arrange selected content into useful named groups, then save one
instance-wide configuration. Keep source selection on its existing pages.
Identity: existing ConsoleShell, branding and semantic theme tokens.
Density: compact operational editor with populated preview and a single Save boundary.
Desktop: dominant preview beside one contextual inspector for the selected group or
item. Page settings and advanced options start folded. Save/status/recovery remain
visible during scrolling. Mobile: preview before inspector, direct settings/back
navigation and named movement controls. Opaque IDs are secondary details.

Use existing Goshtoso controls listed in the capability assessment. Generic movement
and layout come from the upstream prerequisite; raw prototype controls are not copied.

## State/action ledger

| State | Action/request | Allowed and expected response | Preserved context | Focus/destination | Durable effects |
|---|---|---|---|---|---|
| Clean saved layout | Open editor / GET | 200; saved revision and canonical preview | Theme, selection IDs, current group | Editor heading, then first useful control | 0 |
| Virtual revision 0 | Open / GET | 200; default source groups, no DB write | Existing pins/selections | Editor heading | 0 |
| Create form open | Cancel / Escape | Dismiss form; no group added | Existing draft and selected group | Create trigger | 0 |
| Create form, valid title | Submit once / repeat while pending | One provisional UUID/group; current preview requested; duplicate attempt disabled | Title/UUID through retry, existing groups | New group heading after matching valid preview | 0 until Save |
| Create form, invalid title | Submit | Local/422 field error, no accepted preview | Valid draft and attempted title | Summary linked to title input | 0 |
| Create attempt, invalid/duplicate UUID | Preview/forged PUT | 422; explicit Generate another ID recovery in UI | Title and other draft fields | Recovery action/error summary | 0 |
| 32 groups | Create / forged over-limit write | UI disabled with explanation; API 409 limit_exceeded | Existing group state | Capacity notice | 0 |
| Provisional empty group | Save then reload/restart | One persisted group; fields/defaults agree with creation contract | UUID, title and selected group | Group heading | 1 changed revision |
| Preview pending | New edit / delayed old success/error | New generation; old responses ignored; Save remains disabled | Latest input, last valid preview labelled updating | Initiating control | 0 |
| Current preview transport failure | Retry preview | New requestId, same generation; last preview labelled stale; no silent write | Exact dirty draft and base revision | Retry control; error summary if repeated failure | 0 |
| Old retained preview while pending | Try selecting/moving/reordering old entities | Controls disabled; delayed events rejected against current generation/IDs | Exact newest draft; readable stale preview | Existing current control or removal fallback | 0 |
| Duplicate group/item names | Select/move/reorder/delete | Disambiguated labels and one stable selected identity throughout | Preview, inspector and destination ID agreement | Same logical handle or documented removal fallback | 0 until Save |
| Ready current preview | Activate explicit Edit group/Edit item button by Enter/Space/pointer | Same ID transition as inspector Select; aria-pressed and root aria-current agree | Inactive widget bodies, natural geometry and existing content | Desktop: initiating logical button; mobile or inspector selection: contextual heading | 0 |
| Ready current preview | Click a card body or its service/plugin link | No navigation, widget action or selection transition | Editor route, exact draft and selection | Existing editor focus | 0 |
| Ready current preview | Press and drag any card body into another group | Mouse/pen pickup without prior selection; same validated local move event as handle | Widget identity and natural geometry | Moved item's handle; no mobile settings jump | 0 until Save |
| Clean or dirty draft | Change layout, name, heading or spacing | Local draft + bounded preview; no persist | Other group settings and selected item | Initiating field | 0 |
| Alphabetical parent | Drag or earlier/later | Valid move sets Custom in that parent only | Identity, source, renderer | Logical handle/action, announced position | 0 until Save |
| Custom parent | Reset alphabetical | Local draft shows canonical name order | Membership, renderers, other parents | Reset/mode control | 0 until Save |
| Any group | Move selected item to named destination | Atomic draft membership update | Source identity and presentation | Item movement control, destination announced | 0 until Save |
| Empty/collapsed destination | Move item there | Same valid movement without opening content first | Other collapse states | Item control or destination heading if needed | 0 until Save |
| Drag in progress | Escape / target removed | Cancel provisional movement | Complete pre-pickup order/membership | Original handle, or named group fallback | 0 |
| Configured group | Rename | ID stable; alphabetical position can change | Item references/default mappings | Name control | 0 until Save |
| Nonempty/default-mapped group | Delete | Dialog requires destination; one draft replacement rehomes members/defaults | Widget identities and source state | Destination group heading | 0 until Save |
| Delete dialog | Cancel / Escape | No draft mutation | Exact draft and selection | Delete trigger | 0 |
| Last group | Forged deletion or UI action | UI disabled; API rejects 409 limit_exceeded | Saved and draft config | Inline error summary | 0 |
| Valid current draft | Save layout | One conditional API write; 200 canonical document + ETag | Theme, selected logical group/item where retained | Saved status then continuing editor action | 1 changed revision |
| Current identical document | Repeat desired Save at current revision | 200, same revision | All configuration | Saved status | 0 new revisions |
| Stale revision | Save / forged API PUT | 412; retain draft and original precondition | Every editable value and valid selection | Conflict summary | 0 |
| Conflict shown | View saved version | Read-only comparison, no rebase | Dirty draft | Comparison heading; return restores focus | 0 |
| Conflict shown | Discard draft and reload | Explicit confirmed replacement with fresh saved state | Theme and route | Editor heading | 0 |
| Valid draft, changed complete parent member-ID set | Targeted order PUT | 409 membership_changed; reload preview, no automatic resubmit | Ordering intent and dirty draft | Conflict summary | 0 |
| Invalid geometry / duplicate or foreign ranks | Save or preview | Visible 422 field errors; no write | Other valid values | Error summary linked to actual fields | 0 |
| Unknown document field | Forged preview/PUT | 400 invalid_request; no write | Exact draft and saved state | Error summary | 0 |
| Missing If-Match, or required per-group membership token | Forged targeted write | 428 | Saved config | API error, BFF inline if applicable | 0 |
| Transport fails before/after Save response | Retry/read-back | Show outcome unknown; GET confirms matching document or offers conflict | Draft, expected revision and selected item | Recovery summary / read-back action | 0 or 1, never assumed |
| Source observation unavailable | Refresh/preview | Show retained values or named placeholder; successful sources remain | Saved membership/order, selected item | No forced focus change | 0 |
| Item intentionally unselected | Refresh | Dormant saved reference; no homepage item | Saved group/order for reselection | If focused item disappears, group heading fallback | 0 layout writes |
| Remote API client saves layout | SSE to clean homepage | One resolved structural update | Runtime collapse/focus where identities remain | Existing logical control | 0 local writes |
| Remote API client saves layout | Update while editor dirty | Announce conflict; do not replace draft | Exact dirty draft | Nonintrusive status, conflict action available | 0 local writes |
| Dirty editor | Navigate / Cancel | Goshtoso confirmation; Cancel keeps draft | Every valid edit | Original action if dismissed | 0 |
| Read-only demo | Any forged new write route | API 403 and BFF denied editing routes | Synthetic data and saved configuration | Read-only explanation | 0 |
| Unsupported/corrupt saved schema | Open | Explicit 503 with temporary default-view recovery only | Stored bytes unchanged | Recovery heading | 0 |

## Required runtime proof

- Exercise mixed host, service and non-card plugin components through real rendered
  Goshtoso containers. Do not create a service-row renderer merely to test grouping.
- Reproduce each ledger row independently, including forged requests, two stale
  clients, ambiguous response after commit, restart and provider observation outage.
- Verify server truth and effect count. A disabled UI action is not server validation.
- At 390 and 1440 px, check page/preview overflow, hierarchy, focus visibility and
  last-control clearance. In Goshtoso/Minimal, light/dark, check text/control contrast.
- Verify selected IDs, visible outlines, aria-current, inspector controls, destination
  labels and announcements agree after duplicate-name changes and reconciliation.
- Keyboard-only path must create a group, move an item, change order, reset order,
  cancel deletion, save, inspect a conflicted draft, cancel a discard confirmation, explicitly reload
  the saved version and leave the editor.
- Pointer movement and explicit touch controls must agree with API reads and DOM order.
- Inspect actual focus after removal, reordering, failed Save and dialog Cancel.
- Verify one SSE connection, no per-card streams, no duplicate handlers/observers,
  and no scroll jump on content-only updates.
- Record console/accessibility findings separately from visual approval. Real screen
  reader and touch-device checks remain runtime evidence, not spec acceptance claims.

## API schema and validator proof

Round-trip both JSON fixtures through the generated SDK. Exercise nonempty
GroupPlacement objects, ID-sorted configuration serialization, custom rank order,
per-group membership token keys, alphabetical `ids: []`, complete custom sequences,
duplicate/foreign/incomplete ranks and malformed/missing/stale headers. A renamed
source or changed selection must leave configuration GET bytes and strong ETag
unchanged while preview/dashboard display order changes. A forged removal of an
in-use group without rehoming every prior dormant/active reference must write zero
changes; an intentional reset while the group survives and subsequent deletion
must both work. Renderer-override input is an unknown field in this first slice.

Selection/lifecycle proof additionally arranges a hidden pinned service, missing
pinned service, selected host with no telemetry and enabled plugin with unavailable
runtime. Only the latter two remain active. A provider-read outage keeps locally
eligible identities active; cold-start groups always join existing flat item IDs.
Check preview complete members versus dashboard active-only arrays with a dormant
ranked reference and its later reselection. Clean, dirty-moved, draft-only-group,
stale and externally changed-selection preview tokens must have their documented
targeted-order outcomes. Invalid relational sets write zero changes: missing,
duplicate or foreign homepage placements, nonexistent default targets and duplicate
cross-group item references. Nested fieldErrors pointers retain request indices,
map to actual stable controls, and use the summary for aggregate/unmapped errors.
