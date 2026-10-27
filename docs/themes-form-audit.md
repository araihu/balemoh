# Goshtoso Form fit for theme create/edit

Reviewed 2026-10-27 against the active Goshtoso workspace checkout. The basic
Form/FieldGroup adoption described below has now been implemented and verified.
This does not establish a published-module check.

## Implementation status

The create/edit form now uses Form, with typed FieldGroups for the name, base
Select and logo FileInput. Name required state and its visible marker belong to
FieldGroup. Existing Select/FileInput labels remain with those controls to retain
their accessibility, preview styles and error presentation. Their field-wrapper
IDs are distinct from the preserved control IDs.

The existing public Save Button stays in Form's supported children slot. Upload,
Toast, CSS import, editor/preview layout and in-place feedback remain intact.
No upstream Form API was expanded. The agreed boundary for further work is to
align the API design with the user before adding any Form customization surface.


## Decision

Use Goshtoso Form for the theme form structure and FieldGroup for individual
fields. No upstream change blocks that adoption. Preserve the existing JSON save
adapter and controlled ThemeEditor. They own draft state, uploads, revisions,
idempotent retries and preview synchronization, which are application behavior.

The clearest duplication found was field presentation. The custom theme name's
manual Required hint has been replaced by FieldGroup's required marker and
native/accessibility state.

## Existing public components that fit

| Current need | Public API | Integration |
| --- | --- | --- |
| Main `<form>` | `form.Form`, `Config.RootAttrs` | Preserve `presentation-save`, action and children. Keep HTMX unset for the existing JSON submit handler. |
| Named sections | `form.Section`, `Columns: "1"` | Group identity and appearance; the editor/preview retains its own two-column layout. Colors remain expanded. |
| Required name | `form.FieldGroup`, `Input`, `Required`, `Errors`, `Hints` | Preserve the actual input ID, name, pattern, max length and `data-presentation-name`. Remove duplicate labels/hints. |
| Base-theme selector | `FieldGroup.Select` | Retains the public Select, trailing origin badges and font/color previews. |
| Logo input | `FieldGroup.FileInput` | Retains the existing FileInput and upload hooks. Errors associate with the native control and its validation state. |
| Error summary | `form.FormErrors`, `FormErrorItem.TargetID`, `FocusTargetID()` | Supports server-rendered summaries with links to actual fields. |
| Public editor | `Form` children, optional `FieldGroup` children | ThemeEditor already renders without a nested form; no new adapter field type is required. |
| Save button | Public `button.Button` as a child, `Footer: nil` | Preserves `data-presentation-save`, loading attributes and disabled control ownership today. |
| Multipart forms | `RootAttrs["enctype"]`, `HTMX.Encoding` | Available for the separate asset forms; not a reason to change the JSON theme save. |

`Form` prevents Enter submission by default. Set `PreventEnterSubmit` explicitly
to false during a behavior-preserving migration, then decide Enter behavior as a
separate product change.

Keep rename/delete/font-upload forms outside the main form, as today.

## Narrow upstream opportunities

1. **Footer submit customization.** `FooterConfig` has labels, cancel navigation,
   `SubmitDisabled` and `Sticky`, but no submit attrs, component slot or Button
   options. `formFooter` renders its button directly. A supported submit slot or
   attrs/options pass-through would let this application use the complete Footer
   while retaining its Save hook and public Button behavior. Until then, Form's
   existing children composition is sufficient; no private DOM selectors are
   needed.

2. **Client-side field and summary errors.** `FieldGroup.Errors` and
   `FormErrors.Items` are rendered from Go. Empty summaries render no markup;
   field validation responses replace the field with HTML. No public runtime
   helper/event was found for updating errors in place. Logo rejection and local
   CSS import need immediate feedback while retaining the selected file and
   focus. A field-error update contract would remove our manual message handling.
   Swapping a file input on failure would clear its FileList, so that is not a
   suitable substitute for the current retry behavior.

3. **Structured ThemeEditor errors.** `themeeditor:change` exposes definition,
   previewMode, dirty and valid. It does not expose the invalid fields, messages
   or focus targets. Providing those would allow FormErrors to summarize color
   and font validation without inspecting the editor's private DOM. The existing
   valid flag remains enough to gate Save in the first migration.

These are integration improvements, not prerequisites to use Form/FieldGroup.
No GitHub issue or upstream publication was created by this audit.

## Transport and persistence stay with Balemoh

`components/form/validation.Handle` reads `r.ParseForm()` into `map[string]string`
and renders field responses for HTMX. The current save endpoint strictly decodes
`PresentationSave` JSON, including a nested complete Definition, reference IDs,
expected settings/theme revisions and command ID. It also handles unknown network
outcomes by replaying the same command. Those contracts do not become ordinary
form-urlencoded submission by adopting Form.

Use Form without HTMX for the initial migration. Continue submitting through the
existing adapter and validating at the API boundary. FieldGroup presentation can
be adopted independently of `validation.Handle`.

## Smallest migration

1. Replace the main native form wrapper with `form.Form`, retaining its ID,
   action, native behavior and JSON listener.
2. Move name, base-theme and logo presentation to typed FieldGroups. Reuse
   Section for organization and keep ThemeEditor/preview as children.
3. Retain the public Save Button, upload handlers, field messages and conflict
   controls until the optional upstream contracts above are available.
4. Verify required/whitespace names, CSS import Cancel/Confirm/Reset, logo
   rejection/retry, draft preservation, same-command replay, and 390/1440 px
   keyboard/layout behavior before removing any superseded markup.

## Evidence and limits

- `go test -count=1 ./components/form ./components/form/validation` passed.
- Existing browser tests for field-value preservation, error clearing and the
  responsive Footer passed.
- A compiled composition probe rendered Form + FieldGroup + FormErrors + the real
  ThemeEditor + the current public Save Button. It verified stable IDs/hooks,
  native/ARIA validation, error focus links, multipart attrs, Enter policy and
  exactly one native form. Source and receipt are retained in
  `data/themes-form-fit.go.txt` and `data/themes-form-fit.log`.
- API inspection used `components/form/types.go`, `form.templ`, `errors.templ`,
  `validation/handle.go`, `assets/js/src/components/theme-tools.js`, and the form
  demo/validation examples in the Goshtoso worktree.
- Consumer comparison used `ui/internal/view/themes.templ`,
  `ui/internal/view/static/presentation.js`, and
  `ui/internal/bff/presentation.go`.
- The initial audit changed no UI. The subsequent consumer migration updated the
  local UI binary and passed native required/pattern validation, Enter submit,
  same-command conflict replay, CSS import Save payloads, logo error/Toast and
  24 responsive Browse/Create/Edit cases. All write tests used mocked responses.
  Receipt: `data/theme-form-migration-browser.log`.
- No persisted theme or dependency pin was changed. Source resolution is through
  the local workspace, not `GOWORK=off`.

The editor now uses public Section cards for Theme details, Site identity and
Appearance. The whole form occupies one desktop column, with the preview in a
sibling column and stacked below on smaller screens. This uses existing API.
