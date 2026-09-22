# Facility setup page

The browser wizard served at `https://<instance>.local/seed-data` and the data it loads. See [docs/seed-data.md](../docs/seed-data.md) for how it works, how it is built into the kit, and how to update the master sheet.

```bash
cd frontend
npm ci
npm run dev        # against a local CARE backend on http://localhost:9000
npm run build      # -> ../../deployments/seed-data/
npm run convert    # ../data/master-repo.xlsx -> ../data/activity-definitions/
```
