import { expect, test } from "@playwright/test";

const RELEASES_URL = "https://github.com/vllnt/dig/releases";

test("install page shows every install method with zero console errors", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(m.text());
  });

  const response = await page.goto("/install");
  expect(response?.status(), "install page should not error").toBeLessThan(400);

  await expect(
    page.getByRole("heading", { level: 1, name: "Install dig" }),
  ).toBeVisible();
  await expect(
    page.getByText("curl -fsSL https://dig.vllnt.com/install.sh | sh"),
  ).toBeVisible();
  await expect(
    page.getByText("go install github.com/vllnt/dig/cmd/dig@latest"),
  ).toBeVisible();

  expect(errors, "no console errors on load").toEqual([]);
});

test("releases link points at GitHub releases in a new tab", async ({
  page,
}) => {
  await page.goto("/install");

  const cta = page.getByRole("link", { name: "View releases" });
  await expect(cta).toBeVisible();
  await expect(cta).toHaveAttribute("href", RELEASES_URL);
  await expect(cta).toHaveAttribute("target", "_blank");
});

test("a visitor reaches install from the homepage agent CTA", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("link", { name: "Add to your agent" }).first().click();

  await expect(page).toHaveURL(/\/install$/);
  await expect(
    page.getByRole("heading", { level: 1, name: "Install dig" }),
  ).toBeVisible();
  await expect(
    page.getByText("curl -fsSL https://dig.vllnt.com/install.sh | sh"),
  ).toBeVisible();
});

test("install is reachable from the sitemap", async ({ request }) => {
  const response = await request.get("/sitemap.xml");
  expect(response.status()).toBe(200);
  const body = await response.text();
  expect(body).toContain("/install");
  expect(body).toContain("/leaderboard");
});
