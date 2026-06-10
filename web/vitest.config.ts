import path from "node:path";
import { fileURLToPath } from "node:url";

import { defineConfig } from "vitest/config";

const rootDir = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  resolve: {
    alias: {
      "@": rootDir,
    },
  },
  test: {
    coverage: {
      exclude: ["**/*.d.ts", "**/*.config.*", "tests/**"],
      include: ["lib/**"],
      provider: "v8",
      reporter: ["text", "html", "lcov"],
      thresholds: {
        branches: 90,
        functions: 90,
        lines: 90,
        statements: 90,
      },
    },
    environment: "node",
    exclude: ["tests/e2e/**", "node_modules/**", "dist/**", "build/**"],
    include: ["tests/unit/**/*.{test,spec}.{ts,tsx,js,jsx}"],
  },
});
