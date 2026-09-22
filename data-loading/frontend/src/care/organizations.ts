import { api, listAll } from "@/lib/api";

export type Organization = {
  id: string;
  name: string;
  org_type: "govt" | "role" | "team" | "product_supplier";
  parent?: { id: string; name: string } | null;
  level_cache?: number;
  metadata?: Record<string, unknown>;
};

export const ROLE_ORGANIZATIONS = [
  "Administrator",
  "Facility Admin",
  "Doctor",
  "Nurse",
  "Staff",
  "Pharmacist",
  "Volunteer",
];

const COUNTRY = "India";

export function listStates(): Promise<Organization[]> {
  return listAll<Organization>("/organization/?org_type=govt&level_cache=0");
}

export function listDistricts(stateId: string): Promise<Organization[]> {
  return listAll<Organization>(`/organization/?org_type=govt&parent=${stateId}`);
}

export function listAllDistricts(): Promise<Organization[]> {
  return listAll<Organization>("/organization/?org_type=govt&level_cache=1");
}

export function createState(name: string): Promise<Organization> {
  return api.post<Organization>("/organization/", {
    name,
    org_type: "govt",
    description: "",
    active: true,
    metadata: { country: COUNTRY, govt_org_type: "state", govt_org_children_type: "district" },
  });
}

export function createDistrict(stateId: string, name: string): Promise<Organization> {
  return api.post<Organization>("/organization/", {
    name,
    org_type: "govt",
    parent: stateId,
    description: "",
    active: true,
    metadata: { country: COUNTRY, govt_org_type: "district", govt_org_children_type: "local_body" },
  });
}

export function listRoleOrganizations(): Promise<Organization[]> {
  return listAll<Organization>("/organization/?org_type=role");
}

export function createRoleOrganization(name: string): Promise<Organization> {
  return api.post<Organization>("/organization/", {
    name,
    org_type: "role",
    description: "",
    active: true,
    metadata: {},
  });
}

export const sameName = (a: string, b: string) => a.trim().toLowerCase() === b.trim().toLowerCase();
