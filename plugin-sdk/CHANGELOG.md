# Changelog

## v0.0.1-alpha.2

- Optional `ui.ServiceCatalog` callback and additive `Host.ResolveServices` RPC.
- Bounded service ID batches and local icon presentation, without catalog endpoints or credentials.
- Existing `ui.HostServices` implementations remain source compatible. Older hosts can omit this optional capability.


## v0.0.1-alpha.1

First MIT-licensed release of `github.com/araihu/balemoh/plugin-sdk`.

- Closed manifest/schema validation and immutable package digests.
- Separate UI/backend processes through HashiCorp go-plugin and gRPC.
- Scoped queries, notification publication, routes, DOM IDs and rendering.
- Author CLI with versioned scaffolds, packaging, health checks and fixture preview.
- gRPC v1.83.2 fixes GO-2026-6443 on the v1.83 release branch.
- x/net v0.60.0 includes the HTTP/2 fixes for GO-2026-6603, GO-2026-6611,
  GO-2026-6612 and GO-2026-6617.

Protocol remains `balemoh.plugins/v1alpha1`; UI profile remains
`balemoh.goshtoso/v1alpha2`. This release retains the existing host contracts.
Install only trusted plugins. Process isolation does not sandbox filesystem,
network access or plugin HTML. Alpha APIs may change in later releases.
