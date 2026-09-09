# Icon library

Balemoh combines Goshtoso's bundled icons, the complete pinned selfh.st catalog,
and user uploads. `/icons` provides name/tag search, source filters, pagination,
uploads, and uploaded-icon management. Choose an icon in the Staging editor and
save; reset clears the explicit choice and restores the discovery default.

Discovery defaults use exact resource names and a small explicit alias map.
User display-name changes never affect the default. Bundled icons are immutable.
Uploads support PNG, JPEG, WebP, and passive SVG, up to 2 MiB and 4096 pixels.
SVG scripts, CSS, embedded images, and external references are rejected.

Uploads and metadata are stored together in SQLite. Replacement preserves the
icon ID and updates every service referencing it. Revision checks reject stale
edits. Deletion is allowed only when no service references the icon. Library
metadata and service choices survive discovery syncs. The API contract is in
`api/openapi.yaml`, migration 6 adds the stored icon data and service references.

## Pinned collection

The selfh.st source is pinned in `ui/iconpacks/selfhst/.iconpack.yaml`. The
checked-in library under `client/iconassets/selfhst` contains 2,893 catalog
entries plus declared light/dark variants. PNG provides complete coverage.
`NOTICE`, `LICENSES`, and `PROVENANCE` preserve attribution and source locks.
Images have content-hashed filenames and are requested individually. These generated
images remain development artifacts; executables embed only catalog metadata and
licenses. The UI also embeds the source configuration and integrity lock.

Use the Goshtoso version pinned in `ui/go.mod`:

```sh
GOWORK=off go -C ui list -m github.com/araihu/goshtoso
# Substitute that exact version for VERSION.
go run github.com/araihu/goshtoso/cmd/iconpack@VERSION -library \
  -config ui/iconpacks/selfhst/.iconpack.yaml \
  -out client/iconassets/selfhst -check
```

To update, pin and review a new source revision, explicitly establish trust,
and generate into a new directory. Compare catalog identities, image counts,
licenses, and removals before replacing the previous generated directory.
The generator deliberately refuses to overwrite different existing output.

## Runtime storage

In the background after startup the UI calls Goshtoso `iconpack.Generate` to fetch and verify the
pinned selfh.st source when its disk cache is missing or invalid. It does not use
`Trust` at runtime. A valid cache is verified locally and reused without network
access. The API only uses embedded catalog metadata and never downloads icons.

Mount persistent, writable storage at `BALEMOH_UI_ICON_CACHE_DIR`, default
`./data/icons`. The cache is keyed by the source configuration and lock digest;
changing the pin creates a separate cache. `BALEMOH_UI_ICON_STARTUP_TIMEOUT`
defaults to `10m`. The UI starts listening immediately. Icon requests return HTTP 503 with
`Cache-Control: no-store` until the verified library is ready. Download or
validation failures leave the UI running and icons unavailable. Structured logs
report preparation start, completion with elapsed time, failure, or cancellation.
Preparation runs once per process and is cancelled on shutdown; restarting retries
a failed preparation. Icons become available without restarting after success.

The current GitHub source archive is about 307 MiB, although the selected PNG
files total about 142 MiB. First startup needs network access and disk space for
the archive, source cache, and generated library. Later starts need neither a
fresh download nor source regeneration. Invalid image files trigger generation
of a verified replacement before the damaged output is removed. Old revision
caches are retained and can be removed when no running UI uses them.


Back up the SQLite database using SQLite's backup API or `VACUUM INTO`, including
`uploaded_icons` and `service_edits`. Do not copy a live WAL database file alone.
A restored database must pass `PRAGMA integrity_check`; verify pinned services,
icon references, upload bytes, and migration version before switching traffic.

The existing homelab DevSpace deployment uses `/tmp/balemoh.db` without a PVC.
It must migrate to persistent storage before uploaded icons can survive pod
replacement. Local validation uses an isolated database restored from a backup;
production data is never used for upload/delete tests.

Cache repairs publish a new revision atomically. Existing processes keep their
previous revision available. Old revisions remain on disk; remove an unused cache
directory only after all UI processes using it have stopped.
