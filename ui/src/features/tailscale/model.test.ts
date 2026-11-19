import { describe, expect, it } from "vitest";
import { ACTIONS, applicable, phase, type TsInfo } from "./model";

const info = (o: Partial<TsInfo> & { state?: string }): TsInfo => ({ formula: true, daemon_plist: true, error: null, tailnet: o.state ? { state: o.state, self_online: true, peers: 1, peers_online: 1, health: [], ip: null, name: null, version: null } : null, ...o });

describe("tailscale", () => {
  it("fase dari status", () => {
    expect(phase(info({ formula: false }))).toBe("missing");
    expect(phase(info({ state: "Running" }))).toBe("running");
    expect(phase(info({ state: "Stopped" }))).toBe("stopped");
    expect(phase(info({ state: "NeedsLogin" }))).toBe("login");
    expect(phase(info({}))).toBe("unknown");
  });
  it("tindakan yang masuk akal per fase", () => {
    const ok = (id: string, p: Parameters<typeof applicable>[1]) => applicable(ACTIONS.find((a) => a.id === id)!, p).ok;
    expect(ok("install", "missing")).toBe(true);
    expect(ok("install", "running")).toBe(false);
    expect(ok("stop", "missing")).toBe(false);
    expect(ok("stop", "running")).toBe(true);
    expect(ok("start", "running")).toBe(false);
    expect(ok("start", "stopped")).toBe(true);
    expect(ok("down", "stopped")).toBe(false);
  });
  it("semua yang memutus akses bertanda danger, tidak ada perintah yang menjalankan tailscale up otomatis lewat lab", () => {
    for (const id of ["down", "stop", "uninstall"]) expect(ACTIONS.find((a) => a.id === id)!.danger).toBe(true);
    for (const id of ["install", "login", "start", "restart"]) expect(ACTIONS.find((a) => a.id === id)!.danger).toBe(false);
    expect(ACTIONS.filter((a) => a.run.startsWith("lab ")).every((a) => !/\bup\b/.test(a.run))).toBe(true);
    expect(ACTIONS.filter((a) => a.id !== "login").every((a) => a.run.includes("--apply") && !!a.plan && !a.plan.includes("--apply"))).toBe(true);
  });
});
