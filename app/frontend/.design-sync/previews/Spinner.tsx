import { Button, Spinner } from "care-desktop-ui";

export const Sizes = () => (
  <div className="flex items-center gap-4 text-muted-foreground">
    <Spinner className="size-[11px]" />
    <Spinner className="size-3.5" />
    <Spinner />
    <Spinner className="size-6 text-brand" />
  </div>
);

export const InContext = () => (
  <div className="flex flex-col items-start gap-3">
    <Button disabled>
      <Spinner className="size-3.5" />
      Checking…
    </Button>
    <div className="flex items-start gap-2.5 text-[13px] text-ink2">
      <Spinner className="mt-0.5 flex-none text-brand" />
      <span>Starting the clinic. This takes about a minute.</span>
    </div>
  </div>
);
