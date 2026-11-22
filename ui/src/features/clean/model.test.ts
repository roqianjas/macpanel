import { describe, expect, it } from "vitest";
import { ageLabel, appUsedMb, estimate, group, isStale, parseRun, pressure, runSummary, scanAge, sizeLabel, type Scan } from "./model";

const row = (id: string, tier: "safe" | "careful" | "show", bytes: number, complete = true) => ({ id, label: id, tier, bytes, complete, why: "", cost: "", paths: [] });
const scan: Scan = { at: 1000, safe_bytes: 0, careful_bytes: 0, disk_free: 1, rows: [row("a", "safe", 100), row("b", "safe", 50), row("bk:x", "careful", 1000), row("hermes", "show", 99999)] };

describe("analisis disk", () => {
  it("mengelompokkan per tingkat", () => {
    const g = group(scan);
    expect(g.safe.map((r) => r.id)).toEqual(["a", "b"]);
    expect(g.careful).toHaveLength(1);
    expect(g.show).toHaveLength(1);
    expect(group(null)).toEqual({ safe: [], careful: [], show: [], detected: [] });
  });
  it("perkiraan menjumlah yang dipilih, mengabaikan show dan id basi", () => {
    expect(estimate(scan, ["a", "b"])).toBe(150);
    expect(estimate(scan, ["a", "hermes"])).toBe(100);
    expect(estimate(scan, ["tidak-ada"])).toBe(0);
    expect(estimate(null, ["a"])).toBe(0);
  });
  it("umur analisis dan basi", () => {
    expect(scanAge(1000, 1030)).toBe("baru saja");
    expect(scanAge(1000, 1000 + 600)).toBe("10 menit lalu");
    expect(scanAge(1000, 1000 + 7200)).toBe("2 jam lalu");
    expect(scanAge(1000, 1000 + 3 * 86400)).toBe("3 hari lalu");
    expect(isStale(0, 86401)).toBe(true);
    expect(isStale(0, 86400)).toBe(false);
  });
  it("label umur dan ukuran jujur untuk yang tidak terbaca", () => {
    expect(ageLabel(null)).toBe("umur tidak terbaca");
    expect(ageLabel(0)).toBe("dibuat hari ini");
    expect(ageLabel(5)).toBe("5 hari lalu");
    expect(sizeLabel({ bytes: 2048, complete: false })).toContain("≥");
    expect(sizeLabel({ bytes: 2048, complete: true })).not.toContain("≥");
  });
});

describe("hasil pembersihan", () => {
  const ok = JSON.stringify({ dry_run: false, items: [{ id: "a", label: "a", tier: "safe", bytes: 1048576, error: null }], skipped: [], bytes: 1048576 });
  it("membaca JSON di baris terakhir dan menolak sampah", () => {
    expect(parseRun("log awal\n" + ok)?.bytes).toBe(1048576);
    expect(parseRun("bukan json")).toBeNull();
    expect(parseRun("")).toBeNull();
    expect(parseRun('{"items":1}')).toBeNull();
  });
  it("ringkasan memberi peringatan bila ada yang gagal atau dilewati", () => {
    expect(runSummary(parseRun(ok)).tone).toBe("ok");
    const r = parseRun(JSON.stringify({ dry_run: false, items: [{ id: "a", label: "a", tier: "safe", bytes: 5, error: "izin" }], skipped: [{ id: "b", reason: "proses go berjalan" }], bytes: 5 }));
    const s = runSummary(r);
    expect(s.tone).toBe("warn");
    expect(s.text).toContain("sebagian gagal");
    expect(s.text).toContain("proses go berjalan");
    expect(runSummary(null).tone).toBe("warn");
  });
});

describe("memori", () => {
  it("tekanan dari persen bebas", () => {
    expect(pressure(80).tone).toBe("ok");
    expect(pressure(40).tone).toBe("ok");
    expect(pressure(39).tone).toBe("warn");
    expect(pressure(20).tone).toBe("warn");
    expect(pressure(19).tone).toBe("bad");
    expect(pressure(undefined).tone).toBe("mute");
  });
  it("memori aplikasi tidak menghitung cache (inactive)", () => {
    expect(appUsedMb({ wired_mb: 1000, active_mb: 2000, compressed_mb: 500, inactive_mb: 9000 })).toBe(3500);
    expect(appUsedMb({ wired_mb: 1000 })).toBeNull();
  });
});

import { SMALL_BYTES, canPick, pickableIds, sumBytes, DETECTED_FACETS, type ScanRow } from "./model";

const d = (id: string, cls: "aman" | "tinjau" | "terkunci", bytes: number, extra: Partial<ScanRow> = {}): ScanRow =>
  ({ id, label: id, tier: "detected", cls, src: "app", bytes, paths: [], why: "", cost: "", ...extra }) as ScanRow;

describe("cache terdeteksi", () => {
  const rows = [d("lc:a", "aman", 9e7, { orphan: true }), d("lc:b", "tinjau", 6e7), d("lc:c", "terkunci", 5e8, { src: "apple" })];
  it("baris terkunci tidak pernah bisa dipilih", () => {
    expect(rows.map(canPick)).toEqual([true, true, false]);
    expect(pickableIds(rows)).toEqual(["lc:a", "lc:b"]);
  });
  it("jumlah byte hanya dari yang bisa dipilih, id tak dikenal diabaikan", () => {
    expect(sumBytes(rows, ["lc:a", "lc:b", "lc:c", "zz"])).toBe(1.5e8);
  });
  it("saringan menurut kelas, yatim, dan Apple", () => {
    const n = (id: string) => rows.filter(DETECTED_FACETS.find((f) => f.id === id)!.test).map((r) => r.id);
    expect(n("aman")).toEqual(["lc:a"]);
    expect(n("tinjau")).toEqual(["lc:b"]);
    expect(n("terkunci")).toEqual(["lc:c"]);
    expect(n("yatim")).toEqual(["lc:a"]);
    expect(n("apple")).toEqual(["lc:c"]);
  });
  it("ambang kecil memakai basis 1000", () => {
    expect(SMALL_BYTES).toBe(50_000_000);
  });
});

import { whyKey } from "./model";
describe("alasan terdeteksi", () => {
  it("diturunkan dari kelas", () => {
    expect(whyKey(d("a", "terkunci", 1, { src: "apple" }))).toBe("apple");
    expect(whyKey(d("a", "terkunci", 1, { running: true }))).toBe("running");
    expect(whyKey(d("a", "terkunci", 1))).toBe("unreadable");
    expect(whyKey(d("a", "aman", 1))).toBe("orphan");
    expect(whyKey(d("a", "tinjau", 1))).toBe("review");
  });
});

import { isBusy } from "./model";
describe("baris terblokir proses berjalan", () => {
  it("isBusy dan ringkasan 0 B", () => {
    expect(isBusy({ ...d("a", "aman", 1), busy: ["Google Chrome"] })).toBe(true);
    expect(isBusy({ ...d("a", "aman", 1), busy: [] })).toBe(false);
    expect(isBusy(d("a", "aman", 1))).toBe(false);
    const r = runSummary({ dry_run: false, items: [], skipped: [{ id: "x", reason: "proses terkait sedang berjalan: node" }], bytes: 0 });
    expect(r.tone).toBe("warn");
    expect(r.text).toMatch(/node/);
    expect(r.text).not.toMatch(/dibebaskan|freed/);
  });
  it("baris terblokir tidak bisa dipilih di cache terdeteksi", () => {
    expect(canPick({ ...d("lc:a", "aman", 1), busy: ["X"] })).toBe(false);
  });
});
