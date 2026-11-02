# Balemoh

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Homelab operators organize services, hosts and plugin widgets on a shared homepage.

## Product purpose

Aggregate selected infrastructure content and let operators arrange it into useful groups.

## Capabilities and constraints

The API owns layout, grouping and ordering. Groups can mix component kinds. Page
geometry and group geometry are independent. Alphabetical order is the default;
custom order and explicit memberships override it. Editing uses a local draft,
server preview and one conditional Save. Conflicts and uncertain results preserve work.
The application uses Go, templ and Goshtoso components with existing themes.

## Confirmed editing preference

The user selected a preview-first editor with contextual settings. Settings should
follow the selected group or item. Advanced configuration must remain available. On the layout editor page, clicks on
card bodies must not navigate or trigger widget actions. Operators can press and
drag any card body between groups without selecting it first.

## Evidence

Local demonstrations use synthetic content and a real plugin renderer. Existing
brand assets and theme tokens belong to Balemoh and Goshtoso. No new claims are needed.

Settings preference: use an on-demand left drawer with Page, Group and Item tabs.
Open it on Settings or explicit group/item selection. Keep preview full-width when
closed. Numeric positions set group and item order. Remove limits documentation
from the editing form; keep validation and actionable errors.
