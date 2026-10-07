# Goshtoso integration notes

## Host telemetry, 2026-10-07

- Goshtoso Charts v0.0.3 line chart renders `figure role="img"`. Axe reports `aria-allowed-role`, minor severity. Its accessible name and adjacent exact-value table remain available. No public role override exists. Upstream correction remains pending; no consumer DOM rewrite added.
- Goshtoso v0.3.2 Card title is h3 and its default width is capped. Compose under h2 sections and use the documented RootClass to fill the dashboard column.
- Goshtoso v0.3.2 Table has no container attribute slot for tabindex. A text-only overflowing workload table is not keyboard-scrollable by default. Use an application-owned, named focusable scroll region and the documented RootClass to avoid nested scroll containers. Axe no longer reports that blocker.
- Charts use fixed SVG dimensions. Width 360, larger axis labels and time-only labels keep a single-column mobile chart legible. Exact data remains available at one-minute resolution.

- Charts exact-value tables also lack a focusable scroll-container option. A named, focusable application region owns chart/table scrolling, with inner overflow removed through scoped CSS on native details markup. This keeps expanded 24h data keyboard-accessible without rewriting generated HTML.
