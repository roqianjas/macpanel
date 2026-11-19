// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";
import { TabIcon } from "./tabicons";
import { SITE_FACETS } from "./features/sites/facets";

afterEach(cleanup);

describe("ikon tab", () => {
  it("id dikenal mendapat ikon 16px dekoratif; id tak dikenal hanya titik kecil", () => {
    const a = render(<TabIcon id="proxy" />).container.querySelector("svg")!;
    expect(a.getAttribute("aria-hidden")).toBe("true");
    expect(a.getAttribute("width")).toBe("16");
    const b = render(<TabIcon id="status-aneh" />).container.querySelector("svg")!;
    expect(b.getAttribute("width")).toBe("8");
  });
  it("setiap filter situs punya ikon relevan (bukan titik)", () => {
    for (const f of SITE_FACETS) {
      const w = render(<TabIcon id={f.id} />).container.querySelector("svg")!.getAttribute("width");
      expect(w, f.id).toBe("16");
    }
  });
});
