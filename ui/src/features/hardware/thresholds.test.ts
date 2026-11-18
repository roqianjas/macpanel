import { describe, expect, it } from "vitest";
import { changed, isDefault, parseNum, thrError, type Thr } from "./thresholds";

const D: Thr = { cpu_warn: 85, cpu_bad: 95, ssd_warn: 70, ssd_bad: 80, press_warn: 80, press_bad: 90, swap_mb: 2048 };
const m = (o: Partial<Thr>): Thr => ({ ...D, ...o });

describe("thrError", () => {
  it("bawaan sah", () => expect(thrError(D)).toBe(""));
  it.each([
    [m({ cpu_warn: 20 }), "Suhu CPU/GPU"],
    [m({ cpu_bad: 300 }), "Suhu CPU/GPU"],
    [m({ cpu_warn: 95 }), "lebih rendah"],
    [m({ ssd_warn: 90, ssd_bad: 80 }), "Suhu SSD"],
    [m({ press_bad: 101 }), "Tekanan memori"],
    [m({ press_warn: 10 }), "Tekanan memori"],
    [m({ swap_mb: 0 }), "Swap"],
    [m({ swap_mb: 1e9 }), "Swap"],
  ])("menolak %#", (t, kata) => expect(thrError(t)).toContain(kata));
  it("NaN dan Infinity ditolak, bukan dianggap 0", () => {
    expect(thrError(m({ cpu_warn: NaN }))).toContain("angka");
    expect(thrError(m({ swap_mb: Infinity }))).toContain("angka");
  });
  it("batas bawah dan atas yang sah diterima", () => {
    expect(thrError(m({ cpu_warn: 40, cpu_bad: 41, swap_mb: 256 }))).toBe("");
    expect(thrError(m({ cpu_bad: 120, press_bad: 100, swap_mb: 65536 }))).toBe("");
  });
});

describe("parseNum", () => {
  it("menerima koma desimal Indonesia", () => expect(parseNum("72,5")).toBe(72.5));
  it("kosong dan teks menjadi NaN, bukan 0", () => {
    expect(parseNum("")).toBeNaN();
    expect(parseNum("   ")).toBeNaN();
    expect(parseNum("panas")).toBeNaN();
  });
  it("spasi dipangkas", () => expect(parseNum(" 80 ")).toBe(80));
});

describe("isDefault dan changed", () => {
  it("isDefault", () => {
    expect(isDefault(D, D)).toBe(true);
    expect(isDefault(m({ cpu_warn: 80 }), D)).toBe(false);
  });
  it("changed hanya mengirim field yang berbeda", () => {
    expect(changed(m({ cpu_warn: 70, cpu_bad: 78 }), D)).toEqual({ cpu_warn: 70, cpu_bad: 78 });
    expect(changed(D, D)).toEqual({});
  });
});
