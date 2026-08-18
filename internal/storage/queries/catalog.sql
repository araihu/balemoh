-- name: UpsertDiscoveredService :exec
INSERT INTO discovered_services (
    id,
    source_kind,
    source_id,
    resource_kind,
    resource_namespace,
    resource_name,
    display_name,
    description,
    metadata_json,
    images_json,
    observed_at,
    created_at,
    updated_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(source_kind),
    sqlc.arg(source_id),
    sqlc.arg(resource_kind),
    sqlc.arg(resource_namespace),
    sqlc.arg(resource_name),
    sqlc.arg(display_name),
    sqlc.arg(description),
    sqlc.arg(metadata_json),
    sqlc.arg(images_json),
    sqlc.arg(observed_at),
    sqlc.arg(created_at),
    sqlc.arg(updated_at)
)
ON CONFLICT (id) DO UPDATE SET
    source_kind = excluded.source_kind,
    source_id = excluded.source_id,
    resource_kind = excluded.resource_kind,
    resource_namespace = excluded.resource_namespace,
    resource_name = excluded.resource_name,
    display_name = excluded.display_name,
    description = excluded.description,
    metadata_json = excluded.metadata_json,
    images_json = excluded.images_json,
    observed_at = excluded.observed_at,
    updated_at = excluded.updated_at;

-- name: GetDiscoveredService :one
SELECT
    id,
    source_kind,
    source_id,
    resource_kind,
    resource_namespace,
    resource_name,
    display_name,
    description,
    metadata_json,
    images_json,
    observed_at,
    pinned_at,
    created_at,
    updated_at
FROM discovered_services
WHERE id = sqlc.arg(id)
LIMIT 1;

-- name: ListDiscoveredServices :many
SELECT
    id,
    source_kind,
    source_id,
    resource_kind,
    resource_namespace,
    resource_name,
    display_name,
    description,
    metadata_json,
    images_json,
    observed_at,
    pinned_at,
    created_at,
    updated_at
FROM discovered_services
ORDER BY lower(display_name), id;

-- name: ListPinnedDiscoveredServices :many
SELECT
    id,
    source_kind,
    source_id,
    resource_kind,
    resource_namespace,
    resource_name,
    display_name,
    description,
    metadata_json,
    images_json,
    observed_at,
    pinned_at,
    created_at,
    updated_at
FROM discovered_services
WHERE pinned_at IS NOT NULL
ORDER BY lower(display_name), id;

-- name: DeleteServiceEndpoints :exec
DELETE FROM service_endpoints
WHERE service_id = sqlc.arg(service_id);

-- name: InsertServiceEndpoint :exec
INSERT INTO service_endpoints (
    service_id,
    name,
    url,
    port,
    protocol,
    provenance
) VALUES (
    sqlc.arg(service_id),
    sqlc.arg(name),
    sqlc.arg(url),
    sqlc.arg(port),
    sqlc.arg(protocol),
    sqlc.arg(provenance)
);

-- name: ListServiceEndpoints :many
SELECT
    id,
    service_id,
    name,
    url,
    port,
    protocol,
    provenance
FROM service_endpoints
WHERE service_id = sqlc.arg(service_id)
ORDER BY id;

-- name: PinDiscoveredService :execrows
UPDATE discovered_services
SET pinned_at = sqlc.arg(pinned_at),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: UnpinDiscoveredService :execrows
UPDATE discovered_services
SET pinned_at = NULL,
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);
