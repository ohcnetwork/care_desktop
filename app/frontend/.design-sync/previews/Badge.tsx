import { Badge } from "care-desktop-ui";

export const Variants = () => (
  <div className="flex flex-wrap items-center gap-2">
    <Badge>To do</Badge>
    <Badge variant="ok">All good</Badge>
    <Badge variant="warn">Getting full</Badge>
    <Badge variant="bad">2 issues</Badge>
    <Badge variant="plain">Manual</Badge>
    <Badge variant="plainOk">Encrypted</Badge>
  </div>
);

export const Small = () => (
  <div className="flex flex-wrap items-center gap-2">
    <Badge size="sm" variant="ok">Enough room</Badge>
    <Badge size="sm" variant="warn">Getting full</Badge>
    <Badge size="sm" variant="bad">Full</Badge>
    <Badge size="sm" variant="plain">Daily</Badge>
  </div>
);
