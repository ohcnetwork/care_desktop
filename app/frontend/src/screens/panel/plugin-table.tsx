// Backend plugins are pip packages baked into the backend image
// (ADDITIONAL_PLUGS), so saving rebuilds it. Frontend plugins are not here on
// purpose: CARE's own admin pages already manage them, live, without a rebuild.
import { ChevronDown } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { toast } from "@/components/ui/sonner";
import { bridge } from "@/lib/bridge";
import { errorText, firstLine } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useCare } from "@/state/care-store";
import type { CarePlugin } from "@/types";

type ConfigRow = { key: string; value: string };
type Row = {
  uid: number;
  name: string;
  url: string;
  version: string;
  configs: ConfigRow[];
};
type CatalogEntry = { value: string; label: string; row: Omit<Row, "uid"> };

// Empty on purpose: this clinic ships no suggested plugins. Add entries here to
// offer them in the picker.
const CATALOG: CatalogEntry[] = [];

function parseConfigValue(raw: string): unknown {
  const t = raw.trim();
  if (t === "true") return true;
  if (t === "false") return false;
  if (/^-?\d+$/.test(t)) return parseInt(t, 10);
  if (/^-?\d*\.\d+$/.test(t)) return parseFloat(t);
  return raw;
}

function serializeBackend(rows: Row[]): CarePlugin[] {
  return rows
    .filter((p) => p.name.trim() !== "" && p.url.trim() !== "")
    .map((p) => {
      const out: CarePlugin = { name: p.name.trim(), package_name: p.url.trim() };
      if (p.version.trim() !== "") out.version = p.version.trim();
      const configs: Record<string, unknown> = {};
      for (const c of p.configs) {
        if (c.key.trim() !== "") configs[c.key.trim()] = parseConfigValue(c.value);
      }
      if (Object.keys(configs).length) out.configs = configs;
      return out;
    });
}

const CELL =
  "h-[30px] rounded-sm border-transparent bg-transparent px-2 font-mono text-[12.5px] hover:border-line focus-visible:border-brand focus-visible:bg-white";

export function PluginTable({ adminPassword }: { adminPassword: string }) {
  const { busy, runAction, log } = useCare();
  const [rows, setRows] = useState<Row[]>([]);
  const [expanded, setExpanded] = useState<ReadonlySet<number>>(new Set());
  const nextUid = useRef(0);
  const [problem, setProblem] = useState<string | null>(null);

  const take = () => nextUid.current++;

  const load = useCallback(async () => {
    setExpanded(new Set());
    setProblem(null);
    try {
      const raw = await bridge.ReadPlugins(adminPassword);
      setRows(
        raw.map((p) => ({
          uid: nextUid.current++,
          name: p.name ?? "",
          url: p.package_name ?? "",
          version: p.version ?? "",
          configs: Object.entries(p.configs ?? {}).map(([key, value]) => ({
            key,
            value: String(value),
          })),
        })),
      );
      setProblem("");
    } catch (e) {
      setRows([]);
      setProblem(errorText(e));
    }
  }, [adminPassword]);

  useEffect(() => {
    void load();
  }, [load]);

  const patchRow = (uid: number, values: Partial<Row>) =>
    setRows((prev) => prev.map((r) => (r.uid === uid ? { ...r, ...values } : r)));

  const patchConfig = (uid: number, index: number, values: Partial<ConfigRow>) =>
    setRows((prev) =>
      prev.map((r) =>
        r.uid === uid
          ? { ...r, configs: r.configs.map((c, i) => (i === index ? { ...c, ...values } : c)) }
          : r,
      ),
    );

  const toggleExpanded = (uid: number) =>
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(uid)) next.delete(uid);
      else next.add(uid);
      return next;
    });

  const save = async () => {
    if (busy || problem !== "") return;
    try {
      await bridge.SavePlugins(serializeBackend(rows), adminPassword);
      toast("Rebuilding the backend");
      await runAction("rebuild-backend", adminPassword);
    } catch (e) {
      log(`error saving plugins: ${errorText(e)}`);
      toast(firstLine(errorText(e)));
    }
  };

  const addFromPicker = (value: string) => {
    if (value === "custom") {
      setRows((prev) => [
        ...prev,
        { uid: take(), name: "", url: "", version: "", configs: [] },
      ]);
      return;
    }
    const entry = CATALOG.find((c) => c.value === value);
    if (entry) {
      setRows((prev) => [
        ...prev,
        { ...entry.row, uid: take(), configs: entry.row.configs.map((c) => ({ ...c })) },
      ]);
    }
  };

  return (
    <>
      {problem ? (
        <Alert variant="danger">
          <span>{problem}</span>
          <Button disabled={busy} onClick={() => void load()}>Retry</Button>
        </Alert>
      ) : null}
      <div>
        <div className="flex items-center gap-2.5 px-5 pb-2 text-[11.5px] font-bold tracking-[0.05em] text-faint uppercase">
          <span className="w-[190px] flex-none">Name</span>
          <span className="flex-1">Source</span>
          <span className="w-[90px] flex-none">Version</span>
          <span className="w-[26px] flex-none" />
        </div>

        <div className="overflow-hidden rounded-lg border border-line">
          {rows.length === 0 ? (
            <div className="p-5 text-center text-[13px] text-faint">No plugins yet</div>
          ) : null}

          {rows.map((row, i) => {
            const open = expanded.has(row.uid);
            return (
              <div key={row.uid}>
                <div
                  className={cn(
                    "flex items-center gap-2.5 p-3",
                    i > 0 && "border-t border-hair",
                  )}
                >
                  <Input
                    className={cn(CELL, "w-[190px] flex-none text-[13px] font-semibold text-ink")}
                    spellCheck={false}
                    placeholder="care_hcx"
                    value={row.name}
                    onChange={(e) => patchRow(row.uid, { name: e.target.value })}
                  />
                  <Input
                    className={cn(CELL, "flex-1")}
                    spellCheck={false}
                    placeholder="git+https://github.com/org/repo.git"
                    value={row.url}
                    onChange={(e) => patchRow(row.uid, { url: e.target.value })}
                  />
                  <Input
                    className={cn(CELL, "w-[90px] flex-none")}
                    spellCheck={false}
                    placeholder="@main"
                    value={row.version}
                    onChange={(e) => patchRow(row.uid, { version: e.target.value })}
                  />
                  <Button
                    size="icon"
                    title="remove plugin"
                    className="border-danger-line text-danger-ink hover:border-danger-line hover:bg-danger-bg hover:text-danger-ink"
                    onClick={() => setRows((prev) => prev.filter((r) => r.uid !== row.uid))}
                  >
                    ×
                  </Button>
                </div>

                <button
                  type="button"
                  onClick={() => toggleExpanded(row.uid)}
                  className="flex w-full cursor-pointer items-center gap-[7px] px-3 pb-2.5 text-left text-[12.5px] font-semibold text-muted-foreground hover:text-brand-ink"
                >
                  <ChevronDown
                    className={cn("size-[13px] transition-transform", open && "rotate-180")}
                    strokeWidth={2.5}
                  />
                  <span>
                    Configuration{row.configs.length ? ` (${row.configs.length})` : ""}
                  </span>
                </button>

                {open ? (
                  <div className="flex flex-col gap-[7px] px-3 pb-3">
                    {row.configs.map((config, ci) => (
                      <div key={ci} className="flex items-center gap-2.5">
                        <Input
                          className="h-9 w-[250px] flex-none rounded-[8px] px-[11px] font-mono text-[13px]"
                          placeholder="CONFIG_KEY"
                          spellCheck={false}
                          value={config.key}
                          onChange={(e) => patchConfig(row.uid, ci, { key: e.target.value })}
                        />
                        <Input
                          className="h-9 flex-1 rounded-[8px] px-[11px] text-[13px]"
                          placeholder="value"
                          spellCheck={false}
                          value={config.value}
                          onChange={(e) => patchConfig(row.uid, ci, { value: e.target.value })}
                        />
                        <Button
                          size="icon"
                          className="border-danger-line text-danger-ink hover:border-danger-line hover:bg-danger-bg hover:text-danger-ink"
                          onClick={() =>
                            patchRow(row.uid, {
                              configs: row.configs.filter((_, i2) => i2 !== ci),
                            })
                          }
                        >
                          ×
                        </Button>
                      </div>
                    ))}
                    <Button
                      className="self-start"
                      onClick={() =>
                        patchRow(row.uid, { configs: [...row.configs, { key: "", value: "" }] })
                      }
                    >
                      Add config
                    </Button>
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      </div>

      <div className="flex items-center gap-2.5">
        {/* Controlled with an always-empty value so the picker acts as a menu:
            choosing an entry adds a row and the trigger falls back to its label. */}
        <Select value="" disabled={busy || problem !== ""} onValueChange={addFromPicker}>
          <SelectTrigger className="w-auto min-w-[190px]" aria-label="Add a plugin">
            <SelectValue placeholder="Add a plugin" />
          </SelectTrigger>
          <SelectContent className="w-auto">
            {CATALOG.map((entry) => (
              <SelectItem key={entry.value} value={entry.value}>
                {entry.label}
              </SelectItem>
            ))}
            <SelectItem value="custom">
              {CATALOG.length ? "Custom, add by URL" : "Add by URL"}
            </SelectItem>
          </SelectContent>
        </Select>
        <span className="flex-1" />
        <Button variant="primary" disabled={busy || problem !== ""} onClick={() => void save()}>
          Save and rebuild backend
        </Button>
      </div>
    </>
  );
}
