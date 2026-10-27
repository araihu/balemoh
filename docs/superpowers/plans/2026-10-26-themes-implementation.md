# Themes implementation checkpoints

Specification: [Themes](../specs/2026-10-26-themes-design.md).

## Frozen bases and paths

- Balemoh: 884a668461482108ec40d5e914335201f4fd918e, current themes worktree.
- Goshtoso: 85bdf307779dc0cfb095b17bfa6bb80456004346, dedicated public-theme-tools worktree.
- App Shells: 3f73b62f4ec30a62297f6c8567516cdc812cb639, dedicated console-system-mode worktree.
- Existing spec/journal changes are part of this task. Preserve other worktrees and primary checkout state.

## Checkpoints

- [x] G1: public themes Definition v1, complete preset materialization, validation/rendering and tests.
- [x] G2: public editor/playground runtime and components, site uses public APIs, component/catalog/generated references and tests.
- [x] S1: ConsoleShell System mode tracks OS changes with cleanup and no browser preference persistence.
- [x] B1: SQLite presentation/themes/font stores, immutable references, CAS/save replay and migration tests.
- [x] B2: bounded font container metadata validation and format/limit tests.
- [x] B3: OpenAPI, generated client/server, application commands, HTTP endpoints and tests.
- [x] B4: BFF/UI controls, SDK integration, global first paint, preview/recovery/conflicts and tests.
- [x] V1: local browser journey, variable upload, Save/full reload, restart, stale-tab preservation and modal cancellation verified; server/runtime failure receipts recorded. Extended browser/release coverage remains listed in the acceptance document.
- [ ] D1: standalone published-dependency gates after authorized upstream publication.

## Delivery boundaries

Implementation/testing is authorized. No push, merge, tag, package release or homelab deployment was requested for this feature. Local integration may use an explicit temporary workspace while upstream changes are unpublished; it is not final published-dependency evidence. Finish reviewable local implementation before requesting any required publication action.

## Execution

Single-agent execution. The reviewed multi-agent workflow is not active because this session has not authorized its multi-agent role/worktree protocol. No subagents are dispatched.

## Validation receipts

See [local acceptance](../../themes-acceptance.md) for executed checks, fixture
journeys and explicit coverage limits. Root/client/UI unit suites, Goshtoso root
suite, 93 site packages, focused Themes/Landing E2E, ConsoleShell tests and
`go vet` passed. OpenAPI scored 100/100. Lint is blocked by the installed parser's
unsupported Go 1.27 export format, including after rebuilding the same release.

The closing checkpoint identified upstream candidates for static Combobox search,
controlled option/selection updates with per-option label attributes, and popup
width with long options, plus
skill documentation for the published contracts, Dialog confirmations and templ
style emission. These are in the snag journal. No GitHub issue or skill edit was
requested.
