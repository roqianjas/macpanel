import { describe, expect, it } from "vitest";
import { applyFacet, countFacets } from "../list/paging";
import { SITE_FACETS } from "./facets";
import type { Site } from "./types";

const mk = (name: string, mode: string, o: Partial<Site> = {}): Site => ({ name, host: name + ".lab", mode, target: "", system: false, enabled: true, http: 200, ms: 1, ssl_days: 100, ...o });
const sites = [mk("a", "php"), mk("b", "proxy"), mk("c", "proxy", { system: true }), mk("d", "kustom", { system: true }), mk("e", "php", { enabled: false })];

describe("filter situs", () => {
  it("menghitung per jenis dari seluruh daftar", () => {
    const c = Object.fromEntries(countFacets(sites, SITE_FACETS).map((f) => [f.id, f.count]));
    expect(c).toMatchObject({ all: 5, php: 2, proxy: 2, statis: 0, kustom: 1, sistem: 2, jeda: 1 });
  });
  it("menerapkan filter; id tak dikenal = semua", () => {
    expect(applyFacet(sites, SITE_FACETS, "php").map((s) => s.name)).toEqual(["a", "e"]);
    expect(applyFacet(sites, SITE_FACETS, "jeda").map((s) => s.name)).toEqual(["e"]);
    expect(applyFacet(sites, SITE_FACETS, "tidak-ada")).toHaveLength(5);
  });
});
