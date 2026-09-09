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

export type SeedMember = {
  username: string;
  first_name: string;
  last_name: string;
  email: string;
  phone_number: string;
  gender: string;
  role: string;
  password: string;
};

export type ClinicSeed = {
  geo_organization: string;
  facility: {
    name: string;
    facility_type: string;
    address: string;
    pincode: string;
    phone_number: string;
    description: string;
  };
  members: SeedMember[];
};

/** Which of the two .env files / plugin sets an editor is pointed at. */
export type Section = "backend" | "frontend";
