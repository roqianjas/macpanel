import { afterEach, describe, expect, it } from "vitest";
import { setLang } from "./i18n";
import { alertDetail, alertTitle, svcDesc } from "./serverText";

afterEach(() => setLang("id"));

describe("teks buatan server diterjemahkan lewat kunci stabil", () => {
  const a = { title: "shop.lab tidak menjawab", detail: "Pemeriksaan HTTP gagal", code: "site.down", args: { host: "shop.lab" } };
  it("id: sama dengan teks server", () => {
    expect(alertTitle(a)).toBe("shop.lab tidak menjawab");
    expect(alertDetail(a)).toBe("Pemeriksaan HTTP gagal");
  });
  it("en: diterjemahkan dengan argumen, dan jamak mengikuti n", () => {
    setLang("en");
    expect(alertTitle(a)).toBe("shop.lab is not responding");
    expect(alertDetail({ title: "", detail: "x", code: "ssl.soon", args: { host: "a.lab", n: 1 } })).toBe("1 day left");
    expect(alertDetail({ title: "", detail: "x", code: "ssl.soon", args: { host: "a.lab", n: 5 } })).toBe("5 days left");
  });
  it("kode tak dikenal atau tanpa kode: pakai teks server (tak pernah kosong)", () => {
    setLang("en");
    expect(alertTitle({ title: "Judul server", detail: "d" })).toBe("Judul server");
    expect(alertTitle({ title: "Judul server", detail: "d", code: "belum.ada" })).toBe("Judul server");
    expect(svcDesc("tidak-ada", "Teks server")).toBe("Teks server");
  });
  it("deskripsi layanan en", () => {
    setLang("en");
    expect(svcDesc("nginx", "x")).toMatch(/Reverse proxy/);
  });
});
