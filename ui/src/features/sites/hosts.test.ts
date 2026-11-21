import { describe, expect, it } from "vitest";
import { normalizeHost, normalizeHosts } from "./hosts";

describe("normalizeHost", () => {
  it("URL lengkap menjadi nama host saja", () => {
    expect(normalizeHost("https://dreams-qualities-jake-mark.trycloudflare.com")).toBe("dreams-qualities-jake-mark.trycloudflare.com");
    expect(normalizeHost("https://Mesin.Contoh.ts.net/login?x=1#a")).toBe("mesin.contoh.ts.net");
    expect(normalizeHost("http://a.example.com:8443/")).toBe("a.example.com");
    expect(normalizeHost("  www.contoh.lab.  ")).toBe("www.contoh.lab");
  });
  it("nama biasa tidak berubah, bagian ketikan tidak dirusak", () => {
    expect(normalizeHost("www.contoh.lab")).toBe("www.contoh.lab");
    expect(normalizeHost("https:")).toBe("https:");
    expect(normalizeHost("")).toBe("");
  });
  it("tidak menyelundupkan isi lewat userinfo atau jalur", () => {
    expect(normalizeHost("https://evil.com@good.example.com/x")).toBe("good.example.com");
    expect(normalizeHost("a.com/../b.com")).toBe("a.com");
  });
  it("banyak baris: kosong dibuang", () => {
    expect(normalizeHosts("https://a.com/\n\n b.lab \n")).toEqual(["a.com", "b.lab"]);
  });
});
