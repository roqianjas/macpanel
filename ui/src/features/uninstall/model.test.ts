import { describe, expect, it } from "vitest";
import { applyCommand, LEVELS, planCommand, TAILSCALE } from "./model";

describe("uninstall guide", () => {
  it("rencana tidak pernah memuat --apply, perintah jalan selalu memuatnya", () => {
    for (const l of LEVELS) {
      expect(planCommand(l.id)).not.toContain("--apply");
      expect(applyCommand(l.id).endsWith(" --apply")).toBe(true);
    }
  });
  it("tingkat 3 wajib folder backup di luar ~/MacPanel", () => {
    expect(planCommand(3)).toContain("--backup-dir ~/Backups-macpanel");
    expect(planCommand(3)).not.toContain("MacPanel/");
    expect(planCommand(1)).not.toContain("--backup-dir");
  });
  it("hanya tingkat 3 yang berbahaya", () => {
    expect(LEVELS.map((l) => l.danger)).toEqual([false, false, true]);
  });
});

describe("tailscale", () => {
  it("perintah rencana tidak memuat --apply dan perintah jalan memuatnya", () => {
    expect(TAILSCALE.installPlan).not.toContain("--apply");
    expect(TAILSCALE.removePlan).not.toContain("--apply");
    expect(TAILSCALE.installApply.endsWith(" --apply")).toBe(true);
    expect(TAILSCALE.removeApply.endsWith(" --apply")).toBe(true);
  });
  it("melepas selalu tingkat 4 dan tidak ada di LEVELS (tingkat 1-3 tak pernah menyentuhnya)", () => {
    expect(TAILSCALE.removePlan).toContain("--level 4");
    expect(LEVELS.map((l) => l.id)).toEqual([1, 2, 3]);
  });
  it("peringatan menyebut putusnya akses jarak jauh", () => {
    expect(TAILSCALE.removeWarn.join(" ")).toMatch(/jarak jauh/);
  });
});
