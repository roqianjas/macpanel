// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";
import { isImgSrc, LOGOS, parseLogo } from "./logo";
import { Skeleton, State } from "../../ui";

afterEach(cleanup);
const PNG = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==";

describe("logo tersimpan", () => {
  it("menerima ikon yang dikenal dan PNG data; selain itu kembali ke kunci", () => {
    expect(parseLogo("icon:rocket")).toEqual({ kind: "icon", id: "rocket" });
    expect(parseLogo(`img:${PNG}`)).toEqual({ kind: "img", src: PNG });
    for (const bad of [null, "", "icon:tidak-ada", "img:javascript:alert(1)", "img:https://evil.test/x.png", "img:data:image/svg+xml;base64,PHN2Zz4=", "img:data:text/html;base64,PGgxPg=="]) {
      expect(parseLogo(bad), String(bad)).toEqual({ kind: "icon", id: "key" });
    }
  });
  it("isImgSrc menolak apa pun selain PNG data", () => {
    expect(isImgSrc(PNG)).toBe(true);
    expect(isImgSrc("data:image/svg+xml;base64,PHN2Zz4=AAAAAAAA")).toBe(false);
    expect(isImgSrc("https://x/y.png")).toBe(false);
    expect(isImgSrc(5)).toBe(false);
  });
  it("id ikon unik dan ada ikon bawaan", () => {
    const ids = LOGOS.map((l) => l.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect(ids).toContain("key");
  });
});

describe("skeleton", () => {
  it("State memuat memakai skeleton beraksesibilitas (bukan teks lepas), dengan bentuk sesuai jenis", () => {
    const a = render(<State loading error="">isi</State>).container;
    expect(a.querySelector('[role="status"][aria-busy="true"]')).toBeTruthy();
    expect(a.querySelector(".sr")?.textContent).toBe("Memuat…");
    expect(a.querySelectorAll(".sk-row").length).toBe(5);
    expect(render(<Skeleton kind="cards" />).container.querySelectorAll(".sk-card").length).toBe(4);
    expect(render(<Skeleton kind="chart" />).container.querySelectorAll(".sk-plot").length).toBe(1);
  });
  it("tidak ada skeleton saat data sudah ada", () => {
    expect(render(<State loading={false} error="">isi</State>).container.querySelector(".sk-wrap")).toBeNull();
  });
});
