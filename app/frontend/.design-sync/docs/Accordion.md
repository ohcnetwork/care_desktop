---
category: Layout
---

Stack of collapsible white cards (the setup wizard sections). Compose `Accordion` (`type="single" collapsible` or `type="multiple"`, `value`/`defaultValue`) > `AccordionItem value="…"` > `AccordionTrigger` (header row: put `StepDot`, `SectionTitle` and a `Badge` in it; the chevron is added for you) + `AccordionContent` (body; lays children out as a column with 16px gaps). Red-outline an item that has a problem with `className="border-danger-line"`.
