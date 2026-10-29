CREATE TABLE web_push_state (singleton INTEGER PRIMARY KEY CHECK(singleton=1), public_key TEXT NOT NULL, enabled INTEGER NOT NULL);
INSERT INTO web_push_state VALUES(1, '', 0);
CREATE TABLE web_push_subscriptions (
 id TEXT PRIMARY KEY,
 endpoint TEXT NOT NULL UNIQUE,
 p256dh TEXT NOT NULL,
 auth TEXT NOT NULL,
 expires_at INTEGER NOT NULL
);
CREATE TABLE web_push_deliveries (
 subscription_id TEXT NOT NULL REFERENCES web_push_subscriptions(id) ON DELETE CASCADE,
 notification_id TEXT NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
 attempts INTEGER NOT NULL DEFAULT 0,
 next_attempt INTEGER NOT NULL DEFAULT 0,
 lease_until INTEGER NOT NULL DEFAULT 0,
 expires_at INTEGER NOT NULL,
 PRIMARY KEY(subscription_id, notification_id)
);
CREATE INDEX web_push_due ON web_push_deliveries(next_attempt, lease_until);
CREATE TRIGGER notification_web_push AFTER INSERT ON notifications
WHEN (SELECT enabled FROM web_push_state WHERE singleton=1)=1
BEGIN
 INSERT INTO web_push_deliveries(subscription_id, notification_id, expires_at)
 SELECT id, NEW.id, unixepoch()+86400 FROM web_push_subscriptions WHERE expires_at>unixepoch();
END;
