// The install is a long stream of build output; these patterns turn it into the
// steps the operator actually sees. First match wins, and progress only
// ever moves forward, so a late line from an earlier stage can't rewind the bar.
export type RunStep = { re: RegExp; pct: number; label: string };

export const RUN_STEPS: RunStep[] = [
  { re: /secret key/i, pct: 8, label: "Preparing the configuration" },
  { re: /backup image|backup encryption key/i, pct: 15, label: "Securing the backups" },
  { re: /Building the Caddy/i, pct: 22, label: "Building the secure gateway" },
  { re: /Cloning care|care_fe|frontend \(/i, pct: 36, label: "Downloading CARE" },
  { re: /Building the backend image/i, pct: 62, label: "Building the backend" },
  { re: /Building the frontend image/i, pct: 80, label: "Building the app" },
  { re: /Starting CARE/i, pct: 90, label: "Starting the services" },
  { re: /database migrations/i, pct: 94, label: "Setting up the database" },
  { re: /become healthy|CARE is up/i, pct: 97, label: "Waiting for CARE to answer" },
];
