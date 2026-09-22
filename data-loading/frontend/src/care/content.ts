import { api, listAll, Paginated } from "@/lib/api";

export type QuestionnaireFixture = {
  id?: string;
  slug: string;
  title: string;
  description?: string;
  version?: string;
  status: string;
  subject_type: string;
  styling_metadata?: Record<string, unknown>;
  questions: unknown[];
};

export type TemplateFixture = {
  name: string;
  slug_value: string;
  description?: string;
  template_type: string;
  context: string;
  default_format: string;
  status: string;
  template_data: string;
  options?: Record<string, unknown>;
};

export type Questionnaire = { id: string; slug: string; title: string };
export type Template = { id: string; slug: string; name: string };

export async function findQuestionnaire(slug: string): Promise<Questionnaire | null> {
  const page = await api.get<Paginated<Questionnaire>>(
    `/questionnaire/?slug=${encodeURIComponent(slug)}&limit=1`,
  );
  return page.results.find((q) => q.slug === slug) ?? null;
}

// Instance-level questionnaires are shared with organizations through a separate
// endpoint; the create payload only carries the questionnaire itself.
export async function createQuestionnaire(
  fixture: QuestionnaireFixture,
  organizations: string[],
): Promise<Questionnaire> {
  const { id: _id, ...body } = fixture;
  const created = await api.post<Questionnaire>("/questionnaire/", {
    ...body,
    auth_context: "instance",
  });
  if (organizations.length > 0) {
    await api.post(`/questionnaire/${created.id}/set_organizations/`, { organizations });
  }
  return created;
}

export function listTemplates(facilityId: string): Promise<Template[]> {
  return listAll<Template>(`/template/?facility=${facilityId}`);
}

export function createTemplate(fixture: TemplateFixture, facilityId: string): Promise<Template> {
  return api.post<Template>("/template/", { ...fixture, options: fixture.options ?? {}, facility: facilityId });
}
