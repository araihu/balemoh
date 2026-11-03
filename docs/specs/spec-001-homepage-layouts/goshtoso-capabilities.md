# Goshtoso capability assessment

Original research checked 2026-11-01 before implementation. Decision applies to the selected public dependency APIs, not
the standalone HTML study. The assessment below records that original baseline.

## Inspected versions and evidence

Balemoh source `6455744557a447f2381f3eecd0a601f60e945a5e` selects the
following **UI-module** dependencies, which supply the inspected editor controls:

| Module | Selected version |
|---|---|
| Goshtoso | `v0.4.8-0.20261101060924-c5f68d0bb167` |
| App Shells | `v0.1.9` |
| Charts | `v0.0.5` |

Root go.mod selects the same Goshtoso pseudo-version. The generated client module
instead selects `v0.4.6-0.20261027195056-cea8387a3c88` for its existing typed theme
contracts. App Shells and Charts belong to UI, not root/client. No layout component
is imported by the client: the new layout wire DTOs remain plain data. Do not upgrade
the client's unrelated dependency merely to match UI; inspect its actual imports
when implementation chooses the new upstream version. Root/UI pins must remain
compatible with their existing shared theme APIs.

Discovery used `GOWORK=off go list -m all`, version listings for all three modules
and Margo, and `go doc` against the selected packages. Margo is absent from the UI
module and is unnecessary for the runtime editor. Its absence is not a capability gap.
Goshtoso latest tagged release was v0.4.7. Current upstream main was
`6fed2b98f6b13e65e0f419a2d028b94d91afc1b9`.

Inspected exact selected public component files, public component inventory,
`docs/COMPONENT_MODEL.md`, runtime input manifest, and relevant usage in Balemoh's
`ui/internal/view/pages.templ`, `widgets.templ`, `presentation.templ`, `themes.templ`
and `shell.go`. Closest semantic matches were inspected rather than relying on a
missing package-name search: Table sorting changes column value order; FileInput
drag/drop accepts files; neither moves arbitrary existing renderable children.

## Reuse and composition decisions

| Need | Inspected public symbols / usage | Decision |
|---|---|---|
| Existing application frame | `consoleshell.Config`, current `ConsolePage` | Reuse; preserve navigation, branding, theme and runtime |
| Neutral group/editor surface | `panel.Panel`, `panel.Config` Header/Actions/Body/RootAttrs and density | Compose domain headings and content in supported slots |
| Fixed layout/order choices | `select.Select`, `radio.RadioBar`, `checkbox.Checkbox`, current theme forms | Reuse; choose Select for long choices and RadioBar for a small mode set |
| Group name and placement fields | `textinput.TextInput`, `combobox.Combobox`, `form.FieldGroupConfig`, `FocusTargetID` | Reuse with actual validation targets |
| Actions and overflow | `button.Button`, `actiongroup.ActionGroup`, `dropdown.Dropdown`, `tooltip.Tooltip` | Compose Save/Cancel, explicit movement and narrow-screen alternatives |
| Dirty-draft/deletion confirmation | `modal.Dialog`, current theme editor patterns | Reuse; preserve draft/focus on Cancel |
| Collapse state | `accordion.Accordion`, `form.CollapsibleSection`, existing disclosure patterns | Compose where suitable; app owns heading and draft meaning |
| Status/failure/retry | `alert.Alert`, `toast.Toast`, `emptystate.EmptyState`, `spinner.Spinner`, `badge.Badge` | Reuse; failure stays inline, not toast-only |
| Optional future tabs | `tabs.Tabs`, `Tab.LabelSlot`, `SyncHash`, inline Content and HTMX lazy content | Reuse when tabs are in scope; not a first-slice dependency |
| Bounded secondary list | `scrollregion.ScrollRegion` Content/ViewportClass | Reuse only if the editor actually needs bounded scrolling |
| Charts inside items | Charts `line.Line` and selected renderer contracts | Reuse existing renderers; no new SVG/chart runtime in Balemoh |
| Generic component identity | `components.Component`, `Kind()`, `AllKinds()` | Reuse type identity; keep application instance identity separate |
| Table sorting | `table.Column.Sortable`, `SortBy`, `SortDir`, sort URL helpers | Not a substitute for arbitrary child placement or per-parent ranking |
| File drop | `fileinput.FileInput`, drag state and file-selection contract | Not a substitute for cross-container item movement |
| Natural-height container geometry | Public inventory has no generic Stack/Grid/Masonry container API | Gap: generic direct-child geometry and measurement lifecycle |
| Arbitrary item reorder/move | No selected public component provides pointer/keyboard movement across compatible containers | Gap: generic accessible placement interaction, independent of persistence |

The gap is the missing behavior, not a missing visual Card. Public `templ.Component`
slots already permit arbitrary content. RootClass/RootAttrs permit composition but
do not supply movement semantics, keyboard announcements or observer cleanup.

## Upstream-first dependency

Proposed upstream owner is `araihu/goshtoso`, with the public generic
`components/layout` contract in
`docs/specs/spec-001-layout-containers/spec-001-layout-containers.md`.
It supplies container geometry, item wrappers, optional movement handles, cancellable
movement events and HTMX/Alpine lifecycle handling. It does not know Balemoh groups,
pins, source kinds, order mode, API routes, SQLite or shared settings.

The current Balemoh masonry runtime is useful behavior evidence, not a library
implementation to copy blindly. The upstream work must include nested containers,
content-size changes, direct-child selection, no-measurement fallback, geometry
ownership and disposal. Its accepted public implementation/release must precede the
Balemoh UI integration. Pin a verified immutable version and replace the old
app-specific measurement runtime once integration is proven. Do not duplicate it.

Specs and independent design reviews come first in this task. The upstream component,
demos, generated assets/catalog and browser tests remain implementation work, not
capabilities assumed available from main or an unreleased pseudo-version.

## Initial verified upstream integration, 2026-11-01

PR araihu/goshtoso#363 merged as c4bbff5b89274d31432d6e0a154093ce2e7f8eaa.
Release v0.5.0 passed full verification and publication before consumer changes.
Public CSS matches the tag, the external no-replace module consumer passes, and
main documentation deployment passed. Root and UI initially selected v0.5.0; the client
retained its existing theme dependency. Public layout.Container owns page and group
geometry, DOM order and optional movement. Balemoh retains drafts and persistence.

Controls compose form.FieldGroup, Select, TextInput, Checkbox, Button and
modal.Dialog. Inspector fragments replace their form controls when choices change;
preview fragments morph stable library item roots. This avoids preserving stale
Select choices while keeping widget content and movement identities stable.
App-owned coordination uses public Layout events and native disabled fieldsets.

### Direct card dragging follow-up

User requested inactive card clicks and direct body pickup in the editor. Goshtoso
PR #364 introduced immutable `layout.Instance.WithContentDrag(true)` without changing
public config structs or the default handle-only behavior. The UI composes that
method with native inert widget-body wrappers. Settings pencils and movement
handles remain active outside those wrappers; normal homepage content stays active.

Outer groups and nested item containers opt in independently. The nearest item
retains its own movement scope; nested background can pick up the enclosing group.

### Current release integration, 2026-11-03

PR #364 merged as `57b800042a41a167935774c9961106af2556050f`. Goshtoso v0.6.0
passed full release verification and publication before the final consumer pins.
Root and UI now select that release through the public Go proxy, replacing the
earlier temporary UI pseudo-version. No Goshtoso local replacement is committed;
the client's unrelated theme dependency remains unchanged.

The [verification record](../../verification/homepage-layout-redesign.md#published-goshtoso-v060-2026-11-03)
records the exact-tag consumer fixture, asset checksums, coverage, badges, module
graph, consumer tests and runtime checks. The upstream release prerequisite is met.
