import { describe, expect, it } from "vitest";
import { dnsStatus, fmtMs, internetStatus, netFreshness, tailscaleStatus, uptimePct, type NetData } from "./model";

const ts = (o: object = {}) => ({ state: "Running", self_online: true, peers: 6, peers_online: 2, health: [], ...o });

describe("tailscaleStatus", () => {
  it("tersambung", () => expect(tailscaleStatus(ts())).toEqual({ tone: "ok", text: "Tersambung" }));
  it("berhenti dan perlu login adalah penting, karena akses jarak jauh hilang", () => {
    expect(tailscaleStatus(ts({ state: "Stopped" }))).toEqual({ tone: "bad", text: "Berhenti" });
    expect(tailscaleStatus(ts({ state: "NeedsLogin" }))).toEqual({ tone: "bad", text: "Perlu login" });
    expect(tailscaleStatus(ts({ state: "Starting" })).tone).toBe("bad");
  });
  it("berjalan tetapi tidak online", () => expect(tailscaleStatus(ts({ self_online: false }))).toEqual({ tone: "bad", text: "Tidak online" }));
  it("tidak terbaca bukan 'tersambung'", () => expect(tailscaleStatus(null).tone).toBe("mute"));
});

describe("internetStatus", () => {
  it("normal, lambat (di atas ambang server), terputus", () => {
    expect(internetStatus({ ok: true, ms: 6 }, 300).tone).toBe("ok");
    expect(internetStatus({ ok: true, ms: 300 }, 300).tone).toBe("ok");
    expect(internetStatus({ ok: true, ms: 301 }, 300)).toEqual({ tone: "warn", text: "Lambat" });
    expect(internetStatus({ ok: false, ms: null }, 300)).toEqual({ tone: "bad", text: "Terputus" });
    expect(internetStatus(undefined, 300).tone).toBe("mute");
  });
});

describe("dnsStatus", () => {
  it("tidak dites bukan gagal", () => expect(dnsStatus(null).tone).toBe("mute"));
  it("terjawab dan gagal", () => {
    expect(dnsStatus({ name: "a", ok: true, ms: 1 }).tone).toBe("ok");
    expect(dnsStatus({ name: "a", ok: false, ms: null }).tone).toBe("bad");
  });
});

describe("uptimePct", () => {
  it("tanpa sampel tidak mengarang angka", () => expect(uptimePct(0, 0)).toBe("n/a"));
  it("semua baik = 100", () => expect(uptimePct(0, 500)).toBe("100%"));
  it("masih ada kegagalan tidak pernah dibulatkan ke 100", () => {
    expect(uptimePct(1, 5000)).toBe("99,9%");
    expect(uptimePct(1, 100000)).toBe("99,9%");
  });
  it("angka biasa", () => {
    expect(uptimePct(5, 100)).toBe("95%");
    expect(uptimePct(1, 3)).toBe("66,7%");
  });
});

const base = (o: Partial<NetData> = {}): NetData => ({
  now: { at: 1000, tailscale: null, internet: { ok: true, ms: 5 }, dns_local: null, dns_public: null, errors: [] },
  stale: false, range: "1h", bucket: 60, interval: 60, series: {}, samples: 1, ts_down: 0, inet_down: 0, dns_local_down: 0, dns_local_n: 0, inet_max_ms: 0, thresholds: { slow_ms: 300 },
  ...o,
});

describe("netFreshness dan fmtMs", () => {
  it("belum ada sampel", () => expect(netFreshness(null, 5).tone).toBe("mute"));
  it("basi", () => expect(netFreshness(base({ stale: true }), 1000 + 1200).text).toContain("20 menit"));
  it("galat dari sumber ditampilkan", () => {
    const d = base();
    d.now = { ...d.now!, errors: ["tailscale tidak ditemukan"] };
    expect(netFreshness(d, 1010)).toEqual({ tone: "warn", text: "tailscale tidak ditemukan" });
  });
  it("normal", () => expect(netFreshness(base(), 1030).text).toBe("Diperbarui 30 detik lalu."));
  it("format ms memakai koma dan tidak mengarang", () => {
    expect(fmtMs(5.8)).toBe("5,8 ms");
    expect(fmtMs(123.4)).toBe("123 ms");
    expect(fmtMs(null)).toBe("n/a");
    expect(fmtMs(NaN)).toBe("n/a");
  });
});
