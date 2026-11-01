# Browser notification settings

The notification tray and inbox no longer embed the browser notification
preferences block. Config > Notifications opens `/settings/notifications`,
with the current device state, privacy explanation and enable/disable action.
The existing tray action menu provides the same action and a settings link.
Failures appear above the tray list. The shared demo hides these controls and
rejects direct navigation to the settings route.

## Component decisions

- Reuse ConsoleShell's existing Config navigation and Sidebar item model.
- Compose PageHeader, Panel and Button for the settings page.
- Reuse Dropdown items, their public IDs and click expressions for the tray.
  The text-only browser action updates its native button label without querying
  private component markup. Its state uses the existing Web Push controller.
- Keep PushManager, permission, persistence and retry behavior unchanged.
  No dependencies, custom CSS or new preference storage.

## Validation

- Standalone UI `go test ./...` and `go vet ./...` passed.
- All 13 JavaScript tests passed, including menu synchronization, pending state
  and visible cleanup-failure feedback. Existing tests cover denied/dismissed
  permission, unsupported browsers, storage errors, renewal and retry.
- HTTP tests cover settings navigation, no-store responses, removal of embedded
  preferences, menu/sidebar destinations and read-only demo denial.
- Consumer templ files regenerated. `git diff --check` and Impeccable's detector
  passed.
- Local preview used a disposable database and test-only Web Push credentials.
  Settings screenshots cover Goshtoso and Minimal, light and dark, at 390 and
  1440 px. Mobile document width stayed within the viewport. The mobile kebab
  stayed within the viewport with readable wrapping.
- Sidebar selection, nested menu Escape and focus return checked in browser.
  The unavailable state appeared while the local server lacked credentials;
  configured preview correctly showed notifications off with the action enabled.

Native permission prompts, real push delivery and an automated accessibility
scanner were not exercised for this placement change. No production change.
