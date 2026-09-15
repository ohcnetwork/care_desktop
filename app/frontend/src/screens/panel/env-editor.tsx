// The clinic's settings, drawn from backend.env and frontend.env as one page of
// plain-language controls (see env-schema.ts), plus an "Other settings" list for
// any key the page does not describe. Both files are round-tripped: only the
// lines the operator changed are rewritten, everything else stays byte for byte.
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "@/components/ui/sonner";
import { bridge } from "@/lib/bridge";
import {
  applyChanges,
  ENV_KEY_RE,
  getValue,
  parseEnv,
  serializeEnv,
  type EnvChange,
  type EnvLine,
} from "@/lib/env-file";
import { errorText, firstLine } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useCare } from "@/state/care-store";
import type { Section } from "@/types";
import { normaliseLogo, SettingControl, splitList } from "./env-controls";
import {
  fileForKey,
  GROUPS,
  isHiddenKey,
  MANAGED_NOTES,
  SETTING_BY_KEY,
  SETTINGS,
  type Setting,
} from "./env-schema";

const SECTIONS: Section[] = ["backend", "frontend"];

type Files = Record<Section, { text: string; lines: EnvLine[] }>;

/** Raw value per described key; undefined means the key is not in the file. */
type Draft = Record<string, string | undefined>;

type CustomRow = {
  uid: number;
  key: string;
  value: string;
  file: Section;
  /** Typed in this session, so the name is still editable. */
  isNew: boolean;
};

// What the file means, as the controls should show it: a shipped placeholder
// like EMAIL_HOST=123 reads as "not set".
function initialDraft(files: Files): Draft {
  const draft: Draft = {};
  for (const s of SETTINGS) {
    const v = getValue(files[s.file].lines, s.key);
    draft[s.key] = v !== undefined && "blank" in s && s.blank?.includes(v) ? undefined : v;
  }
  return draft;
}

function customRows(files: Files, take: () => number): CustomRow[] {
  const rows: CustomRow[] = [];
  for (const file of SECTIONS) {
    const seen = new Set<string>();
    for (const l of files[file].lines) {
      if (l.kind !== "kv" || SETTING_BY_KEY.has(l.key) || isHiddenKey(l.key) || seen.has(l.key)) continue;
      seen.add(l.key);
      rows.push({ uid: take(), key: l.key, value: getValue(files[file].lines, l.key) ?? "", file, isNew: false });
    }
  }
  return rows;
}

// "Same as before" has to account for what CARE does when a key is absent: an
// untouched radio shows its fallback, and picking that fallback for a key that
// was never in the file is not a change worth writing.
function effective(s: Setting, v: string | undefined): string {
  switch (s.kind) {
    case "radio":
    case "select":
      return v ?? s.fallback;
    case "multi":
      return v === undefined ? s.fallback.join(",") : splitList(v).join(",");
    case "logo":
      return v === undefined ? "" : normaliseLogo(v);
    default:
      return (v ?? "").trim();
  }
}

function isDirty(s: Setting, draft: Draft, initial: Draft): boolean {
  return effective(s, draft[s.key]) !== effective(s, initial[s.key]);
}

function validate(s: Setting, v: string | undefined): string | null {
  switch (s.kind) {
    case "int": {
      const t = (v ?? "").trim();
      if (t === "") return s.required ? "Enter a number." : null;
      if (!/^-?\d+$/.test(t)) return "Enter a whole number.";
      const n = Number(t);
      if (s.min !== undefined && n < s.min) return `Must be at least ${s.min}.`;
      if (s.max !== undefined && n > s.max) return `Must be at most ${s.max}.`;
      return null;
    }
    case "multi":
      return v !== undefined && splitList(v).length === 0 ? "Choose at least one." : null;
    default:
      return null;
  }
}

export function EnvEditor({ adminPassword }: { adminPassword: string }) {
  const { busy, runAction, log } = useCare();
  const [files, setFiles] = useState<Files | null>(null);
  const [initial, setInitial] = useState<Draft>({});
  const [draft, setDraft] = useState<Draft>({});
  const [customInitial, setCustomInitial] = useState<CustomRow[]>([]);
  const [custom, setCustom] = useState<CustomRow[]>([]);
  const [problem, setProblem] = useState<string | null>(null);
  const nextUid = useRef(0);
  const take = () => nextUid.current++;

  const readFiles = useCallback(async (): Promise<Files> => {
    const [backend, frontend] = await Promise.all(
      SECTIONS.map((s) => bridge.ReadEnv(s, adminPassword)),
    );
    return {
      backend: { text: backend, lines: parseEnv(backend) },
      frontend: { text: frontend, lines: parseEnv(frontend) },
    };
  }, [adminPassword]);

  const load = useCallback(async () => {
    setProblem(null);
    try {
      const next = await readFiles();
      const d = initialDraft(next);
      const rows = customRows(next, take);
      setFiles(next);
      setInitial(d);
      setDraft(d);
      setCustomInitial(rows);
      setCustom(rows);
      setProblem("");
    } catch (e) {
      setFiles(null);
      setProblem(errorText(e));
    }
  }, [readFiles]);

  useEffect(() => {
    void load();
  }, [load]);

  const errors = useMemo(() => {
    const out: Record<string, string> = {};
    for (const s of SETTINGS) {
      const err = validate(s, draft[s.key]);
      if (err) out[s.key] = err;
    }
    const names = new Map<string, number>();
    for (const r of custom) {
      const k = r.key.trim();
      if (k === "" && r.value === "") continue;
      names.set(`${r.file}:${k}`, (names.get(`${r.file}:${k}`) ?? 0) + 1);
    }
    for (const r of custom) {
      const k = r.key.trim();
      if (k === "" && r.value === "") continue;
      if (k === "") out[`custom:${r.uid}`] = "Give the setting a name.";
      else if (!ENV_KEY_RE.test(k)) out[`custom:${r.uid}`] = "Letters, digits and _ only; no spaces.";
      else if ((names.get(`${r.file}:${k}`) ?? 0) > 1) out[`custom:${r.uid}`] = "Listed twice.";
    }
    return out;
  }, [draft, custom]);

  const dirtyKeys = useMemo(
    () => SETTINGS.filter((s) => isDirty(s, draft, initial)).map((s) => s.key),
    [draft, initial],
  );
  // Rows dropped since load, and rows (new or existing) whose text differs.
  const customDelta = useMemo(() => {
    const before = new Map(customInitial.map((r) => [r.uid, r]));
    const removed = customInitial.filter((r) => !custom.some((c) => c.uid === r.uid));
    const changed = custom.filter((r) =>
      r.isNew ? r.key.trim() !== "" || r.value !== "" : before.get(r.uid)?.value !== r.value,
    );
    return { removed, changed };
  }, [custom, customInitial]);

  const changeCount = dirtyKeys.length + customDelta.removed.length + customDelta.changed.length;
  const touched = (file: Section) =>
    dirtyKeys.some((k) => SETTING_BY_KEY.get(k)?.file === file) ||
    [...customDelta.removed, ...customDelta.changed].some((r) => r.file === file);

  const canSave =
    !busy && problem === "" && files !== null && changeCount > 0 && Object.keys(errors).length === 0;

  const save = async () => {
    if (!canSave || !files) return;
    const written: Section[] = [];
    try {
      // Re-read rather than trust the copy loaded earlier: the Plugins table
      // writes backend.env too, and a stale copy would silently undo it.
      const fresh = await readFiles();
      for (const file of SECTIONS) {
        const changes: EnvChange[] = [];
        for (const s of SETTINGS) {
          if (s.file !== file || !isDirty(s, draft, initial)) continue;
          const v = effective(s, draft[s.key]);
          changes.push({ key: s.key, value: v === "" ? undefined : v });
        }
        // Other settings go last so a name typed there wins over the page above.
        for (const r of customDelta.removed) {
          if (r.file === file) changes.push({ key: r.key, value: undefined });
        }
        for (const r of customDelta.changed) {
          if (r.file === file && r.key.trim() !== "") changes.push({ key: r.key.trim(), value: r.value });
        }
        const text = serializeEnv(applyChanges(fresh[file].lines, changes, file));
        if (text === fresh[file].text) continue;
        await bridge.WriteEnv(file, text, adminPassword);
        written.push(file);
      }
    } catch (e) {
      log(`error saving settings: ${errorText(e)}`);
      toast(firstLine(errorText(e)));
      return;
    }
    await load();
    if (written.length === 0) {
      toast("Nothing to apply");
      return;
    }
    // Start rebuilds the app too when frontend.env changed (the image is keyed
    // on it), so it covers both files; only an app-only change takes the
    // shorter rebuild path.
    const action = written.includes("backend") ? "start" : "rebuild-frontend";
    toast(action === "start" ? "Applying settings" : "Rebuilding the app with new settings");
    await runAction(action, adminPassword);
  };

  const discard = () => {
    setDraft(initial);
    setCustom(customInitial);
  };

  const patchCustom = (uid: number, values: Partial<CustomRow>) =>
    setCustom((prev) => prev.map((r) => (r.uid === uid ? { ...r, ...values } : r)));

  const disabled = busy || problem !== "" || files === null;
  const willRestart = touched("backend");
  const willRebuild = touched("frontend");

  return (
    <div className="@container flex flex-col gap-4">
      {problem ? (
        <Alert variant="danger">
          <span>{problem}</span>
          <Button disabled={busy} onClick={() => void load()}>Retry</Button>
        </Alert>
      ) : null}

      {GROUPS.map((g) => (
        <section key={g.id} className="rounded-lg border border-line">
          <header className="border-b border-hair px-4 py-3">
            <div className="text-[14px] font-semibold text-ink">{g.title}</div>
            <div className="mt-0.5 text-[12.5px] text-muted-foreground">{g.summary}</div>
          </header>
          <div className="px-4">
            {g.settings.map((s) => (
              <SettingRow
                key={s.key}
                setting={s}
                changed={dirtyKeys.includes(s.key)}
                error={errors[s.key]}
              >
                <SettingControl
                  setting={s}
                  value={draft[s.key]}
                  onChange={(v) => setDraft((prev) => ({ ...prev, [s.key]: v }))}
                  disabled={disabled}
                  invalid={errors[s.key] !== undefined}
                />
              </SettingRow>
            ))}
          </div>
        </section>
      ))}

      <section className="rounded-lg border border-line">
        <header className="border-b border-hair px-4 py-3">
          <div className="text-[14px] font-semibold text-ink">Other settings</div>
          <div className="mt-0.5 text-[12.5px] text-muted-foreground">
            For anything not listed above. Ask whoever supports your CARE before changing these.
          </div>
        </header>
        <div className="flex flex-col gap-[7px] p-4">
          {custom.length === 0 ? (
            <div className="py-1 text-[13px] text-faint">Nothing added yet.</div>
          ) : null}
          {custom.map((r) => (
            <CustomRowView
              key={r.uid}
              row={r}
              error={errors[`custom:${r.uid}`]}
              disabled={disabled}
              onKeyChange={(key) => patchCustom(r.uid, { key, file: fileForKey(key) })}
              onValueChange={(value) => patchCustom(r.uid, { value })}
              onRemove={() => setCustom((prev) => prev.filter((c) => c.uid !== r.uid))}
              existing={r.isNew && r.key.trim() !== "" && files ? getValue(files[r.file].lines, r.key.trim()) : undefined}
            />
          ))}
          <Button
            className="self-start"
            disabled={disabled}
            onClick={() =>
              setCustom((prev) => [...prev, { uid: take(), key: "", value: "", file: "backend", isNew: true }])
            }
          >
            Add setting
          </Button>
        </div>
      </section>

      <div className="flex flex-wrap items-center gap-3">
        <span className="min-w-[220px] flex-1 text-[13px] text-muted-foreground">
          {changeCount === 0
            ? "No unsaved changes."
            : `${changeCount} unsaved ${changeCount === 1 ? "change" : "changes"}. ` +
              (willRebuild && willRestart
                ? "Saving rebuilds the app and restarts the clinic; a few minutes."
                : willRebuild
                  ? "Saving rebuilds the app; a few minutes."
                  : "Saving restarts the clinic; about a minute.")}
        </span>
        <Button disabled={disabled || changeCount === 0} onClick={discard}>
          Discard
        </Button>
        <Button variant="primary" disabled={!canSave} onClick={() => void save()}>
          Save and apply
        </Button>
      </div>
    </div>
  );
}

function SettingRow({
  setting,
  changed,
  error,
  children,
}: {
  setting: Setting;
  changed: boolean;
  error?: string;
  children: ReactNode;
}) {
  return (
    <div className="grid grid-cols-1 gap-x-6 gap-y-2.5 border-t border-hair py-3.5 first:border-t-0 @lg:grid-cols-[minmax(0,1fr)_minmax(300px,360px)] @lg:items-center">
      <div className="min-w-0">
        <div className="flex items-center gap-2 text-[13.5px] font-semibold text-ink">
          <span>{setting.label}</span>
          {changed ? <Badge variant="plainOk" size="sm">changed</Badge> : null}
        </div>
        {setting.help ? (
          <div className="mt-0.5 text-[12.5px] leading-[1.5] text-muted-foreground">{setting.help}</div>
        ) : null}
        <div className="mt-1 font-mono text-[11px] text-faint">{setting.key}</div>
      </div>
      <div className="flex min-w-0 flex-col gap-1.5 @lg:items-end">
        {children}
        {error ? <div className="text-[12.5px] text-danger-ink">{error}</div> : null}
      </div>
    </div>
  );
}

const CELL = "h-9 rounded-[8px] px-[11px] text-[13px]";

function CustomRowView({
  row,
  error,
  disabled,
  existing,
  onKeyChange,
  onValueChange,
  onRemove,
}: {
  row: CustomRow;
  error?: string;
  disabled: boolean;
  /** For a name being typed: the value the file already holds for it. */
  existing?: string;
  onKeyChange: (key: string) => void;
  onValueChange: (value: string) => void;
  onRemove: () => void;
}) {
  const key = row.key.trim();
  const described = SETTING_BY_KEY.get(key);
  const note = !row.isNew
    ? null
    : MANAGED_NOTES[key]
      ? MANAGED_NOTES[key]
      : described
        ? `Already on this page as “${described.label}”. This value wins.`
        : existing !== undefined
          ? `Replaces the built-in value${existing ? ` (${existing})` : ""}.`
          : null;
  return (
    <div className="flex flex-col gap-1">
      <div className="flex flex-wrap items-center gap-2.5">
        {row.isNew ? (
          <Input
            className={cn(CELL, "w-[250px] max-w-full flex-none font-mono")}
            placeholder="SETTING_NAME"
            spellCheck={false}
            autoCapitalize="characters"
            value={row.key}
            onChange={(e) => onKeyChange(e.target.value)}
            disabled={disabled}
            aria-invalid={error !== undefined || undefined}
          />
        ) : (
          <label className="w-[250px] max-w-full flex-none truncate font-mono text-[12.5px] font-medium text-muted-foreground">
            {row.key}
          </label>
        )}
        <Input
          className={cn(CELL, "min-w-[160px] flex-1")}
          spellCheck={false}
          value={row.value}
          onChange={(e) => onValueChange(e.target.value)}
          disabled={disabled}
        />
        <Badge variant="plain" size="sm" className="w-[52px] justify-center font-mono">
          {row.file === "frontend" ? "app" : "server"}
        </Badge>
        <Button
          size="icon"
          title="remove"
          disabled={disabled}
          onClick={onRemove}
          className="border-danger-line text-danger-ink hover:border-danger-line hover:bg-danger-bg hover:text-danger-ink"
        >
          ×
        </Button>
      </div>
      {error ? (
        <div className="text-[12.5px] text-danger-ink">{error}</div>
      ) : note ? (
        <div className="text-[12.5px] text-muted-foreground">{note}</div>
      ) : null}
    </div>
  );
}
