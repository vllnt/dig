import { expect, test } from "@playwright/test";

// Regression guard for the standalone-Docker `public/` omission: Next.js
// `output: 'standalone'` does not bundle public/, so the runtime image must
// copy it in (web/Dockerfile) or these files 404 in production.
//
// NOTE: `next dev` (the local webServer) serves public/ regardless, so this
// suite only FAILS against the bug when pointed at the real image — i.e. the
// `ntk promote` e2e gate (NTK_PREVIEW_URL) or BASE_URL set to the deployment.

test("llms.txt is served from the site root for AI discovery", async ({
  request,
}) => {
  const response = await request.get("/llms.txt");
  expect(response.status()).toBe(200);
  expect(response.headers()["content-type"]).toContain("text/plain");

  const body = await response.text();
  expect(body).toContain("# dig");
  expect(body).toContain("## Docs");
});

test("llms-full.txt is served from the site root for AI discovery", async ({
  request,
}) => {
  const response = await request.get("/llms-full.txt");
  expect(response.status()).toBe(200);
  expect(response.headers()["content-type"]).toContain("text/plain");

  const body = await response.text();
  expect(body).toContain("# dig — full reference for LLMs");
  expect(body).toContain("## 4. CLI surface");
});
