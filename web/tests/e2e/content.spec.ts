import { expect, test } from "@playwright/test";

// One representative page per SEO content cluster (compare / learn / use-cases).
// Each must render its frontmatter title as the <h1>, render MDX body prose, and
// raise no console errors — proving the [cluster]/[slug] route serves the content.
const PAGES = [
  {
    body: /reversible/i,
    heading: /dig vs zep/i,
    url: "/compare/zep",
  },
  {
    body: /hit@5/i,
    heading: /recall@k and hit@k/i,
    url: "/learn/recall-at-k",
  },
  {
    body: /recall/i,
    heading: /local second brain/i,
    url: "/use-cases/second-brain",
  },
];

PAGES.forEach(({ body, heading, url }) => {
  test(`content page ${url} renders its title and body`, async ({ page }) => {
    const errors: string[] = [];
    page.on("console", (m) => {
      if (m.type() === "error") errors.push(m.text());
    });

    const response = await page.goto(url);
    expect(response?.status(), "page loads").toBeLessThan(400);

    await expect(
      page.getByRole("heading", { level: 1, name: heading }),
    ).toBeVisible();
    await expect(page.getByText(body).first()).toBeVisible();

    expect(errors, "no console errors").toEqual([]);
  });
});

test("new content pages are crawlable via the sitemap", async ({ request }) => {
  const response = await request.get("/sitemap.xml");
  expect(response.status()).toBe(200);
  const sitemap = await response.text();
  PAGES.forEach(({ url }) => {
    expect(sitemap, `sitemap lists ${url}`).toContain(url);
  });
});
