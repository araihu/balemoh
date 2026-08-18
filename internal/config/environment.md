# Environment Variables

## Options

Options contains Balemoh's runtime environment configuration.

 - `BALEMOH_HTTP_ADDR` (default: `:8080`) - HTTPAddr is the address used by the HTTP server.
 - `BALEMOH_DATABASE_PATH` (default: `./data/balemoh.db`) - DatabasePath is the path to Balemoh's SQLite database.
 - `BALEMOH_KUBERNETES_ENABLED` (default: `false`) - KubernetesEnabled enables the in-cluster Kubernetes discovery source.
 - `BALEMOH_KUBERNETES_SOURCE_ID` - KubernetesSourceID is the stable identity used to scope Kubernetes candidates.
 - `BALEMOH_KUBERNETES_NAMESPACE` - KubernetesNamespace limits Kubernetes discovery to one namespace.
