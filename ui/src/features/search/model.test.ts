import { describe, expect, it } from "vitest";
import { fromDatabases, fromServices, fromSites, fromTasks, PAGES, rank, score, type Item } from "./model";

const labels = (q: string, extra: Item[] = [], role = "admin") => rank([...PAGES, ...extra], q, role).map((h) => h.item.label);

describe("pencarian global", () => {
  it("tanpa kata kunci hanya halaman utama (bukan tab atau subbagian)", () => {
    const l = labels("");
    expect(l).toContain("Ringkasan");
    expect(l).toContain("Worker dan cron");
    expect(l).not.toContain("Tailscale");
    expect(l).not.toContain("Cron job (terjadwal)");
  });
  it("menemukan tab lewat nama dan lewat istilah yang dicari orang", () => {
    expect(labels("tailscale")[0]).toBe("Tailscale");
    expect(labels("vpn")).toContain("Tailscale");
    expect(labels("uninstall")).toContain("Tailscale");
    expect(labels("crontab")).toContain("Cron job (terjadwal)");
    expect(labels("bersihkan cache")[0]).toBe("Ruang dan memori");
    expect(labels("favicon")).toContain("Pengaturan");
  });
  it("awal label mengalahkan isi label dan kata kunci", () => {
    const hits = rank(PAGES, "sistem");
    expect(hits[0].item.label).toBe("Sistem");
    expect(score(PAGES.find((p) => p.label === "Sistem")!, "sistem")).toBeGreaterThan(score(PAGES.find((p) => p.label === "Keamanan")!, "sistem"));
  });
  it("semua kata harus cocok", () => {
    expect(labels("cron jadwal")).toContain("Cron job (terjadwal)");
    expect(labels("cron zzzz")).toEqual([]);
  });
  it("huruf besar dan aksen diabaikan; tidak ada hasil bila tak cocok", () => {
    expect(labels("TAILSCALE")).toContain("Tailscale");
    expect(labels("xqzv")).toEqual([]);
  });
  it("halaman khusus admin tersembunyi bagi viewer", () => {
    expect(labels("pengguna")).toContain("Pengguna");
    expect(labels("pengguna", [], "viewer")).not.toContain("Pengguna");
  });
  it("data dinamis: situs, layanan, database, tugas, dan batas per grup", () => {
    const items = [
      ...fromSites([{ name: "urusin", host: "urusin.lab", mode: "php", system: false, enabled: true }]),
      ...fromServices([{ id: "redis", label: "Redis", group: "Cache dan antrean", state: "running" }]),
      ...fromDatabases({ mysql: ["urusin_demo"], pgsql: [] }),
      ...fromTasks("job", [{ name: "backup-malam", status: "terjadwal" }]),
    ];
    const l = rank([...PAGES, ...items], "urusin");
    expect(l.map((h) => h.item.group)).toEqual(["Website", "Database"]);
    expect(rank([...PAGES, ...items], "backup").some((h) => h.item.hash === "#/tugas/job")).toBe(true);
    expect(rank(items, "redis")[0].item.hash).toBe("#/aplikasi");
    const many = Array.from({ length: 30 }, (_, i) => fromSites([{ name: `x${i}`, host: `x${i}.lab`, mode: "php", system: false, enabled: true }])[0]);
    expect(rank(many, "x").length).toBe(6);
  });
  it("tujuan situs di-encode dan tidak bisa menyuntik hash", () => {
    const [it] = fromSites([{ name: "a/b c", host: "h", mode: "php", system: false, enabled: true }]);
    expect(it.hash).toBe("#/website/a%2Fb%20c");
  });
});
