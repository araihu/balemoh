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
The UI temporarily pins reviewed upstream commit `a8c72cf65a05` as
`v0.5.1-0.20261102031929-a8c72cf65a05`, available through the Go module proxy.
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

## Pointer feedback follow-up

Goshtoso now renders a passive ghost at the pickup offset and a destination gap
that shifts surrounding cards before release. Balemoh consumes that shared runtime
without another pointer handler or persisted setting. Hover preserves real item
order and does not update the draft. One accepted drop uses the existing move
event and conditional Save workflow. Reduced motion skips card shift animations.

Independent upstream review accepted frozen `a8c72cf65a05` after three rounds.
The review found and closed customized built-in lifecycle activation, generated
CSS drift, and exceptions copying valid HTML colon attributes. Browser regressions
cover constructor/connected counts after `is` removal, safe colon attributes,
ghost offset, populated/empty destinations, cleanup and a single accepted move.
The Layout suite exercises Stack, Grid and Masonry, both supported themes,
light/dark and reduced motion. JS lint/drift, canonical CSS regeneration, asset
tests, site lint, current-source integration and published-consumer checks pass.

The consumer was rebuilt with `GOWORK=off` from Go-proxy module bytes; its full
UI race suite and vet pass. Native browser verification on port 8681 captured
Pico's ghost over Services while the target opened a gap, then confirmed Pico
first in that group. Plugin content moved into the newly empty Remote hosts
group. A service-link click stayed on the editor without changing selection.
After drop, all ten widget bodies remained inactive, no ghost/gap/source markers
remained, and browser error logs were empty. Saved API document and revision 32
were identical before/after; only the QA draft was discarded. Preview reopened
clean with Save disabled. Held-gesture evidence is retained outside Git.

Current-head CI is tracked on the PRs. No merge, release or production rollout
is included. Physical pen/touch, real screen readers and Firefox/WebKit remain
unchecked. The temporary pin still requires an upstream release before merge.

## Settings drawer and numeric ordering follow-up

The limits documentation panel is removed. The preview uses the full content
width, and Settings or an explicit entity edit opens a left Drawer below the
shell header. Public Tabs separate Page, Group and Item forms. Public Radio
leading slots pair layout icons with labels. Numeric Group position and Position
in group fields replace Earlier/Later buttons. Complete sibling IDs, including
dormant members, use the existing custom-order API document. No schema changes.

The user chose a drawer that starts closed and opens on demand. Public Goshtoso
Drawer, Tabs, Radio, TextInput and Form components own the controls. Preview
refreshes retain tab context. Invalid numeric proposals reset locally with an
alert, preserve the accepted draft and cannot block another valid Save. Recovery
states close the drawer and focus visible error/status with Retry available.
Error links reopen the matching tab. Dormant/removed controls return focus to a
visible group or Settings action. Capacity hints appear only when an operation
is unavailable. The general limits panel stays absent.

Independent review accepted frozen runtime `235f767285407c5340b412afe0f242de4aa23878`
after fixes for hidden invalid scratch values, trapped recovery, dormant focus
fallback, focus changes during awaited swaps, and inaccurate capacity advice.
Final cleanup removes an unused width-label helper and obsolete inspector style.

Validation passed with the pinned module and `GOWORK=off`: 36 model cases, full
UI race suite, vet, build and regenerated templ. The Impeccable detector reported
only existing design-token advisories. Standalone
full golangci-lint also reports existing repository debt; the scoped diff check
is recorded separately. Browser evidence remains the usability gate.

Native browser checks cover numeric group/item movement, invalid range handling,
Page/Group/Item navigation, layout mode independence, modal Cancel, Escape focus
return and a retained dormant item. A stopped isolated API proved that failure
closes the drawer, reveals Retry and focuses the visible error. Restart and Retry
retained the exact edited group name. The test draft was then discarded.

An isolated API on 8880 was cloned from the original synthetic 8480 database. One
UI Save advanced layout revision 33 to 34 with the exact numeric group/item order;
API readback and reload confirmed it. Test selection changes and the saved QA
layout were restored afterward, leaving the clone at revision 35 with the initial
document. Original 8480 remained unchanged at revision 33, and the original 8681
editor's unsaved draft was not reloaded or discarded. New preview uses port 8781.

The screenshot matrix covers Goshtoso and Minimal, light/dark, 1440px and 390px;
an additional check uses the user's 1404px width. Drawer bounds are positive and
start at y=64, with no horizontal overflow or duplicate IDs. Screenshots and
measurements are retained outside Git. Physical pen/touch, real screen readers
and Firefox/WebKit remain untested. No release or production deployment.
