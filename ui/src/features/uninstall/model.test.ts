import { describe, expect, it } from "vitest";
import { applyCommand, LEVELS, planCommand } from "./model";

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
