# Collapsible Config navigation

Config is a disclosure group. It starts closed on Home, Hosts and plugin pages,
and opens when Appearance, Notifications or Plugins/Widgets is active. Editor
and detail routes inherit their owning navigation ID. The operator can still
toggle the group manually. No preference storage or application data changes.

## Component integration

Use the existing Sidebar Item `Collapsible` and `Open` fields. The selected
Goshtoso implementation ignored `Collapsible` for top-level icon navigation, so
the fix belongs in `components/sidebar`, preserving icons, child links and
keyboard focus. The upstream patch adds a native disclosure button with Alpine
state, plus a demo and browser test. Balemoh computes Open from its configured
children after inserting plugin management.

Pin Goshtoso v0.4.7, including the fix from
[Goshtoso PR #357](https://github.com/araihu/goshtoso/pull/357).
No Goshtoso workspace replacement is required. Goshtoso requires templ
v0.3.1100; align the UI modules with that runtime and
regenerate consumer templates. Three native checkbox change handlers now use
typed templ.JSFuncCall values, as required by the newer renderer's sanitization.

The isolated published-consumer fixture passed against v0.4.7 with GOWORK=off.
Root and UI module verification resolved that tag without a Goshtoso replacement.

## Validation

Library unit tests cover open/closed rendering, disabled state, icons, badges
and child links. The focused site E2E checks Enter and Space interaction.
Consumer tests cover each active Config child, unrelated pages and checkbox
submission handlers. Browser checks exercise Home, Settings and full-page
navigation with the local module patch.

Passed the library unit suite, the focused sidebar E2E, site current-source
integration across 93 non-E2E packages, and the published-consumer baseline.
Balemoh API and UI standalone tests and the UI build passed with GOWORK=off.
UI vet also passed during local integration. Captures
cover 390 and 1440 px in Goshtoso and Minimal, light and dark. The Config group
stays within the viewport, exposes its expanded state and toggles with Enter and
Space. No Alpine errors appeared in the local checks. The Goshtoso PR and main
CI also passed the complete E2E suite. Native iPhone testing was not run.
