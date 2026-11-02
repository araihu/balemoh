# Homepage layout editor redesign

The first editor exposed too much configuration at once. Independent assessments
found preview subordinate to forms, duplicated selection paths, inline movement
forms changing widget height, mixed page/group/item settings, and Save/status out
of reach during scrolling. Baseline static detection found zero issues; this did
not establish usability.

The user chose preview-first editing with contextual settings. The replacement
uses one dominant preview, a persistent Save/status bar and one contextual inspector.
Page options, default assignments, limits and IDs use progressive disclosure.
Widget selection remains separate from its existing content. Following the user's
subsequent interaction request, editor card bodies are inactive and support direct
mouse/pen dragging. Normal homepage actions remain available.

## Component decisions

- Geometry and drag: reuse public layout.Container and stable movement events.
- Selection and actions: compose button.Button and Heroicons; keep widget bodies arbitrary.
- Layout mode: radio.RadioBar with segmented radio.Radio controls, shared name and labels.
- Exact columns/spacing/order: reuse select.Select through form.FieldGroup.
- Conditional page/advanced sections: semantic disclosures, application-owned open state.
- Inspector: application-owned responsive placement around public form controls.
- Save/status/dialog recovery: preserve existing buttons, live region and modal.Dialog.

No new API fields or persisted layout settings are introduced. The user's open,
unsaved original tab is preserved; the redesign is staged on a separate local instance.

## Validation

Validation uses a separate synthetic SQLite clone on ports 8480/8481. The original
8280/8281 instance and its unsaved browser draft were left intact. The clone uses
real host/service cards and the Alertmanager plugin renderer with synthetic alerts.

- UI `go test -race ./...` and `go vet ./...` pass with `GOWORK=off`.
- All 34 JavaScript model cases pass, including movement boundaries and the current-group no-op.
- View tests assert preview-before-inspector order, one selected context, movement controls outside widget content, stable selection, dormant items and empty drop targets.
- Browser checks cover group/item selection, independent page/group modes, named and pointer/keyboard movement, alphabetical reset, create/delete dialogs, validation errors, Save, cancel/discard and conflicting revisions.
- API readback proves one explicit UI Save increments one layout revision. Draft movements, group creation/deletion, selection and discard do not persist configuration.
- Selected plugin height remains unchanged. Conflict comparison retains local field values; recovery actions remain visible in the sticky bar.
- Desktop widths 1440, 1404 and 1280 and mobile width 390 were captured. Both Goshtoso and Minimal themes were checked in light/dark modes. No horizontal overflow or duplicate IDs were found in the sampled views.
- Static detector findings are empty. One invalid light mobile capture was rejected by the independent finish reviewer and replaced before review continued.

Screenshots capture the app viewport. ConsoleShell scrolls its main content internally;
these files do not claim to show every group below the viewport. The screenshot packet
includes resting, contextual selection and conflict states. Local reports are retained
outside Git; `.impeccable/review/` captures are ignored.

Physical touch, actual screen-reader announcements and Firefox/WebKit were not tested.
There is no production deployment in this change.

## Independent review

Separate design and technical assessments established the baseline problems. A fresh
finish reviewer inspected the screenshot matrix and source using the Impeccable
contract. The packaged reviewer profile was unavailable, so an independent reviewer
agent used the same read-only contract.

The reviewer requested a valid replacement for one blank mobile capture, then found
one material defect: scrolled content leaked through ConsoleShell's padded gap above
the sticky editor header. The bar now offsets that 1.5rem scroll-root padding. A new
390px deep-scroll capture places both the scroll root and bar at y=64px. The reviewer
scored the replacement mobile and desktop views, found no visible regressions, and
returned `disposition: ship`.

## Direct card pickup follow-up

The user requested inactive card clicks and direct press-and-drag between groups.
The editor composes upstream `layout.Instance.WithContentDrag(true)` with an inert
wrapper around arbitrary widget content. Pencils and public keyboard/touch movement
handles remain outside that wrapper. Page and group settings keep their own actions.
The upstream implementation and independent review precede this consumer change.
The UI temporarily pins reviewed upstream commit `ebcaa29d9103` as
`v0.5.1-0.20261102011108-ebcaa29d9103`, available through the Go module proxy.
Balemoh remains a draft PR until upstream PR #364 is merged and a release replaces
that temporary pin.

Validation for this follow-up passes against the downloaded module with `GOWORK=off`:
full UI race suite, vet and view/BFF tests. Native browser checks clicked host,
service and plugin links without changing route or selection, then dragged their
card bodies between groups, including an empty destination. Alt+Up still reordered
the moved plugin. Saved API configuration remained identical at revision 31; QA
draft changes were discarded. At 390px, contextual settings/back navigation retained
visible focus and no horizontal overflow. All ten card bodies retained `inert`
after preview morphs. Normal homepage rendering remains interactive.

A separate consumer reviewer accepted the inert boundary, pencil/handle placement,
OOB widget targets and temporary immutable pin with no blocking findings. Native
pen hardware, physical touch and real screen-reader announcement sequences remain
unchecked. Touch continues to use the existing public handles.
