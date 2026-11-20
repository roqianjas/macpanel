// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { State } from "./ui";

describe("State: folder di luar wwwroot", () => {
  it("bukan galat: pesan jelas tanpa tombol coba lagi", () => {
    render(<State loading={false} error="folder proyek harus di bawah ~/MacPanel/wwwroot" onRetry={() => {}}>x</State>);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByRole("status").textContent).toContain("~/MacPanel/wwwroot");
    expect(screen.queryByText(/coba lagi/i)).toBeNull();
  });
  it("galat lain tetap tampil sebagai galat", () => {
    render(<State loading={false} error="koneksi putus" onRetry={() => {}}>x</State>);
    expect(screen.getByRole("alert").textContent).toContain("koneksi putus");
  });
});
