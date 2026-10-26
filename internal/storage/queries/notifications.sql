-- name: GetNotificationByIdentity :one
SELECT sequence, id, source_kind, source_id, connection_id, event_id, severity,
       title, body, occurred_at, received_at, destination_json, payload_json, read_at
FROM notifications
WHERE source_kind = ? AND source_id = ? AND connection_id = ? AND event_id = ?;

-- name: CountNotifications :one
SELECT COUNT(*) FROM notifications;

-- name: InsertNotification :one
INSERT INTO notifications (
    id, source_kind, source_id, connection_id, event_id, severity, title, body,
    occurred_at, received_at, destination_json, payload_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING sequence, id, source_kind, source_id, connection_id, event_id, severity,
          title, body, occurred_at, received_at, destination_json, payload_json, read_at;

-- name: ListNotifications :many
SELECT sequence, id, source_kind, source_id, connection_id, event_id, severity,
       title, body, occurred_at, received_at, destination_json, payload_json, read_at
FROM notifications
WHERE (? IS NULL OR sequence < ?)
  AND (? = 0 OR read_at IS NULL)
ORDER BY sequence DESC
LIMIT ?;

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM notifications WHERE read_at IS NULL;

-- name: GetNotificationByID :one
SELECT sequence, id, source_kind, source_id, connection_id, event_id, severity,
       title, body, occurred_at, received_at, destination_json, payload_json, read_at
FROM notifications
WHERE id = ?;

-- name: MarkNotificationRead :exec
UPDATE notifications
SET read_at = COALESCE(read_at, ?)
WHERE id = ?;

-- name: ClearNotifications :exec
DELETE FROM notifications;
