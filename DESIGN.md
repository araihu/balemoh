---
name: Balemoh
description: Theme-aware homelab interface built with Goshtoso.
colors:
  primary: "var(--color-primary)"
  on-primary: "var(--color-on-primary)"
  primary-dark: "var(--color-primary-dark)"
  on-primary-dark: "var(--color-on-primary-dark)"
  secondary: "var(--color-secondary)"
  secondary-dark: "var(--color-secondary-dark)"
  surface: "var(--color-surface)"
  surface-alt: "var(--color-surface-alt)"
  surface-dark: "var(--color-surface-dark)"
  surface-dark-alt: "var(--color-surface-dark-alt)"
  on-surface: "var(--color-on-surface)"
  on-surface-strong: "var(--color-on-surface-strong)"
  on-surface-dark: "var(--color-on-surface-dark)"
  on-surface-dark-strong: "var(--color-on-surface-dark-strong)"
  on-surface-muted: "var(--color-on-surface-muted)"
  on-surface-dark-muted: "var(--color-on-surface-dark-muted)"
  outline: "var(--color-outline)"
  outline-dark-strong: "var(--color-outline-dark-strong)"
  control-outline: "var(--color-control-outline)"
  control-outline-dark: "var(--color-control-outline-dark)"
typography:
  body:
    fontFamily: "system-ui, sans-serif"
  title:
    fontFamily: "system-ui, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 600
  label:
    fontFamily: "system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 500
  context:
    fontFamily: "system-ui, sans-serif"
    fontSize: "0.8rem"
rounded:
  radius: "var(--radius-radius)"
  button: "1rem"
spacing:
  1: "0.25rem"
  2: "0.5rem"
  3: "0.75rem"
  4: "1rem"
  5: "1.25rem"
  6: "1.5rem"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    rounded: "{rounded.button}"
    padding: "0.5rem 1rem"
  button-alternate:
    backgroundColor: "{colors.surface-alt}"
    textColor: "{colors.on-surface-strong}"
    rounded: "{rounded.button}"
    padding: "0.5rem 1rem"
  button-plain:
    backgroundColor: "transparent"
    textColor: "inherit"
    padding: "0 0.25rem"
  text-input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.on-surface-strong}"
    rounded: "{rounded.radius}"
    padding: "0.5rem"
  card:
    backgroundColor: "{colors.surface-alt}"
    textColor: "{colors.on-surface}"
    rounded: "{rounded.radius}"
    padding: "1.5rem"
  group:
    rounded: "{rounded.radius}"
    padding: "0.75rem"
---
# Design System: Balemoh

## Overview

**Creative North Star: "Balemoh / Goshtoso"**

Balemoh keeps its existing brand assets and Goshtoso theme system. Theme selection and light/dark preferences determine semantic color bindings; the shell supplies inherited typography. This document records the implemented system; it does not establish a replacement identity.

Controls use familiar forms, named actions and visible focus. Widgets keep their own content and natural geometry. Structural separators and theme surfaces distinguish editing controls from the content they affect.

**Key Characteristics:**

- Semantic theme bindings across light and dark modes.
- Compact controls with visible keyboard focus.
- Natural-height service, host and plugin content.

## Colors

The selected Goshtoso theme owns color. Frontmatter stores live CSS bindings, rather than copying one theme's resolved colors. The default `goshtoso` theme uses Yale blue and sky aqua for primary actions, forest moss and lime moss for secondary roles, and blue slate for neutrals. Custom themes can replace these values.

### Primary

Primary and on-primary pairs identify filled actions. Light and dark variants are separate tokens. The layout editor aliases them through `--layout-accent` for selection and focus.

### Secondary

Secondary roles remain available through Goshtoso. The homepage editor primarily uses primary, alternate and plain actions; it does not add another accent.

### Neutral

Surface and alternate-surface pairs support the page, cards and controls. Strong foregrounds carry headings and field values. Muted foregrounds carry hints and status context. Balemoh binds `--balemoh-border` to outline in light mode and outline-dark-strong in dark mode. Control boundaries use the distinct control-outline pair.

**The Theme Binding Rule.** Reuse the active theme's semantic tokens and their dark counterparts. A default-theme snapshot is not a product-wide palette.

## Typography

ConsoleShell v0.1.9 sets its body to `system-ui, sans-serif`, which the inspected editor inherits. Goshtoso exposes `--font-body` and `--font-title`; its default theme binds those to Inter and Poppins. Those theme tokens and uploaded-font support remain available, but they are not proof of the editor's rendered font. Editor headings change size and weight without assigning the title font. Confirm computed styles before documenting a different rendered family.

Editor inspector and dashboard section headings use the frontmatter title role. Preview group headings use 1rem and weight 600. Context text uses the frontmatter context role. Field legends use the label role. Standard Goshtoso buttons use 0.875rem, weight 500 and 1.25rem line height; small buttons use 0.75rem. Hints use 1.5 line height. IDs use inherited monospace treatment at 0.75rem and wrap anywhere.

The service/card system can use larger titles and metric values. Widget titles use 1.5rem, weight 600 and 2rem line height. Do not flatten widget typography to inspector typography.

## Layout

Balemoh's content class caps width at 96rem and centers itself. ConsoleShell also wraps it in an 88rem content cap with 1.5rem main padding, so the inner cap alone does not establish the available width. Default padding is 2rem 1rem 4rem, changing below 720px to 1.25rem 0.75rem 3rem. Contained homepages cap at 72rem. Repeated spacing steps appear in frontmatter; they describe observed spacing rather than a new token API.

The homepage editor is a specific composition, documented in `.impeccable/surfaces/homepage-layout.md`. A sticky top bar holds Save, Cancel and status. Its -1.5rem top offset cancels ConsoleShell scroll-root padding, keeping the bar flush beneath the shell during scrolling. Compact Settings and Create group controls precede the preview. The preview uses
the full content width. A left Goshtoso Drawer starts closed and opens on Settings
or an explicit group/item edit action. Page, Group and Item tabs separate form
contexts. The panel uses the public small width and begins 4rem below ConsoleShell.
Closing returns focus to the initiating action. Numeric fields set one-based group
and item positions; mode radios pair Heroicons with text. Desktop and mobile share
one DOM and retain the same draft/save contract.

Selection updates the inspector. It does not insert controls into widget content. Page geometry and each group's geometry remain independent. Goshtoso `layout.Container` owns Stack, Grid and Masonry geometry and movement. Balemoh owns draft selection and persistence. Natural-height content must remain free to resize. Min-width zero, wrapping titles and `overflow-wrap:anywhere` protect narrow layouts.

## Elevation & Depth

The inspected editor uses theme surfaces, borders and outlines for hierarchy, without new decorative shadows. The sticky top bar has z-index 20. Selection uses an accent outline or accent group border. Goshtoso offers control, raised and overlay shadow tokens for components that need elevation; the editor does not apply them to every group or inspector.

Motion follows Goshtoso component state transitions. Balemoh's reduced-motion rule cuts transition and animation durations to 0.01ms and animation iterations to one. Preserve that behavior.

## Shapes

Fields, cards and editor groups follow `--radius-radius`, which the active theme can change. Filled Goshtoso buttons use a separate 1rem radius. Plain buttons have transparent backgrounds and no border. Passive editor group boundaries are dashed; the current group boundary becomes solid and accent-colored. Selected items use a 2px accent outline with 3px offset.

## Components

### Buttons

Reuse Goshtoso Button. Primary Save uses filled primary/on-primary colors. Alternate actions use alternate surface and strong foreground. Plain actions inherit text color and underline on hover. Filled and alternate buttons use the frontmatter padding, minimum 2.75rem height/width, weight 500 and wide tracking. Editor actions retain at least 2.75rem height. Hover increases contrast to 1.25; active restores contrast and reduces outline offset. Keyboard focus is a 2px outline with 2px offset. Disabled buttons use 0.75 opacity and a not-allowed cursor. Disabled Save reflects draft state rather than hiding the action.

### Cards / Containers

Goshtoso cards use alternate surfaces, outline borders and theme corners. The standard content region uses frontmatter card padding and 1rem internal gap. Service cards allow content and menus to extend when there is no clipped media. Preview widgets reuse the real renderer; they are not substitute editor illustrations.

### Inputs / Fields

Reuse Form FieldGroup, TextInput, Select, Checkbox and RadioBar. Text inputs use surface/strong foreground, control-outline boundaries, theme corners and frontmatter padding. Focus uses a 2px primary outline with 2px offset. Error boundaries use the danger role. Disabled fields use alternate surfaces, muted text, 0.5 opacity and a not-allowed cursor. Keep labels, helper text and error targets attached to their fields. Mode choices retain a 2.75rem minimum height.

### Navigation

Reuse ConsoleShell's icon menu and drawer. The active link carries `aria-current="page"` and weight 700. Balemoh supplies its dark primary token override for the active link. Keep existing brand art, navigation labels and destinations. Do not reproduce shell navigation as a custom editor toolbar.

### Tags and status

Reuse Goshtoso Badge for compact metadata. A medium badge uses 0.75rem type, 0.5rem horizontal and 0.25rem vertical padding. Use tone and appearance APIs for semantic states. Editor status uses a polite live region; errors use an alert region. Busy and disabled states reflect ongoing preview or save work. Conflict and uncertain-save recovery remain visible inline.

### Contextual editor

The group title selects its group. Item edit controls appear on hover, focus-within and selection, and remain visible on devices without hover. Selection controls remain outside inactive widget bodies. Editor card bodies use native inert semantics, so clicks do not navigate or run actions; mouse or pen can drag the body directly. Public movement handles remain available for keyboard and touch. The inspector exposes basic settings first and advanced options in native disclosures. Empty groups retain a 4rem drop region. Summary focus and inspector heading focus use a 2px accent outline with 3px offset; headings and errors reserve 7rem scroll margin below sticky controls.

Preview, dirty draft, save, conflict and uncertain result remain separate states. Preserve the draft and the available recovery actions when a request fails.

## Do's and Don'ts

### Do:

- Do use Goshtoso components and their semantic light/dark token pairs.
- Do preserve arbitrary widget presentation while disabling body actions in the layout editor. Keep normal homepage links and actions usable.
- Do keep focus, selection, disabled and error states distinguishable.
- Do verify the homepage editor at 1440px, 1404px and 390px in both color modes.

### Don't:

- Don't replace configured themes, fonts or existing brand assets with fixed palette values.
- Don't treat the homepage editor composition as a rule for every product screen.
- Don't use preview appearance as proof that a draft has been saved.
- Don't remove named movement actions or keyboard access when adding drag interactions.

Evidence checked: `ui/go.mod`, `ui/internal/view/shell.go`, `ui/internal/view/layout_editor.templ`, `ui/internal/view/static/balemoh.css`, `PRODUCT.md`, and the homepage surface brief. Component and theme definitions come from the locally installed Goshtoso v0.5.0 and App Shells v0.1.9 modules. This scan documents source behavior, not browser or accessibility certification.
