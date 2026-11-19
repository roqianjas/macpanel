/// <reference types="node" />
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const walk = (d: string): string[] => readdirSync(d).flatMap((n) => (statSync(join(d, n)).isDirectory() ? walk(join(d, n)) : /\.tsx?$/.test(n) && !n.includes("test") ? [join(d, n)] : []));

describe("useEffect", () => {
  it("tidak ada efek berbadan-ekspresi: nilai baliknya dianggap cleanup oleh React dan memanggilnya saat tutup ('x is not a function', layar putih)", () => {
    const bad: string[] = [];
    for (const f of walk("src")) {
      readFileSync(f, "utf8").split("\n").forEach((l, i) => {
        if (/use(Layout)?Effect\(\(\) => [^{]/.test(l)) bad.push(`${f}:${i + 1}`);
      });
    }
    expect(bad).toEqual([]);
  });
});
