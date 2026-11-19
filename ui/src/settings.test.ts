// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.stubGlobal("matchMedia", () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
vi.stubGlobal("fetch", () => Promise.resolve({ ok: false }));

const S = await import("./settings");
const el = document.documentElement;

beforeEach(() => {
  localStorage.clear();
  S.setAccent(null);
  S.setDevicePref("light");
});

describe("gaya dan aksen perangkat", () => {
  it("bawaan kertas bila belum memilih atau nilai rusak", () => {
    expect(S.currentSkin()).toBe("kertas");
    localStorage.setItem("panel-skin", "<script>");
    expect(S.currentSkin()).toBe("kertas");
  });
  it("setSkin menyimpan dan memasang data-skin", () => {
    S.setSkin("bengkel");
    expect(localStorage.getItem("panel-skin")).toBe("bengkel");
    expect(el.getAttribute("data-skin")).toBe("bengkel");
    S.setSkin("mesin");
    expect(el.getAttribute("data-skin")).toBe("mesin");
  });
  it("aksen: hanya #rrggbb, huruf kecil; rusak diabaikan; null mengembalikan bawaan gaya", () => {
    S.setAccent("#C2410C");
    expect(S.currentAccent()).toBe("#c2410c");
    S.setAccent("merah");
    expect(S.currentAccent()).toBe("#c2410c"); // masukan rusak tidak menimpa
    localStorage.setItem("panel-accent", "javascript:alert(1)");
    expect(S.currentAccent()).toBeNull();
    S.setAccent(null);
    expect(el.style.getPropertyValue("--accent")).toBe("");
  });
  it("aksen memasang variabel aman kontras dan menghitung ulang saat mode berubah", () => {
    S.setSkin("bengkel");
    S.setAccent("#c6f432");
    expect(el.style.getPropertyValue("--accent")).toBe("#c6f432");
    const lightPrimary = el.style.getPropertyValue("--primary");
    expect(lightPrimary).not.toBe("#c6f432"); // limau tidak terbaca sebagai teks di latar terang
    S.setDevicePref("dark");
    expect(el.getAttribute("data-theme")).toBe("dark");
    expect(el.style.getPropertyValue("--primary")).not.toBe(lightPrimary);
  });
  it("memilih warna bawaan menghapus semua variabel inline", () => {
    S.setAccent("#1d4ed8");
    expect(el.style.getPropertyValue("--primary")).not.toBe("");
    S.setAccent(null);
    for (const v of ["--accent", "--on-accent", "--primary", "--on-tint", "--focus", "--tint"]) expect(el.style.getPropertyValue(v)).toBe("");
  });
});
