import { expect, test } from "@playwright/test";

// The cluster hub index pages (/compare, /learn, /use-cases) make every content
// page browsable — a card grid linking into the cluster, reachable from the nav.
const CLUSTERS = ["compare", "learn", "use-cases"];

CLUSTERS.forEach((cluster) => {
  test(`hub /${cluster} renders and links into the cluster`, async ({
    page,
  }) => {
    const errors: string[] = [];
    page.on("console", (m) => {
      if (m.type() === "error") errors.push(m.text());
    });

    const response = await page.goto(`/${cluster}`);
    expect(response?.status(), "hub loads").toBeLessThan(400);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

    const card = page.locator(`a[href*="/${cluster}/"]`).first();
    await expect(card).toBeVisible();
    await expect(card).toHaveAttribute(
      "href",
      new RegExp(`/${cluster}/[a-z0-9-]+$`),
    );

    expect(errors, "no console errors").toEqual([]);
  });
});

test("the footer Resources column reaches a cluster hub", async ({ page }) => {
  await page.goto("/");
  await page
    .getByRole("contentinfo")
    .getByRole("link", { name: "Compare" })
    .click();
  await expect(page).toHaveURL(/\/compare$/);
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
});

test("cluster hubs are listed in the sitemap", async ({ request }) => {
  const response = await request.get("/sitemap.xml");
  expect(response.status()).toBe(200);
  const sitemap = await response.text();
  CLUSTERS.forEach((cluster) => {
    expect(sitemap, `sitemap lists the /${cluster} hub`).toMatch(
      new RegExp(`/${cluster}</loc>`),
    );
  });
});
