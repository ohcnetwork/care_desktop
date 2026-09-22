import { api, request, setTokens } from "@/lib/api";

export type CurrentUser = {
  id: string;
  username: string;
  first_name: string;
  last_name: string;
  is_superuser: boolean;
};

export async function login(username: string, password: string): Promise<CurrentUser> {
  const tokens = await request<{ access: string; refresh: string }>(
    "POST",
    "/auth/login/",
    { username, password },
    { auth: false },
  );
  setTokens(tokens);
  return api.get<CurrentUser>("/users/getcurrentuser/");
}

export function logout() {
  setTokens(null);
}
