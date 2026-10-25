# Brand assets

Balemoh uses the shared distribution from `araihu/assets`. The release commit,
archive SHA-256, `release.json` SHA-256 and allowlisted source/destination mappings
are recorded in `araihu-assets.json`.

The UI embeds those selected release files. Its header uses the Console Shell
`Brand.Logo` slot with explicit light and dark assets. This keeps the logo in
sync with the app's color scheme override, even when it differs from the OS.
The shell's single `ManagedLogo` URL cannot express both app-selected schemes.
Existing shell navigation, theme behavior and layout remain in the public
Console Shell components.

The shell's favicon URL, Apple touch icon and manifest icon entries use the
published Web/PWA package. `/ui/brand/` serves the embedded files locally, so
rendering the brand needs no request to GitHub or a CDN. The manifest describes
icons only and does not add offline or installation behavior.

The social preview is app-owned copy composed with the released dark logo.
To regenerate it after a deliberate brand update:

```sh
rsvg-convert --width 1280 --height 640 \
  ui/internal/view/static/social-v2.svg > ui/internal/view/static/social-v2.png
```

Update brand files only from a verified published archive. Confirm its tag
commit, archive checksum, release inventory and catalog entry hashes before
changing the pin. Run UI tests, regenerate templ files, and inspect the header
at 390 px and 1440 px with Goshtoso/Minimal and light/dark schemes.
