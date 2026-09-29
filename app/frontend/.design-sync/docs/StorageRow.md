---
category: Storage
---

One disk row: tinted icon tile, label + monospace path, a level `Badge` ("Enough room" / "Getting full" / "Full"), a `StorageMeter`, a status message and "X GB free of Y GB". Props: `label`, `path`, `free`, `total` (bytes), `level`, `message`, `note`, `icon` (a lucide icon component, default `HardDrive`), `action` (e.g. a `size="sm"` Button). Stack several inside a `Card className="overflow-hidden"`, separating rows with `className="border-t border-hair"`.
