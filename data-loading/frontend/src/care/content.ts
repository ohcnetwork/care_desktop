import { api, listAll, ApiError } from "@/lib/api";

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
  try {
    return await api.get<Questionnaire>(`/questionnaire/${encodeURIComponent(slug)}/`);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) return null;
    throw e;
  }
}

export function createQuestionnaire(
  fixture: QuestionnaireFixture,
  organizations: string[],
): Promise<Questionnaire> {
  const { id: _id, ...body } = fixture;
  return api.post<Questionnaire>("/questionnaire/", { ...body, organizations });
}

export function listTemplates(facilityId: string): Promise<Template[]> {
  return listAll<Template>(`/template/?facility=${facilityId}`);
}

export function createTemplate(fixture: TemplateFixture, facilityId: string): Promise<Template> {
  return api.post<Template>("/template/", { ...fixture, options: fixture.options ?? {}, facility: facilityId });
}
