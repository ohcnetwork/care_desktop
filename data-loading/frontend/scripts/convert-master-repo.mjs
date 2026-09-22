import { mkdirSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import ExcelJS from "exceljs";

const here = dirname(fileURLToPath(import.meta.url));
const dataDir = join(here, "..", "..", "data");
const workbookPath = join(dataDir, "master-repo.xlsx");
const outDir = join(dataDir, "activity-definitions");

const ACTIVITY_SHEETS = ["all_activity_definition", "procedure_activity_definition"];
const CLASSIFICATIONS = new Set(["laboratory", "imaging", "counselling", "surgical_procedure", "education"]);
const STATUSES = new Set(["draft", "active", "retired", "unknown"]);
const SLUG = /^[a-z0-9][a-z0-9_-]*$/;

const CLASSIFICATION_BY_CATEGORY = {
  biochemistry: "laboratory",
  microbiology: "laboratory",
  pathology: "laboratory",
  radiology: "imaging",
  radiodiagnosis: "imaging",
  procedures: "surgical_procedure",
};

const text = (cell) => {
  if (cell === null || cell === undefined) return "";
  if (typeof cell === "object") {
    if ("richText" in cell) return text(cell.richText.map((r) => r.text).join(""));
    if ("text" in cell) return text(cell.text);
    if ("result" in cell) return text(cell.result);
    if (cell instanceof Date) return cell.toISOString();
  }
  return String(cell).replace(/\s+/g, " ").trim();
};

const code = (value) => value.replace(/\.0+$/, "");
const list = (value) => value.split(",").map((s) => s.trim()).filter(Boolean);
const slugify = (value) =>
  value.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");

const coding = (system, value, display) =>
  system || value || display ? { system, code: code(value), display } : null;

function rows(sheet) {
  const header = [];
  sheet.getRow(1).eachCell({ includeEmpty: true }, (cell, i) => {
    header[i] = text(cell.value).toLowerCase();
  });
  const out = [];
  sheet.eachRow((row, n) => {
    if (n === 1) return;
    const record = {};
    let empty = true;
    row.eachCell({ includeEmpty: true }, (cell, i) => {
      if (!header[i]) return;
      const v = text(cell.value);
      if (v) empty = false;
      record[header[i]] = v;
    });
    if (!empty) out.push({ line: n, ...record });
  });
  return out;
}

function toActivity(r, sheet, warn) {
  const source = `${sheet}:${r.line}`;
  const codes = list(r.diagnostic_report_code);
  const displays = list(r.diagnostic_report_display);
  const reportCodes = codes.map((c, i) => ({
    system: r.diagnostic_report_system,
    code: code(c),
    display: displays[i] ?? "",
  }));
  let slug = r.slug_value;
  if (!slug && r.title) {
    slug = slugify(r.title);
    warn(`${source}: no slug, derived ${slug} from the title`);
  }
  let status = (r.status || "active").toLowerCase();
  if (!STATUSES.has(status)) {
    warn(`${source}: status ${JSON.stringify(r.status)} is not a status, using active`);
    status = "active";
  }
  let classification = (r.classification || "").toLowerCase();
  if (!classification && r.category_name) {
    classification = CLASSIFICATION_BY_CATEGORY[r.category_name.toLowerCase()] ?? "";
    if (classification) warn(`${source}: no classification, inferred ${classification} from ${r.category_name}`);
  }
  return {
    source,
    title: r.title,
    slug_value: slug,
    description: r.description ?? "",
    usage: r.usage ?? "",
    status,
    classification,
    category: r.category_name,
    code: coding(r.code_system, r.code_value, r.code_display),
    body_site: coding(r.body_site_system, r.body_site_code, r.body_site_display),
    diagnostic_report_codes: reportCodes,
    specimen_slugs: list(r.specimen_slugs ?? ""),
    observation_slugs: list(r.observation_slugs ?? ""),
    charge_item_slugs: list(r.charge_item_slugs ?? ""),
    derived_from_uri: r.derived_from_uri || null,
  };
}

const workbook = new ExcelJS.Workbook();
await workbook.xlsx.readFile(workbookPath);

const known = (name, column) =>
  new Set(rows(workbook.getWorksheet(name)).map((r) => r[column]).filter(Boolean));
const specimens = known("specimen_definition", "slug_value");
const observations = known("observation_definition", "slug_value");
const charges = known("all_charge_item_definition", "slug_value");

const problems = [];
const warnings = [];
const seen = new Map();
const byCategory = new Map();

for (const name of ACTIVITY_SHEETS) {
  const sheet = workbook.getWorksheet(name);
  if (!sheet) {
    problems.push(`missing sheet ${name}`);
    continue;
  }
  for (const r of rows(sheet)) {
    const a = toActivity(r, name, (w) => warnings.push(w));
    const where = a.source;
    if (!a.title) problems.push(`${where}: missing title`);
    if (!SLUG.test(a.slug_value)) problems.push(`${where}: bad slug ${JSON.stringify(a.slug_value)}`);
    if (!a.category) problems.push(`${where}: missing category_name`);
    if (!CLASSIFICATIONS.has(a.classification)) problems.push(`${where}: bad classification ${JSON.stringify(a.classification)}`);
    if (!a.code || !a.code.system || !a.code.code) problems.push(`${where}: missing code`);
    if (seen.has(a.slug_value)) problems.push(`${where}: duplicate slug ${a.slug_value} (also ${seen.get(a.slug_value)})`);
    seen.set(a.slug_value, where);
    for (const s of a.specimen_slugs) if (!specimens.has(s)) warnings.push(`${where}: unknown specimen ${s}`);
    for (const s of a.observation_slugs) if (!observations.has(s)) warnings.push(`${where}: unknown observation ${s}`);
    for (const s of a.charge_item_slugs) if (!charges.has(s)) warnings.push(`${where}: unknown charge item ${s}`);
    const key = slugify(a.category);
    if (!byCategory.has(key)) byCategory.set(key, { name: a.category, items: [] });
    byCategory.get(key).items.push(a);
  }
}

for (const w of warnings) console.warn(`warning: ${w}`);
if (problems.length) {
  for (const p of problems) console.error(`error: ${p}`);
  process.exit(1);
}

mkdirSync(outDir, { recursive: true });
for (const f of readdirSync(outDir)) rmSync(join(outDir, f));

const index = [];
for (const [key, { name, items }] of [...byCategory].sort((a, b) => a[1].name.localeCompare(b[1].name))) {
  items.sort((a, b) => a.title.localeCompare(b.title));
  writeFileSync(join(outDir, `${key}.json`), JSON.stringify({ category: name, items }, null, 1) + "\n");
  index.push({
    key,
    name,
    count: items.length,
    classifications: [...new Set(items.map((i) => i.classification))].sort(),
    needs: {
      specimens: new Set(items.flatMap((i) => i.specimen_slugs)).size,
      observations: new Set(items.flatMap((i) => i.observation_slugs)).size,
      charge_items: new Set(items.flatMap((i) => i.charge_item_slugs)).size,
    },
  });
}
writeFileSync(join(outDir, "index.json"), JSON.stringify(index, null, 2) + "\n");

console.log(`${seen.size} activity definitions in ${index.length} categories, ${warnings.length} warnings`);
for (const c of index) console.log(`  ${c.name}: ${c.count}`);
