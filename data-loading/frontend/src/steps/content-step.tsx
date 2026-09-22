import { useState } from "react";

import questionnaireData from "../../../data/questionnaire_fixtures.json";
import templateData from "../../../data/template_fixtures.json";
import {
  createQuestionnaire,
  createTemplate,
  findQuestionnaire,
  listTemplates,
  type QuestionnaireFixture,
  type TemplateFixture,
} from "@/care/content";
import { BatchPanel } from "@/components/batch-panel";
import { Screen, ScreenBody, ScreenHead } from "@/components/screen";
import { StepFoot } from "@/components/step-foot";
import { Alert } from "@/components/ui/alert";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { runBatch, type BatchProgress } from "@/lib/batch";
import { errorText } from "@/lib/format";
import { useWizard } from "@/state/wizard";

const QUESTIONNAIRES = questionnaireData as QuestionnaireFixture[];
const TEMPLATES = templateData as TemplateFixture[];

function Picker<T>({
  title,
  items,
  keyOf,
  labelOf,
  selected,
  onToggle,
  disabled,
}: {
  title: string;
  items: T[];
  keyOf: (item: T) => string;
  labelOf: (item: T) => string;
  selected: Set<string>;
  onToggle: (key: string) => void;
  disabled: boolean;
}) {
  return (
    <div>
      <Label className="mb-2 block">{title}</Label>
      <div className="flex flex-wrap gap-2">
        {items.map((item) => {
          const k = keyOf(item);
          const on = selected.has(k);
          return (
            <label
              key={k}
              className={`flex cursor-pointer items-center gap-2 rounded-full border bg-white px-3 py-1.5 text-[12.5px] font-medium ${on ? "border-brand text-brand-ink" : "border-line text-muted-foreground"}`}
            >
              <Checkbox checked={on} disabled={disabled} onCheckedChange={() => onToggle(k)} />
              {labelOf(item)}
            </label>
          );
        })}
      </div>
    </div>
  );
}

export function ContentStep() {
  const { progress, complete, skip } = useWizard();
  const [questionnaires, setQuestionnaires] = useState(new Set(QUESTIONNAIRES.map((q) => q.slug)));
  const [templates, setTemplates] = useState(new Set(TEMPLATES.map((t) => t.slug_value)));
  const [busy, setBusy] = useState(false);
  const [qBatch, setQBatch] = useState<BatchProgress | null>(null);
  const [tBatch, setTBatch] = useState<BatchProgress | null>(null);
  const [problem, setProblem] = useState("");
  const [finished, setFinished] = useState(false);

  const toggle = (set: Set<string>, apply: (s: Set<string>) => void) => (key: string) => {
    const next = new Set(set);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    apply(next);
  };

  const run = async () => {
    setBusy(true);
    setProblem("");
    try {
      const organizations = [...new Set([...Object.values(progress.roleOrganizations), progress.districtId])].filter(Boolean);
      const qReport = await runBatch(
        QUESTIONNAIRES.filter((q) => questionnaires.has(q.slug)),
        (q) => q.title,
        async (q) => {
          if (await findQuestionnaire(q.slug)) return "skipped";
          await createQuestionnaire(q, organizations);
          return "created";
        },
        setQBatch,
        2,
      );
      const existing = await listTemplates(progress.facilityId);
      const present = new Set(existing.map((t) => t.slug.replace(/^(f-[0-9a-f-]{36}-|i-)/, "")));
      const tReport = await runBatch(
        TEMPLATES.filter((t) => templates.has(t.slug_value)),
        (t) => t.name,
        async (t) => {
          if (present.has(t.slug_value)) return "skipped";
          await createTemplate(t, progress.facilityId);
          return "created";
        },
        setTBatch,
        2,
      );
      if (qReport.failed === 0 && tReport.failed === 0) setFinished(true);
    } catch (e) {
      setProblem(errorText(e));
    } finally {
      setBusy(false);
    }
  };

  const nothing = questionnaires.size === 0 && templates.size === 0;

  return (
    <Screen>
      <ScreenHead
        kicker="Step 9"
        title="Forms and reports"
        subtitle="Standard questionnaires and report templates. Every role group can view, fill and submit the questionnaires."
      />
      <ScreenBody>
        <div className="flex max-w-[680px] flex-col gap-5">
          <Picker
            title="Questionnaires"
            items={QUESTIONNAIRES}
            keyOf={(q) => q.slug}
            labelOf={(q) => q.title}
            selected={questionnaires}
            onToggle={toggle(questionnaires, setQuestionnaires)}
            disabled={busy || finished}
          />
          <Picker
            title="Report templates"
            items={TEMPLATES}
            keyOf={(t) => t.slug_value}
            labelOf={(t) => t.name}
            selected={templates}
            onToggle={toggle(templates, setTemplates)}
            disabled={busy || finished}
          />
          {qBatch ? <BatchPanel title="Questionnaires" progress={qBatch} running={busy && !tBatch} /> : null}
          {tBatch ? <BatchPanel title="Report templates" progress={tBatch} running={busy} /> : null}
          {problem ? <Alert variant="danger">{problem}</Alert> : null}
        </div>
      </ScreenBody>
      <StepFoot
        primary={finished ? "Finish" : "Load selected"}
        primaryDisabled={!finished && nothing}
        onPrimary={finished ? () => complete("content") : () => void run()}
        busy={busy}
        onSkip={finished ? undefined : () => skip("content")}
      />
    </Screen>
  );
}
