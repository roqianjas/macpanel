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
    expect(group(null)).toEqual({ safe: [], careful: [], show: [] });
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
