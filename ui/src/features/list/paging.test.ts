import { describe, expect, it } from "vitest";
import { appendPage, emptyCursor, filterItems, newestName, norm, pageOf, pagePath, summary } from "./paging";

const nums = (n: number) => Array.from({ length: n }, (_, i) => i + 1);

describe("norm", () => {
  it("huruf kecil dan spasi dirapikan", () => expect(norm("  Foo   BAR\t x ")).toBe("foo bar x"));
});

describe("filterItems", () => {
  const sites = ["api.lab", "blog.lab", "urusin-demo.lab", "Urusin Landlord"];
  it("kueri kosong mengembalikan salinan semua, bukan referensi yang sama", () => {
    const r = filterItems(sites, "", (s) => s);
    expect(r).toEqual(sites);
    expect(r).not.toBe(sites);
  });
  it("kueri hanya spasi dianggap kosong", () => expect(filterItems(sites, "   ", (s) => s)).toHaveLength(4));
  it("tak peka huruf besar", () => expect(filterItems(sites, "URUSIN", (s) => s)).toEqual(["urusin-demo.lab", "Urusin Landlord"]));
  it("semua kata harus cocok (AND)", () => {
    expect(filterItems(sites, "urusin demo", (s) => s.replace("-", " "))).toEqual(["urusin-demo.lab"]);
    expect(filterItems(sites, "urusin tidakada", (s) => s)).toEqual([]);
  });
  it("karakter regex dicari apa adanya, tanpa galat", () => {
    expect(filterItems(["a.b", "axb", "a(b"], "a.b", (s) => s)).toEqual(["a.b"]);
    expect(filterItems(["a(b", "ab"], "(", (s) => s)).toEqual(["a(b"]);
    expect(filterItems(["x*y", "xy"], "*", (s) => s)).toEqual(["x*y"]);
    expect(filterItems(["100%", "100"], "%", (s) => s)).toEqual(["100%"]);
  });
  it("urutan asli dipertahankan", () => expect(filterItems(["b2", "a1", "b1"], "b", (s) => s)).toEqual(["b2", "b1"]));
  it("tidak mengubah input", () => {
    const src = ["b", "a"];
    filterItems(src, "a", (s) => s);
    expect(src).toEqual(["b", "a"]);
  });
});

describe("pageOf", () => {
  it("halaman pertama dan terakhir", () => {
    const a = pageOf(nums(25), 1, 10);
    expect([a.rows[0], a.rows.length, a.pages, a.from, a.to, a.total]).toEqual([1, 10, 3, 1, 10, 25]);
    const c = pageOf(nums(25), 3, 10);
    expect([c.rows, c.from, c.to]).toEqual([[21, 22, 23, 24, 25], 21, 25]);
  });
  it("tepat kelipatan ukuran halaman tidak menambah halaman kosong", () => {
    expect(pageOf(nums(20), 2, 10).pages).toBe(2);
    expect(pageOf(nums(21), 2, 10).pages).toBe(3);
  });
  it("kosong: satu halaman, from=0", () => {
    const e = pageOf([], 1, 10);
    expect([e.rows, e.pages, e.page, e.from, e.to, e.total]).toEqual([[], 1, 1, 0, 0, 0]);
  });
  it("halaman di luar rentang dijepit, bukan kosong", () => {
    expect(pageOf(nums(25), 99, 10).page).toBe(3);
    expect(pageOf(nums(25), 99, 10).rows).toHaveLength(5);
    expect(pageOf(nums(25), 0, 10).page).toBe(1);
    expect(pageOf(nums(25), -4, 10).page).toBe(1);
    expect(pageOf(nums(25), NaN, 10).page).toBe(1);
  });
  it("ukuran tidak sah jatuh ke bawaan, tidak membagi nol", () => {
    expect(pageOf(nums(25), 1, 0).rows).toHaveLength(10);
    expect(pageOf(nums(25), 1, -5).rows).toHaveLength(10);
    expect(pageOf(nums(25), 1, NaN).rows).toHaveLength(10);
  });
  it("satu item", () => {
    const v = pageOf([7], 1, 10);
    expect([v.rows, v.from, v.to, v.pages]).toEqual([[7], 1, 1, 1]);
  });
});

describe("summary", () => {
  it("kosong tanpa dan dengan pencarian", () => {
    expect(summary(pageOf([], 1), 0, false)).toBe("Kosong.");
    expect(summary(pageOf([], 1), 47, true)).toBe("Tidak ada yang cocok dari 47.");
  });
  it("muat satu halaman", () => {
    expect(summary(pageOf(nums(3), 1), 3, false)).toBe("3 item.");
    expect(summary(pageOf(nums(3), 1), 47, true)).toBe("3 dari 47 cocok.");
  });
  it("beberapa halaman", () => {
    expect(summary(pageOf(nums(47), 2, 10), 47, false)).toBe("11 sampai 20 dari 47");
    expect(summary(pageOf(nums(15), 1, 10), 47, true)).toBe("1 sampai 10 dari 15 (cocok dari 47)");
  });
});

describe("appendPage", () => {
  type R = { id: number; v: string };
  const r = (id: number): R => ({ id, v: `r${id}` });
  it("menambah baris dan memakai cursor serta total terbaru", () => {
    const s = appendPage<R>(emptyCursor(), { rows: [r(9), r(8)], next: 8, total: 20 });
    const t = appendPage(s, { rows: [r(7), r(6)], next: 0, total: 20 });
    expect(t.rows.map((x) => x.id)).toEqual([9, 8, 7, 6]);
    expect([t.next, t.total]).toEqual([0, 20]);
  });
  it("id ganda dibuang (halaman tumpang tindih)", () => {
    const s = appendPage<R>({ rows: [r(9), r(8)], next: 8, total: 5 }, { rows: [r(8), r(7)], next: 7, total: 6 });
    expect(s.rows.map((x) => x.id)).toEqual([9, 8, 7]);
  });
  it("rows null dari server tidak membuat galat", () => {
    const s = appendPage<R>({ rows: [r(1)], next: 1, total: 1 }, { rows: null as unknown as R[], next: 0, total: 1 });
    expect(s.rows).toHaveLength(1);
    expect(s.next).toBe(0);
  });
});

describe("pagePath", () => {
  it("limit saja", () => expect(pagePath("/api/audit", { limit: 25 })).toBe("/api/audit?limit=25"));
  it("cursor dan q, q dipangkas dan di-encode", () => {
    expect(pagePath("/api/audit", { limit: 25, cursor: 40, q: "  login gagal & x " })).toBe("/api/audit?limit=25&cursor=40&q=login+gagal+%26+x");
  });
  it("cursor 0/negatif dan q kosong tidak dikirim", () => {
    expect(pagePath("/api/jobs", { limit: 10, cursor: 0, q: "  " })).toBe("/api/jobs?limit=10");
    expect(pagePath("/api/jobs", { limit: 10, cursor: -3 })).toBe("/api/jobs?limit=10");
  });
});

describe("newestName", () => {
  it("yang waktunya terbesar, tidak peduli urutan daftar", () => {
    expect(newestName([{ name: "lama", at: 1 }, { name: "baru", at: 9 }, { name: "tengah", at: 5 }])).toBe("baru");
    expect(newestName([{ name: "baru", at: 9 }, { name: "lama", at: 1 }])).toBe("baru");
  });
  it("seri: yang lebih dulu di daftar", () => expect(newestName([{ name: "a", at: 3 }, { name: "b", at: 3 }])).toBe("a"));
  it("kosong: undefined, dan at=0 tetap dikenali", () => {
    expect(newestName([])).toBeUndefined();
    expect(newestName([{ name: "x", at: 0 }])).toBe("x");
  });
  it("tetap benar setelah difilter: yang terbaru dihitung dari daftar PENUH", () => {
    const all = [{ name: "b3", at: 3 }, { name: "b2", at: 2 }, { name: "b1", at: 1 }];
    const filtered = filterItems(all, "b2", (x) => x.name);
    expect(newestName(all)).toBe("b3");
    expect(filtered[0].name).toBe("b2"); // posisi 0 di hasil filter BUKAN yang terbaru
    expect(filtered[0].name === newestName(all)).toBe(false);
  });
});

import { applyFacet, countFacets, type Facet } from "./paging";

describe("facet daftar", () => {
  const items = [1, 2, 3, 4, 5, 6];
  const f: Facet<number>[] = [
    { id: "genap", label: "Genap", test: (n) => n % 2 === 0 },
    { id: "besar", label: "Besar", test: (n) => n > 4 },
  ];
  it("hitung dan terapkan", () => {
    expect(countFacets(items, f)).toEqual([
      { id: "all", label: "Semua", count: 6 },
      { id: "genap", label: "Genap", count: 3 },
      { id: "besar", label: "Besar", count: 2 },
    ]);
    expect(applyFacet(items, f, "besar")).toEqual([5, 6]);
    expect(applyFacet(items, f, "all")).toEqual(items);
    expect(applyFacet(items, f, "hilang")).toEqual(items);
  });
  it("tanpa facet hanya Semua", () => {
    expect(countFacets(items, [])).toHaveLength(1);
  });
});
