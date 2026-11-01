# Homepage layout integration verification

## Upstream release

Goshtoso v0.5.0 uses commit c4bbff5b89274d31432d6e0a154093ce2e7f8eaa.
PR araihu/goshtoso#363 merged with the explicitly authorized organization-admin
bypass. Release workflow 41350776824 passed. Published CSS matches the tag bytes.
The external consumer passed with GOWORK=off and no replacement. Documentation
production smoke passed. Authored coverage was 94.2%; Codecov accepted the upload.

## Automated validation

`hack/check-plugin-modules.sh` passed with pinned protoc 33.0. It regenerated,
tidied, tested and vetted all modules and checked generated output. Root, client,
UI and plugin SDK race suites passed. OpenAPI lint scored 100/100. The editor has
14 top-level Node tests, including ten malformed-confirmation subtests.

API/storage execution covers virtual revision 0, restart, conditional writes,
concurrent writers, cancellation, stale identical writes, membership tokens,
dormant references, invalid fields/relations, deletion invariants, read-only
routes and corrupt-layout default-view recovery. Local selection eligibility is
tested independently of runtime telemetry, plugin availability and cold providers.

BFF/Node execution covers strict forwarding, cross-origin denial, stale bootstrap,
reverse live readbacks, complete comparison with zero writes, preview ownership,
provisional UUID repair, active-to-dormant selection fallback and uncertain Save.
Save responses require a complete valid configuration, a signed-64 revision and
a plausible transition matching the attempted document. Invalid confirmations and
ambiguous gateway responses preserve the attempt and trigger readback.

## Browser execution

Tests used loopback-only synthetic catalog/telemetry and the real Alertmanager
plugin package. Hosts, services and arbitrary plugin content rendered in one group.
Named cross-group movement, actual pointer dragging, keyboard reorder/reset and
focus restoration worked. Creation, deletion cancellation and dirty-discard dialogs
preserved draft values and returned focus. A provisional UUID rejection exposed
regeneration, preserved the title and unrelated width change, then saved once.

Two independent browser clients exercised concurrent Save. The dirty client kept
its draft and disabled Save. Width and membership/default conflicts showed both
saved and draft values. Comparison and cancelled discard wrote nothing. Explicit
reload recovered the saved version. Input value and defaultValue remained aligned
after inspector replacement with HTMX settle:0.

Moving a selected active item into a collapsed group opened that group and retained
one visible movement toolbar with focus on Move item to group. Inspector selection
opened it too. Closing the selected disclosure kept the item visible. Its saved
Start collapsed flag stayed true. Normal homepage disclosure honored the flag.

Seven post-commit response faults were exercised through the actual BFF/browser:
200 empty object, 200 null, missing document, empty revision/incomplete document,
502, 504 and malformed intermediary 503. Every case reported Saved layout confirmed
after GET readback, retained its edited title and disabled Save. Seven attempted
writes produced exactly seven revision increments, with no automatic repeat PUT.

The matrix in homepage-layout-matrix.json records Goshtoso/Minimal, light/dark,
and 390/1440 CSS pixels. All eight combinations had the expected theme/mode,
page controls above preview, no horizontal overflow and no duplicate DOM IDs.
No console errors appeared during the final browser checks.

## Independent review and limits

API/storage review accepted at 8a123dc; BFF lifecycle review accepted at 4276380.
Later reviews corrected complete comparison, UUID recovery, inspector ownership,
collapsed-selection visibility and malformed Save confirmation. Final source review
is pending after the Save boundary correction.

These are local validation results. No Balemoh deployment or production migration
ran. Physical touch devices, real screen readers, Firefox and WebKit were not tested.
Automated state/API tests complement browser checks; they do not claim a manual
browser replay of every row in the design ledger or full accessibility certification.
