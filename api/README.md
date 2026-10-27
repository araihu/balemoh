# API contract and documentation

`openapi.yaml` defines the HTTP contract used by the API server, generated Go
client, and static Manja reference. The published documentation is at
[balemoh.araihu.com/openapi/](https://balemoh.araihu.com/openapi/).

The documented local server is `http://127.0.0.1:8080`. Replace it with your API
origin for integrations. The public static documentation host is not an API
gateway. Non-federation endpoints rely on your network or reverse-proxy access
boundary; the two federation import endpoints require a source-bound Bearer token.

## Validate and regenerate

```sh
bash hack/lint-openapi.sh
GOWORK=off go generate ./internal/api
(cd client && GOWORK=off go generate)
GOWORK=off go test ./...
(cd client && GOWORK=off go test ./...)
(cd ui && GOWORK=off go test ./...)
bash hack/build-site.sh
```

The lint helper installs Vacuum v0.30.0 into ignored `bin/`. It runs the default
recommended OpenAPI rules plus required operation summaries and descriptions from
`vacuum.yaml`. Warnings or a quality score below 100/100 fail the command.
The static build runs the same lint
before exporting Margo and Manja, and saves its report under ignored
`site/.build/vacuum-report.txt`. The site workflow performs this check on pull
requests and before publishing main.

Keep examples synthetic and consistent with their operation. Manual creation
returns an unpinned `manual/local` service; unpin returns `pinned: false`. Reuse a
manual creation requestId across retries. Icon updates and deletion need the
current digest. Federation examples use fixed timestamps for readability;
replace them with current UTC when exercising ingestion. History examples may
be explicitly abbreviated, while actual 1h and 24h responses have 60 and 1440 slots.

Describe actual response formats. Catalog failures commonly use `ErrorResponse`
JSON, while validation, host lookup, widget selection, and uploaded-icon errors
can be plain text. Binary icon downloads use the stored image Content-Type and
can return 304 for a matching If-None-Match header. A 204 or 304 has no body.

For documentation changes, retain existing operation IDs, field names, required
fields, and validation limits. Regenerate the server and client because the
descriptions become Go comments and documented response variants become typed
client response fields. Do not hand-edit generated Go files.

The nullable response image-reference arrays reflect the existing API's `null`
when no images are stored. Their `x-go-type-skip-optional-pointer` extension keeps
the existing generated `[]string` field types for Go consumers.
