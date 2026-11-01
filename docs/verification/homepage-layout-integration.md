# Homepage layout integration verification

Upstream prerequisite: Goshtoso v0.5.0, commit
c4bbff5b89274d31432d6e0a154093ce2e7f8eaa, PR araihu/goshtoso#363.
The authorized organization-admin merge preceded tag publication. Release workflow
41350776824 passed full verification and publication. Published styles.css and
goshtoso-theme.css match the tag byte-for-byte. The isolated external consumer
passed with GOWORK=off and no replace; documentation deployment passed. Authored
coverage reported 94.2%, and Codecov accepted the upload for processing.

Balemoh API review accepted at 8a123dc2209c2e7b4e5b6965db4399694ce95b70,
review identity 4629856f9e6927713cd7911698933e6922fb24429812c0162b688f7d5547f7ef.
Three review rounds corrected typed omitted/null/string move serialization,
targeted error pointers, reserved builtin identity reuse and generated-file drift.
The reviewer independently passed root/client suites, generation/tidy checks and
OpenAPI lint, which scored 100/100. UI review is a separate pending gate.

Implementation tests cover strict JSON and preconditions, SQLite CAS races and
cancellation, stale identical writes, membership-token changes, dormant references,
corrupt-layout recovery, local source eligibility during cold provider outages,
generated SDK move effects, BFF raw forwarding and cross-origin/read-only denial,
ambiguous committed writes and read-back, and SSE structure versus content updates.
Node tests cover late previews, dormant ordering, creation/deletion and Save state.

Local browser checks have exercised populated hosts/services, real Alertmanager
plugin widgets, draft group creation, named item movement, independent page geometry,
Save and discard-dialog cancellation. Further keyboard, responsive, theme and
conflict checks remain part of UI acceptance. These notes describe local validation;
no Balemoh deployment or production migration has run.
