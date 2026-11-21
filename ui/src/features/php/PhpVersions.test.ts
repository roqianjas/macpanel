import { describe, expect, it } from "vitest";
import { webSummary } from "./PhpVersions";

describe("webSummary", () => {
  it("menghitung situs per versi, terurut, situs tanpa PHP diabaikan", () => {
    const sites = [
      { site: "a", version: "8.4" },
      { site: "b", version: "8.3" },
      { site: "c", version: "8.3" },
      { site: "d", version: null },
      { site: "e", version: "8.10" },
    ];
    expect(webSummary(sites)).toEqual([{ version: "8.3", n: 2 }, { version: "8.4", n: 1 }, { version: "8.10", n: 1 }]);
  });
  it("ratusan situs tetap hanya satu baris ringkas per versi", () => {
    const many = Array.from({ length: 500 }, (_, i) => ({ site: `s${i}`, version: i % 2 ? "8.3" : "8.4" }));
    expect(webSummary(many)).toEqual([{ version: "8.3", n: 250 }, { version: "8.4", n: 250 }]);
  });
  it("kosong", () => {
    expect(webSummary([])).toEqual([]);
  });
});
