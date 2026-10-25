# Goshtoso integration notes

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
