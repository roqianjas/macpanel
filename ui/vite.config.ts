import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  build: { outDir: "dist", emptyOutDir: true, sourcemap: false, target: "es2022" },
  test: { setupFiles: ["./src/test.setup.ts"] },
  server: { proxy: { "/api": "http://127.0.0.1:9100" } },
});
