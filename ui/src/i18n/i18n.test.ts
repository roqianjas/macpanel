import { describe, expect, it } from "vitest";
import { LOCALES, DEFAULT_LANG, format, translate } from "./index";

const ph = (s: string) => [...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort().join(",");

describe("i18n", () => {
  const base = LOCALES[DEFAULT_LANG].messages;
  for (const [code, l] of Object.entries(LOCALES)) {
    it(`${code}: kunci dan placeholder sama dengan bahasa dasar, tanpa teks kosong`, () => {
      expect(Object.keys(l.messages).sort()).toEqual(Object.keys(base).sort());
      for (const k of Object.keys(base) as (keyof typeof base)[]) {
        expect(l.messages[k].trim(), `${code}:${k}`).not.toBe("");
        expect(ph(l.messages[k]), `${code}:${k}`).toBe(ph(base[k]));
      }
    });
  }
  it("format mengganti placeholder dan membiarkan yang tak dikenal", () => {
    expect(format("{n} a {x}", { n: 3 })).toBe("3 a {x}");
  });
  it("bahasa tak dikenal jatuh ke bahasa dasar", () => {
    expect(translate("zz", "nav.apps")).toBe("Aplikasi");
    expect(translate("en", "nav.apps")).toBe("Apps");
  });
  it("tiap bahasa punya nama asli untuk dropdown", () => {
    for (const l of Object.values(LOCALES)) expect(l.native.length).toBeGreaterThan(1);
  });
  it("jamak: en memakai bentuk .one untuk 1; id tidak membedakan", () => {
    expect(translate("en", "common.alerts", { n: 1 })).toBe("1 alert");
    expect(translate("en", "common.alerts", { n: 2 })).toBe("2 alerts");
    expect(translate("en", "fmt.day", { n: 1 })).toBe("1 day");
    expect(translate("id", "common.alerts", { n: 1 })).toBe("1 peringatan");
  });
  it("kode bahasa aneh tidak membuat galat", () => {
    expect(translate("xx-yy", "common.alerts", { n: 3 })).toBe("3 peringatan");
  });
});
