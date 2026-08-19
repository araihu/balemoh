# Balemoh Dashboard Feature Analysis Handoff

> **Slice type:** research and product-scope analysis only. Do not implement
> features in this slice.

## Objective

Analyze the current feature sets of Homarr and Homepage (`gethomepage/homepage`)
and use the comparison to define Balemoh's next product slices. The goal is to
learn from both projects without turning Balemoh into a copy of either one.

The output must answer:

1. Which capabilities are already present in each reference project?
2. Which capabilities fit Balemoh's discovery → staging → pin → homepage model?
3. Which capabilities should Balemoh implement next, defer, or explicitly reject?
4. What are the smallest coherent follow-up slices, in priority order?

## Current Balemoh context

Read these first:

- `README.md`
- `docs/roadmap.md`
- `docs/superpowers/specs/2026-08-17-balemoh-discovery-design.md`
- `ui/internal/view/pages.templ`
- `ui/internal/view/model.go`

The current product boundary is:

- Kubernetes and Docker/Podman adapters discover read-only evidence.
- Kubernetes discovery resolves `HTTPRoute`/Ingress → Service → Pod and
  retains host/path, images, and source metadata.
- Container discovery retains running containers, Compose services, images,
  and published host-port observations; it does not infer exact hostnames.
- Multiple Balemoh instances can publish snapshots to a gateway for a unified,
  mixed-environment staging view.
- Discovered resources remain candidates until the user pins them.
- Pinned services render as homepage cards; cards with an observed HTTP(S)
  endpoint open in a new tab.
- The API and generated client are authoritative boundaries; the Goshtoso UI
  is a separate BFF module.

Do not weaken the existing principles: discovery must not publish automatically,
adapters are read-only by default, evidence must be distinguished from
inference, and user pin state must survive discovery reconciliation.

## Research scope

Inspect the current official documentation, source repository, examples, and
release notes for both projects. Confirm the exact project identity and record
the revision/version and access date used for every material conclusion.

Use primary sources wherever possible. A feature counts as available only when
there is evidence that it is implemented or officially documented; label
roadmap ideas, community integrations, themes, and assumptions separately.

Compare at least these areas:

| Area | Questions to answer |
| --- | --- |
| Discovery and integrations | Docker/Compose, Kubernetes, labels/annotations, remote hosts, multi-cluster or multi-instance support, refresh behavior |
| Endpoint and metadata evidence | Hostname/path discovery, port handling, health checks, images, icons, descriptions, status, provenance, stale resources |
| Homepage composition | Cards, groups, layouts, drag/drop, ordering, responsive behavior, tabs, bookmarks, search, filters, custom fields |
| User decisions | Pin/favorite semantics, staging or review workflow, hide/archive behavior, edits and overrides, persistence |
| Widgets and dashboards | Built-in widgets, service-specific widgets, metrics, status, links, dashboards beyond a service catalog |
| Configuration and storage | File/config model versus UI/database model, reload behavior, secrets, backups, API surface, import/export |
| Federation and operations | Gateway/agent model, remote sources, auth, RBAC, tenancy, auditability, observability, failure handling |
| Extensibility and deployment | Plugins, integrations, themes, custom components, Docker/Kubernetes deployment, resource cost, upgrade complexity |
| Security and safety | Network calls, credential handling, write operations against discovered platforms, URL allowlists, SSRF or trust boundaries |

For each row, capture what Homarr and Homepage offer, how it works, and what
Balemoh would have to add or change. Explicitly distinguish:

- built-in versus external integration;
- discovery evidence versus manually authored configuration;
- local-only versus remote/multi-host behavior;
- UI capability versus API/domain capability;
- read-only observation versus an operation that mutates an external system.

## Required deliverables

Create an analysis document under `docs/` with:

1. A source register containing URLs, project versions/commits, access date,
   and a one-line statement of what each source proves.
2. A feature matrix with columns for Homarr, Homepage, current Balemoh,
   evidence links, implementation complexity, and recommendation.
3. A gap analysis focused on the next homepage experience, not a generic
   dashboard wishlist.
4. A prioritized proposal for the next three to five Balemoh slices. Each
   slice needs a user outcome, API/domain impact, UI impact, migration or
   compatibility concerns, dependencies, and an explicit non-goal.
5. A short “adopt / adapt / defer / reject” decision list. Explain why a
   capability belongs in Balemoh, especially when it conflicts with the
   discovery-and-staging product identity.
6. Open questions and decisions that require product input, kept separate from
   conclusions supported by evidence.

Use P0/P1/P2 priority labels and estimate effort qualitatively as S/M/L. Do
not let the analysis silently expand into authentication, arbitrary widgets,
full monitoring, an extension marketplace, or a general-purpose dashboard.

## Suggested slice candidates to evaluate

These are hypotheses to validate, not predetermined decisions:

- homepage card customization and grouping while preserving generated
  endpoint/source evidence;
- source, namespace, status, and endpoint-confidence filters in staging;
- stale-candidate status and reconciliation controls;
- lightweight health/status observations that remain read-only;
- user-owned labels, ordering, and display overrides without mutating platform
  resources;
- richer federation/source health and gateway operations;
- optional, explicitly scoped integrations for exact external hostnames.

The final analysis may replace these with better slices or reject them.

## Acceptance criteria

- Every material upstream feature claim has a primary-source link.
- The report records the versions/revisions examined; no “latest” claim is
  left unqualified.
- Current Balemoh behavior is verified against the repository, not inferred
  from the product description alone.
- Recommendations preserve read-only discovery, explicit pin decisions, and
  source/provenance visibility.
- The proposed slices are independently shippable and ordered by user value
  and dependency, with no hidden implementation work.
- This handoff slice changes documentation only; it does not add dependencies,
  schema changes, API routes, UI behavior, deployments, or external writes.

## Completion handoff

Return the analysis with a concise executive recommendation first, followed by
the evidence matrix and the proposed slice queue. Flag any uncertainty that
could materially change scope. Do not begin implementation until the product
scope is explicitly selected from the analysis.
