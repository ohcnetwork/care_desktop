import { Input, Label } from "care-desktop-ui";

export const WithInput = () => (
  <div className="flex w-[360px] flex-col">
    <Label htmlFor="adminpw" className="mb-2 block">
      Admin password
    </Label>
    <Input id="adminpw" type="password" placeholder="At least 12 characters" />
  </div>
);
