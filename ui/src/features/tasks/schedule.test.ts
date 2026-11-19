import { describe as d, expect, it } from "vitest";
import { DEFAULT_PICK, describe, nextRuns, parseCron, toBody, type Pick } from "./schedule";

const P = (o: Partial<Pick>): Pick => ({ ...DEFAULT_PICK, ...o });

d("toBody: pilihan -> kolom API", () => {
  it("setiap N menit/jam memakai every, rentang dijaga", () => {
    expect(toBody(P({ mode: "minutes", n: "15" }))).toEqual({ body: { every: "15m" } });
    expect(toBody(P({ mode: "hours", n: "6" }))).toEqual({ body: { every: "6h" } });
    for (const n of ["0", "60", "", "x", "-1", "1.5"]) expect("error" in toBody(P({ mode: "minutes", n })), n).toBe(true);
    expect("error" in toBody(P({ mode: "hours", n: "1000" }))).toBe(true);
  });
  it("harian memakai at; mingguan dan bulanan menjadi cron", () => {
    expect(toBody(P({ mode: "daily", time: "07:30" }))).toEqual({ body: { at: "07:30" } });
    expect(toBody(P({ mode: "weekly", time: "08:05", days: [5, 1, 3] }))).toEqual({ body: { cron: "5 8 * * 1,3,5" } });
    expect(toBody(P({ mode: "monthly", time: "00:00", dom: "15" }))).toEqual({ body: { cron: "0 0 15 * *" } });
    expect(toBody(P({ mode: "hourly", minute: "20" }))).toEqual({ body: { cron: "20 * * * *" } });
    expect(toBody(P({ mode: "boot" }))).toEqual({ body: { cron: "@reboot" } });
  });
  it("menolak isian tidak sah", () => {
    expect("error" in toBody(P({ mode: "weekly", days: [] }))).toBe(true);
    expect("error" in toBody(P({ mode: "monthly", dom: "32" }))).toBe(true);
    expect("error" in toBody(P({ mode: "daily", time: "" }))).toBe(true);
    expect("error" in toBody(P({ mode: "custom", cron: "0 9 15 * 1" }))).toBe(true);
  });
});

d("parseCron: sama dengan cronspec.py", () => {
  const ok = ["* * * * *", "*/5 * * * *", "0 3 * * *", "30 8-17/3 * * mon-fri", "0 0 1 * *", "0 0 * * 7", "@reboot", "@weekly", "1,2,3 * * * *", "0 9 * jan,jul *"];
  const bad = ["", "* * * *", "* * * * * *", "61 * * * *", "* 24 * * *", "* * 0 * *", "* * * 13 *", "*/0 * * * *", "5-1 * * * *", "a * * * *", "0 9 15 * 1", ",1 * * * *", "*/2 */2 1-31/2 * *"];
  it.each(ok)("sah: %s", (e) => expect("error" in parseCron(e)).toBe(false));
  it.each(bad)("ditolak: %s", (e) => expect("error" in parseCron(e)).toBe(true));
  it("hari 7 sama dengan Minggu", () => {
    const r = parseCron("0 0 * * 7");
    expect("sets" in r && r.sets[4]).toEqual([0]);
  });
});

d("describe", () => {
  it.each([
    ["every 5m", "Setiap 5 menit"],
    ["every 2h", "Setiap 2 jam"],
    ["* * * * *", "Setiap menit"],
    ["*/5 * * * *", "Setiap 5 menit"],
    ["0 * * * *", "Setiap jam tepat"],
    ["20 * * * *", "Setiap jam pada menit 20"],
    ["30 3 * * *", "Setiap hari pukul 03:30"],
    ["0 8 * * 1-5", "Pukul 08:00, hari Senin–Jumat"],
    ["0 9 * * 1,3", "Pukul 09:00, hari Senin, Rabu"],
    ["0 0 1 * *", "Pukul 00:00, tanggal 1"],
    ["0 6 * jan *", "Setiap hari pukul 06:00, bulan Januari"],
    ["@reboot", "Saat Mac mini menyala (setelah login)"],
    ["@weekly", "Pukul 00:00, hari Minggu"],
  ])("%s", (spec, text) => expect(describe(spec)).toBe(text));
  it("tidak melempar untuk masukan aneh", () => {
    expect(describe("???")).toBe("???");
    expect(describe("")).toBe("");
  });
});

d("nextRuns", () => {
  const from = new Date(2026, 0, 5, 10, 7, 30); // Senin 5 Jan 2026 10:07:30
  it("menghitung tiga waktu berikutnya", () => {
    const r = nextRuns("*/15 * * * *", 3, from);
    expect(r.map((x) => `${x.getHours()}:${x.getMinutes()}`)).toEqual(["10:15", "10:30", "10:45"]);
  });
  it("hari kerja melewati akhir pekan", () => {
    const fri = new Date(2026, 0, 9, 9, 0, 0);
    const r = nextRuns("0 8 * * 1-5", 2, fri);
    expect(r.map((x) => x.getDate())).toEqual([12, 13]);
  });
  it("tanggal 31 melewati bulan pendek; @reboot dan galat kosong", () => {
    const r = nextRuns("0 0 31 * *", 2, new Date(2026, 0, 31, 12, 0, 0));
    expect(r.map((x) => `${x.getMonth() + 1}/${x.getDate()}`)).toEqual(["3/31", "5/31"]);
    expect(nextRuns("@reboot")).toEqual([]);
    expect(nextRuns("bukan cron")).toEqual([]);
  });
});
