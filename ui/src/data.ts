import { get } from "./api";
import { usePoll } from "./ui";
import type { Pt, RangeKey } from "./lib";

export interface NetNow {
  iface: string;
  label: string;
  ip: string;
  rx_bps: number;
  tx_bps: number;
}
export interface ProcInfo {
  name: string;
  cpu: number;
  rss_mb: number;
}
export interface SystemInfo {
  host: string;
  os: string;
  cores: number;
  mem_total: number;
  disk_total: number;
  disk_free: number;
  boot: number;
  load: [number, number, number];
  cpu: number;
  mem: number;
  disk: number;
  sample_at: number;
  tailnet_ip: string;
  nets: NetNow[] | null;
  top: ProcInfo[] | null;
  backup: { at: number; count: number; local_only: boolean; name: string } | null;
}
export interface NetSeries {
  iface: string;
  label: string;
  rx: Pt[];
  tx: Pt[];
  total_rx: number;
  total_tx: number;
}
export interface MetricsData {
  now: number;
  range: string;
  bucket: number;
  interval: number;
  cpu: Pt[];
  mem: Pt[];
  disk: Pt[];
  load: Pt[];
  nets: NetSeries[];
}
export interface ServiceInfo {
  id: string;
  label: string;
  group: string;
  kind: string;
  version: string;
  state: "running" | "stopped" | "not_installed" | "error";
  port?: number;
  installed: boolean;
  ctl: boolean;
  optional: boolean;
  desc: string;
  has_log: boolean;
}

export const useSystem = () => usePoll(() => get<SystemInfo>("/api/system"), 15000);
export const useMetrics = (range: RangeKey) => usePoll(() => get<MetricsData>(`/api/metrics?range=${range}`), 30000, [range]);
export const useServices = () => usePoll(() => get<{ services: ServiceInfo[] }>("/api/services"), 30000);

export const stateText: Record<ServiceInfo["state"], string> = { running: "Berjalan", stopped: "Berhenti", not_installed: "Belum terpasang", error: "Galat" };
export const stateTone = (s: ServiceInfo["state"]) => (s === "running" ? "ok" : s === "error" ? "bad" : s === "stopped" ? "warn" : "mute") as "ok" | "bad" | "warn" | "mute";
