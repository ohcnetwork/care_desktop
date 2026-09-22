import { api, ApiError, listAll } from "@/lib/api";

export type FacilityOrganization = {
  id: string;
  name: string;
  org_type: "dept" | "team" | "root" | "role" | "other";
  parent?: { id: string } | null;
};

export type FacilityLocation = {
  id: string;
  name: string;
  form: string;
  mode: "kind" | "instance";
};

export const ADMINISTRATION = "Administration";

export function listDepartments(facilityId: string): Promise<FacilityOrganization[]> {
  return listAll<FacilityOrganization>(`/facility/${facilityId}/organizations/`);
}

export function createDepartment(facilityId: string, name: string): Promise<FacilityOrganization> {
  return api.post<FacilityOrganization>(`/facility/${facilityId}/organizations/`, {
    name,
    org_type: "dept",
    description: "",
    active: true,
  });
}

export function listLocations(facilityId: string): Promise<FacilityLocation[]> {
  return listAll<FacilityLocation>(`/facility/${facilityId}/location/`);
}

export function createDepartmentLocation(
  facilityId: string,
  name: string,
  organizations: string[],
): Promise<FacilityLocation> {
  return api.post<FacilityLocation>(`/facility/${facilityId}/location/`, {
    name,
    description: "",
    form: "wa",
    status: "active",
    operational_status: "O",
    mode: "kind",
    parent: null,
    organizations,
  });
}

export async function listLocationOrganizations(facilityId: string, locationId: string): Promise<FacilityOrganization[]> {
  const data = await api.get<FacilityOrganization[] | { results: FacilityOrganization[] }>(
    `/facility/${facilityId}/location/${locationId}/organizations/`,
  );
  return Array.isArray(data) ? data : data.results;
}

const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));

export async function ensureLocationOrganization(
  facilityId: string,
  locationId: string,
  organizationId: string,
): Promise<boolean> {
  const linked = await listLocationOrganizations(facilityId, locationId);
  if (linked.some((o) => o.id === organizationId)) return false;
  for (let attempt = 0; ; attempt++) {
    try {
      await api.post(`/facility/${facilityId}/location/${locationId}/organizations_add/`, {
        organization: organizationId,
      });
      return true;
    } catch (e) {
      if (e instanceof ApiError && /already exists/i.test(e.message)) return false;
      if (e instanceof ApiError && e.status >= 500 && attempt < 2) {
        await wait(500 * (attempt + 1));
        continue;
      }
      throw e;
    }
  }
}
