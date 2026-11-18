import { describe, expect, it } from "vitest";
import { fanText, forecastText, freshness, fmtTemp, num, pressureStatus, smartStatus, tempStatus, throttleStatus, verdict, type HwData, type HwForecast } from "./model";

const th = { cpu_warn: 85, cpu_bad: 95, ssd_warn: 70, ssd_bad: 80, press_warn: 80, press_bad: 90 };

describe("num dan fmtTemp", () => {
  it("memakai koma desimal", () => expect(num(10.84, 1)).toBe("10,8"));
  it("tidak mengarang angka untuk nilai kosong", () => {
    for (const v of [null, undefined, NaN, Infinity]) expect(num(v as number)).toBe("n/a");
    expect(fmtTemp(null)).toBe("n/a");
  });
  it("nol adalah angka nyata, bukan kosong", () => {
    expect(num(0)).toBe("0");
    expect(fmtTemp(0)).toBe("0 °C");
  });
  it("membulatkan suhu", () => expect(fmtTemp(43.7)).toBe("44 °C"));
});

describe("tempStatus", () => {
  it.each([
    [40, "ok", "Normal"],
    [84.9, "ok", "Normal"],
    [85, "warn", "Panas"],
    [94.9, "warn", "Panas"],
    [95, "bad", "Sangat panas"],
    [130, "bad", "Sangat panas"],
  ])("%s °C", (v, tone, text) => expect(tempStatus(v, 85, 95)).toEqual({ tone, text }));
  it("tidak terbaca bukan 'normal'", () => expect(tempStatus(null, 85, 95)).toEqual({ tone: "mute", text: "Tidak terbaca" }));
});

describe("pressureStatus", () => {
  it("memakai label tekanan, bukan label panas", () => {
    expect(pressureStatus(85, th.press_warn, th.press_bad).text).toBe("Tinggi");
    expect(pressureStatus(95, th.press_warn, th.press_bad).text).toBe("Sangat tinggi");
    expect(pressureStatus(39, th.press_warn, th.press_bad).tone).toBe("ok");
  });
});

describe("smartStatus", () => {
  it("hanya Verified yang sehat", () => {
    expect(smartStatus("Verified").tone).toBe("ok");
    expect(smartStatus("Failing").tone).toBe("bad");
    expect(smartStatus("About to fail").tone).toBe("bad");
  });
  it("status tidak diketahui tidak dilabeli sehat", () => {
    for (const s of ["", null, undefined, "Not Supported"]) expect(smartStatus(s as string).tone).toBe("mute");
  });
});

describe("throttleStatus dan fanText", () => {
  it("membedakan dibatasi dan tidak", () => {
    expect(throttleStatus({ speed_limit: 100, throttled: false }).tone).toBe("ok");
    expect(throttleStatus({ speed_limit: 70, throttled: true })).toEqual({ tone: "bad", text: "Dibatasi 70%" });
    expect(throttleStatus(null).tone).toBe("mute");
  });
  it("kipas 0 rpm tetap angka nyata", () => {
    expect(fanText({ rpm: 0, max: 4900 })).toBe("0 rpm (0% dari maks 4900)");
    expect(fanText({ rpm: 1000, max: 4900 })).toBe("1000 rpm (20% dari maks 4900)");
    expect(fanText({ rpm: 1200, max: null })).toBe("1200 rpm");
    expect(fanText(null)).toBe("n/a");
  });
});

const base = (over: Partial<HwData> = {}): HwData => ({
  now: { at: 1000, source: "macmon", temps: { cpu: 50 }, fan: null, power: null, mem: null, thermal: null, errors: [] },
  stale: false,
  ssd: null,
  forecast: { ok: false, reason: "data belum cukup", pct_per_day: 0, r2: 0, span_days: 0 },
  range: "1h",
  bucket: 30,
  interval: 30,
  series: {},
  peak: {},
  samples: 1,
  throttled_samples: 0,
  thresholds: th,
  ...over,
});

describe("freshness", () => {
  it("belum ada sampel", () => {
    expect(freshness(null, 5).tone).toBe("mute");
    expect(freshness(base({ now: null }), 5).text).toContain("Belum ada sampel");
  });
  it("basi diberi peringatan, tidak tampil seolah terkini", () => {
    const f = freshness(base({ stale: true }), 1000 + 600);
    expect(f.tone).toBe("warn");
    expect(f.text).toContain("10 menit");
  });
  it("tanpa sumber memakai pesan galat terakhir", () => {
    const d = base();
    d.now = { ...d.now!, source: "none", errors: ["a", "pasang macmon"] };
    expect(freshness(d, 1010)).toEqual({ tone: "warn", text: "pasang macmon" });
  });
  it("Netdata cadangan diberi catatan skala", () => {
    const d = base();
    d.now = { ...d.now!, source: "netdata" };
    expect(freshness(d, 1010).text).toContain("Skala suhu berbeda");
  });
  it("normal menyebut sumber dan umur data", () => {
    expect(freshness(base(), 1010)).toEqual({ tone: "ok", text: "Sumber: macmon. Diperbarui 10 detik lalu." });
    expect(freshness(base(), 1000 + 150).text).toContain("2 menit");
  });
});

describe("forecastText", () => {
  const f = (o: Partial<HwForecast>): HwForecast => ({ ok: true, pct_per_day: 1, r2: 0.9, span_days: 5, ...o });
  it("tanpa data cukup tidak menampilkan angka", () => {
    const r = forecastText({ ok: false, reason: "data belum cukup (butuh minimal 24 jam)", pct_per_day: 0, r2: 0, span_days: 0.2 });
    expect(r.tone).toBe("mute");
    expect(r.text).not.toMatch(/\d+ hari lagi/);
  });
  it("stabil", () => expect(forecastText(f({ pct_per_day: 0, reason: "stabil atau menurun" })).tone).toBe("ok"));
  it("menyebut hari dan laju", () => {
    const r = forecastText(f({ days_to_full: 20, pct_per_day: 2, gb_per_day: 4.9 }));
    expect(r.text).toContain("sekitar 20 hari lagi");
    expect(r.text).toContain("2,00%/hari");
    expect(r.text).toContain("4,9 GB/hari");
    expect(r.tone).toBe("ok");
  });
  it("nada mengikuti ambang server (3 dan 14 hari)", () => {
    expect(forecastText(f({ days_to_full: 2 })).tone).toBe("bad");
    expect(forecastText(f({ days_to_full: 10 })).tone).toBe("warn");
    expect(forecastText(f({ days_to_full: 14 })).tone).toBe("ok");
  });
  it("tren berisik (R2 rendah) tidak memicu peringatan dan diberi catatan", () => {
    const r = forecastText(f({ days_to_full: 2, r2: 0.2 }));
    expect(r.tone).toBe("ok");
    expect(r.text).toContain("kasar");
  });
});

describe("verdict", () => {
  const s = (tone: "ok" | "warn" | "bad" | "mute") => ({ tone, text: "x" });
  it("bad menang atas warn", () => expect(verdict([s("ok"), s("warn"), s("bad"), s("bad")])).toEqual({ tone: "bad", text: "2 komponen perlu dicek sekarang" }));
  it("warn bila tidak ada bad", () => expect(verdict([s("ok"), s("warn")])).toEqual({ tone: "warn", text: "1 komponen perlu diperhatikan" }));
  it("semua normal", () => expect(verdict([s("ok"), s("ok")])).toEqual({ tone: "ok", text: "Semua komponen normal" }));
  it("sebagian tak terbaca tidak disebut 'semua normal'", () => expect(verdict([s("ok"), s("mute")]).text).toBe("Yang terbaca normal, sebagian belum terbaca"));
  it("tanpa data sama sekali bukan normal", () => {
    expect(verdict([]).tone).toBe("mute");
    expect(verdict([s("mute"), s("mute")])).toEqual({ tone: "mute", text: "Belum ada data komponen" });
  });
});
