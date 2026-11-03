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


## Right drawer and grouped commands, 2026-11-02

- Reused public Drawer `SideRight`; group headings and unused group background
  open contextual Group settings. Card clicks, handles and collapse disclosures
  remain independent. No API schema or persistence changes.
- Reused Goshtoso ActionGroup for Layout actions and Preview actions, with public
  OnClick command events for Settings, Create and Cancel. Save retains its initial
  disabled state and validated native command path. Both Create copies obey
  readiness/capacity checks. Overflow closes before modal handoff.
- Independent review caught empty-link overflow actions and static disabled
  styling. The final public command channel resolves both. Review accepted the
  final interaction source; narrow browser proofs remained producer-owned.
- Passed 37 Node tests, native-button render regression, view/BFF tests, focused
  UI race tests, vet, build and diff check. Native pointer and keyboard both
  activated overflow Create at 320px and Cancel at a forced 210px stress width.
  Canceling the modal returned focus to More actions with its menu closed.
- Verified group-background and keyboard selection, inactive card-body click,
  separate collapse controls, Save enabled after an edit, and dirty-draft Cancel
  protection. At 390px the right panel is 320px wide, right-aligned, with no
  duplicate IDs or document overflow. The wider action row returns after resize.
- Preview restored on 8781 using the existing isolated API8880 database. Only
  producer QA draft edits were discarded. No saved layout mutation, commit,
  push, merge, release or deployment. Physical touch and screen readers unchecked.

## Icon movement handles, 2026-11-02

- Toolbar hint now reads "Drag cards and groups to arrange homepage."
- Goshtoso PR #364 commit `3706088900aa2e8d39e316d215dab34aa08c01f9` replaces
  visible movement labels with decorative six-dot grips. The existing localized
  names remain visually hidden and are repeated in tooltips. Handles retain
  native buttons, instructions and keyboard movement, with 44x44 minimum targets.
- Independent upstream review and full focused Layout browser suite passed before
  integration. Layout/assets unit tests, vet, root/site diff lint and both
  site-current-source/published-consumer contracts passed.
- Balemoh UI pins public proxy version
  `v0.5.1-0.20261102091851-3706088900aa`, with GOWORK=off and no replacement.
  Focused UI race tests, vet, module verification and consumer review passed.
- Native preview verified group keyboard reordering and cross-group card transfer
  with grip controls. No drawer opened and no navigation fired. At 390px and
  1404px, grips remain at least 44x44, hidden labels measure 1x1, and the document
  has no horizontal overflow. Only the producer QA draft was discarded; saved
  isolated API layout remains revision 35. Existing user tabs were not reloaded.
- Required metadata checks found an inherited homepage canonical on this editor.
  Set PageData.Path to the registered editor route and added it to existing brand
  metadata coverage. Initial HTML for homepage/editor now has each required title,
  description, canonical, Open Graph and X tag once. Both local and production
  social image return 200 PNG, 1280x640, 46963 bytes.
- Screenshots: report `grip-handles.jpg`, `grip-handles-mobile.jpg`.
  No merge/release/deployment. Physical touch and screen readers remain unchecked.

## Drawer and dialog spacing, 2026-11-02

- Added application-owned vertical stacks to Item settings and all four dialog
  bodies. Numeric errors stay with their fields. Action rows wrap on desktop and
  stack full-width on mobile. Existing controls, command IDs, form submission,
  focus targets and HTMX roots are unchanged.
- Confirmed the selected Goshtoso Modal already owns adequate panel padding;
  arbitrary nested form content lacked spacing. No upstream change is warranted.
- Independent design assessment and source review passed. View tests with race,
  vet and focused BFF Layout/metadata tests passed. Native desktop1106/mobile390
  checks measured selector-to-Move12px, Move-to-Position16px, inline error8px,
  input-to-dialog-actions24px and action-to-action12px. Controls remain44px high.
- Native checks covered all drawer tabs, invalid position reset and message
  placement, Create via Enter, Delete/Cancel over the drawer, and Discard. Create
  and Delete mobile action buttons measured324px wide with no dialog/document
  overflow. Saved comparison retains the same spacing wrappers and passed source
  and render review; a fresh conflict was not created for this visual-only change.
- Only producer QA draft changes were discarded. Saved API8880 remains revision 35.
  User tabs/forms were not reloaded. Preview 8781 uses the downloaded pinned module
  with GOWORK=off. Initial homepage/editor metadata and production social image
  checks passed. No merge, release, deployment or dependency change.
- Screenshots in report `spacing/`: item-desktop.jpg, item-mobile.jpg,
  create-desktop.jpg, create-mobile.jpg. Physical touch/screen readers unchecked.

## Empty group drop, 2026-11-02

- Reproduced with a native mouse drag: Grafana into collapsed Remote hosts was
  canceled; expanding the zero-item group allowed the drop. The saved preference
  `InitiallyCollapsed:true` hid the nested item container.
- Zero-active editor groups now always expose their existing 64px drop area.
  Dormant references do not make a visually empty group collapsible. Populated
  groups keep the existing disclosure behavior; homepage preferences are unchanged.
- Native mouse checks passed without selecting/expanding the destination, then
  moved the last card out and dropped into the recreated empty group again.
  The empty/populated wrapper transitions preserved movement and focus after
  HTMX refresh. Only the QA draft was discarded; saved revision 35 and the true
  homepage collapse preference remain unchanged.
- Added empty/dormant-only render regression. View tests with race, vet, focused
  BFF Layout checks, diff lint and independent review passed. Metadata/image
  verification passed. No JS, API, upstream library or dependency changes.
- Screenshots: `empty-drop-success.jpg`, `empty-drop-target.jpg` in the report.
  Original user tabs were not reloaded. Physical touch remains unchecked.

## Whole-group dragging and explicit pencils, 2026-11-02

- Outer groups and inner cards opt into Goshtoso body drag independently. Group
  padding, headings and nested background pick up the group; nested cards retain
  their own scope. Group/body/title clicks do not open the drawer. Shared 44px
  pencil actions open the corresponding Group or Item tab.
- Upstream `c6de263385f6a9ea1f871271662a6666c667369d` is pushed in PR #364 and
  integrated through public pseudo-version `v0.5.1-0.20261102104218-c6de263385f6`.
  No local module replacement is used. Upstream independent review and the entire
  Layout E2E suite passed, including new nested-background/padding/handle-row tests.
  JS drift/lint, unit/vet, diff lint and both module contracts passed.
- Consumer independent review, view race tests, all 37 Node tests, full BFF tests,
  vet and module verification passed. Native mouse QA dragged the empty group's
  body before another group, then dragged a plain title to restore order. It also
  clicked title/card bodies without opening/navigating, and moved atlas between
  groups while keeping group order unchanged. Both pencil buttons open the right
  settings tab and have identical classes and 44x44 geometry.
- Only producer QA draft changes were discarded. Original user tabs were not
  reloaded; saved layout remains revision 35. Preview 8781 uses the exact downloaded
  module. Metadata and public social image checks passed. No merge/release/deploy.
- Screenshot `group-drag-edit.jpg` shows both shared pencils. Physical touch and
  screen readers remain unchecked; touch body keeps scrolling and uses handles.


## Published Goshtoso v0.6.0, 2026-11-03

- [Goshtoso #364](https://github.com/araihu/goshtoso/pull/364) merged with the
  authorized organization-admin bypass at `57b800042a41a167935774c9961106af2556050f`.
  Independent review accepted `e11ebc52`; its source tree matches the merge/tag.
  The Layout implementation and generated runtime remain identical to reviewed
  `c6de2633`. Main's existing dependency updates were included before final CI.
- [Main CI](https://github.com/araihu/goshtoso/actions/runs/41625264029) passed
  before tagging. [Release verification and publication](https://github.com/araihu/goshtoso/actions/runs/41627265688)
  passed, including full browser coverage, Codecov upload and the isolated
  exact-tag published consumer. Authoritative authored Go coverage is 94.2%.
  Both coverage profiles, summaries and HTML reports are retained locally.
- [v0.6.0](https://github.com/araihu/goshtoso/releases/tag/v0.6.0) is a published
  release. Downloaded CSS assets match tag SHA-256 values. Release and
  coverage badges match the release artifact. Codecov exposes a completed report
  for the merge commit. The docs deployment and exact-tag skill discovery passed.
- The public Go proxy resolves v0.6.0 to the merge commit. Its CSS and JavaScript
  match the tag. Both Balemoh modules now select v0.6.0 with GOWORK=off and no
  Goshtoso replacement; stale pseudo-version sums are removed. YAML v3.0.6 and
  x/net v0.62.0 follow the release's dependency requirements.
- API and UI full race suites, vet, CGO-disabled builds and module verification
  passed. Templ generation has no drift. All 50 editor/notification Node tests
  passed. Current UI diff lint reports no new findings; full standalone lint
  was not repeated, and earlier repository findings remain documented above.
- Preview API8880 and UI8781 were restored from the release-pinned binaries using
  the existing isolated database. HTTP checks verified the editor, canonical route
  and exact release JavaScript. A fresh native tab rendered the saved editor.
  Later browser refresh/reconnection timed out, so fresh native movement testing
  was not repeated. Earlier native interaction receipts cover the identical
  Layout runtime. No original user tab was reloaded and no draft was discarded.
- Saved layout remains revision 35 with the same document SHA-256 before/after
  startup. No Balemoh production deployment. Physical pen/touch, real screen
  readers and Firefox/WebKit remain unchecked.
- Receipts are under the existing local report's `release-v0.6.0/` directory.
