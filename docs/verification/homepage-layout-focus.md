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
