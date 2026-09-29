import { Button, Spinner } from "care-desktop-ui";
import { ArrowLeft, Download } from "lucide-react";

export const Variants = () => (
  <div className="flex flex-wrap items-center gap-2">
    <Button>View backups</Button>
    <Button variant="primary">Start</Button>
    <Button variant="soft">Back up now</Button>
    <Button variant="destructive">Remove CARE</Button>
    <Button variant="ghost">
      <ArrowLeft className="size-4" strokeWidth={2.2} />
      Back
    </Button>
  </div>
);

export const Sizes = () => (
  <div className="flex flex-wrap items-center gap-2">
    <Button size="sm">Free up space</Button>
    <Button>Check again</Button>
    <Button variant="primary" size="lg">
      Install and start
    </Button>
    <Button size="icon" aria-label="Download">
      <Download className="size-3.5" />
    </Button>
  </div>
);

export const OnDarkSurface = () => (
  <div className="flex flex-wrap gap-2 rounded-xl bg-brand-deep p-5">
    <Button variant="white">Open</Button>
    <Button variant="glass">Copy</Button>
    <Button variant="outlineGlass">Open docs</Button>
  </div>
);

export const States = () => (
  <div className="flex flex-wrap items-center gap-2">
    <Button disabled>
      <Spinner className="size-3.5" />
      Checking…
    </Button>
    <Button variant="primary" disabled>
      Install and start
    </Button>
  </div>
);

export const Block = () => (
  <div className="w-[320px]">
    <Button variant="primary" size="block">
      Continue
    </Button>
  </div>
);
