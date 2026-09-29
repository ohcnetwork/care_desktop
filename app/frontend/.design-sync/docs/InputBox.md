---
category: Forms
---

A 42px bordered box that holds an `Input` plus an adornment (a ".local" suffix, a Show/Hide button, a match note) and carries the validity colour via `tone` (`neutral` | `ok` | `bad`). Pair with `BoxNote` (props `tone`, `children`): the small bold word at the right edge ("Match", ".local"). The inner Input must be stripped of its own border: `className="h-full flex-1 rounded-none border-none bg-transparent px-0 focus-visible:border-none"`.
