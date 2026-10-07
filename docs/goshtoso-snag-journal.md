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
