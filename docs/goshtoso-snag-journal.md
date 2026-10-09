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
