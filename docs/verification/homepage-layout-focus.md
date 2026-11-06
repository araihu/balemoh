# Layout editor focus regression, 2026-11-03

The released editor replaces its inspector after a debounced preview response.
Its Group panel is initially hidden while Alpine applies the selected tab.
Restoring Name focus before that update fails and leaves focus on the Group tab.
Awaiting Alpine's next tick before restoring focus fixes the sequencing.

Native browser regression used separate before/after UI binaries and a copied
synthetic database. No production tab or saved layout was changed.

- Before: rename Remote hosts to Priority. After preview completes, active
  element is the Group tab instead of Name.
- After: the same rename retains Name focus, value Priority and caret 8.
- Insert Top at the beginning, then replace a middle selection. Subsequent
  previews retain the edited value and caret.
- Select characters 1 through 3 while preview is pending. After completion,
  Name remains focused with selectionStart 1 and selectionEnd 3.
- Rename, then switch to Page before preview completes. Page stays selected
  and focused; completion does not restore focus to Name.
- Repeat rename at 390px and 1440px. Name stays focused after completion,
  without horizontal overflow or console errors.

View and BFF tests passed, including all 37 existing editor Node tests.
Independent review approved the public Alpine lifecycle hook and retained
interaction guards. No browser harness was added for a source-order assertion;
the regression depends on actual Alpine panel visibility.

Initial homepage and editor HTML passed title, description, per-route canonical,
Open Graph and explicit X Card checks. The existing public social image returned
PNG, 1280 by 640 pixels, below 1 MB. No page metadata changes were needed.

## Preview swap churn

The following mitigation keeps the static toolbar and Goshtoso Tabs scope
mounted. Preview responses update only changed Page, Group and Item content
slots owned by Balemoh. The selected panel no longer needs reinitialization.
Item movement controls are updated once through their containing slot.

Four Node regressions cover scoped targets, unchanged-section reuse, rejection
of incomplete fragments before any mutation, and retry after a partial failure.
All 41 editor Node tests and the view/BFF tests passed. Independent review
approved the runtime changes.

Native checks on an isolated synthetic database passed:

- Rename, change selected group, and switch List/Grid. Fields and option labels
  update while the Group tab stays selected.
- Create an empty draft group, move pico into it, then delete the group and
  move references back to Widgets. New options appear and removed ones disappear.
- Commit position 999 with Enter. The field returns to its valid position and
  shows the error; changing context clears the old error.
- Switch to Page during a rename. Preview completion retains Page and focus.
- At 390px and 1440px, Name keeps focus and selection through preview updates,
  with no horizontal page overflow or console errors.

Saved layout bytes and revision remain unchanged. No production tab, settings,
library dependency or metadata changed. Fresh homepage/editor HTML and the
public social PNG passed the existing metadata gate.
