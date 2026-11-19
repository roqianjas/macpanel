/// <reference types="node" />
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { ACCENTS, accentTokens, contrast, isHex, luminance, mix, readableOn, SKINS, type Mode } from "./skins";

// Dibaca langsung dari disk: vitest mengosongkan isi CSS yang diimpor.
const read = (n: string) => readFileSync(new URL(n, import.meta.url), "utf8");
const skinsCss = read("./skins.css");
const baseCss = read("./styles.css");

/** Isi blok `selector { ... }` pertama sebagai peta token. */
function tokens(css: string, selector: string): Record<string, string> {
  const i = css.indexOf(selector + " {");
  if (i < 0) throw new Error("blok tidak ada: " + selector);
  const body = css.slice(css.indexOf("{", i) + 1, css.indexOf("}", i));
  const out: Record<string, string> = {};
  for (const m of body.matchAll(/--([a-z0-9-]+):\s*([^;]+);/g)) out[m[1]] = m[2].trim();
  return out;
}
const skinTokens = (id: string, mode: Mode) => tokens(skinsCss, `html[data-skin="${id}"][data-theme="${mode}"]`);
const baseTokens = (mode: Mode) => tokens(baseCss, `html[data-theme="${mode}"]`);

describe("matematika warna", () => {
  it("kontras hitam-putih 21 dan sama 1", () => {
    expect(contrast("#000000", "#ffffff")).toBeCloseTo(21, 1);
    expect(contrast("#777777", "#777777")).toBeCloseTo(1, 5);
    expect(luminance("#ffffff")).toBeCloseTo(1, 5);
  });
  it("mix", () => {
    expect(mix("#000000", "#ffffff", 0.5)).toBe("#808080");
    expect(mix("#102030", "#102030", 0.7)).toBe("#102030");
  });
  it("isHex hanya #rrggbb", () => {
    expect(isHex("#c2410c")).toBe(true);
    for (const v of ["c2410c", "#fff", "#gggggg", "#c2410c0", "", null, 5, "javascript:1"]) expect(isHex(v)).toBe(false);
  });
});

describe("warna aksen selalu aman kontras", () => {
  // pseudo-acak deterministik: tidak bergantung Math.random
  const rnd = (() => {
    let s = 1234567;
    return () => {
      s = (s * 1664525 + 1013904223) % 4294967296;
      return s;
    };
  })();
  const colors = [...ACCENTS.map((a) => a.hex), "#000000", "#ffffff", "#777777", "#808080", "#ff0000", "#00ff00", "#0000ff", "#fff200", ...Array.from({ length: 250 }, () => "#" + (rnd() & 0xffffff).toString(16).padStart(6, "0"))];
  for (const skin of SKINS) {
    for (const mode of ["light", "dark"] as Mode[]) {
      it(`${skin.id}/${mode}: teks aksen >= 4,5 di semua latar dan teks di atas tombol aksen >= 4,5`, () => {
        const { bg, surface } = skin.preview[mode];
        for (const c of colors) {
          const t = accentTokens(c, bg, surface, mode === "dark");
          for (const back of [bg, surface, t.tint]) expect(contrast(t.primary, back), `${c} primary di ${back}`).toBeGreaterThanOrEqual(4.5);
          expect(contrast(t.onAccent, t.accent), `${c} on-accent`).toBeGreaterThanOrEqual(4.5);
        }
      });
    }
  }
  it("warna yang sudah aman tidak diubah", () => {
    expect(readableOn("#111111", ["#ffffff"], false)).toBe("#111111");
  });
  it("warna tak aman diubah mengikuti mode (gelap dicerahkan, terang digelapkan)", () => {
    const light = readableOn("#c6f432", ["#ffffff"], false);
    expect(luminance(light)).toBeLessThan(luminance("#c6f432"));
    const dark = readableOn("#1d4ed8", ["#14140f"], true);
    expect(luminance(dark)).toBeGreaterThan(luminance("#1d4ed8"));
  });
});

describe("token gaya di skins.css", () => {
  for (const skin of SKINS) {
    for (const mode of ["light", "dark"] as Mode[]) {
      const t = () => skinTokens(skin.id, mode);
      it(`${skin.id}/${mode}: pratinjau di skins.ts sama dengan CSS`, () => {
        const p = skin.preview[mode];
        expect(t().bg).toBe(p.bg);
        expect(t().surface).toBe(p.surface);
        expect(t().text).toBe(p.ink);
        expect(t().accent).toBe(p.accent);
      });
      it(`${skin.id}/${mode}: kontras teks (AA 4,5)`, () => {
        const k = t();
        const pairs: [string, string, string][] = [
          ["text", "bg", k.text + "|" + k.bg],
          ["text", "surface", k.text + "|" + k.surface],
          ["text", "raised", k.text + "|" + k.raised],
          ["muted", "bg", k.muted + "|" + k.bg],
          ["muted", "surface", k.muted + "|" + k.surface],
          ["muted", "raised", k.muted + "|" + k.raised],
          ["primary", "bg", k.primary + "|" + k.bg],
          ["primary", "surface", k.primary + "|" + k.surface],
          ["primary", "tint", k.primary + "|" + k.tint],
          ["on-tint", "tint", k["on-tint"] + "|" + k.tint],
          ["on-accent", "accent", k["on-accent"] + "|" + k.accent],
          ["on-hero", "hero", k["on-hero"] + "|" + k.hero],
          ["focus", "surface", k.focus + "|" + k.surface],
        ];
        const bad = pairs.filter(([, , v]) => contrast(...(v.split("|") as [string, string])) < 4.5).map(([a, b, v]) => `${a} di ${b} ${contrast(...(v.split("|") as [string, string])).toFixed(2)}`);
        expect(bad).toEqual([]);
      });
      it(`${skin.id}/${mode}: warna status bersama terbaca di latar gaya`, () => {
        const k = t();
        const b = baseTokens(mode);
        const bad: string[] = [];
        for (const s of ["ok", "warn", "bad"]) {
          const c = k[s] ?? b[s]; // gaya boleh menimpa warna status bila bawaan kurang kontras di latarnya
          for (const back of [k.bg, k.surface]) if (contrast(c, back) < 4.5) bad.push(`${s} di ${back}`);
          if (contrast(c, k[s + "-bg"] ?? b[s + "-bg"]) < 4.5) bad.push(`${s} di ${s}-bg`);
        }
        expect(bad).toEqual([]);
      });
    }
  }
});

describe("kelengkapan", () => {
  it("setiap gaya punya blok terang dan gelap dan aturan struktur", () => {
    for (const s of SKINS) {
      expect(skinsCss).toContain(`html[data-skin="${s.id}"][data-theme="light"]`);
      expect(skinsCss).toContain(`html[data-skin="${s.id}"][data-theme="dark"]`);
      expect(skinsCss).toContain(`html[data-skin="${s.id}"] .hero`);
    }
  });
  it("tombol utama memakai token aksen", () => {
    expect(baseCss).toMatch(/\.btn-primary \{[^}]*var\(--accent/);
    expect(baseCss).toMatch(/\.btn-primary \{[^}]*var\(--on-accent/);
  });
  it("gerak dihormati: bengkel mematikan transisi bila prefers-reduced-motion", () => {
    expect(skinsCss).toMatch(/prefers-reduced-motion: reduce\)[\s\S]*bengkel/);
  });
});

describe("aturan tabel di gaya", () => {
  it("selektor th harus dibatasi thead: th di tbody adalah sel nama baris, bukan judul kolom (cacat garis hitam)", () => {
    const bad = skinsCss.split("\n").filter((l) => /\.tbl\s+th\b/.test(l) && !/\.tbl\s+thead\s+th\b/.test(l));
    expect(bad).toEqual([]);
  });
});
