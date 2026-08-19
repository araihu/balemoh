# Environment Variables

## Options

Options configures the Balemoh server-rendered UI BFF.

 - `BALEMOH_UI_HTTP_ADDR` (default: `:8081`) - HTTPAddr is the address where the UI BFF listens.
 - `BALEMOH_UI_API_BASE_URL` (default: `http://127.0.0.1:8080`) - APIBaseURL is the absolute URL of the Balemoh API upstream.
 - `BALEMOH_UI_REQUEST_TIMEOUT` (default: `5s`) - RequestTimeout bounds one UI-to-API request.
 - `BALEMOH_UI_SHUTDOWN_TIMEOUT` (default: `5s`) - ShutdownTimeout bounds graceful UI shutdown.
