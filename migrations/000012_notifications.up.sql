CREATE TABLE IF NOT EXISTS notifications (
 sequence INTEGER PRIMARY KEY AUTOINCREMENT,
 id TEXT NOT NULL UNIQUE,
 source_kind TEXT NOT NULL,
 source_id TEXT NOT NULL,
 connection_id TEXT NOT NULL,
 event_id TEXT NOT NULL,
 severity TEXT NOT NULL CHECK (severity IN ('info', 'success', 'warning', 'critical')),
 title TEXT NOT NULL,
 body TEXT NOT NULL,
 occurred_at TEXT NOT NULL,
 received_at TEXT NOT NULL,
 destination_json TEXT NOT NULL,
 payload_json TEXT NOT NULL,
 read_at TEXT,
 UNIQUE (source_kind, source_id, connection_id, event_id)
);

CREATE INDEX IF NOT EXISTS notifications_unread_sequence_idx
 ON notifications (read_at, sequence DESC);
