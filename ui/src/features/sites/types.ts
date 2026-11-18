import type { Tone } from "../../ui";

export interface Site {
  name: string;
  host: string;
  mode: string;
  target: string;
  php?: string;
  system: boolean;
  enabled: boolean;
  http: number;
  ms: number;
  ssl_days: number | null;
}
export interface Rewrite {
  from: string;
  to: string;
  flag: string;
}
export interface Model {
  mode: string;
  target: string;
  php?: string;
  preset?: string;
  rewrites: Rewrite[];
  max_body_mb: number;
  read_timeout_s: number;
  force_https: boolean;
  gzip: boolean;
  aliases?: string[];
  auth?: boolean;
  allow_ips?: string[];
}

export const MODE_LABEL: Record<string, string> = { php: "PHP", proxy: "Proxy", statis: "Statis", kustom: "Kustom" };
export const PHP_VERSIONS = ["8.2", "8.3", "8.4", "8.5"];

export function health(s: Site): { tone: Tone; text: string } {
  if (!s.enabled) return { tone: "mute", text: "Dijeda" };
  if (s.http === 0) return { tone: "bad", text: "Tidak menjawab" };
  if (s.http >= 500) return { tone: "bad", text: `Galat ${s.http}` };
  if (s.http >= 400) return { tone: "warn", text: `${s.http}` };
  return { tone: "ok", text: `${s.http} · ${s.ms} ms` };
}

export interface Stats {
  exists: boolean;
  requests: number;
  bytes: number;
  days: { day: string; n: number }[];
  status: Record<string, number>;
  paths: { path: string; n: number }[];
  ips: { ip: string; n: number }[];
  note: string;
}
