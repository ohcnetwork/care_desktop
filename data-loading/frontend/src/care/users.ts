import { api, listAll, ApiError } from "@/lib/api";

export type Role = {
  id: string;
  name: string;
  is_system?: boolean;
  is_archived?: boolean;
};

export type User = {
  id: string;
  username: string;
  first_name: string;
  last_name: string;
};

export type Gender = "male" | "female" | "non_binary" | "transgender";

export const GENDERS: { value: Gender; label: string }[] = [
  { value: "male", label: "Male" },
  { value: "female", label: "Female" },
  { value: "non_binary", label: "Non-binary" },
  { value: "transgender", label: "Transgender" },
];

export type UserInput = {
  username: string;
  first_name: string;
  last_name: string;
  email: string;
  phone_number: string;
  gender: Gender;
  password: string;
  geo_organization: string;
  role_orgs: { organization: string; role: string }[];
};

export async function listRoles(): Promise<Role[]> {
  const roles = await listAll<Role>("/role/");
  return roles.filter((r) => !r.is_archived);
}

export async function findUser(username: string): Promise<User | null> {
  try {
    return await api.get<User>(`/users/${encodeURIComponent(username)}/`);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) return null;
    throw e;
  }
}

export function createUser(input: UserInput): Promise<User> {
  return api.post<User>("/users/", { ...input, prefix: null, suffix: null, is_service_account: false });
}

export async function addUserToDepartment(
  facilityId: string,
  organizationId: string,
  userId: string,
  roleId: string,
): Promise<void> {
  await api.post(`/facility/${facilityId}/organizations/${organizationId}/users/`, {
    user: userId,
    role: roleId,
  });
}

export type Membership = { id: string; user: { id: string; username: string }; role: { id: string } };

export function listMembers(facilityId: string, organizationId: string): Promise<Membership[]> {
  return listAll<Membership>(`/facility/${facilityId}/organizations/${organizationId}/users/`);
}
