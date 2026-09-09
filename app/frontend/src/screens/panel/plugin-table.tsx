// Backend plugins are pip packages baked into the backend image
// (ADDITIONAL_PLUGS, rebuild on save). Frontend plugins are CARE plug_config
// rows the browser loads at runtime (a database write, no rebuild). Same table,
// two adapters.
import { ChevronDown } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
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
import type { CarePlugin, FrontendPlugin, Section } from "@/types";

type ConfigRow = { key: string; value: string };
type Row = {
  uid: number;
  name: string;
  url: string;
  version: string;
  configs: ConfigRow[];
  meta?: Record<string, unknown>;
};
type CatalogEntry = { value: string; label: string; row: Omit<Row, "uid"> };

// Empty on purpose: this clinic ships no suggested plugins. Add entries here to
// offer them in the picker.
const BACKEND_CATALOG: CatalogEntry[] = [];
const FRONTEND_CATALOG: CatalogEntry[] = [];

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

function serializeFrontend(rows: Row[]): FrontendPlugin[] {
  return rows
    .filter((p) => p.name.trim() !== "" && p.url.trim() !== "")
    .map((p) => {
      const meta: Record<string, unknown> = {
        ...(p.meta ?? {}),
        name: p.name.trim(),
        url: p.url.trim(),
      };
      const config: Record<string, unknown> = {};
      for (const c of p.configs) {
        if (c.key.trim() !== "") config[c.key.trim()] = parseConfigValue(c.value);
      }
      if (Object.keys(config).length) meta.config = config;
      else delete meta.config;
      return { slug: p.name.trim(), meta };
    });
}

const CELL =
  "h-[30px] rounded-sm border-transparent bg-transparent px-2 font-mono text-[12.5px] hover:border-line focus-visible:border-brand focus-visible:bg-white";

export function PluginTable({ section }: { section: Section }) {
  const { busy, runAction, log } = useCare();
  const [rows, setRows] = useState<Row[]>([]);
  const [expanded, setExpanded] = useState<ReadonlySet<number>>(new Set());
  const [status, setStatus] = useState("");
  // Frontend saves overwrite the whole plug_config set, so a save without a
  // clean baseline read could silently delete rows. Track whether we got one.
  const [frontendLoaded, setFrontendLoaded] = useState(false);
  const nextUid = useRef(0);

  const take = () => nextUid.current++;

  const loadFrontend = useCallback(async (): Promise<Row[]> => {
    const raw = (await bridge.ReadFrontendPlugins()) ?? [];
    return raw.map((p) => {
      const meta = (p.meta ?? {}) as Record<string, unknown>;
      const cfg = (meta.config ?? {}) as Record<string, unknown>;
      return {
        uid: nextUid.current++,
        name: p.slug,
        url: typeof meta.url === "string" ? meta.url : "",
        version: "",
        configs: Object.entries(cfg).map(([key, value]) => ({ key, value: String(value) })),
        meta,
      };
    });
  }, []);

  const load = useCallback(async () => {
    setExpanded(new Set());
    setStatus("");
    if (section === "backend") {
      setFrontendLoaded(false);
      try {
        const raw = await bridge.ReadPlugins();
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
      } catch {
        setRows([]);
      }
      return;
    }
    setFrontendLoaded(false);
    setStatus("Checking…");
    try {
      setRows(await loadFrontend());
      setFrontendLoaded(true);
      setStatus("");
    } catch (e) {
      setRows([]);
      setStatus(firstLine(errorText(e)));
    }
  }, [loadFrontend, section]);

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
    if (busy) return;
    const backend = section === "backend";
    try {
      if (backend) {
        await bridge.SavePlugins(serializeBackend(rows));
        toast("Rebuilding the backend");
        await runAction("rebuild-backend");
        return;
      }
      let current = rows;
      if (!frontendLoaded) {
        // The panel opened before CARE was ready, so we never got a clean
        // snapshot. Read one now — without a baseline a save could delete rows
        // we failed to read — then keep the operator's edits on top.
        const fresh = await loadFrontend();
        const bySlug = new Map(fresh.map((p) => [p.name, p]));
        for (const p of rows) if (p.name.trim() || p.url.trim()) bySlug.set(p.name, p);
        current = [...bySlug.values()];
        setRows(current);
        setFrontendLoaded(true);
        setStatus("");
      }
      await bridge.SaveFrontendPlugins(serializeFrontend(current));
      toast("Frontend plugins saved — staff refresh their browser");
      setRows(await loadFrontend());
    } catch (e) {
      log(`error saving plugins: ${errorText(e)}`);
      toast(firstLine(errorText(e)));
    }
  };

  const catalog = section === "backend" ? BACKEND_CATALOG : FRONTEND_CATALOG;

  const addFromPicker = (value: string) => {
    if (value === "custom") {
      setRows((prev) => [
        ...prev,
        {
          uid: take(),
          name: "",
          url: "",
          version: "",
          configs: [],
          meta: section === "frontend" ? {} : undefined,
        },
      ]);
      return;
    }
    const entry = catalog.find((c) => c.value === value);
    if (entry) {
      setRows((prev) => [
        ...prev,
        { ...entry.row, uid: take(), configs: entry.row.configs.map((c) => ({ ...c })) },
      ]);
    }
  };

  return (
    <>
      <div>
        <div className="flex items-center gap-2.5 px-5 pb-2 text-[11.5px] font-bold tracking-[0.05em] text-faint uppercase">
          <span className="w-[190px] flex-none">Name</span>
          <span className="flex-1">Source</span>
          <span className="w-[90px] flex-none">Version</span>
          <span className="w-[26px] flex-none" />
        </div>

        <div className="overflow-hidden rounded-lg border border-line">
          {status ? (
            <div className="p-5 text-center text-[13px] text-faint">{status}</div>
          ) : rows.length === 0 ? (
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
                    placeholder={section === "backend" ? "care_hcx" : "care_hello_fe"}
                    value={row.name}
                    onChange={(e) => patchRow(row.uid, { name: e.target.value })}
                  />
                  <Input
                    className={cn(CELL, "flex-1")}
                    spellCheck={false}
                    placeholder={
                      section === "backend"
                        ? "git+https://github.com/org/repo.git"
                        : "https://host/assets/remoteEntry.js"
                    }
                    value={row.url}
                    onChange={(e) => patchRow(row.uid, { url: e.target.value })}
                  />
                  {section === "backend" ? (
                    <Input
                      className={cn(CELL, "w-[90px] flex-none")}
                      spellCheck={false}
                      placeholder="@main"
                      value={row.version}
                      onChange={(e) => patchRow(row.uid, { version: e.target.value })}
                    />
                  ) : (
                    <span className="w-[90px] flex-none px-2 font-mono text-[12.5px] text-faint">
                      latest
                    </span>
                  )}
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
        <Select value="" disabled={busy} onValueChange={addFromPicker}>
          <SelectTrigger className="w-auto min-w-[190px]" aria-label="Add a plugin">
            <SelectValue placeholder="Add a plugin" />
          </SelectTrigger>
          <SelectContent className="w-auto">
            {catalog.map((entry) => (
              <SelectItem key={entry.value} value={entry.value}>
                {entry.label}
              </SelectItem>
            ))}
            <SelectItem value="custom">
              {catalog.length ? "Custom, add by URL" : "Add by URL"}
            </SelectItem>
          </SelectContent>
        </Select>
        <span className="flex-1" />
        <Button variant="primary" disabled={busy} onClick={() => void save()}>
          {section === "backend" ? "Save and rebuild backend" : "Save frontend plugins"}
        </Button>
      </div>
    </>
  );
}
