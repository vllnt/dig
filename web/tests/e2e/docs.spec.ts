import { expect, test } from "@playwright/test";

test("docs page shows quickstart, commands, and policy with zero console errors", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(m.text());
  });

  const response = await page.goto("/docs");
  expect(response?.status(), "docs page should not error").toBeLessThan(400);

  await expect(
    page.getByRole("heading", { exact: true, level: 1, name: "Docs" }),
  ).toBeVisible();
  await expect(page.getByRole("heading", { name: "Quickstart" })).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Command reference" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Policy reference" }),
  ).toBeVisible();

  await expect(page.getByText("dig reconcile").first()).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "[retrieval]" }),
  ).toBeVisible();

  expect(errors, "no console errors on load").toEqual([]);
});

test("a visitor reaches docs from the nav and reads the command reference", async ({
  isMobile,
  page,
}) => {
  test.skip(isMobile, "the docs nav link is hidden on mobile");

  await page.goto("/");
  await page.getByRole("link", { name: "Docs" }).click();

  await expect(page).toHaveURL(/\/docs$/);
  await expect(
    page.getByRole("cell", { name: "dig find <query>" }),
  ).toBeVisible();
});

test("docs Integrate section opens the Vercel AI SDK guide", async ({
  page,
}) => {
  await page.goto("/docs");
  await expect(page.getByRole("heading", { name: "Integrate" })).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Vercel AI SDK" }),
  ).toBeVisible();

  await page.getByRole("link", { name: /AI SDK guide/ }).click();
  await expect(page).toHaveURL(/\/learn\/vercel-ai-sdk$/);
  await expect(
    page.getByRole("heading", { level: 1, name: /Vercel AI SDK/ }),
  ).toBeVisible();
  await expect(page.getByText("digTools(dig)").first()).toBeVisible();
});
