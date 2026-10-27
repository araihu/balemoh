# Themes local acceptance

Validated 2026-10-27 in the dedicated Balemoh, Goshtoso and App Shells worktrees.
No publication or production deployment is included.

## Receipts

| Contract | Evidence |
| --- | --- |
| All 16 literal preset snapshots and derived edits | `themes/definition_test.go`, public editor Node runtime tests |
| Public controlled editor and frame | Goshtoso root unit suite; frame origin/source/instance/version/sequence tests |
| Site extraction and public catalog | `just site-current-source-integration`, 93 non-E2E packages and server build |
| Site interactions | Focused `TestThemePage_*` and `TestLanding_*` E2E suite, including public catalog/social contract, modal reset, font search, isolation, export and fragment navigation |
| Responsive component composition | `TestThemePage_ResponsiveThemeMatrix`, 390/1440 px, Goshtoso/Minimal, light/dark |
| System-mode OS tracking | App Shells ConsoleShell Go/Node tests with cleanup and storage disabled |
| Atomic save, exact replay, stale revisions | SQLite store, HTTP, BFF SDK and browser-runtime tests; concurrent Save has one winner |
| Referenced-font/active-theme deletion blocks and independent copies | SQLite store and forged HTTP delete tests against current server state |
| Font containers | TTF, WOFF, CFF OTF, actual variable Roboto WOFF2 fixtures; malformed/bomb/collection tests |
| Quotas | Store tests reject the 129th theme, 257th font and over-budget font bytes; exact theme replay works at quota |
| Unknown transport outcome | Runtime test retains the exact command and draft, then replays successfully |
| Upload limits | HTTP accepts structural TTF fixture padded to exactly 10 MiB, rejects 10 MiB + 1; validator decoded/metadata bounds |
| Backup/restart | WAL database `VACUUM INTO`, close/reopen original and backup, compare settings/references/font bytes |
| Actual local browser save | Uploaded variable Roboto, selected it through font Combobox, saved a named custom theme, full reload left recovery; custom CSS/font reference present |
| Actual local restart | Restarted API process; reloaded UI retained selected custom theme and immutable font ID |
| Stale browser | Repeated Save from older editor returned conflict twice; name and font draft remained; Escape canceled discard and restored focus |
| Font rendering | Font options and trigger use actual family; host remains System while preview is isolated; screenshot `data/themes-font-combobox.png` |
| Global font failure | Node test verifies temporary Goshtoso/System through shell initialization and later OS change; saved input unchanged |
| API documentation | OpenAPI lint score 100/100; generated server/client checked; root/client/UI Go suites and `go vet` |

Representative fixture IDs in the development database are test-only. Existing
user records and other worktrees were preserved.

## Coverage limits

- Integration uses an ignored local `go.work`. Published-dependency verification
  remains pending until separately authorized upstream publication and pinning.
- `golangci-lint` v2.12.2 cannot decode Go 1.27 export data, including after a
  same-version rebuild with Go 1.27. This is a blocked tool check, not a pass.
- The in-app browser logged an unassigned MutationObserver error during document
  load. The focused Chromium site suite reported no page errors; attribution in
  the embedded browser remains unresolved.
- Full repository-wide E2E, every ledger row as a rendered Balemoh browser flow,
  every preset as a pixel matrix, automated accessibility scanning, blocked browser
  storage in the integrated Balemoh page, real-browser transport loss and broad race
  stress were not run. Server/runtime coverage above does not replace these.
- Font validation is bounded container/metadata validation. The browser still
  sanitizes glyph data. No release-readiness claim is made for this exploratory
  delivery.

## Control refinement, 2026-10-27

- Public Select rich options use typed leading content and label attributes.
- Theme options render literal primary/secondary/surface swatches and private
  title-font previews, including the selected trigger label.
- Public editor inspection uses a sun/moon toggle with action label and pressed
  state. E2E verifies mouse/Space toggling, isolated frame mode and unchanged
  definition and host mode.
- Focused Themes E2E and Select/editor unit tests cover the new contracts.

Select, Themes and Landing focused E2E regression suite passed after the control
refinement, including the existing native-change/HTMX synchronization contract.

## Production card preview and English UI, 2026-10-27

The frame uses production ServiceCard, HostWidget and DashboardGrid renderers
with fixture Jellyfin/Grafana services and a reporting NAS. It loads the actual
Balemoh stylesheet. UI labels, dialogs, errors, recovery notices and upload
feedback use English. SDK/HTTP tests verify card content, local preview script,
stylesheet and form-navigation policy; Node checks block link navigation and
form submission while allowing local card controls. No SSE is mounted.
An upstream runtime test covers loading an unused uploaded title font without a
false unavailable warning.

## Global mode segments, 2026-10-27

Global mode composes RadioBar and three segmented Radio controls with decorative
window/sun/moon icons and visible System/Light/Dark labels. A native fieldset
provides the group name; native radio selection and RadioNodeList provide the
value. UI tests verify three controls and saved mode payload. Radio's LabelLeading
slot has rendering and native keyboard regression coverage.

Radio E2E passed, including click and arrow-key cycling. Manual Balemoh browser
validation confirmed one checked mode, native Light/Dark selection, and preview
mode updates without saving global preferences. Screenshot: data/themes-global-mode.jpg.

## Palette Select shells, 2026-10-27

ThemeEditor reuses the existing Palette inside Select Shell for every editable
color role. Triggers show swatches and literal values; an Exact value input keeps
hex/sRGB/OKLCH and alpha editing. Palette picks materialize named colors at once,
and Reset restores the role's loaded baseline. Pick/Escape focus restoration is
deferred through a frame so closure can finish without stealing later focus.

Unit tests cover literal materialization, reset and invalid palette values. Browser
tests cover mouse/keyboard picks, reset, invalid exact input, focus return and
390/1440 px popup bounds. Manual Balemoh verification selected blue-700 for the
primary dark role and displayed the literal OKLCH value. Screenshot:
`data/themes-color-palette.jpg`.

## Preview color regression, 2026-10-27

Dark-mode filtering now matches actual counterpart names, including
`surface-dark-alt`, `on-surface-dark-strong` and `outline-dark-strong`.
Light counterparts no longer appear while inspecting dark colors. The browser
regression reproduced the incorrect visible `surface-alt` control before the fix.
Tests check the frame's computed primary variable and rendered light/dark body
background after palette picks. Manual Balemoh verification changed
`surface-dark-alt` to blue-700 and confirmed the real host/service cards changed
without Save. Screenshot: `data/themes-color-preview.jpg`.

## Separate theme selection and editing, 2026-10-27

Themes contains saved-theme selection, global mode, read-only frame preview and
Apply. Create/Edit links carry the selected identity and recovery state to a
dedicated editor route. Presets and explicit new copies create independent IDs;
editing a custom theme keeps its expected revision. Name/colors/fonts/uploads
and catalog management remain on the editor page. Editor Save still applies
globally through the original atomic command and returns to Themes.

UI/runtime tests cover browsing without mutations, source/instance-checked frame
readiness, applying saved identities without drafts, exact command replay after
conflict/uncertainty, route isolation, recovery and unknown-target handling.
Manual browser checks selected Minimal, switched preview to Light without
applying, followed Create to its own page, verified Cancel preserves the draft,
and confirmed explicit discard returns to Themes. Edit opened the saved custom
theme without a base selector; leaving unchanged needed no draft confirmation.
Screenshots: `data/themes-selection.jpg`, `data/themes-editor-page.jpg`.

A temporary browser fixture against the running Balemoh replica passed all 24
Browse/Create/Edit × 390/1440 px × Goshtoso/Minimal × light/dark cases, including
open selectors and no page overflow. The shared font grid and Balemoh grid tracks
now constrain long uploaded-font labels. Public ThemePage E2E retains the long-font
regression. Local fixture/receipt: data/themes-layout-check.go.txt and data/themes-layout-check.log. No real Save
was issued in these browser navigation/layout checks.

## Theme branding, 2026-10-27

Migration 14 adds a nullable image reference per saved theme. Store tests cover
atomic save/replay, unknown image rejection, copy inheritance, compatibility with
older drafts, explicit default restoration and referenced-image update/delete
protection. Runtime tests cover pending/failed/oversized uploads, draft-only logo
previews, saved payloads and source/origin/instance/sequence checks.

A disposable API/UI pair with its own database passed real browser upload, Save,
default restoration, second upload with a new reference, header/favicon/touch-icon
and manifest assertions, and the fixed Balemoh/Arai Hû/Goshtoso footer. Its saved
brand reference survived an API restart. Main development preferences were not
changed by these save checks. A SQLite backup precedes the main migration.

Metadata tests require one title, description, canonical, Open Graph and explicit
X Card set for homepage and representative subpages, including the editor. The
existing production HTTPS social image returned 200 image/png, 1280x640 and
46,963 bytes. Footer site links use https://araihu.com/ and
https://goshtoso.araihu.com/.

The final browser run also verified that successful upload clears FileInput's
visible filename through its native change contract, and all 24 responsive
Browse/Create/Edit cases still pass with the branding controls and fixed footer.
Screenshots: data/themes-brand-editor.jpg and data/themes-brand-footer.jpg.
The disposable QA processes were stopped; the selected image reference and PNG
bytes remained readable on disk afterward. Fixtures, receipt and combined log
are retained under data/brand-qa-c138b951/.

The main preferences received interactive changes during development. Those
changes were preserved; the before-migration backup was not restored.


## Complete interface preview, 2026-10-27

Both saved-theme selection and the dedicated editor use the expanded isolated
frame. It composes public Goshtoso controls with real Balemoh cards, including
buttons with primary/secondary/alternate/danger/disabled treatments, menu sections,
badges, tabs, form validation, Select, Checkbox, Toggle, segmented Radio and a
native Goshtoso Dialog. Title-font and body-font samples are visible.

View/BFF tests passed with the existing navigation/submit and branding protocol
regressions. The read-only browser matrix passed all eight Goshtoso/Minimal,
light/dark, 390/1440 px combinations. It checked all tab panels for horizontal
overflow, Select selection, editable fields, checkbox/switch state, menu actions,
modal cancel/focus return and zero non-GET requests or script errors.
Fixture/receipt: data/themes-expanded-preview-check.go.txt and
data/themes-expanded-preview-check.log. Screenshot: data/themes-complete-preview.jpg.
No production publication or saved-preference changes were part of these checks.


Logo upload failures now dispatch the existing Goshtoso danger Toast and retain
an inline message associated with an aria-invalid FileInput. Missing/oversized
files and server errors preserve the draft. New selection/default restoration/
success clears the error. View/BFF/runtime checks passed; library FileInput E2E
covers both appearances, Goshtoso/Minimal, light/dark and 390/1440 px. Consumer
mock-rejection browser receipt: data/theme-logo-error-check.log. No asset or
preference writes were performed. Screenshot: data/theme-logo-upload-error.jpg.

Preview inspection now uses a public Goshtoso RadioBar with visible Light/Dark
labels, sun/moon icons and a Preview mode legend. Component and consumer tests
passed. Public E2E checks pointer selection, native arrow keys, one checked mode,
unchanged theme definition/host appearance, color-role filtering and the frame's
rendered colors. A local browser check confirmed initial saved-mode synchronization
and Light selection. Screenshot: data/theme-preview-mode-segments.jpg.

Contrast advisory now uses live Aa/inverse samples, ratio and grade badges in
Surface/Primary/Secondary panels, following the Goshtoso site's checker pattern.
Twenty browser cases passed across 390/1440 px, light/dark and AAA/AA/AA large/
Fail/Not rated outcomes. They verify 21:1 black/white, inverse computed colors,
active badges and responsive bounds. Public editor and consumer view/BFF tests
also passed. Translucent pairs are explicitly Not rated. No preference writes.
Screenshot: data/theme-visual-contrast.jpg.


CSS import passed the 16 built-in preset fixtures, browser Export/Import round
trips for System/Google/bundled fonts, confirmation Cancel/Confirm and Reset,
and empty/oversized/malformed/incomplete/multiple-theme/unknown-font/invalid-value
rejection. A detached stylesheet test confirmed untrusted import/font/image URLs
were not fetched and host styles were unchanged. Component and consumer tests
passed. Balemoh browser checks at 390/1440 px verified imported Save payloads,
retained name/logo/global mode and conflict draft preservation with mocked Save,
without writing global preferences or assets. Fixtures/receipts are retained as
data/theme-css-import-check.go.txt and data/theme-css-import-check.log.
Screenshot: data/theme-css-import.jpg. Example: data/theme-import-example.css.


Theme origin now appears as a right-aligned Built-in/Custom Badge instead of a
name prefix, in the menu and selected trigger. Source slots, typography, swatches
and IDs remain intact. Select render/keyboard E2E and consumer Browse/Create
checks passed at 390/1440 px, including badge geometry and unchanged raw labels.
Receipt: data/theme-origin-badges-check.log. Screenshot: data/theme-origin-badges.jpg.


Create/edit now compose Goshtoso Form and typed FieldGroups while retaining the
public Select/FileInput/ThemeEditor/Save Button and the existing layout. Required
name uses FieldGroup's marker and native/ARIA validation. No upstream API changed.
View/BFF/runtime checks passed. Read-only browser QA covered required/whitespace
names, Enter submit, same-command retry after mocked conflict, logo rejection/
Toast, imported Save payloads and all 24 responsive Browse/Create/Edit cases.
Receipts: data/theme-form-migration-unit.log and data/theme-form-migration-browser.log.
Screenshot: data/theme-goshtoso-form.jpg. No saved preferences or assets were written.

## Automatic logo upload, 2026-10-27

Selecting a logo now uploads immediately, without an Upload logo button. Reset
remains explicit. Runtime checks cover in-flight control/Save blocking, one request
per selection, size/server errors, same-file retry, error clearing and draft-only
branding. UI/BFF tests pass. Mocked browser writes at 390/1440 px verify automatic
rejection/success, field error plus Toast, cleared chooser for retry, typed Save
reference and reset. No saved settings or assets changed during QA. Receipt:
data/theme-logo-auto-upload-browser.log; fixture: data/theme-logo-auto-upload.go.txt.
Screenshot: data/theme-logo-auto-upload.jpg.

## Form sections and preview columns, 2026-10-27

Use public form.Section cards for Theme details, Site identity and Appearance.
The complete form and management controls occupy the left column; the independent
preview sits in the right column, sticky on large screens. Below lg the columns
stack. No library API change. UI/BFF tests and the 24-case browser matrix pass,
covering browse/create/edit, 390/1440 px, Goshtoso/Minimal and light/dark. Editor
checks assert all three sections, separate form/preview columns and mobile
stacking, plus contained controls and open/close font combobox behavior.
Receipt: data/theme-sections-layout.log; screenshot: data/theme-sections-layout.jpg.

## Independent logo and icon, 2026-10-27

Site identity now has separate logo and icon FileInputs, each uploading on
selection with its own default/reset, inline error and Toast. Either pending or
failed upload blocks Save. The logo drives the navigation image; the icon drives
favicon, manifest and touch icons. The isolated preview receives both references
with the existing origin/source/sequence checks. Footer credits remain fixed.

Migration 15 initializes the new logo column from the legacy shared image.
Legacy replacement drafts preserve an omitted logo; new definitions omitting
brandLogoId retain the previous shared-image behavior. Explicit empty values
select defaults independently. Both referenced images remain immutable and
protected from deletion. Copying a theme retains both references.

Store/migration, HTTP, BFF, view/runtime and generated-client checks pass, with
Go vet and OpenAPI validation. Browser QA on a disposable database at 390/1440 px
performs two real uploads, verifies distinct preview images, saves/reloads and
restores each image independently. It checks the applied logo/favicon, footer
and responsive width. Main local settings were preserved during migration; no
QA image/theme was written to the user's database. Receipts: data/separate-brand-
validation.log, data/separate-brand-vet.log, data/separate-brand-openapi.log and
data/separate-brand-roundtrip.log. Screenshot: data/separate-brand-controls.jpg.

## Automatic CSS import, 2026-10-27

The public Goshtoso ThemeEditor imports on FileInput change; Import CSS button
removed. Existing validation, detached CSS parsing, unsaved-draft confirmation,
Cancel, Reset, field errors and Toast remain. Clear the chooser on all outcomes
so the same file can be selected again after cancellation/error. An empty change
event does nothing. Library tests cover every preset, export/import round-trip
with system/Google/bundled fonts, confirmation/cancel/reselection, reset and invalid
files. Generated JS drift check and component/theme tests pass. Consumer checks
at 390/1440 px verify automatic import, button absence, responsive width and
name/logo/icon/mode preservation through a mocked Save conflict. No settings or
assets were written during QA. Receipts: data/css-auto-import-library.log and
data/css-auto-import-consumer-browser.log.

## Radius slider regression, 2026-10-27

Reproduced a track blending into Form.Section's surface-alt background and a
radius definition/preview that stayed unchanged until mouse release. Public
Range now uses outline-strong track colors in both modes. ThemeEditor shares
its existing validated update handler between change and radius input events,
so pointer dragging updates the draft/preview immediately.

The native drag regression fails before the fix and passes afterward, including
keyboard Home/ArrowRight. Range, ThemeEditor, CSS import and consumer tests pass.
Consumer QA covers 390/1440 px, Goshtoso/Minimal and light/dark, checking the draft
and iframe radius before mouse-up. No settings/assets were written. Receipts:
data/radius-regression-before.log, data/radius-library.log and
data/radius-consumer-browser.log. Screenshot: data/radius-fixed.png.

## Preview follows editor scrolling, 2026-10-27

Replace unavailable prebuilt lg:sticky/lg:top-6 utilities with an application-owned
preview-column rule. At >=1024 px the preview stays sticky in the shell's main
scroll area; its iframe height fits the viewport. Mobile keeps normal stacked
flow. UI/BFF tests pass. Browser checks scroll the form to 750 and 1500 px at
1440x900, 1024x600 and 390x844, asserting the pinned desktop position, fully
visible iframe and ordinary mobile flow without horizontal overflow. No settings
were changed. Receipt: data/preview-follow-scroll.log. Screenshot:
data/preview-follow-scroll.png.

## Appearance and Fonts tab, 2026-10-27

Rename the page/document/sidebar label to Appearance and add Fonts beside Icons
and Themes. Move font upload/list/delete from the theme editor into that tab,
using public Form.Section, FieldGroup and FileInput. The editor keeps the font
selectors. Font management has a standalone script and Goshtoso confirmation
modal, retains immutable upload identities on uncertain retries, creates a new
identity when the request changes, preserves the 10 MiB check and server usage
errors, and reloads the selected Fonts tab after success.

All UI tests and Go vet pass. Node checks cover uncertain retry identities,
changed uploads, size rejection, Cancel/Escape and referenced-font errors. Browser
QA at 390/1440 px uses a disposable database for real font upload, list refresh,
editor font availability, cancellation and deletion, while preserving the Fonts
hash and avoiding horizontal overflow. No main-database font/settings changed.
Receipts: data/appearance-fonts-unit.log, data/appearance-fonts-all-ui.log,
data/appearance-fonts-vet.log and data/appearance-fonts-browser.log. Screenshot:
data/appearance-fonts-tab.png.

## Theme deletion in header actions, 2026-10-27

Remove the duplicate Manage custom themes rename/delete list. Custom theme name
in the main form remains the rename control. Public PageHeader.Actions renders
an ActionGroup Delete button only for saved themes in edit mode. New presets,
new copies and unavailable editors have no deletion action. Existing Dialog
confirms deletion; the request carries the loaded theme revision. Failures show
feedback beside the action and preserve the draft. Success clears draft guards
and returns to Themes, retaining recovery mode when applicable. Active-theme
protection stays in the existing API.

All UI tests and Go vet pass. Runtime checks cover action visibility, Cancel,
Escape, confirmed navigation, recovery and revision-preserving retries after
conflict. Disposable browser QA at 390/1440 px verifies header placement, no
duplicate list/name field, hidden action for new copies, draft preservation on
Cancel and actual deletion of inactive saved themes. Main data was untouched.
Receipts: data/theme-header-delete-unit.log, data/theme-header-delete-all-ui.log,
data/theme-header-delete-vet.log and data/theme-header-delete-browser.log.
Screenshot: data/theme-header-delete.png.

## Compact branding reset actions, 2026-10-27

Replace the two full Restore default buttons with public plain, neutral icon
buttons beside the logo/icon previews. Use Heroicons XMark and public Tooltips
with explicit accessible names/descriptions, retaining the existing reset hooks,
40 px hit targets, upload guards and draft-only behavior. Consumer UI/BFF tests
pass. Mocked browser uploads at 390/1440 px verify independent Enter-key resets,
icon-only rendering, tooltips on focus, viewport containment, side-by-side
placement and zero settings writes.

During inspection a portal tooltip animated its top/left after inner scrolling,
briefly detaching from the trigger. Fix the shared Goshtoso portal runtime by
limiting transitionProperty to opacity. Tooltip browser coverage and a dedicated
nested-scroll regression pass, checking attachment within two animation frames.
Receipts: data/branding-reset-icons-browser.log, data/branding-tooltip-scroll.log
and data/branding-tooltip-unit.log. Screenshot: data/branding-reset-icons.png.

## Reset X only for custom images, 2026-10-27

Render each reset wrapper hidden when its image reference is empty and update
visibility on upload, base-theme selection and reset. The X is tied only to a
custom reference, never to upload errors or pending state. Reset returns keyboard
focus to its file input and closes the tooltip. Keep failed uploads blocking Save;
provide a separate Discard failed upload action beside errors so a default image
needs no X to recover. Discarding a failed replacement preserves the old image.

UI/BFF and runtime tests pass. Mocked browser QA at 390/1440 px verifies initial
hidden actions, custom image visibility, independent keyboard/pointer resets,
tooltip closure, hidden-default error recovery, width containment and zero global
writes. Receipts: data/custom-branding-x-unit.log and
data/custom-branding-x-browser.log. Screenshot: data/custom-branding-x-hidden.png.
