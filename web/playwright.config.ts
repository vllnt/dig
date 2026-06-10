import { defineConfig, devices } from "@playwright/test";

// NTK_PREVIEW_URL is set by `ntk promote` (the e2e gate runs this suite against
// the live preview deployment — no local server in that mode). Falls back to
// BASE_URL, then the fixed local dev port.
const REMOTE_URL = process.env.NTK_PREVIEW_URL || process.env.BASE_URL;
const BASE_URL = REMOTE_URL || "http://localhost:3977";

export default defineConfig({
  forbidOnly: !!process.env.CI,
  fullyParallel: true,
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "mobile-chrome", use: { ...devices["Pixel 7"] } },
  ],
  reporter: process.env.CI ? "github" : "list",
  retries: process.env.CI ? 2 : 0,
  testDir: "./tests/e2e",
  use: {
    baseURL: BASE_URL,
    screenshot: "only-on-failure",
    trace: "on-first-retry",
  },
  // Boots the real app locally. Skipped when targeting a remote deployment
  // (NTK_PREVIEW_URL / BASE_URL), e.g. the ntk promote e2e gate.
  webServer: REMOTE_URL
    ? undefined
    : {
        command: "pnpm dev",
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
        url: BASE_URL,
      },
});
