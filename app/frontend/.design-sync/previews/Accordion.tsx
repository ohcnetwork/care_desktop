import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
  Badge,
  Button,
  SectionTitle,
  StepDot,
} from "care-desktop-ui";

export const SetupSteps = () => (
  <div className="w-[640px]">
    <Accordion type="single" collapsible defaultValue="backup">
      <AccordionItem value="checks">
        <AccordionTrigger>
          <StepDot done>1</StepDot>
          <SectionTitle title="Computer check" summary="Everything this clinic needs is ready" />
          <Badge variant="ok">All good</Badge>
        </AccordionTrigger>
        <AccordionContent>
          <p className="text-[13px] text-muted-foreground">Docker, network and disk checks passed.</p>
        </AccordionContent>
      </AccordionItem>
      <AccordionItem value="backup">
        <AccordionTrigger>
          <StepDot>2</StepDot>
          <SectionTitle title="Backup" summary="Drive and password for daily backups" />
          <Badge>To do</Badge>
        </AccordionTrigger>
        <AccordionContent>
          <p className="text-[13px] text-muted-foreground">
            Backups are saved every day at 04:30 to the folder below.
          </p>
          <div className="flex items-center gap-3 rounded-lg border border-line bg-background px-3.5 py-[13px]">
            <span className="min-w-0 flex-1 truncate font-mono text-[12.5px] text-ink2">
              /Users/clinic/Desktop/CARE Backups
            </span>
            <Button size="sm">Change</Button>
          </div>
        </AccordionContent>
      </AccordionItem>
      <AccordionItem value="admin">
        <AccordionTrigger>
          <StepDot>3</StepDot>
          <SectionTitle title="Admin login" summary="The first account for this clinic" />
          <Badge>To do</Badge>
        </AccordionTrigger>
        <AccordionContent>
          <p className="text-[13px] text-muted-foreground">Add staff accounts later from inside CARE.</p>
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  </div>
);

export const WithProblem = () => (
  <div className="w-[640px]">
    <Accordion type="single" collapsible>
      <AccordionItem value="checks" className="border-danger-line">
        <AccordionTrigger>
          <StepDot>1</StepDot>
          <SectionTitle title="Computer check" summary="Open to see what failed" />
          <Badge variant="bad">2 issues</Badge>
        </AccordionTrigger>
        <AccordionContent>
          <p className="text-[13px] text-muted-foreground">Docker is not running.</p>
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  </div>
);
