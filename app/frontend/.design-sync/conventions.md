# CARE Desktop UI: how to build with it

CARE Desktop is a calm, clinical desktop app: a dark-green sidebar on the left, a light grey page on the right, white cards, and green (`brand`) for the one main action. Everything is on `window.CareUI` (components plus every `lucide-react` icon, e.g. `CareUI.HardDrive`). No provider or theme wrapper is needed. Mount `<Toaster />` once if you call `toast("…")`.

## Styling: Tailwind utility classes on CARE tokens

Style your own layout with Tailwind classes. The CSS is **precompiled** (no JIT), so stick to the vocabulary below and the classes used in the component examples. Invented arbitrary values like `w-[437px]` will not resolve.

| Family | Classes |
|---|---|
| Text colour | `text-ink` (headings), `text-ink2` (body/labels), `text-muted-foreground` (secondary), `text-faint` (hints), `text-brand-ink`, `text-danger-ink`, `text-warn-ink`, `text-white` |
| Surfaces | `bg-background` (page, #f9fafb), `bg-card`/`bg-white`, `bg-hair` (grey tint), `bg-brand-bg`, `bg-danger-bg`, `bg-warn-bg`, `bg-brand-deep` (dark green hero/sidebar) |
| Borders | `border border-line`, `border-hair` (row dividers), `border-brand-line`, `border-danger-line`, `border-warn-line` |
| Solid accents | `bg-brand`, `bg-danger`, `bg-warn` |
| Radius / shadow | `rounded-md` (9px controls), `rounded-lg`, `rounded-xl` (cards), `rounded-2xl` (dialogs), `rounded-full`; `shadow-card`, `shadow-pop` |
| Type | `text-2xl font-bold` page title; `text-[15px] font-semibold` section title; `text-[14.5px] font-semibold` card title; `text-sm`/`text-[13px]` body; `text-[12.5px]` helper text; kicker `text-[11.5px] font-bold tracking-[0.06em] uppercase`; `font-mono` for addresses and paths |
| Spacing / layout | `flex`, `grid`, `grid-cols-2`, `gap-2…gap-6`, `p-5`, `p-6`, `px-5`, `py-4`, `space-y-3`, `min-w-0 flex-1`, `mx-auto max-w-2xl` |

The font is IBM Plex Sans / Mono (bundled). All tokens are CSS variables (`var(--color-brand)`, `--color-ink`, `--shadow-card` …) defined in `_ds_bundle.css`, which `styles.css` imports. Read that file and each component's `.prompt.md` before styling.

## App frame

A page is a full-height row: a 318px sidebar (`w-[318px] flex-none bg-brand-deep px-6 py-[26px] text-brand-bg`, nav items `rounded-lg px-3 py-[11px] text-[13.5px] font-semibold text-[#d3f5e5]`, active item `bg-white/[0.14] text-white`) plus `Screen` (`ScreenHead` / `ScreenBody` / `ScreenFoot`). The sidebar isn't a library component, so compose it from these classes.

## Rules that matter

- One `variant="primary"` button per view. Use `default` for everything else, `soft` for the secondary positive action, and `destructive` only for irreversible actions.
- `white`, `glass` and `outlineGlass` buttons go only on `bg-brand-deep` surfaces.
- Status is shown with `Badge` (`ok`, `warn`, `bad`, `default`) next to a `SectionTitle`, and with `Alert` (`info`, `warn`, `danger`) for sentences.
- `InputBox` is `flex-1`: put it inside a `flex` row, otherwise it collapses to zero height. The `Input` inside it must be stripped: `className="h-full flex-1 rounded-none border-none bg-transparent px-0 focus-visible:border-none"`.
- `Checkbox` is red when checked, because it only gates destructive choices. Use `Switch` or `RadioChip` for normal settings.
- Card has no padding of its own, so add `p-5` or `p-6`.

## Example

```jsx
const { Screen, ScreenHead, ScreenBody, ScreenFoot, Card, SectionTitle, Badge, Button, StorageRow, HardDrive } = window.CareUI;

<Screen>
  <ScreenHead kicker="Control panel" title="Overview" subtitle="Everything staff need to reach the clinic." />
  <ScreenBody className="flex flex-col gap-3">
    <Card className="flex items-center gap-3 p-5">
      <SectionTitle title="Clinic is running" summary="Staff can open https://care.local" />
      <Badge variant="ok">All good</Badge>
    </Card>
    <Card className="overflow-hidden">
      <StorageRow icon={HardDrive} label="Macintosh HD" path="/" free={182 * 2 ** 30} total={494 * 2 ** 30} level="ok" message="Plenty of room." />
    </Card>
  </ScreenBody>
  <ScreenFoot>
    <Button variant="primary">Back up now</Button>
    <Button>View backups</Button>
  </ScreenFoot>
</Screen>
```
