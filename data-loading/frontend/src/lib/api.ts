const BASE = "/api/v1";

type Tokens = { access: string; refresh: string };

let tokens: Tokens | null = null;
let refreshing: Promise<boolean> | null = null;

export class ApiError extends Error {
  status: number;
  body: unknown;
  constructor(status: number, body: unknown, message: string) {
    super(message);
    this.status = status;
    this.body = body;
  }
}

export function setTokens(next: Tokens | null) {
  tokens = next;
}

export function hasSession(): boolean {
  return tokens !== null;
}

function describe(body: unknown): string {
  if (!body || typeof body !== "object") return typeof body === "string" ? body : "";
  const b = body as Record<string, unknown>;
  if (Array.isArray(b.errors)) {
    return b.errors
      .map((e) => {
        const err = e as Record<string, unknown>;
        const loc = Array.isArray(err.loc) ? err.loc.filter((x) => typeof x === "string").join(".") : "";
        const msg = typeof err.msg === "string" ? err.msg : JSON.stringify(err);
        return loc ? `${loc}: ${msg}` : msg;
      })
      .join("; ");
  }
  if (typeof b.detail === "string") return b.detail;
  if (Array.isArray(b.non_field_errors)) return b.non_field_errors.join("; ");
  return Object.entries(b)
    .map(([k, v]) => `${k}: ${Array.isArray(v) ? v.join(", ") : String(v)}`)
    .join("; ");
}

async function parse(res: Response): Promise<unknown> {
  const text = await res.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

async function refresh(): Promise<boolean> {
  if (!tokens) return false;
  if (!refreshing) {
    const current = tokens;
    refreshing = (async () => {
      try {
        const res = await fetch(`${BASE}/auth/token/refresh/`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ refresh: current.refresh }),
        });
        if (!res.ok) return false;
        const body = (await res.json()) as { access: string; refresh?: string };
        tokens = { access: body.access, refresh: body.refresh ?? current.refresh };
        return true;
      } catch {
        return false;
      } finally {
        refreshing = null;
      }
    })();
  }
  return refreshing;
}

export async function request<T>(
  method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE",
  path: string,
  body?: unknown,
  options: { auth?: boolean; retry?: boolean } = {},
): Promise<T> {
  const { auth = true, retry = true } = options;
  const headers: Record<string, string> = { Accept: "application/json" };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (auth && tokens) headers.Authorization = `Bearer ${tokens.access}`;
  let res: Response;
  try {
    res = await fetch(`${BASE}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, null, "CARE did not answer. Check that it is running.");
  }
  if (res.status === 401 && auth && retry && (await refresh())) {
    return request<T>(method, path, body, { auth, retry: false });
  }
  const data = await parse(res);
  if (!res.ok) {
    const detail = describe(data);
    throw new ApiError(res.status, data, detail || `${method} ${path} failed (${res.status})`);
  }
  return data as T;
}

export const api = {
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body: unknown) => request<T>("POST", path, body),
  put: <T>(path: string, body: unknown) => request<T>("PUT", path, body),
};

export type Paginated<T> = { count: number; results: T[] };

export async function listAll<T>(path: string, pageSize = 200): Promise<T[]> {
  const joiner = path.includes("?") ? "&" : "?";
  const all: T[] = [];
  let offset = 0;
  for (;;) {
    const page = await api.get<Paginated<T>>(`${path}${joiner}limit=${pageSize}&offset=${offset}`);
    all.push(...page.results);
    offset += page.results.length;
    if (page.results.length === 0 || offset >= page.count) return all;
  }
}
