# Instance themes

`Config > Appearance > Themes` selects and previews saved themes. Its Apply
action commits the chosen saved identity and global System/Light/Dark mode, then
reloads the browser. Browsing never edits or creates a theme.

Create from this theme and Edit theme open `/presentation/themes/editor` with
the selected snapshot. Presets always start an independent custom theme. Editing
an existing custom theme keeps its saved revision. Colors, typography, logo/icon uploads,
theme renaming and deletion belong to this editor page. Save and apply globally
retains the existing atomic save contract and returns to Themes. Other pages use
the saved selection on their next full navigation.

Rename a theme using Custom theme name in the editor and Save. Saved themes have
a Delete action beside the editor title, with confirmation. Create and copy drafts
have no Delete action. Failed deletions preserve the draft; successful deletion
returns to Themes. The separate management list has been removed.

Theme editing and applying require JavaScript. Without it, Save and Apply remain disabled and show an explanation. Failed saves focus the status summary while preserving the draft and retry command.

Every preset comes from Goshtoso. Editing a preset creates an independent named
snapshot. A custom theme retains its saved literal colors, typography references,
and radius when presets change. Duplicating a saved theme copies those values;
it does not establish inheritance.

The Themes interface uses English. Its isolated preview includes interactive
Goshtoso buttons, menus, status badges and Overview/Settings/Activity tabs.
Overview retains the production ServiceCard, HostWidget and DashboardGrid with
sample data. Settings shows fields, selection, validation, disabled controls,
checkboxes, a switch and radio segments. Activity shows semantic alerts; the
sample dialog exercises modal surfaces and focus return. Theme colors, title/body
fonts and radius apply throughout. Navigation, form submissions and sample actions
stay in the frame; no discovery, live updates or saved-setting writes run.

Color roles use Goshtoso Palette inside Select shells, with a swatch and exact
hex/sRGB/OKLCH entry. Per-color Reset restores the loaded baseline; named
swatches save as literal values.

The editor uses Goshtoso Select and Combobox controls. Font options render in their
own typeface. Theme options show primary/secondary/surface swatches and their
title face, including the selected label. The sun/moon inspection button changes
only the isolated preview. Color contrast is read-only feedback below the preview, outside the editor form; global System/Light/Dark uses an icon-based Goshtoso RadioBar and remains a saved preference.
Google previews use private font-family aliases, so opening a font
picker does not change the surrounding application's typography. Google assets
load on demand. Uploaded files remain local assets.

## Saving and conflicts

The UI preserves the draft, command ID, resource ID, and original expected
revisions after an unsuccessful or unconfirmed Save. Exact immediate replay is
idempotent. A conflict offers a separate read-only saved version and an explicit
Discard draft and reload action. It never silently updates the expected revision.

Goshtoso dialogs confirm draft replacement and deletion. Cancel or Escape keeps
the draft and returns focus. Icons/Themes tabs keep selection/preview state. Editor navigation uses its own
draft confirmation. Browsers retain
their standard close/reload protection for unsaved documents.

## Fonts

`Config > Appearance > Fonts` handles font upload and deletion at
`/presentation#fonts`. The theme editor keeps only its font selectors. Uploaded
fonts appear when opening the editor; successful management operations reload
Fonts to refresh the list. Deletion requires confirmation and saved-theme
references remain protected.

Font sources include system stacks, on-demand Google Fonts, a packaged Go Regular
font, and uploaded files. Each upload is independent, including files with the
same name. Fonts use immutable references and per-file CSS family aliases.

WOFF2, WOFF, TTF, and OTF accept files up to 10 MiB, including variable fonts at
default axes. Container/metadata inspection bounds decompression to 64 MiB, limits
the directory to 256 tables, and limits each metadata table to 1 MiB. Validation
runs one at a time. It does not replace the browser's complete glyph sanitizer.

The upload catalog holds at most 256 files and 512 MiB of original bytes.
There are at most 128 saved custom themes. A font referenced by any saved theme
cannot be deleted. Apply another theme before deleting the active custom theme.
No command replaces referenced font bytes or silently chooses a fallback.

Uploaded bytes revalidate with an ETag and
`Cache-Control: private, max-age=0, must-revalidate` so deletion remains visible.
System/Google/packaged font choices come from the library catalog, not from the
uploaded-files endpoint.

## Recovery and backups

Open `/presentation?appearance=recovery#themes` for temporary Goshtoso/System
chrome. It loads actual saved values and revisions. Only an ordinary successful
Save changes global settings and leaves recovery. Appearance or font failures
show a temporary fallback without resetting stored preferences.

Themes, settings, font metadata, reference rows, and original font BLOBs share
the existing SQLite database. Use SQLite's backup API or `VACUUM INTO`; copying
an active database file alone can omit WAL changes. The backup/reopen test checks
selection, revision, font references, and bytes together.

Migration 13 is additive. Binary rollback leaves its tables intact. Its down
migration deliberately removes theme data and is not a routine rollback step.

## Development integration

Root, client and UI pin the same immutable Goshtoso commit, with an immutable
App Shells System-mode fix. Standalone generation, tests and vet run with
`GOWORK=off`; the ignored development workspace is optional. Dependency PRs are
[Goshtoso #352](https://github.com/araihu/goshtoso/pull/352) and
[App Shells #56](https://github.com/araihu/goshtoso-app-shells/pull/56).
Merge, tags and homelab rollout remain separate stages.

The OpenAPI document describes the API commands and error envelope. The UI uses
the generated SDK, including a separate 30-second font-upload client. Normal
request timeouts and existing live-service streams remain unchanged.

## Site identity

Theme creation and editing offer separate Goshtoso FileInputs for the logo and
icon. The logo appears in the site navigation; the icon supplies the favicon,
manifest and touch icon. PNG, JPEG, WebP and passive SVG follow the existing
icon-library validation, up to 2 MiB and 4096 px for raster dimensions. A square
PNG gives the best icon results.

Each field uploads on selection. An X icon appears only beside custom images and restores the default image.
It disappears after reset; its tooltip and accessible name identify the action.
A failed upload can be discarded beside its error without changing the current image. Save is
blocked while either upload is pending or failed. Failures show a Toast and an
error on the relevant field; selecting the same file again retries. Uploads and
resets affect only the draft until Save applies the theme globally.

Every upload gets a new image ID. Saved themes and copies retain both independent
references. Referenced image bytes cannot be replaced or deleted through the
icon library; metadata edits remain available. Older clients omitting branding
in replacement drafts preserve the saved references.

The footer always renders the original Balemoh logo and the fixed credit
"is made by Arai Hû with Goshtoso", with organization/library site links. Existing
icon-license attribution remains. Recovery temporarily uses original branding.

Migration 14 adds an optional image reference to saved themes and an index.
Migration 15 separates the logo reference and initializes it from the existing
icon reference, preserving the appearance of previously saved themes. The
Goshtoso definition schema is unchanged. Images remain in the existing uploaded-icons table and SQLite backup.


## Import a theme CSS file

In the theme editor, selecting a file under Import theme CSS imports it
automatically.
The file must contain one complete Goshtoso theme export, up to 1 MiB, using
literal hex/sRGB/OKLCH theme colors and rem radius. Font references must already
be available in this instance. Multiple-theme exports and arbitrary site CSS
are outside this import format.

Import updates only the draft colors, fonts and radius. Name, logo, icon and
global mode remain unchanged. Existing editor changes require confirmation; Cancel
preserves them and Reset returns to the saved baseline. Inspect the preview,
then Save and apply globally to persist. Failed imports show a field error and
Toast without replacing the draft. Export CSS files retain font source identity.
