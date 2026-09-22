import index from "../../../data/activity-definitions/index.json";
import { api, listAll } from "@/lib/api";
import { runBatch, type BatchProgress, type BatchReport } from "@/lib/batch";
import { slugify } from "@/lib/format";

export type Coding = { system: string; code: string; display: string };

export type ActivityDefinitionRow = {
  source: string;
  title: string;
  slug_value: string;
  description: string;
  usage: string;
  status: string;
  classification: string;
  category: string;
  code: Coding;
  body_site: Coding | null;
  diagnostic_report_codes: Coding[];
  specimen_slugs: string[];
  observation_slugs: string[];
  charge_item_slugs: string[];
  derived_from_uri: string | null;
};

export type Category = {
  key: string;
  name: string;
  count: number;
  classifications: string[];
  needs: { specimens: number; observations: number; charge_items: number };
};

export const CATEGORIES: Category[] = index as Category[];

const files = import.meta.glob<{ default: { category: string; items: ActivityDefinitionRow[] } }>(
  "../../../data/activity-definitions/*.json",
);

export async function loadCategoryRows(key: string): Promise<ActivityDefinitionRow[]> {
  const loader = files[`../../../data/activity-definitions/${key}.json`];
  if (!loader) throw new Error(`no data for category ${key}`);
  return (await loader()).default.items;
}

type Slugged = { id: string; slug: string; slug_config?: { slug_value?: string } };
type ResourceCategory = { id: string; slug: string; title: string; resource_type: string };

const bySlugValue = (list: Slugged[]) => {
  const map = new Map<string, string>();
  for (const item of list) {
    const value = item.slug_config?.slug_value ?? item.slug.replace(/^(f-[0-9a-f-]{36}-|i-)/, "");
    map.set(value, item.slug);
  }
  return map;
};

async function ensureCategory(facilityId: string, name: string): Promise<string> {
  const existing = await listAll<ResourceCategory>(
    `/facility/${facilityId}/resource_category/?resource_type=activity_definition&title=${encodeURIComponent(name)}`,
  );
  const found = existing.find((c) => c.title.trim().toLowerCase() === name.trim().toLowerCase());
  if (found) return found.slug;
  const created = await api.post<ResourceCategory>(`/facility/${facilityId}/resource_category/`, {
    title: name,
    description: "",
    resource_type: "activity_definition",
    resource_sub_type: "other",
    slug_value: slugify(`${name}-activity-definition`),
    parent: null,
    is_child: false,
  });
  return created.slug;
}

export async function loadActivityDefinitions(
  facilityId: string,
  keys: string[],
  onProgress: (p: BatchProgress) => void,
  concurrency = 3,
): Promise<BatchReport> {
  const rows = (await Promise.all(keys.map(loadCategoryRows))).flat();
  const [specimens, observations, charges, activities] = await Promise.all([
    listAll<Slugged>(`/facility/${facilityId}/specimen_definition/`),
    listAll<Slugged>(`/observation_definition/?facility=${facilityId}`),
    listAll<Slugged>(`/facility/${facilityId}/charge_item_definition/`),
    listAll<Slugged>(`/facility/${facilityId}/activity_definition/`),
  ]);
  const specimenSlugs = bySlugValue(specimens);
  const observationSlugs = bySlugValue(observations);
  const chargeSlugs = bySlugValue(charges);
  const existing = new Set(bySlugValue(activities).keys());

  const categorySlugs = new Map<string, string>();
  for (const name of new Set(rows.map((r) => r.category))) {
    categorySlugs.set(name, await ensureCategory(facilityId, name));
  }

  const resolve = (values: string[], known: Map<string, string>, what: string) =>
    values.map((v) => {
      const slug = known.get(v);
      if (!slug) throw new Error(`${what} ${v} is not loaded in this facility`);
      return slug;
    });

  return runBatch(
    rows,
    (r) => `${r.title} (${r.source})`,
    async (r) => {
      if (existing.has(r.slug_value)) return "skipped";
      await api.post(`/facility/${facilityId}/activity_definition/`, {
        title: r.title,
        slug_value: r.slug_value,
        status: r.status,
        description: r.description,
        usage: r.usage,
        classification: r.classification,
        kind: "service_request",
        code: r.code,
        body_site: r.body_site,
        diagnostic_report_codes: r.diagnostic_report_codes,
        derived_from_uri: r.derived_from_uri,
        locations: [],
        specimen_requirements: resolve(r.specimen_slugs, specimenSlugs, "specimen"),
        observation_result_requirements: resolve(r.observation_slugs, observationSlugs, "observation"),
        charge_item_definitions: resolve(r.charge_item_slugs, chargeSlugs, "charge item"),
        healthcare_service: null,
        category: categorySlugs.get(r.category) ?? null,
      });
      existing.add(r.slug_value);
      return "created";
    },
    onProgress,
    concurrency,
  );
}
