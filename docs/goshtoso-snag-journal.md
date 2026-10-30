# Goshtoso integration notes

## Shared demo service logos, 2026-10-30

- Reuse the locked selfh.st catalog and existing `LibraryImage`/`ServiceGlyph` rendering. No custom component, CSS, JavaScript or new image source is needed.
- Full-library cold preparation exceeds the demo's small temporary-storage budget. Embed only its twelve existing default PNGs, verify bytes against catalog hashes, and serve them through the normal icon-library route. Attribution and licenses remain on the existing page.
- Closing check requires twelve loaded brand images, preserved read-only actions and desktop/mobile light/dark inspection after publication.

## Host telemetry, 2026-10-07

- Goshtoso Charts v0.0.3 line chart renders `figure role="img"`. Axe reports `aria-allowed-role`, minor severity. Its accessible name and adjacent exact-value table remain available. No public role override exists. Upstream correction remains pending; no consumer DOM rewrite added.
- Goshtoso v0.3.2 Card title is h3 and its default width is capped. Compose under h2 sections and use the documented RootClass to fill the dashboard column.
- Goshtoso v0.3.2 Table has no container attribute slot for tabindex. A text-only overflowing workload table is not keyboard-scrollable by default. Use an application-owned, named focusable scroll region and the documented RootClass to avoid nested scroll containers. Axe no longer reports that blocker.
- Charts use fixed SVG dimensions. Width 360, larger axis labels and time-only labels keep a single-column mobile chart legible. Exact data remains available at one-minute resolution.

- Charts exact-value tables also lack a focusable scroll-container option. A named, focusable application region owns chart/table scrolling, with inner overflow removed through scoped CSS on native details markup. This keeps expanded 24h data keyboard-accessible without rewriting generated HTML.

## Live host telemetry, 2026-10-07

- Charts v0.0.3 interactive `LiveData` requires finite numbers in every series slot; it cannot represent missing samples. Use Goshtoso v0.3.2's bundled HTMX 4 SSE HTML stream with static Line `Missing` points. Consider native Charts streaming when its public snapshot contract supports gaps.
- HTMX 4 uses `hx-sse:connect`, unnamed HTML events and `innerMorph`; the local older ticker example still uses HTMX 2 attributes. The pinned migration guide and runtime are the applicable contract.
- Compose public `chartcontrol.Wrapper` around a stable application-owned chart target and set the nested line's wrapper mode to omitted. This keeps expansion lifecycle outside SSE swaps, including when the target moves into the modal. A scoped `htmx_before_morph_attr` extension hook preserves the native `details` open attribute.
- The HTMX 4 morph attribute callback is an extension hook, not a dispatched DOM event. Use public `htmx.registerExtension`; a document event listener silently fails to preserve disclosure state.

## Host page display cleanup, 2026-10-07

- User requested chart captions and exact-value disclosures removed. Clear the public Caption field. Charts v0.0.3 has no disclosure visibility option, so scoped CSS hides its native details element under the application-owned host chart container. Remove the now-unused disclosure morph hook and scroll region. Replace this selector when Charts exposes a visibility option.
- Absolutely positioned table captions and hidden chart controls escaped the shell's main scroll area and enlarged document scroll height. Give the application-owned main ID a positioned containing block; keep the existing main scroll container and fixed modal behavior.

## Host history partial navigation, 2026-10-07

- Range links and Refresh use Goshtoso Link attributes with HTMX 4 `innerMorph`, one shared `replace` request group, and URL history. `HX-Request-Type` selects fragments; history requests marked full receive the complete shell. Link `WithAttrs` replaces its attribute map, so assemble all attributes before passing it.
- Changing the SSE URL during morph reproduced the documented hx-sse 4.0.0 unhandled AbortError. The connection event wraps abort to cancel the reader first, following the pinned migration guide. Only expected AbortError is ignored; other cancellation errors remain visible. Queued events from a superseded range are discarded before swaps.
- Give each range's SSE connector a distinct DOM ID. Reusing the same element lets the old extension loop reconnect after cleanup while a replacement starts. Removing the old element ends its loop; the new range owns a new connector. Refresh keeps the existing connector and does not push duplicate history entries.

## Filter request stall, 2026-10-07

- Repeated range changes reproduced a connection leak: each HTMX `innerMorph` swap opened two SSE requests and closed one. With six open HTTP/1 connections, browser requests for Last hour/24h stalled while API and HTML responses still took about 1 ms and 11 ms outside the browser. A single connector in the DOM did not prove one live connection.
- Use `innerHTML` for explicit host-detail navigation so HTMX tears down the old connector before initializing its replacement. Keep `innerMorph` for SSE chart snapshots. `hx-preserve` retains the status notice during explicit navigation; `hx-morph-skip` retains it during SSE updates. Ten successive alternating filter requests completed with one server-observed active stream and 1–11 ms server durations. Stable action IDs allow focus restoration after replacement.

## 2026-10-10 interactive host charts

- Reuse the public `components/interactive/line` component and `components/chart` options for tooltips, percentage axes and responsive sizing. Compose the existing outer `chartcontrol.Wrapper` with omitted nested controls. No consumer renderer code added.
- Interactive Line v0.0.3 cannot represent missing readings. Charts v0.0.4 adds `Data.Missing` to preserve explicit null slots while retaining zero readings. The UI now pins that published release; the temporary local workspace is removed.
- `dependencies.Dependencies()` must precede chart markup because each chart initializes inline. Render it in full `PageContent`, outside the partial `HostContent` response, so range changes and SSE snapshots reuse the runtime. Existing chart assets are already mounted.
- Pointer tooltips supply exact values. Charts has no public keyboard point-navigation API; its existing accessible chart name and keyboard expansion controls remain. The requested hidden exact-value disclosure remains hidden.
- The public dependency component loads the chart runtime plus word-cloud, liquid, 3D and map registrations even for line-only pages. There is no public per-chart dependency selection; keep the supported component rather than copying private script lists.
- Morphing interactive figures preserves the unchanged theme script while replacing the generated chart host. Only CPU/RAM chart contents now use `innerHTML` OOB swaps, so public chart initialization runs with a fresh figure. Outer chart controls and expanded modal targets stay stable; measurements, status and inventory still morph. Upstream owns detached-instance disposal.

## Homepage widgets, 2026-10-14

- Compose Card Body/Footer slots with application-owned metric `dl` content. A dashboard renderer returns `templ.Component`; the grid does not require Card. A registered inline note renderer test verifies this boundary.
- HTMX 4 dispatches `htmx:error` for transport failures and `htmx:response:error` for HTTP failures. The nonexistent `htmx:request:error` event would leave failed selection requests without feedback. Verified the pinned runtime and used the supported event.
- Removing a card also removes a focus initializer nested inside its selection control. Retarget the successful removal response to the persistent live-status element, update the grid out of band, then focus status. Browser acceptance confirmed focus remains on `dashboard-live-status`.
- The shell exposes no appearance selector in Balemoh. A temporary local preview wrapper overrides only shell appearance configuration to exercise Goshtoso/Minimal and light/dark at 390 and 1440 px. Production behavior and dependencies stay unchanged.
- Minimal's existing solid info Badge uses white 12px text on `oklch(0.685 0.169 237.323)`. This needs upstream contrast review. This feature reuses the existing status treatment; alternate-theme visual inspection is not a complete accessibility certification.

## Per-widget refresh, 2026-10-14

- Disabling a focused refresh button during HTMX requests drops browser focus even when its wrapper survives morphing. Restore focus through `htmx:finally:request` after Alpine's next tick. Browser verification confirmed focus returns to the initiating button and the dashboard keeps one SSE connector.

## Dashboard masonry, 2026-10-14

- The pinned Goshtoso components and App Shells expose no masonry layout. Keep their Card components and the application-owned widget grid. A ResizeObserver measures generic widget wrappers into one-pixel CSS Grid row spans, preserving DOM order and the existing breakpoints. Ordinary grid remains the fallback without JavaScript. Reconcile spans after HTMX swaps and observe new/removed widgets; unchanged metric heights do not move cards. No component-internal markup or new dependency is required.

- Host-first grouping composes semantic sections inside the existing shell. Each section owns the same generic masonry layout; source metadata is independent of renderer selection.

## Address menu baseline, 2026-10-15

- The dropdown's inline trigger adds a baseline pixel to the address row, making multi-address cards 185 px high beside otherwise equivalent 184 px cards. Masonry then shifts items between columns. Compose the public Dropdown RootClass with an application flex class so the trigger uses its actual height. Keep the menu component and natural card sizing.

## Search empty-state spacing, 2026-10-20

- Modal Dialog's public Compact option intentionally leaves content unpadded. Compose its Content slot with app-owned empty-message spacing matching the result rows. Hide the empty result list so its padding does not add a blank strip above the message. Keep the existing status announcement and query bindings.

## Host card status dot, 2026-10-20

- Card v0.3.3 offers a non-interactive TitlePrefix but no title action/suffix slot. An omitted Title still emits an empty heading. Compose the host widget with Panel's public Body, BodyClass, and RootAttrs instead, supplying article semantics and one app-owned heading with a focusable Button and portaled Tooltip. Badge's public RootClass supplies the static dot shape while Tone keeps existing fresh/partial/stale/unavailable colors. NotificationDot is fixed red and AnimatingDot adds unrequested motion, so neither matches this status indicator.

## Host card navigation, 2026-10-20

- Compose Panel's body with Link in the host heading. Reuse the homepage's stretched-link CSS pattern to make the whole card navigate, while keeping Tooltip and refresh Button as independent controls above the link. Panel has no whole-surface navigation option; no custom click handler is needed.

## Dashboard swaps and masonry settlement, 2026-10-21

- Full-grid SSE morphs remove client-owned row spans and the masonry class, temporarily collapsing the grid and clamping the shell scroll position. Send widget-level OOB morphs for value changes. Reconcile sections only on connection or membership/order/source changes, and preserve masonry attributes with HTMX 4's `htmx_before_morph_attr` extension hook. Re-measure after settlement as well as resize. Use layout-pixel heights rather than transformed visual bounds. Restore refresh focus with `preventScroll`, without stealing focus from another control.

## Alert filters and combobox keyboard focus, 2026-10-25

- Combobox v0.3.3 and v0.4.2 rendered enabled options without a focusable tabindex. Arrow navigation could not move focus to those options. [Upstream PR 343](https://github.com/araihu/goshtoso/pull/343) fixes option focus, disabled-option skipping, clear in client/server modes and delayed callbacks that could steal focus. The Alertmanager plugin uses the native component. Visual profile v1alpha2 pins Goshtoso v0.4.3 across host assets and scaffold; deploy matching plugin packages with that host.
- A plugin fragment must not put native fallback inputs inside `noscript`: browser fragment parsing can create hidden successful controls even with JavaScript enabled. Render native inputs only for the host's `__native=1` iframe request. Keep the normal form stable and swap only results so search drafts, focus and open menus survive filtering.

## Alert filter counts and Escape cancellation

- Alert facets need supporting option counts that stay outside the closed selection label. Goshtoso's public `Option.Trailing` slot in [PR344](https://github.com/araihu/goshtoso/pull/344) covers this without copying option markup or changing selection values.
- The full combobox suite exposed a pending keyboard open that could run after Escape and reopen the menu. Cancel deferred open/focus work on Escape while preserving focus ownership. The host must serve the v0.4.4 runtime together with plugins using the trailing slot; update the scaffold's patch baseline as well.

## Host applications, 2026-10-25

- Compose Toolbar, TextInput, Select, Badge, Table, Link and existing console components. Accordion v0.3.3 only accepts a plain Title plus leading Icon; it cannot express a rich application summary with separate readiness/location information. Use app-owned semantic native details/summary for these domain disclosures, with stable IDs and keyboard behavior. No component-internal CSS overrides.
- ConsoleShell LocalRuntime loads scripts in order and does not expose head.Dependencies' optional goshtosoDependencies promise. Register the disclosure morph extension immediately when HTMX exists; a promise-only registration silently leaves disclosures unprotected. Browser validation reproduced the closure and verified preservation after the correction.
- Select v0.3.3 does not emit bubbling native input/change when a user chooses an option. Use its public AlpineConfig.Model bridge and a watcher in the stable application filter region to apply filtering immediately. Keep controls outside streamed results.
- PageHeader's public HideSeparator keeps 24px bottom padding. On host detail this stacked with the application's 16px section gap. Use its public RootClass to let the host stack own that separation. Conditional application notices share the same 16px grid gap; address and catalog-time metadata form an 8px group.
- Native summary markers placed inside the line wrap above rich headings on narrow screens. Keep the native marker outside an application-owned 16px gutter so long headings wrap without creating an empty marker line.
- Table v0.3.3 exposes no responsive cell-label or stacked-row contract. Reuse it on desktop; compose a semantic read-only workload list with definition lists below 768px. CSS exposes one representation at a time, and both render from the same workload evidence in each streamed application row.
- Tabs v0.3.3 has RootClass but no tab-list or button class hooks. Scope responsive rules through the public host RootClass and semantic tab roles: compact padding, 44px targets and wrapping keep all host labels visible at 320px and with larger text. No library markup copied or selection logic replaced.


## Themes technical contract, 2026-10-26

- The public `themes.BuiltIn()` catalog exports keys and labels, while the editable definitions, preview composition, and runtime remain under the demo site's `internal` packages. Balemoh needs complete saved snapshots that survive preset updates. The spec makes typed definitions, literal materialization, token metadata, and public editor/playground components part of the upstream extraction; copying the site into the consumer would leave a second catalog and runtime to maintain.
- ConsoleShell's `Page.Head` is a public composition hook, but it renders before shell/theme stylesheets. Source inspection was needed to choose a fixed, more specific custom-theme root scope and avoid an unnecessary new head slot. Document cascade and first-paint expectations for custom-theme consumers.
- The inspected ConsoleShell bootstrap reads System color mode once; its runtime does not follow later OS changes. The saved global mode must stay independent of old browser preference keys, and the upstream runtime needs System-mode tracking without a new preference store. Root and UI currently select different Goshtoso patch versions, so the consumer delivery must pin one compatible published theme contract across both modules.

## Themes controls, 2026-10-27

- Native JavaScript confirmations blocked the in-app browser during conflict recovery. Use the public `modal.Dialog` with Cancel, Escape, focus return, and an asynchronous application decision. Apply the same pattern to draft reset and preset changes in the public editor.
- Font options need their actual typography. Use Goshtoso Combobox, add its public `Option.LabelAttrs` hook, and support controlled `combobox:load` state/catalog updates. Static search must filter locally when no OptionsEndpoint is configured, while configured server search retains its existing behavior.
- Font-catalog refresh must preserve the draft, including incomplete color input. Google previews require private family aliases so browsing options cannot change the application's system-font stacks.

- Templ parses a `<style>` body as CSS. Nesting `@templ.Raw` inside it emitted that expression literally. Emit a complete style element containing only the already validated generated CSS, and test the rendered CSS bytes.
- `golangci-lint` v2.12.2 uses a parser that rejects Go 1.27 export-data version 5. Rebuilding the same release with Go 1.27 fixes its compiler-version check but not the parser. Record lint as unavailable; run `go vet` and the unit/source/E2E suites without claiming lint passed.

- Closing checkpoint asked which findings could become upstream issues or skill improvements. Candidates: static Combobox search making unintended HTTP requests; controlled `combobox:load` plus per-option typography; skill examples for those published contracts, Goshtoso Dialog confirmations, and safe generated-style emission. Changes and tests exist locally. Search existing issues before opening a receipt; do not teach unpublished APIs as available in the installed release.

- Font options with long names/identities exposed Combobox's intrinsic popup width: `min-w-full` could let the list grow past its trigger. Use component-owned `w-full`, retain full labels in `title`, and assert open-popup bounds in 390/1440 px matrix tests. This is another upstream candidate, corrected in the same local Combobox change.

## Theme options and icon inspection, 2026-10-27

- Select lacked per-option leading content and label typography. Its Shell slot supports a custom picker, but that would duplicate selection/keyboard behavior in Balemoh. Add public `Option.Leading` and `Option.LabelAttrs`, keep Select's submitted value and keyboard handlers, and expose `themeeditor.Config.PresetOptions()` for snapshot colors/title faces. Rich labels and swatches follow the selected value as well.
- Preset title stacks name fonts that may not be installed locally. For isolated option previews, resolve a matching curated Google face into a private alias and load only visible options. Preserve the saved system-stack identity and the surrounding page typography.
- Replace the public editor's inspection Select with a Goshtoso icon button, accessible action label and pressed state. The global System/Light/Dark preference remains independent until Save.

## Real homepage preview and English copy, 2026-10-27

- The example cards did not match Balemoh. Compose the actual `ServiceCard`, `HostWidget` and `DashboardGrid` using deterministic fixture data, and load `/ui/balemoh.css` through Frame's public Head slot. Keep navigation and refresh local with capture handlers and disallow form navigation in the frame response. No live discovery/SSE is mounted.
- Real cards may not use the selected title face. Checking `document.fonts.check` before an unused uploaded face has loaded produced a false unavailable warning. Load referenced asset faces explicitly before reporting failure, with sequence protection and a regression test.
- All Themes labels, confirmations, recovery notices, upload feedback and errors now use English. Stored display names remain user data.

## Global mode radio segments, 2026-10-27

- Discovery found `radio.RadioBar()` with `radio.Config{Segmented:true}` already implements connected single-choice segments with native keyboard behavior. `RadioGroup` is a vertical bordered list and Toggle is binary, so neither matches System/Light/Dark.
- Radio lacked a label-leading slot for icons. Add public `Config.LabelLeading templ.Component` across Radio variants, with an icon example and native arrow-key regression coverage. Balemoh composes window/sun/moon icons with English labels in a fieldset and reads the native `colorMode` RadioNodeList value. No custom radio-selection runtime is needed.

## Palette in color Select shells, 2026-10-27

- The existing public `palette.Palette` is documented for Select's Shell mode and already emits `select-close`. Compose it directly in ThemeEditor instead of maintaining bare color text fields as the primary controls. Keep the exact-value field in the popup for hex/sRGB/OKLCH and alpha.
- Named swatches must be materialized immediately into literal Color values; saving `var(--color-blue-700)` would break independent snapshots. Per-role Reset restores the loaded baseline. Lazy palette grids limit initial DOM size, and editor-owned trigger hooks restore focus after a pick or Escape. No new Palette API was needed.
- Opening Color roles exposed an implicit grid track growing to the intrinsic width of long color values at 390 px. Use an explicit `grid-cols-1` below the two-column breakpoint; browser tests assert page and popup bounds with the palette open.
- Color edits appeared ineffective in dark preview because light-only `surface-alt`, `on-surface-strong` and `outline-strong` controls remained visible. Their counterpart names insert `-dark` before the qualifier, rather than appending it. Normalize the actual dark token names when filtering roles. The old palette test checked only serialized editor state; extend it to the frame's computed primary color and assert rendered body background changes in both modes.

## Theme selection without editing, 2026-10-27

- ThemeEditor.Config.PresetOptions() rich title previews previously required a
  mounted editor to load fonts. A read-only theme picker should not mount editing
  controls. Options now carry their own font-preview metadata; the shared library
  runtime observes standalone options and loads the same private aliases lazily.
  Select's saved-theme example and a browser regression exercise the picker with
  no ThemeEditor present. No Balemoh font loader or hidden editor was added.

- The separated editor exposed intrinsic one-column grid tracks at 390 px with
  a long uploaded-font label. The grid tracks, not the Combobox trigger, caused
  overflow. Use explicit minmax(0,1fr) tracks in Balemoh content and mobile theme
  grids. Public E2E covers a long controlled font label; a local consumer matrix
  checked Browse/Create/Edit at 390/1440 px, Goshtoso/Minimal and light/dark, with
  selectors open. Consumer fixture and receipt: data/themes-layout-check.go.txt and data/themes-layout-check.log.

## Theme branding, 2026-10-27

- FileInput AppearanceUpload covers the branding upload control. Existing icon
  validation/storage covers PNG/JPEG/WebP/passive SVG. Balemoh stores branding
  alongside its theme metadata, outside Goshtoso's typed color/font definition.
- ConsoleShell Brand.Logo/FaviconURL and Footer slots cover the published brand
  and fixed product attribution without a new shell API. The isolated homepage
  preview receives an app-owned, source/origin/instance/sequence-checked branding
  message containing only an uploaded image ID. No arbitrary preview URLs.
- Browser checks must compare equivalent URL paths: Go PathEscape preserves a
  colon in an image ID, while JavaScript encodeURIComponent encodes it. Decode
  paths for equality rather than requiring identical percent-encoding.

- FileInput's compact upload label listens for native change events. After clearing
  a file input imperatively, dispatch a bubbling change so the component clears
  its filename too. Keep Save disabled until upload finishes or the user explicitly
  restores default branding; disable base-theme replacement during upload.

## Theme picker actions, 2026-10-27

- Public ActionGroup already covers primary/secondary actions and responsive
  overflow. Its link rendering currently uses the same button appearance for
  both priorities. Compose native primary Edit/Create links with the supported
  secondary OnClick action for independent-copy navigation, preserving the
  intended emphasis without a consumer styling fork. Upstream candidate: make
  secondary link appearance match secondary button appearance.
- View/runtime tests cover custom/built-in action switching, copy URLs and Apply
  as the final form element. A read-only consumer browser check verifies no
  horizontal overflow at 390/1440 px and Apply below the preview. Receipt:
  data/theme-actions-layout-check.log. No saved settings were changed.

## Complete interface theme preview, 2026-10-27

- The dashboard-only frame hid most theme roles and interaction states. Compose
  public PageHeader, Panel, Button tones/disabled, Dropdown sections/danger,
  Badge, Tabs, TextInput states, Select, Checkbox, Toggle, RadioBar, Alert and
  Dialog in the existing ThemePlayground Frame.Content slot. Retain the real
  service/host cards in Overview. No new component API or copied demo markup.
- Tabs, menus, fields and modal use their existing local Alpine behavior. Keep
  SyncHash off and HTMX disabled inside the frame, with the existing navigation
  and form-submit guard. Samples never save settings or call discovery.

## Required custom theme name, 2026-10-27

- TextInput.Config.Required enables native validation but adds no visible required
  marker. Compose its existing HelperText with Required and a non-whitespace
  Pattern. Browser validity checks reject empty/spaces-only names and accept a
  normal name. API ValidName already rejects empty/whitespace-only values.

## Logo upload error feedback, 2026-10-27

- ToastContainer already exists in PageContent; use its public notify event with
  danger tone for local validation and upload rejection. Keep the inline message
  beside the field after Toast expires, and associate it through InputAttrs
  aria-describedby. Preserve the current logo and pending-file Save guard.
- FileInput.Config lacked the validation state exposed by other form controls.
  Add public StateDefault/StateError. StateError emits aria-invalid; both compact
  and dropzone controls style border, label and helper from that native attribute.
  Client updates can set/remove aria-invalid without replacing the component or
  reaching into its DOM. Public examples, generated references, render tests and
  light/dark Goshtoso/Minimal browser checks cover the contract.
- Clear the consumer error on selection, explicit default restoration, retry and
  upload success. Runtime tests cover missing/oversized/server-rejected inputs,
  Toast payloads, state cleanup and preserved Save guards. Consumer browser tests
  use a mocked rejection at 390/1440 px, without writing assets or preferences.

## Preview mode affordance, 2026-10-27

- An icon-only button beside Inspect colors did not make the interaction clear.
  Reuse the public RadioBar and segmented Radio with LabelLeading sun/moon icons,
  explicit Light/Dark labels and a Preview mode fieldset legend. No new API.
- Synchronize native checked state for user input and themeeditor:load; remove the
  hidden-mode proxy and old toggle handlers. Pointer and arrow-key E2E checks
  preserve the definition/host appearance and verify color-role/frame updates.

## Visual contrast advisory, 2026-10-27

- The site's older checker is demo-owned markup/runtime, while public ThemeEditor
  had collapsed contrast into one text line. Inspect the live /docs/theme checker
  and original theme page; compose its Aa/inverse, ratio and grade pattern with
  public Panel/Badge inside ThemeEditor instead of copying the demo table or
  introducing a second color picker. This reaches the site and Balemoh together.
- Three pairs track the selected mode: Surface, Primary and Secondary. Use the
  site's AAA/AA/AA large/Fail thresholds. The existing ratio function ignores
  alpha, so translucent pairs are explicitly Not rated rather than showing a
  passing accessibility grade. Literal samples still show the selected colors.
- Browser coverage checks all five ratings at 390/1440 px in light/dark, 21:1
  black/white, inverse computed colors, one active badge and bounds. Existing
  palette-to-frame and mode/keyboard tests also pass.

## CSS theme import, 2026-10-27

- Public ThemeEditor exported CSS but exposed no import control/parser. Compose
  FileInput AppearanceUpload, Button and the existing draft confirmation Dialog.
  Use an unattached CSSStyleSheet to parse declarations, then materialize a
  complete typed Definition; never append uploaded CSS, selectors or font URLs.
- A complete single-theme Goshtoso export is accepted up to 1 MiB, with literal
  hex/sRGB/OKLCH roles, rem radius and known font stacks. Same-file var aliases
  resolve with cycle bounds. Multiple named themes, conditionals, missing roles,
  unavailable fonts and invalid values preserve the current draft with field
  error/notify. Import remains an edit and does not replace the reset baseline.
- System and Google fonts can share one CSS family. Add a validated font-reference
  comment to exports for exact source identity; older unannotated exports resolve
  known Google imports or system stacks. CSS never supplies new font assets.
- Public E2E covers all 16 preset imports, browser export/import with System,
  Google and bundled fonts, Cancel/Confirm/Reset, invalid files, and detached
  parsing with untrusted import/font/image URLs. Balemoh checks at 390/1440 px
  preserve name/logo/global mode and verify the imported Save payload through a
  mocked conflict. No preferences or font/image assets were written by QA.

## Theme origin badges, 2026-10-27

- Prefixing saved-theme labels with Built-in/Custom displaced the actual name.
  Select.Option had leading content and label typography but no trailing slot.
  Add public Option.Trailing for non-interactive right-aligned metadata in both
  list items and the selected trigger, including trailing-only rich options.
- Balemoh composes soft Badge labels through that slot and retains raw stored
  names, swatches and title faces. Selection and keyboard behavior remain owned
  by Select. Public render/browser coverage and consumer Browse/Create checks at
  390/1440 px verify spacing, bounds, labels and selected badge changes.

## Theme Form composition audit, 2026-10-27

- Form/FieldGroup already cover the main wrapper, sections, required marker,
  typed controls and accessible server-rendered errors. The theme form had used
  standalone controls and manual field presentation; use those public wrappers.
  A compiled probe confirms compatibility with the real ThemeEditor and current
  Save Button without duplicate IDs or nested forms.
- Three optional API gaps remain for deeper integration: Footer submit attrs/slot,
  in-place client field/summary errors that preserve FileList and focus, and
  structured ThemeEditor errors/focus targets. JSON/CAS/replay stays application
  behavior rather than a missing general form feature. Detailed source findings
  and checks: docs/themes-form-audit.md. This audit does not change the UI.

## Theme Form adoption, 2026-10-27

- Adopt Form and typed FieldGroup for base Select, required name and logo without
  expanding the library API. Preserve actual input IDs; use distinct wrapper IDs.
  Keep Select/FileInput-owned labels for their existing behavior and presentation.
  FieldGroup owns the name's required marker, replacing the manual Required hint.
- Form's children slot preserves the existing public Save Button and JSON adapter.
  Set PreventEnterSubmit explicitly false to retain native Enter behavior. Leave
  upload error messages in place so the selected file, focus and Toast survive.
  Existing editor sections/layout need no new wrapper or custom styling.
- The user requires alignment before any future upstream Form API expansion.
  Footer submit customization and dynamic error APIs remain design opportunities,
  not changes implemented here. All consumer regressions passed with mocked
  writes, including required/whitespace names, Enter, replay/conflict, import,
  logo errors and the 24-case responsive matrix.

## Immediate logo upload, 2026-10-27

- Reuse FileInput's native change event; remove the consumer's Upload logo button.
  Existing field error and Toast remain. Clear the chooser after failure using a
  bubbling native change so its filename resets and the same file can retry.
  Keep failed-upload Save blocking until a successful replacement or explicit reset.
- Existing FileInput/Form API supports this flow; no upstream expansion needed.

## Form section presentation, 2026-10-27

- User requested stronger section separation and full form/preview columns.
  Public form.Section already provides surface cards, title divider and Columns.
  Reuse it with Columns "1" for details, branding and appearance. Move the preview
  to the sibling column without changing editor hooks or form ownership.
- Earlier Form migration adopted FieldGroup but missed the existing section
  presentation. This was consumer composition friction; no new upstream API.

## Independent site logo and icon, 2026-10-27

- Compose two existing Form.FieldGroup/FileInput controls in Site identity.
  Each keeps immediate upload, accessible field error, Toast and explicit reset.
  No Goshtoso Form extension or custom upload control was needed. Independent
  persistence and draft-only application remain Balemoh responsibilities.
- Browser round-trip QA must wait for the new Themes document after Save, not
  just its URL: history.replaceState precedes the intentional reload. Waiting
  for data-theme-selection avoids evaluating the outgoing editor document.

## Automatic theme CSS import, 2026-10-27

- Remove the extra Import CSS action from public ThemeEditor; reuse FileInput's
  native change event. Keep validation, modal confirmation and error feedback.
  Clear the file selection after success, cancellation and failure to support
  retrying the same file. No public config/API expansion was needed.

## Radius slider on Form.Section, 2026-10-27

- Range used a surface-alt track, which disappeared on Form.Section's surface-alt
  card. Fix the shared public Range default with outline-strong/dark equivalents,
  preserving the same native input and API.
- ThemeEditor listened only to change, leaving the preview frozen during drag.
  Reuse its validated handler for radius input events. Native pointer regression
  checks draft and iframe state before mouse-up; synthetic change tests alone
  had missed this. No upstream API expansion required.

## Preview sticky layout, 2026-10-27

- The consumer used lg:sticky/lg:top-6 classes absent from Goshtoso's prebuilt CSS.
  Author the responsive layout in Balemoh's existing stylesheet, with a named
  preview-column class. Public ThemePlayground composition stays unchanged.
- Verify actual scrolling inside ConsoleShell's main container. Sticky offsets
  include the shell's scroll-area padding; viewport-height checks must include
  the preview heading and spacing. No library API expansion needed.

## Appearance Fonts tab, 2026-10-27

- Reuse public Tabs for Fonts and Form/Section/FieldGroup/FileInput for the moved
  upload form. Existing Dialog handles deletion confirmation. No component API
  extension was required. Keep editor font selectors and font-management lifecycle
  separate; refresh the Fonts list after confirmed writes.

## Theme header deletion action, 2026-10-27

- Public PageHeader.Actions accepts ActionGroup directly, so the saved-theme
  deletion action moves beside the title without custom button markup or a new
  component API. Keep the existing Dialog for confirmation. Remove the duplicate
  theme management forms and their generic runtime handler.

## Branding reset icon and tooltip, 2026-10-27

- Reuse Button.AppearancePlain, ToneAlternate and Icon16SolidXMark inside public
  Tooltip.WithTrigger/WithPortal. Keep aria-label/aria-describedby and existing
  reset selectors; no new component API needed.
- Portal tooltips inherited transition-all while the runtime set fixed top/left.
  After focus/inner scrolling they animated across the viewport, briefly showing
  away from the X. Fix assets/js/src/components/tooltip.js
  goshtosoInitTooltipPortal with opacity-only transitions. Dedicated public
  composition regression: site/tests/e2e/tooltip_portal_scroll_test.go.

## Custom-only reset visibility, 2026-10-27

- Keep public Tooltip/Button composition inside a native hidden wrapper driven
  strictly by the custom image reference, including server-rendered initial state.
  Return focus to FileInput when reset removes its own action. No library API gap.
- Failed uploads intentionally block Save. Hiding the default-image reset must
  retain a recovery path: a separate public plain Discard failed upload button
  clears the failed attempt, preserving any previously selected custom image.

## PR preparation checkpoint, 2026-10-27

- User confirmed no additional Goshtoso friction beyond this journal.
- Include the companion App Shells System-mode fix in its own dependency PR.
- Review and validate immutable dependency pins outside the local go.work before
  declaring the Balemoh PR ready. No merge, tag or deployment is authorized.

## PR review follow-up, 2026-10-27

- Resolved upstream in Goshtoso #352: editing a preset could overwrite unrelated
  derived colors, browser provenance limits differed from Go, and the catalog
  digest was stale after go fix. Regression checks cover every preset, UTF-8
  boundaries and generator drift.
- Public component demos now follow the site's documentation pattern. Shared
  browser helpers live in support files; docs navigation waits for the lazy
  preview table before leaving.
- The authoritative Goshtoso CI templ pin is 0.3.1020 even though go.mod uses
  runtime 0.3.1070. Regenerated with the CI pin. Balemoh uses its own go-tool pin.
- User confirmed no additional friction at the closing checkpoint and authorized
  the companion App Shells PR. No repeated checkpoint was needed.

## Detached contrast advisory, 2026-10-27

- Need: move contrast results out of the theme form into the preview column.
- Inspected `themeeditor.Config`, `contrastAdvisory` and the editor runtime. The
  panel was private and its updates queried only the editor root.
- Upstream composition: public `ContrastAdvisory(id)` plus `Config.ContrastID`
  selects an external target. Inline placement remains the default. Balemoh
  renders the same component under its preview, without copying cards or math.
- Runtime layout checks also caught an unclosed app-owned 480px media query.
  Closing it restores desktop appearance styles, including the sticky preview.
