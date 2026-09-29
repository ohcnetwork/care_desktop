---
category: Actions
---

The app-wide button. `variant="default"` is the quiet white outline button used for most actions; `primary` (solid green) marks the one main action on a screen; `soft` is a tinted secondary; `destructive` is for irreversible actions. `white`, `glass` and `outlineGlass` only go on dark `bg-brand-deep` surfaces. `ghost` is text-only (e.g. a Back link). Sizes: `default`, `sm` (30px, inline row actions), `lg` (hero CTA), `block` (full width), `icon` (26px square), `bare`. Put a lucide icon before the label as a child (`<ArrowLeft className="size-4" />`); the button handles the gap. `asChild` renders the styles onto a child element (e.g. an `<a>`).
