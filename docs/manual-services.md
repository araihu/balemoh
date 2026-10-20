# Manual services

Staging supports services entered by the user, alongside discovered services. Add service opens the existing name, description, address and icon editor. A name and absolute HTTP(S) address without credentials are required. Creation returns to Staging, where the existing pin checkbox adds the service to the homepage.

Manual services use the reserved manual/local source and a random identity independent of name or URL. They stay live across syncs, can be edited, pinned, hidden and restored through existing controls, and do not appear as hosts or discovered resources. Hiding is the removal path in this delivery. Arbitrary HTML, scripts and other URL schemes are not accepted.

## Persistence and transitions

- New request key creates one unpinned live record and its presentation in one transaction.
- Repeated request key returns the current record. It never overwrites edits, visibility or pin state.
- Validation/storage failure leaves no partial record; the form preserves its draft and key for retry.
- Editing a manual service requires an address. Pin/unpin and hide/show retain their existing rules.
- Discovery and federation cannot write the reserved manual source. Other source snapshots cannot mark manual services missing.

POST /api/v1/staging/services accepts requestId, displayName, description, address and optional icon. requestId is a random 16-byte hexadecimal key. Existing catalog tables store manual records, so no migration is required. Deploy API and UI together. Earlier versions retain the rows, but do not offer creation.

Acceptance covers creation, validation, retry after ambiguous failure, edits and pins surviving sync, hide/show, icon validation rollback, restart persistence, and absence from host inventory. Browser checks exercise the form, icon picker, validation drafts, pinning and the resulting home card.
