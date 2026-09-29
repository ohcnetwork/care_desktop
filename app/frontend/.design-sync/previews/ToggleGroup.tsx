import { ToggleGroup, ToggleGroupItem } from "care-desktop-ui";

export const MultipleChoice = () => (
  <ToggleGroup type="multiple" defaultValue={["en", "ml"]} aria-label="Languages">
    <ToggleGroupItem value="en">English</ToggleGroupItem>
    <ToggleGroupItem value="hi">Hindi</ToggleGroupItem>
    <ToggleGroupItem value="ml">Malayalam</ToggleGroupItem>
    <ToggleGroupItem value="ta">Tamil</ToggleGroupItem>
    <ToggleGroupItem value="kn">Kannada</ToggleGroupItem>
  </ToggleGroup>
);

export const SingleChoiceSmall = () => (
  <ToggleGroup type="single" size="sm" defaultValue="daily" aria-label="Backup schedule">
    <ToggleGroupItem value="hourly">Hourly</ToggleGroupItem>
    <ToggleGroupItem value="daily">Daily</ToggleGroupItem>
    <ToggleGroupItem value="weekly">Weekly</ToggleGroupItem>
  </ToggleGroup>
);
