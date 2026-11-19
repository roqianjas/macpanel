import { describe, expect, it } from "vitest";
import { displayName, loopbackCount, sortItems, statusInfo, summary, visibleItems, type ExItem, type ExView, type Status } from "./model";

const it_ = (o: Partial<ExItem>): ExItem => ({ key: "x:1", proc: "x", pid: 1, port: 1, bind: "*", scope: "all", system: false, lan_reachable: true, service: "", hidden: false, status: "flagged", ...o });
const view = (o: Partial<ExView> = {}): ExView => ({ now: { at: 1, lan_ip: "192.0.2.10", errors: [] }, stale: false, items: [], allowed: [], defaults: [], findings: 0, confirm_samples: 2, interval: 300, can_edit: true, ...o });

describe("statusInfo", () => {
  const all: Status[] = ["flagged", "allowed", "system", "tailnet", "blocked", "unknown", "loopback"];
  it("semua status punya teks dan petunjuk (tidak hanya warna)", () => {
    for (const s of all) {
      const i = statusInfo(s);
      expect(i.label.length).toBeGreaterThan(3);
      expect(i.hint.length).toBeGreaterThan(10);
    }
  });
  it("hanya flagged yang bernada bad", () => {
    expect(statusInfo("flagged").tone).toBe("bad");
    for (const s of all.filter((x) => x !== "flagged")) expect(statusInfo(s).tone).not.toBe("bad");
  });
  it("label unik per status", () => expect(new Set(all.map((s) => statusInfo(s).label)).size).toBe(all.length));
});

describe("sortItems / visibleItems", () => {
  const items = [
    it_({ key: "a:5", proc: "a", port: 5, status: "loopback" }),
    it_({ key: "n:80", proc: "nginx", port: 80, status: "allowed" }),
    it_({ key: "m:8025", proc: "mailpit", port: 8025, status: "flagged" }),
    it_({ key: "r:9", proc: "rapportd", port: 9, status: "system" }),
    it_({ key: "z:1", proc: "z", port: 1, status: "flagged" }),
  ];
  it("yang perlu perhatian dulu, lalu menurut port", () => {
    expect(sortItems(items).map((i) => i.key)).toEqual(["z:1", "m:8025", "n:80", "r:9", "a:5"]);
  });
  it("tidak mengubah input", () => {
    const copy = items.map((i) => i.key);
    sortItems(items);
    expect(items.map((i) => i.key)).toEqual(copy);
  });
  it("tabel menyembunyikan loopback tetapi menghitungnya", () => {
    expect(visibleItems(items).some((i) => i.status === "loopback")).toBe(false);
    expect(visibleItems(items)).toHaveLength(4);
    expect(loopbackCount(items)).toBe(1);
  });
});

describe("displayName", () => {
  it("proses terlihat memakai namanya", () => expect(displayName({ proc: "mailpit", hidden: false, service: "" })).toBe("mailpit"));
  it("proses tidak terlihat menyebut tebakan layanan", () => {
    expect(displayName({ proc: "?", hidden: true, service: "ssh" })).toBe("Proses tidak terlihat (ssh)");
    expect(displayName({ proc: "?", hidden: true, service: "" })).toBe("Proses tidak terlihat");
  });
});

describe("summary", () => {
  it("belum ada data", () => expect(summary(view({ now: null }))).toContain("Belum ada pemeriksaan"));
  it("data basi tidak ditampilkan seperti kondisi terkini", () => expect(summary(view({ stale: true, items: [it_({})], findings: 1 }))).toContain("berhenti"));
  it("tanpa IP LAN: jujur bahwa tidak teruji, bukan 'aman'", () => {
    const s = summary(view({ now: { at: 1, lan_ip: null, errors: [] } }));
    expect(s).toContain("tidak dapat diuji");
    expect(s).not.toContain("Tidak ada layanan");
  });
  it("menghitung yang perlu perhatian", () => expect(summary(view({ items: [it_({}), it_({ key: "y:2" })], findings: 2 }))).toBe("2 layanan terbuka ke jaringan rumah dan perlu perhatian."));
  it("terbuka tetapi menunggu konfirmasi", () => expect(summary(view({ items: [it_({ status: "allowed" })], findings: 1 }))).toContain("menunggu konfirmasi"));
  it("aman: hanya bila benar-benar tidak ada", () => expect(summary(view({ items: [it_({ status: "allowed" })] }))).toBe("Tidak ada layanan tak disengaja yang terbuka ke jaringan rumah."));
});
