# Changelog

## v0.0.1-alpha.1

First MIT-licensed release of `github.com/araihu/balemoh/plugin-sdk`.

- Closed manifest/schema validation and immutable package digests.
- Separate UI/backend processes through HashiCorp go-plugin and gRPC.
- Scoped queries, notification publication, routes, DOM IDs and rendering.
- Author CLI with versioned scaffolds, packaging, health checks and fixture preview.

Protocol remains `balemoh.plugins/v1alpha1`; UI profile remains
`balemoh.goshtoso/v1alpha2`. This release retains the existing host contracts.
Install only trusted plugins. Process isolation does not sandbox filesystem,
network access or plugin HTML. Alpha APIs may change in later releases.
