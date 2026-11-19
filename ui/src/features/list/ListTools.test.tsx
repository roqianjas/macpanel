// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FilterTabs, ListSearch, Pager, PagedList, useListView } from "./ListTools";

afterEach(cleanup);

const names = (n: number) => Array.from({ length: n }, (_, i) => `situs-${String(i + 1).padStart(2, "0")}`);

function Demo({ items, size = 10 }: { items: string[]; size?: number }) {
  const lv = useListView(items, (s) => s, size);
  return (
    <div>
      <ListSearch lv={lv} label="Cari situs" />
      <ul>
        {lv.view.rows.map((r) => (
          <li key={r}>{r}</li>
        ))}
      </ul>
      <Pager lv={lv} />
    </div>
  );
}

const items = () => screen.queryAllByRole("listitem").map((li) => li.textContent);
const status = () => screen.queryByRole("status")?.textContent;

describe("ListSearch", () => {
  it("tersembunyi bila daftar kecil dan belum dicari", () => {
    render(<Demo items={names(5)} />);
    expect(screen.queryByLabelText("Cari situs")).toBeNull();
    expect(items()).toHaveLength(5);
  });
  it("tampil bila daftar cukup besar, dengan label terlihat", () => {
    render(<Demo items={names(12)} />);
    expect(screen.getByLabelText("Cari situs")).toBeTruthy();
    expect(status()).toBe("1 sampai 10 dari 12");
  });
  it("mengetik menyaring, mengembalikan ke halaman 1, dan menyebut jumlah cocok", async () => {
    const u = userEvent.setup();
    render(<Demo items={names(30)} />);
    await u.click(screen.getByRole("button", { name: /Berikutnya/ }));
    expect(items()[0]).toBe("situs-11");
    await u.type(screen.getByLabelText("Cari situs"), "situs-2");
    expect(items()[0]).toBe("situs-20"); // kembali ke halaman 1 dari hasil saringan
    expect(items()).toHaveLength(10); // situs-20 sampai situs-29: tepat satu halaman penuh
    expect(status()).toBe("10 dari 30 cocok.");
    expect(screen.queryByRole("navigation")).toBeNull();
  });
  it("tanpa hasil: kotak cari tetap ada supaya bisa dikoreksi, ringkasan menjelaskan", async () => {
    const u = userEvent.setup();
    render(<Demo items={names(12)} />);
    await u.type(screen.getByLabelText("Cari situs"), "zzz");
    expect(items()).toHaveLength(0);
    expect(status()).toBe("Tidak ada yang cocok dari 12.");
    expect(screen.getByLabelText("Cari situs")).toBeTruthy();
    expect(screen.queryByRole("navigation")).toBeNull();
  });
  it("ringkasan memakai role status agar dibacakan pembaca layar", () => {
    render(<Demo items={names(12)} />);
    expect(screen.getByRole("status").getAttribute("aria-live")).toBe("polite");
  });
});

describe("Pager", () => {
  it("tidak muncul bila hanya satu halaman", () => {
    render(<Demo items={names(10)} />);
    expect(screen.queryByRole("navigation", { name: "Halaman daftar" })).toBeNull();
  });
  it("halaman pertama: Sebelumnya nonaktif; terakhir: Berikutnya nonaktif", async () => {
    const u = userEvent.setup();
    render(<Demo items={names(25)} />);
    const prev = () => screen.getByRole("button", { name: /Sebelumnya/ }) as HTMLButtonElement;
    const next = () => screen.getByRole("button", { name: /Berikutnya/ }) as HTMLButtonElement;
    expect([prev().disabled, next().disabled]).toEqual([true, false]);
    await u.click(next());
    await u.click(next());
    expect(screen.getByText("Halaman 3 dari 3")).toBeTruthy();
    expect([prev().disabled, next().disabled]).toEqual([false, true]);
    expect(items()).toHaveLength(5);
  });
  it("REGRESI: sampai di halaman terakhir, fokus pindah ke Sebelumnya (tidak jatuh ke body)", async () => {
    const u = userEvent.setup();
    render(<Demo items={names(25)} />);
    const next = screen.getByRole("button", { name: /Berikutnya/ });
    next.focus();
    await u.keyboard("{Enter}");
    await u.keyboard("{Enter}"); // sekarang di halaman 3, Berikutnya nonaktif
    expect(screen.getByText("Halaman 3 dari 3")).toBeTruthy();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: /Sebelumnya/ }));
  });
  it("REGRESI: kembali ke halaman 1, fokus pindah ke Berikutnya", async () => {
    const u = userEvent.setup();
    render(<Demo items={names(25)} />);
    await u.click(screen.getByRole("button", { name: /Berikutnya/ }));
    const prev = screen.getByRole("button", { name: /Sebelumnya/ });
    prev.focus();
    await u.keyboard("{Enter}");
    expect(screen.getByText("Halaman 1 dari 3")).toBeTruthy();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: /Berikutnya/ }));
  });
  it("di tengah (bukan batas) fokus tetap di tombol yang ditekan", async () => {
    const u = userEvent.setup();
    render(<Demo items={names(45)} />);
    const next = screen.getByRole("button", { name: /Berikutnya/ });
    next.focus();
    await u.keyboard("{Enter}");
    expect(screen.getByText("Halaman 2 dari 5")).toBeTruthy();
    expect(document.activeElement).toBe(next);
  });
  it("menandai halaman sekarang untuk teknologi bantu", () => {
    render(<Demo items={names(25)} />);
    const nav = screen.getByRole("navigation", { name: "Halaman daftar" });
    expect(within(nav).getByText("Halaman 1 dari 3").getAttribute("aria-current")).toBe("page");
  });
});

describe("useListView: item berubah", () => {
  it("halaman dijepit bila item berkurang (mis. dihapus) dan tidak menampilkan halaman kosong", async () => {
    const u = userEvent.setup();
    const { rerender } = render(<Demo items={names(25)} />);
    await u.click(screen.getByRole("button", { name: /Berikutnya/ }));
    await u.click(screen.getByRole("button", { name: /Berikutnya/ }));
    expect(items()).toHaveLength(5);
    rerender(<Demo items={names(12)} />);
    expect(items().length).toBeGreaterThan(0);
    expect(screen.getByText("Halaman 2 dari 2")).toBeTruthy();
  });
});

describe("PagedList", () => {
  it("merender li dari render() dan memakai pencarian serta halaman", async () => {
    const u = userEvent.setup();
    render(<PagedList items={names(23)} text={(s) => s} label="Cari database" render={(s) => <li key={s}>{s}</li>} />);
    expect(items()).toHaveLength(10);
    await u.type(screen.getByLabelText("Cari database"), "-2");
    expect(items().every((t) => t?.includes("-2"))).toBe(true);
    expect(items()).toEqual(["situs-20", "situs-21", "situs-22", "situs-23"]);
  });
  it("daftar kecil: tanpa kotak cari dan tanpa pager", () => {
    render(<PagedList items={names(3)} text={(s) => s} render={(s) => <li key={s}>{s}</li>} />);
    expect(screen.queryByRole("searchbox")).toBeNull();
    expect(screen.queryByRole("navigation")).toBeNull();
    expect(items()).toHaveLength(3);
  });
});

describe("tab filter", () => {
  const items = ["a-php", "b-php", "c-proxy", "d-proxy", "e-proxy", "f-kustom"];
  const facets = ["php", "proxy", "kustom", "statis"].map((k) => ({ id: k, label: k, test: (s: string) => s.endsWith(k) }));
  function F() {
    const lv = useListView(items, (s) => s, 2, facets);
    return (
      <div>
        <FilterTabs lv={lv} label="Filter jenis" />
        <ListSearch lv={lv} label="Cari" minItems={1} />
        <ul>{lv.view.rows.map((r) => <li key={r}>{r}</li>)}</ul>
        <Pager lv={lv} />
      </div>
    );
  }
  it("menyaring, menjaga angka tab, dan kembali ke halaman 1", async () => {
    const u = userEvent.setup();
    render(<F />);
    const bar = screen.getByRole("group", { name: "Filter jenis" });
    expect(within(bar).getByRole("button", { name: /^proxy 3$/ })).toBeTruthy();
    expect(within(bar).queryByRole("button", { name: /statis/ })).toBeNull(); // 0 item disembunyikan
    await u.click(screen.getByRole("button", { name: "Berikutnya" })); // halaman 2 dari Semua
    await u.click(within(bar).getByRole("button", { name: /^proxy/ }));
    expect(screen.getAllByRole("listitem").map((l) => l.textContent)).toEqual(["c-proxy", "d-proxy"]); // halaman 1 lagi
    expect(within(bar).getByRole("button", { name: /^proxy/ }).getAttribute("aria-pressed")).toBe("true");
    expect(within(bar).getByRole("button", { name: /^php 2$/ })).toBeTruthy(); // angka tidak berubah
    await u.type(screen.getByRole("searchbox"), "e-");
    expect(screen.getAllByRole("listitem").map((l) => l.textContent)).toEqual(["e-proxy"]); // filter + cari bersama
  });
});
