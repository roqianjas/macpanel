// Penjaga form: setiap kolom isian teks harus punya placeholder (contoh nyata) kecuali yang memang tidak perlu.
// Memindai sumber, bukan DOM, agar kolom baru yang lupa diberi contoh langsung gagal di make check.
import { describe, expect, it } from "vitest";

const sources = import.meta.glob("./**/*.tsx", { query: "?raw", import: "default", eager: true }) as Record<string, string>;

const SKIP_TYPES = /type="(checkbox|radio|hidden|file|time|search)"/;
const ALLOW = new Set(["./features/sites/ConfigEditor.tsx"]); // editor config penuh: isinya berkas, bukan contoh

describe("form guard", () => {
  it("semua input dan textarea punya placeholder", () => {
    const bad: string[] = [];
    for (const [rel, src] of Object.entries(sources)) {
      if (ALLOW.has(rel)) continue;
      const lines = src.split("\n");
      lines.forEach((ln: string, i: number) => {
        if (!/<(input|textarea)\b/.test(ln)) return;
        let chunk = ln;
        for (let k = i + 1; k < lines.length && !/\/>|<\/textarea>/.test(chunk); k++) chunk += " " + lines[k];
        if (SKIP_TYPES.test(chunk) || chunk.includes("placeholder=")) return;
        bad.push(`${rel}:${i + 1}`);
      });
    }
    expect(bad).toEqual([]);
  });
});
