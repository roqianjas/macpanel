import { useSyncExternalStore } from "react";
import { get } from "./api";

export interface TrackedJob {
  id: number;
  label: string;
  status: "berjalan" | "ok" | "gagal" | "timeout" | "terputus";
  output: string;
  dismissed: boolean;
}

let jobs: TrackedJob[] = [];
const listeners = new Set<() => void>();
const emit = () => {
  jobs = [...jobs];
  listeners.forEach((l) => l());
};

export const useTrackedJobs = () =>
  useSyncExternalStore(
    (cb) => {
      listeners.add(cb);
      return () => listeners.delete(cb);
    },
    () => jobs,
  );

export function dismissJob(id: number) {
  const j = jobs.find((x) => x.id === id);
  if (j) j.dismissed = true;
  emit();
}

/** Pantau job asinkron sampai selesai (polling 1,5 detik, maksimal 35 menit), lalu panggil onDone. */
export function trackJob(id: number, label: string, onDone?: (j: TrackedJob) => void) {
  const j: TrackedJob = { id, label, status: "berjalan", output: "", dismissed: false };
  jobs = [j, ...jobs.filter((x) => x.id !== id)];
  emit();
  const started = Date.now();
  const tick = async () => {
    try {
      const r = await get<{ status: TrackedJob["status"]; output: string }>(`/api/jobs/${id}`);
      j.status = r.status;
      j.output = r.output;
    } catch {
      /* jaringan putus sebentar: coba lagi */
    }
    emit();
    if (j.status === "berjalan" && Date.now() - started < 35 * 60 * 1000) {
      setTimeout(tick, 1500);
    } else if (j.status !== "berjalan") {
      onDone?.(j);
    }
  };
  setTimeout(tick, 600);
}
