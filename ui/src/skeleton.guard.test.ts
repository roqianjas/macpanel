import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const walk = (d: string): string[] => readdirSync(d).flatMap((n) => (statSync(join(d, n)).isDirectory() ? walk(join(d, n)) : n.endsWith(".tsx") && !n.includes("test") ? [join(d, n)] : []));

describe("status memuat konsisten", () => {
  it("tidak ada teks 'Memuat…' lepas di luar Skeleton (pakai <Skeleton/> atau State loading)", () => {
    const bad: string[] = [];
    for (const f of walk("src")) {
      readFileSync(f, "utf8").split("\n").forEach((l, i) => {
        if (/Memuat…/.test(l) && !/busy|Muat \$\{|className="sr"/.test(l)) bad.push(`${f}:${i + 1}`);
      });
    }
    expect(bad).toEqual([]);
  });
  it("skeleton tanpa kartu memakai gaya kartu sendiri; di dalam kartu/dialog dilucuti", () => {
    const css = readFileSync("src/styles.css", "utf8");
    expect(css).toMatch(/\.sk-wrap \{[^}]*border: 1px solid var\(--line\)/);
    expect(css).toMatch(/\.card \.sk-wrap, \.modal \.sk-wrap[^{]*\{[^}]*border: 0/);
  });
  it("State yang sedang memuat selalu menampilkan skeleton, walau anaknya kosong ({data && ...})", () => {
    const src = readFileSync("src/ui.tsx", "utf8");
    expect(src).not.toMatch(/loading && !children\) return null/);
  });
});
