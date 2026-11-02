# Homepage layout editor

Mode: Operate. Code-led. User-pinned structure: preview first, contextual settings.

Keep the existing Balemoh/Goshtoso visual system. Replace this editor's composition;
do not redesign the product identity. Operators use existing light/dark preferences
in a desktop or mobile browser while maintaining their homelab.

First viewport: persistent Save/status, compact Settings and Create group,
and a populated preview using the full content width. Settings start closed in
an on-demand left drawer below the shell header, with Page, Group and Item tabs.
Settings or explicit group/item selection opens the appropriate context. Closing
returns focus to the initiating action. Desktop and mobile use one responsive DOM.

Signature interaction: selecting a group or item updates one inspector without
inserting form controls into the selected widget or changing its geometry.

Page settings, default assignments and IDs remain in contextual tabs. The limits
panel is removed; constraints remain in API validation. Group/item positions use
one-based numeric fields; layout choices pair labelled radios with icons.
Drag, keyboard and named movement remain. Card bodies are inactive in this editor;
mouse or pen can drag them directly between groups. Clicks do not navigate or run
widget actions. Item pencils and group names still open settings.
Save, errors, conflict comparison, readback, discard and provisional ID recovery
retain the reviewed state machine and API contract. One responsive DOM, stable IDs.

Acceptance: desktop 1440 and actual user width 1404; mobile 390. Both themes,
light/dark. Select, edit, move, reorder/reset, create/delete/cancel, save and recover
conflicts. Preserve arbitrary widget content and visible focus.
