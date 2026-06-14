import { expect, test } from "@playwright/test";

const GITHUB_URL = "https://github.com/vllnt/dig";

test("landing page loads with zero console errors and the hero sells dig", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(m.text());
  });

  const response = await page.goto("/");
  expect(response?.status(), "landing should not error").toBeLessThan(400);

  await expect(
    page.getByRole("heading", {
      level: 1,
      name: /memory for your coding agent/i,
    }),
  ).toBeVisible();
  await expect(page.getByText("Open source · MIT · local-first")).toBeVisible();

  expect(errors, "no console errors on load").toEqual([]);
});

test("primary CTA targets the GitHub repo in a new tab", async ({ page }) => {
  await page.goto("/");

  const cta = page.getByRole("link", { name: /Star on GitHub/ }).first();
  await expect(cta).toBeVisible();
  await expect(cta).toHaveAttribute("href", GITHUB_URL);
  await expect(cta).toHaveAttribute("target", "_blank");
});

test("user can reach Integrations via header nav", async ({
  isMobile,
  page,
}) => {
  test.skip(isMobile, "secondary nav links are hidden on mobile");

  await page.goto("/");
  await page.getByRole("link", { name: "Integrations" }).click();

  await expect(page).toHaveURL(/\/integrations$/);
  await expect(
    page.getByRole("heading", { level: 1, name: "Drive dig from your agent" }),
  ).toBeVisible();
});

test("cookie consent can be declined and stays decided after reload", async ({
  page,
}) => {
  await page.goto("/");

  const decline = page.getByRole("button", { name: "Decline" });
  await expect(decline).toBeVisible();
  await decline.click();
  await expect(decline).toBeHidden();

  await page.reload();
  await expect(page.getByRole("button", { name: "Decline" })).toHaveCount(0);
});

test("robots.txt allows crawling and points at the sitemap", async ({
  request,
}) => {
  const response = await request.get("/robots.txt");
  expect(response.status()).toBe(200);
  expect(response.headers()["content-type"]).toContain("text/plain");

  const body = await response.text();
  expect(body).toContain("User-agent: *");
  expect(body).toContain("Allow: /");
  expect(body).toContain("Sitemap: ");
  expect(body).toContain("/sitemap.xml");
});

test("sitemap.xml is a valid urlset listing the landing page with hreflang", async ({
  request,
}) => {
  const response = await request.get("/sitemap.xml");
  expect(response.status()).toBe(200);

  const body = await response.text();
  expect(body).toContain("<urlset");
  expect(body).toContain("<loc>");
  expect(body).toContain('hreflang="en"');
});

test("unknown locale path returns 404, not a broken page", async ({ page }) => {
  const response = await page.goto("/zz");
  expect(response?.status()).toBe(404);
});
