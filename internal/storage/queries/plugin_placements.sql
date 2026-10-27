-- name: ListPluginPlacements :many
SELECT id, plugin_id, connection_id, contribution_id, options, enabled, revision, created_at, updated_at
FROM plugin_placements ORDER BY created_at, id;
