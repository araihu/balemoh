# Host applications

Applications combines the former Services and Workloads tabs into a read-only
runtime view. Existing `#services` and `#workloads` bookmarks select it. The
existing console identity and compact rows remain; runtime state comes first,
with exact workload evidence disclosed on expansion.

Rows show catalog names, workload summaries, namespace and nodes. Expansion
shows addresses, catalog observation time, phase, readiness, current reason,
images and cumulative pod restarts. Addresses are navigation links. This view
has no catalog editing or workload mutation controls.

Search matches names, descriptions, namespace, nodes, workload states, reasons
and images. Filters show current applications, groups needing attention, or
applications absent from discovery. Historical entries stay outside the default
view and show their last catalog observation. A node view includes only groups
with workloads actually reported on that node.

## Association and state

- Source kind and ID must match. Kubernetes ownership uses exact pod kind,
  namespace and name references from catalog resource evidence. Similar names
  never establish ownership. Shared pods can belong to several groups; the
  workload total counts the received inventory once.
- Docker matches exact container names. Compose groups use catalog container
  metadata for the same project/service and suppress duplicate container rows.
- Unmatched workloads remain visible, including when the catalog is unavailable.
  Raw pod catalog entries never become application rows. Missing Kubernetes
  evidence never associates a currently reported pod, even when its namespace
  and name match; the catalog entry stays historical on the cluster page and
  does not establish node placement. Only Docker container/Compose entries may
  associate current inventory while discovery marks them absent, because Docker
  discovery lists running containers and telemetry also lists stopped ones.
- Running and Ready are separate observations. Missing readiness is unknown.
  Pending, failed, not-ready and current error reasons need attention. Completed
  pods and cumulative restart counts alone do not indicate current failure.
  Mixed groups report readiness only for active workloads and count completed
  workloads separately. Unknown readiness on a completed pod does not make
  active workload readiness unknown.
- Docker summaries count running containers. Docker readiness, restart counts
  and per-workload CPU/memory are not collected. Pod readiness does not verify
  HTTP reachability. No workload evidence means unknown runtime state.
- Observation timestamps and stale/incomplete inventory warnings bound all
  counts. Reported workload counts are not desired replica counts.

## Compatibility and live updates

Telemetry adds optional `node`, `ready`, `reason` and `images` fields. Older
agents remain valid and show unknown details. Update receiving APIs before
agents: old APIs reject unknown import fields. Existing JSON storage needs no
migration. Collection uses existing pod reads and requires no new permissions.

The host SSE stream includes catalog associations in its digest, including
catalog-only changes. Runtime changes morph individual rows and observation
metadata; membership/order changes reconcile the results collection. Filters,
tabs and search stay outside replacements. A scoped HTMX extension preserves
native disclosure state and registers directly with ConsoleShell's local
runtime. Only the display changes; this does not sync or modify infrastructure.

Tests cover source/namespace isolation, similar names, history, node scoping,
Compose, stopped containers, unknown readiness, current versus previous error
reasons, catalog failure, and scoped stream output. Browser checks use clearly
illustrative fixtures for filtering, disclosures, live updates and mobile
overflow.
