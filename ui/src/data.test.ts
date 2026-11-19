import { describe, expect, it } from "vitest";
import { serviceView } from "./data";

describe("serviceView", () => {
  it("layanan opsional yang berhenti = dimatikan sengaja, netral", () => {
    expect(serviceView({ state: "stopped", optional: true })).toEqual({ text: "Dimatikan", tone: "mute" });
  });
  it("layanan inti yang berhenti tetap peringatan", () => {
    expect(serviceView({ state: "stopped", optional: false })).toEqual({ text: "Berhenti", tone: "warn" });
  });
  it("galat tetap galat walau opsional; berjalan tetap ok", () => {
    expect(serviceView({ state: "error", optional: true }).tone).toBe("bad");
    expect(serviceView({ state: "running", optional: true })).toEqual({ text: "Berjalan", tone: "ok" });
  });
});
