import { describe, expect, it } from "vitest";
import { byteUnit, fmtBytes, fmtRate, fmtUptime, geometry, niceTop, summarize } from "./lib";

describe("niceTop: empat interval berlabel bulat", () => {
  it("beban 3 -> 4 (langkah 1), beban 5 -> 8 (langkah 2), batas bawah 4", () => {
    expect(niceTop(3, 4)).toBe(4);
    expect(niceTop(5, 4)).toBe(8);
    expect(niceTop(0.2, 4)).toBe(4);
    expect(niceTop(9, 4)).toBe(20);
  });
  it("selalu kelipatan 4 langkah dengan langkah 1, 2, atau 5 x 10^n", () => {
    for (const v of [1, 7, 33, 145, 999, 12345, 5e6, 3e9]) {
      const hi = niceTop(v, 1);
      const step = hi / 4;
      const m = step / Math.pow(10, Math.floor(Math.log10(step)));
      expect([1, 2, 5, 10]).toContain(Math.round(m * 1e6) / 1e6);
      expect(hi).toBeGreaterThanOrEqual(v);
    }
  });
  it("bandwidth: 145 KB/s -> 200 KB/s dengan tick 50, 100, 150, 200", () => {
    const v = 145 * 1024;
    const unit = byteUnit(Math.max(v, 4096) / 4);
    expect(unit).toBe(1024);
    expect(niceTop(v, 4096, unit) / 1024).toBe(200);
  });
});

describe("byteUnit dan format", () => {
  it("memilih satuan terbesar yang muat", () => {
    expect(byteUnit(500)).toBe(1);
    expect(byteUnit(1024)).toBe(1024);
    expect(byteUnit(5 * 1024 * 1024)).toBe(1024 * 1024);
  });
  it("fmtBytes: koma desimal Indonesia, tanpa ',0' sisa", () => {
    expect(fmtBytes(0)).toBe("0 B");
    expect(fmtBytes(1024)).toBe("1 KB");
    expect(fmtBytes(1536)).toBe("1,5 KB");
    expect(fmtBytes(48.8 * 1024)).toBe("48,8 KB");
    expect(fmtBytes(50 * 1024)).toBe("50 KB");
    expect(fmtBytes(16 * 1024 ** 3)).toBe("16 GB");
    expect(fmtBytes(null)).toBe("n/a");
    expect(fmtBytes(NaN)).toBe("n/a");
    expect(fmtBytes(-5)).toBe("0 B");
  });
  it("fmtRate", () => {
    expect(fmtRate(2048)).toBe("2 KB/s");
    expect(fmtRate(undefined)).toBe("n/a");
  });
  it("fmtUptime", () => {
    expect(fmtUptime(29 * 60)).toBe("29 menit");
    expect(fmtUptime(3 * 3600 + 5 * 60)).toBe("3 jam 5 menit");
    expect(fmtUptime(2 * 86400 + 3 * 3600)).toBe("2 hari 3 jam");
    expect(fmtUptime(null)).toBe("tidak diketahui");
  });
});

describe("geometry dan summarize", () => {
  it("memetakan titik ke path dan membatasi nilai di luar domain", () => {
    const g = geometry([[0, 0], [10, 50], [20, 200]], 100, 100, 0, 100, { l: 0, r: 0, t: 0, b: 0 });
    expect(g.xs).toEqual([0, 50, 100]);
    expect(g.ys).toEqual([100, 50, 0]);
    expect(g.line.startsWith("M0.0 100.0")).toBe(true);
  });
  it("summarize", () => {
    expect(summarize([])).toBeNull();
    expect(summarize([[1, 5], [2, 9], [3, 7]])).toEqual({ min: 5, max: 9, last: 7, n: 3 });
  });
});
