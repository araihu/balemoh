# Environment Variables

## Options

Options configures the Balemoh server-rendered UI BFF.

 - `BALEMOH_UI_READ_ONLY` (default: `false`) - ReadOnly requires a loopback synthetic API and serves a shared public demo with operator writes and SSE disabled, a 16-request concurrency budget and a global 20 requests/second rate with a 40-request burst.
 - `BALEMOH_UI_NOTIFICATION_OPERATOR_TOKEN_FILE` - NotificationOperatorTokenFile contains the private notification inbox credential. Empty leaves notification fragments unavailable without blocking base pages.
 - `BALEMOH_UI_THEME` (default: `goshtoso`) - Theme selects the shared visual profile theme, goshtoso or minimal.
 - `BALEMOH_UI_COLOR_SCHEME` (default: `system`) - ColorScheme selects system, light or dark. Explicit themes or modes override stored browser preferences.
 - `BALEMOH_UI_PLUGIN_CONFIG` - PluginConfig enables data-backed trusted plugins from a local JSON file. It cannot be combined with synthetic preview configuration.
 - `BALEMOH_UI_PLUGIN_SERVICE_TOKEN_FILE` - PluginServiceTokenFile contains the private API service credential. It is never passed to a UI child or browser.
 - `BALEMOH_UI_PLUGIN_PREVIEW_CONFIG` - PluginPreviewConfig enables trusted, synthetic P1 UI previews from a local JSON file. Empty disables plugins.
 - `BALEMOH_UI_HTTP_ADDR` (default: `:8081`) - HTTPAddr is the address where the UI BFF listens.
 - `BALEMOH_UI_API_BASE_URL` (default: `http://127.0.0.1:8080`) - APIBaseURL is the absolute URL of the Balemoh API upstream.
 - `BALEMOH_UI_REQUEST_TIMEOUT` (default: `5s`) - RequestTimeout bounds one UI-to-API request.
 - `BALEMOH_UI_ICON_CACHE_DIR` (default: `./data/icons`) - IconCacheDir stores downloaded, verified icon packs across restarts.
 - `BALEMOH_UI_ICON_STARTUP_TIMEOUT` (default: `10m`) - IconStartupTimeout bounds background icon download and cache verification.
 - `BALEMOH_UI_SHUTDOWN_TIMEOUT` (default: `5s`) - ShutdownTimeout bounds graceful UI shutdown.
