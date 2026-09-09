import { useCallback, useEffect, useState } from "react";

import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { bridge } from "@/lib/bridge";
import { errorText } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useCare } from "@/state/care-store";
import { toast } from "@/components/ui/sonner";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import type { Section } from "@/types";

type Entry = {
  kind: "comment" | "blank" | "kv";
  raw?: string;
  key?: string;
  value?: string;
  isNew?: boolean;
};

// Comments, blank lines and the order they appear in are preserved: the file is
// round-tripped, not regenerated, so hand edits made outside the app survive.
function parseEnv(text: string): Entry[] {
  const lines = text.split(/\r?\n/);
  if (lines.length && lines[lines.length - 1] === "") lines.pop();
  return lines.map((line): Entry => {
    if (line.trim() === "") return { kind: "blank" };
    if (line.trimStart().startsWith("#")) return { kind: "comment", raw: line };
    const m = line.match(/^([A-Za-z_][A-Za-z0-9_]*)=(.*)$/);
    return m ? { kind: "kv", key: m[1], value: m[2] } : { kind: "comment", raw: line };
  });
}

function serializeEnv(entries: Entry[]): string {
  return (
    entries
      .map((e) => {
        if (e.kind === "comment") return e.raw ?? "";
        if (e.kind === "blank") return "";
        if (!e.key || e.key.trim() === "") return null;
        return `${e.key}=${e.value ?? ""}`;
      })
      .filter((line): line is string => line !== null)
      .join("\n") + "\n"
  );
}

const ADVANCED_PREFIXES = [
  "POSTGRES_", "MINIO_", "BUCKET_", "CELERY_", "SNS_", "DJANGO_SECURE_",
  "REACT_PUBLIC", "REACT_SENTRY", "REACT_APP_META",
];
const ADVANCED_KEYS = new Set([
  "DATABASE_URL", "REDIS_URL", "DJANGO_SECRET_KEY", "DJANGO_SETTINGS_MODULE",
  "PYTHONPATH", "DJANGO_ALLOWED_HOSTS", "DJANGO_DEBUG", "DJANGO_ADMIN_URL",
  "CSRF_TRUSTED_ORIGINS", "FILE_UPLOAD_BUCKET", "FACILITY_S3_BUCKET",
]);

function isAdvancedKey(key: string): boolean {
  return ADVANCED_KEYS.has(key) || ADVANCED_PREFIXES.some((p) => key.startsWith(p));
}

const CELL = "h-9 rounded-[8px] px-[11px] text-[13px]";

function EnvRow({
  entry,
  onKeyChange,
  onValueChange,
  onRemove,
}: {
  entry: Entry;
  onKeyChange: (value: string) => void;
  onValueChange: (value: string) => void;
  onRemove: () => void;
}) {
  return (
    <div className="flex items-center gap-2.5">
      {entry.isNew ? (
        <Input
          className={cn(CELL, "w-[250px] flex-none font-mono")}
          placeholder="NEW_KEY"
          spellCheck={false}
          value={entry.key ?? ""}
          onChange={(e) => onKeyChange(e.target.value)}
        />
      ) : (
        <label className="w-[250px] flex-none truncate font-mono text-[12.5px] font-medium text-muted-foreground">
          {entry.key}
        </label>
      )}
      <Input
        className={cn(CELL, "flex-1")}
        spellCheck={false}
        value={entry.value ?? ""}
        onChange={(e) => onValueChange(e.target.value)}
      />
      {entry.isNew ? (
        <Button
          size="icon"
          title="remove"
          onClick={onRemove}
          className="border-danger-line text-danger-ink hover:border-danger-line hover:bg-danger-bg hover:text-danger-ink"
        >
          ×
        </Button>
      ) : null}
    </div>
  );
}

export function EnvEditor({ section }: { section: Section }) {
  const { busy, runAction, log } = useCare();
  const [entries, setEntries] = useState<Entry[]>([]);
  const [advancedOpen, setAdvancedOpen] = useState(false);

  const load = useCallback(async () => {
    try {
      setEntries(parseEnv(await bridge.ReadEnv(section)));
    } catch (e) {
      setEntries([{ kind: "comment", raw: `# could not read ${section}.env: ${errorText(e)}` }]);
    }
  }, [section]);

  useEffect(() => {
    setAdvancedOpen(false);
    void load();
  }, [load]);

  const patchEntry = (index: number, values: Partial<Entry>) =>
    setEntries((prev) => prev.map((e, i) => (i === index ? { ...e, ...values } : e)));

  const editable = entries
    .map((entry, index) => ({ entry, index }))
    .filter(({ entry }) => entry.kind === "kv" && entry.key !== "ADDITIONAL_PLUGS");
  const everyday = editable.filter(
    ({ entry }) => entry.isNew || !isAdvancedKey(entry.key ?? ""),
  );
  const advanced = editable.filter(
    ({ entry }) => !entry.isNew && isAdvancedKey(entry.key ?? ""),
  );

  const rowFor = ({ entry, index }: { entry: Entry; index: number }) => (
    <EnvRow
      key={index}
      entry={entry}
      onKeyChange={(key) => patchEntry(index, { key })}
      onValueChange={(value) => patchEntry(index, { value })}
      onRemove={() => setEntries((prev) => prev.filter((_, i) => i !== index))}
    />
  );

  const save = async () => {
    if (busy) return;
    const backend = section === "backend";
    try {
      await bridge.WriteEnv(section, serializeEnv(entries));
      toast(backend ? "Settings applied" : "Rebuilding the app with new settings");
      await runAction(backend ? "start" : "rebuild-frontend");
    } catch (e) {
      log(`error saving env: ${errorText(e)}`);
      toast("Couldn't save settings");
    }
  };

  return (
    <>
      <div className="flex items-center gap-[9px] text-xs font-bold tracking-[0.05em] text-muted-foreground uppercase">
        <span>Everyday settings</span>
        <span className="rounded-full bg-brand-bg px-2.5 py-[3px] font-mono text-[11.5px] font-semibold tracking-normal text-brand-ink normal-case">
          {section}.env
        </span>
      </div>

      <div className="flex flex-col gap-[7px]">{everyday.map(rowFor)}</div>

      <Accordion
        type="single"
        collapsible
        value={advancedOpen ? "advanced" : ""}
        onValueChange={(v) => setAdvancedOpen(v === "advanced")}
      >
        <AccordionItem value="advanced" className="rounded-lg shadow-none">
          <AccordionTrigger className="gap-2.5 px-[13px] py-[11px] text-[13px] font-semibold text-ink2">
            <span className="flex-1">Advanced environment</span>
            <span className="font-mono text-faint">{advanced.length}</span>
          </AccordionTrigger>
          <AccordionContent className="gap-2.5 px-[13px] pb-[13px]">
            <Alert variant="danger">
              <span className="flex size-[17px] flex-none items-center justify-center rounded-full border border-danger-line font-mono text-[11px] font-bold">
                !
              </span>
              <span>Editing these can stop CARE from working.</span>
            </Alert>
            <div className="flex flex-col gap-[7px]">{advanced.map(rowFor)}</div>
          </AccordionContent>
        </AccordionItem>
      </Accordion>

      <div className="flex items-center gap-2.5">
        <Button
          disabled={busy}
          onClick={() =>
            setEntries((prev) => [...prev, { kind: "kv", key: "", value: "", isNew: true }])
          }
        >
          Add setting
        </Button>
        <span className="flex-1" />
        <Button variant="primary" disabled={busy} onClick={() => void save()}>
          {section === "backend" ? "Save and apply" : "Save and rebuild app"}
        </Button>
      </div>
    </>
  );
}
