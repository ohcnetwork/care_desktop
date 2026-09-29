---
category: Forms
---

Styled dropdown built on Radix Select (identical on every OS). Compose: `<Select value onValueChange><SelectTrigger aria-label="…"><SelectValue placeholder="…" /></SelectTrigger><SelectContent><SelectItem value="a">Option A</SelectItem></SelectContent></Select>`. `SelectGroup` + `SelectLabel` group items under a small heading. The trigger matches `Input` (42px, `line` border); the list is a white popover with `shadow-pop`, checked item shows a check mark. Set a width on `SelectTrigger` via `className` (e.g. `w-[220px]`) when not full width.
