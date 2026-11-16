export class ApiError extends Error {
  status: number;
  data: any;
  constructor(status: number, message: string, data?: any) {
    super(message);
    this.status = status;
    this.data = data;
  }
}

export const UNAUTH = "panel:unauth";
let csrf = "";
export const setCsrf = (t: string) => {
  csrf = t;
};

type StepUp = () => Promise<boolean>;
let stepUp: StepUp | null = null;
export const setStepUpHandler = (h: StepUp) => {
  stepUp = h;
};

async function send(method: string, path: string, body?: unknown): Promise<{ res: Response; data: any }> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (method !== "GET") headers["X-CSRF-Token"] = csrf;
  const res = await fetch(path, { method, headers, credentials: "same-origin", body: body === undefined ? undefined : JSON.stringify(body) });
  const text = await res.text();
  let data: any = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = { error: text.slice(0, 200) };
  }
  return { res, data };
}

/** Semua panggilan ke server. Bila server meminta verifikasi TOTP ulang, tampilkan dialog lalu ulangi sekali. */
export async function api<T = any>(method: string, path: string, body?: unknown): Promise<T> {
  let { res, data } = await send(method, path, body);
  if (res.status === 403 && typeof data?.error === "string" && data.error.includes("TOTP ulang") && stepUp) {
    if (await stepUp()) ({ res, data } = await send(method, path, body));
  }
  if (res.status === 401 && !path.startsWith("/api/auth/")) window.dispatchEvent(new Event(UNAUTH));
  if (!res.ok) throw new ApiError(res.status, data?.error || `HTTP ${res.status}`, data);
  return data as T;
}

export const get = <T = any>(p: string) => api<T>("GET", p);
export const post = <T = any>(p: string, b: unknown = {}) => api<T>("POST", p, b);

/** Pesan galat yang aman ditampilkan, termasuk keluaran aksi (422). */
export function errText(e: unknown): string {
  if (e instanceof ApiError) {
    const out = typeof e.data?.output === "string" ? e.data.output.trim() : "";
    return out ? `${e.message}\n${out}` : e.message;
  }
  return e instanceof Error ? e.message : String(e);
}

export function fmtTime(unix: number): string {
  if (!unix) return "-";
  return new Date(unix * 1000).toLocaleString("id-ID", { dateStyle: "medium", timeStyle: "short" });
}
