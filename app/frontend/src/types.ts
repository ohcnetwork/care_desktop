// The shapes the Go bridge hands back. Mirrors app/internal/care — keep in step
// with wailsjs/go/models.ts, which Wails regenerates from the Go structs.

export type DockerStatus = { ok: boolean; message: string };
export type NameStatus = { ok: boolean; message: string; how: string };
export type NetworkStatus = {
  applicable: boolean;
  ok: boolean;
  message: string;
  how: string;
  fixable: boolean;
};
export type Health = { active: boolean; code: number; detail: string };

/** Whether this machine must restart before the prerequisites will work. */
export type RestartPlan = {
  needed: boolean;
  title: string;
  detail: string;
  label: string;
};

/** What the app can do about a prerequisite that isn't ready on this machine. */
export type ToolAction = "" | "install" | "open" | "manual";
export type ToolPlan = {
  tool: string;
  action: ToolAction;
  label: string;
  detail: string;
  needs_admin: boolean;
  url: string;
};
export type AppState = { setup_done: boolean; mdns_name: string; docker: DockerStatus };

export type Backup = {
  db_dump: string;
  files_archive: string;
  label: string;
  manual: boolean;
  encrypted: boolean;
  size_bytes: number;
};

export type CarePlugin = {
  name: string;
  package_name: string;
  version?: string;
  configs?: Record<string, unknown>;
};

export type FrontendPlugin = { slug: string; meta: Record<string, unknown> };

export type ClinicApp = {
  slug: string;
  name: string;
  description: string;
  enabled: boolean;
  managed: boolean;
  ready: boolean;
  url: string;
  warning: string;
  needs_backend_plug: string;
};

/** Which of the two .env files / plugin sets an editor is pointed at. */
export type Section = "backend" | "frontend";
