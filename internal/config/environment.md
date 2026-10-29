# Environment Variables

## Options

Options contains Balemoh's runtime environment configuration.

 - `BALEMOH_WEB_PUSH_VAPID_KEY_FILE` - WebPushVAPIDKeyFile contains one base64url-encoded P-256 private scalar for Web Push. Empty disables push. Keep this file stable across restarts and mount it as a secret.
 - `BALEMOH_WEB_PUSH_SUBJECT` - WebPushSubject is a mailto: or HTTPS contact URI for the VAPID subject. Required with the key file. Web Push also requires the notification operator credential.
 - `BALEMOH_READ_ONLY` (default: `false`) - ReadOnly rejects operator API writes, preserving authenticated federation imports and plugin queries.
 - `BALEMOH_DEMO_ENABLED` (default: `false`) - DemoEnabled seeds and refreshes a bounded synthetic scenario. Requires read-only mode and disables all real discovery and federation configuration.
 - `BALEMOH_DEMO_ALERTMANAGER_ADDR` (default: `127.0.0.1:9093`) - DemoAlertmanagerAddr serves the synthetic Alertmanager source on a loopback address when demo mode is enabled.
 - `BALEMOH_NOTIFICATION_RETENTION_DAYS` (default: `30`) - NotificationRetentionDays permanently expires read and unread notifications after this many 24-hour days from receipt. Range 1 to 3650; cleanup runs at startup and hourly, and reads immediately exclude expired messages.
 - `BALEMOH_PLUGIN_CONFIG` - PluginConfig enables trusted local plugin installations and source connections from a JSON deployment file. Empty disables plugins.
 - `BALEMOH_PLUGIN_SERVICE_TOKEN_FILE` - PluginServiceTokenFile contains the private BFF service credential used only by new plugin API routes.
 - `BALEMOH_NOTIFICATION_OPERATOR_TOKEN_FILE` - NotificationOperatorTokenFile contains the private operator credential used by notification inbox, read-state and clear routes. The trimmed token file value must be at least 32 bytes.
 - `BALEMOH_NOTIFICATION_PUBLISHERS_FILE` - NotificationPublishersFile points to a bounded JSON object mapping external notification source IDs to token-file paths. Each trimmed token file value must be at least 32 bytes. Empty disables external publication.
 - `BALEMOH_TELEMETRY_ENABLED` (default: `false`) - TelemetryEnabled collects host CPU, memory and workload inventory every minute, independently of discovery.
 - `BALEMOH_HOST_PROC_PATH` (default: `/host/proc`) - HostProcPath is a read-only mount of the Docker host proc filesystem. Never use the agent container proc filesystem.
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
