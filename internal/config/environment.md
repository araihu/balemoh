# Environment Variables

## Options

Options contains Balemoh's runtime environment configuration.

 - `BALEMOH_HTTP_ADDR` (default: `:8080`) - HTTPAddr is the address used by the HTTP server.
 - `BALEMOH_DATABASE_PATH` (default: `./data/balemoh.db`) - DatabasePath is the path to Balemoh's SQLite database.
 - `BALEMOH_DISCOVERY_SYNC_INTERVAL` (default: `5m`) - DiscoverySyncInterval controls the initial and periodic discovery sync. Zero disables background synchronization.
 - `BALEMOH_KUBERNETES_ENABLED` (default: `false`) - KubernetesEnabled enables the in-cluster Kubernetes discovery source.
 - `BALEMOH_KUBERNETES_SOURCE_ID` - KubernetesSourceID is the stable identity used to scope Kubernetes candidates.
 - `BALEMOH_KUBERNETES_NAMESPACE` - KubernetesNamespace limits Kubernetes discovery to one namespace; empty discovers all namespaces.
 - `BALEMOH_CONTAINER_ENABLED` (default: `false`) - ContainerEnabled enables the Docker-compatible Docker or Podman container discovery source.
 - `BALEMOH_CONTAINER_SOURCE_ID` - ContainerSourceID is the stable identity used to scope container candidates.
 - `BALEMOH_CONTAINER_HOST` (default: `unix:///var/run/docker.sock`) - ContainerHost is the Docker-compatible API socket or endpoint.
 - `BALEMOH_FEDERATION_GATEWAY_URL` - FederationGatewayURL is the remote Balemoh gateway receiving local snapshots.
 - `BALEMOH_FEDERATION_TOKEN` - FederationToken authenticates this instance when publishing snapshots.
 - `BALEMOH_FEDERATION_ALLOW_INSECURE_HTTP` (default: `false`) - FederationAllowInsecureHTTP permits an explicitly configured loopback HTTP gateway for local development.
 - `BALEMOH_FEDERATION_ALLOWED_SOURCES` - FederationAllowedSources registers source identities accepted by this gateway, formatted as kind/id entries.
 - `BALEMOH_FEDERATION_SOURCE_TOKENS` - FederationSourceTokens maps registered source identities to Bearer credentials, formatted as kind/id=token entries.
