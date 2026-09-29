# design-sync notes: CARE Desktop UI

Target project: "care desktop redesign" (projectId in config.json). Shape: package, no Storybook.

## How the DS package is produced

- The frontend is an app, not a library, so `.design-sync/ds/` turns it into one. `build.mjs` (= `cfg.buildCmd`) runs a Vite library build (`ds/vite.config.mjs`, entry `ds/entry.ts`) and `tsc` declarations (`ds/tsconfig.json`), then writes `.design-sync/.cache/pkg/{package.json,dist/index.js,dist/ds.css,dist/types}`.
- `tsc` keeps the `@/` imports in the `.d.ts` output. `build.mjs` rewrites them to relative paths and moves the entry `.d.ts` to `dist/types/index.d.ts`, because the converter's ts-morph glob skips dot-dirs and has no path mapping.
- `ds/index.ts` is the export list. Add new components there. `Rail` is left out on purpose: it reads everything from `useCare()` (the Wails-backed store), so it can't render outside the app. Its look is described in `conventions.md` instead.
- `ds/ds.css` compiles Tailwind v4 from `src/index.css`. It scans `src/` and `.design-sync/previews/` and adds an `@source inline(...)` safelist of token and layout utilities for the design agent. The CSS is static, so a class nobody uses is simply absent.
- **Order matters:** run `build.mjs` (Tailwind scans the previews) BEFORE `package-build.mjs`. If a preview adds a new class and only the converter reruns, that class renders unstyled.
- Converter invocation (from `app/frontend`): `node .ds-sync/package-build.mjs --config .design-sync/config.json --node-modules ./node_modules --out ./ds-bundle`. `cfg.entry` points at the generated dist.
- Fonts: IBM Plex latin subsets come from `node_modules/@fontsource/*` via `cfg.extraFonts`.
- `lucide-react` is merged onto `window.CareUI` (`cfg.extraEntries`), which takes the bundle to about 2 MB. lucide's `Badge` icon collides with our `Badge` and ours wins (`[EXPORT_COLLISION]` warning, expected).
- Compound sub-parts (`AccordionItem`, `SelectTrigger`, `ScreenHead`, `BoxNote`, `InfoButton` …) are excluded from cards via `componentSrcMap: null`. They're still on the global and are documented in each root's `.design-sync/docs/<Name>.md`, which also sets the group (category).

## Playwright

- The cached Chromium is `chromium-1243`, which needs `playwright@1.63.0` installed in `.ds-sync/`. Run validate/capture with `NODE_PATH=.ds-sync/node_modules`.

## Known render warns

- `Spinner` Sizes: the rings are captured mid-rotation, so the small ones look like arcs. Expected.
- `AlertDialog`: Cancel shows the focus ring because Radix autofocuses it. That's real behaviour.
- `[EXPORT_COLLISION]` lucide `Badge`: expected, see above.

## Re-sync risks

- `ds/index.ts` is hand-maintained. A new component file in `src/components` isn't synced until it's added there (and given a doc in `.design-sync/docs/`).
- The `ds/ds.css` safelist and the class vocabulary in `conventions.md` can drift from `src/index.css`. If tokens are renamed there, re-run the conventions validation (grep the names against `ds-bundle/_ds_bundle.css`).
- The previews hard-code realistic sample content copied from the screens (setup wizard, overview tab). The component APIs are checked at compile time, but copy/wording is not.
- `version` in the generated package.json comes from `app/wails.json` `info.productVersion`.
