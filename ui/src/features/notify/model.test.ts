import { describe, expect, it } from "vitest";
import { genTopic, lastText, serverError, topicError } from "./model";

describe("topicError", () => {
  it("menerima topik sah", () => {
    for (const t of ["abcdefghijklmnop", "A_b-c_d-e_f-g_h-i", "a".repeat(64)]) expect(topicError(t)).toBe("");
  });
  it("menolak yang tidak sah", () => {
    for (const t of ["", "pendek", "a".repeat(65), "ada spasi di sini aaaa", "titik.titik.titik.aaa", "garis/miring/aaaaaaaa", "../../etc/passwd/aaaa", "tanya?x=1aaaaaaaaaaaa"]) expect(topicError(t)).not.toBe("");
  });
});

describe("serverError", () => {
  it("menerima https publik", () => {
    expect(serverError("https://ntfy.sh")).toBe("");
    expect(serverError(" https://ntfy.example.com:8443/sub ")).toBe("");
    expect(serverError("https://100.77.70.34")).toBe("");
  });
  it.each([
    "", "ntfy.sh", "http://ntfy.sh", "https://user:pw@ntfy.sh", "https://ntfy.sh?x=1", "https://ntfy.sh#a",
    "https://localhost", "https://panel.lab", "https://x.local", "https://127.0.0.1", "https://10.1.2.3", "https://192.168.0.9",
    "https://172.16.0.1", "https://172.31.255.1", "https://169.254.169.254", "https://[::1]",
  ])("menolak %s", (s) => expect(serverError(s)).not.toBe(""));
  it("172.32 bukan privat", () => expect(serverError("https://172.32.0.1")).toBe(""));
});

describe("genTopic", () => {
  it("panjang 24, lolos aturan topik, dari alfabet yang diizinkan", () => {
    const t = genTopic();
    expect(t).toHaveLength(24);
    expect(topicError(t)).toBe("");
  });
  it("dua topik berbeda", () => expect(genTopic()).not.toBe(genTopic()));
  it("memetakan semua nilai byte ke karakter sah tanpa bias", () => {
    const all = Uint8Array.from({ length: 256 }, (_, i) => i);
    const counts = new Map<string, number>();
    for (let off = 0; off < 256; off += 24) {
      const chunk = all.slice(off, off + 24);
      if (chunk.length < 24) break;
      for (const c of genTopic(() => chunk)) counts.set(c, (counts.get(c) ?? 0) + 1);
    }
    for (const c of counts.keys()) expect(/[A-Za-z0-9_-]/.test(c)).toBe(true);
    expect(genTopic(() => new Uint8Array(24).fill(255))).toBe("-".repeat(24));
    expect(genTopic(() => new Uint8Array(24).fill(64))).toBe("A".repeat(24));
  });
});

describe("lastText", () => {
  it("belum pernah", () => {
    expect(lastText(undefined, 100)).toBe("Belum pernah mengirim.");
    expect(lastText({ at: 0, ok: false }, 100)).toBe("Belum pernah mengirim.");
  });
  it("berhasil dan gagal", () => {
    expect(lastText({ at: 100, ok: true }, 130)).toBe("Pengiriman terakhir berhasil, 30 detik lalu.");
    expect(lastText({ at: 100, ok: false, error: "HTTP 500" }, 100 + 7300)).toBe("Pengiriman terakhir gagal, 2 jam lalu: HTTP 500");
    expect(lastText({ at: 100, ok: false }, 160)).toContain("alasan tidak diketahui");
  });
});
