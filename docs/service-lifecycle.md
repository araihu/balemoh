# Service lifecycle

Staging has Live, Missing, and Hidden tabs. Only live, pinned groups appear on the homepage.

A successful complete source snapshot marks absent observations missing. Failed discovery leaves that source unchanged. Newer snapshots restore present observations; names, descriptions, address overrides, icon choices, and pin choices survive. Hidden overrides presence in the tab selection and survives discovery. Show returns a hidden service to Live or Missing according to its current presence.

Hiding operates on the selected service group. It never changes Kubernetes resources. Permanent deletion is allowed only while the group is missing and removes its missing observations, saved edits, endpoints, and pins. Any still-live shared observations and uploaded icon files remain. If subsequently rediscovered, permanently deleted resources start with discovery defaults.

| Current state | Action | Result | Repeated or stale request |
|---|---|---|---|
| Live | Hide | Hidden, settings and pin retained | Repeated hide succeeds; missing before submit returns 409 |
| Hidden | Show | Live or Missing | Repeated show succeeds |
| Missing | Delete permanently | Missing observations and settings removed | Repeat returns 404; rediscovered before submit returns 409 |
| Live, including hidden live | Delete permanently | Rejected, no changes | 409 |
| Missing or Hidden | Pin | Rejected, no changes | 409 |
| Any | Failed snapshot | No presence change | Retry with complete snapshot |
| Any | Older/repeated snapshot | No state change | Watermark prevents resurrection |
| Any | Storage/transport failure | No partial group mutation | Visible recovery; refresh before retry |

Mutations use POST/Redirect/GET in the UI. Permanent deletion requires confirmation. API guards run against current catalog state in the same SQLite transaction as the group mutation, so old browser tabs cannot delete a service that has returned. SQLite migration 7 adds presence and visibility flags with existing rows initially live and visible. The next successful scan determines which are missing.
