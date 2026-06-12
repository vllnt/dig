import { expect, test } from "@playwright/test";

test("leaderboard loads with the headline result and zero console errors", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(m.text());
  });

  const response = await page.goto("/leaderboard");
  expect(response?.status(), "leaderboard should not error").toBeLessThan(400);

  await expect(
    page.getByRole("heading", { level: 1, name: /beats the published bar/i }),
  ).toBeVisible();
  await expect(page.getByText("98.0%").first()).toBeVisible();
  await expect(
    page.getByText(/MemPalace's published 96.6%/).first(),
  ).toBeVisible();

  expect(errors, "no console errors on load").toEqual([]);
});

test("every published benchmark and its winning score is shown", async ({
  page,
}) => {
  await page.goto("/leaderboard");

  for (const name of ["LongMemEval-S", "LoCoMo", "BEAM (128K tier)"]) {
    await expect(
      page.getByRole("heading", { name, exact: true }),
    ).toBeVisible();
  }

  await expect(
    page.getByText("Published numbers from other systems"),
  ).toBeVisible();
  await expect(page.getByRole("heading", { name: "Method" })).toBeVisible();
});

test("a visitor reaches the leaderboard from the landing nav", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("link", { name: "Benchmarks" }).click();

  await expect(page).toHaveURL(/\/leaderboard$/);
  await expect(
    page.getByRole("heading", { level: 1, name: /beats the published bar/i }),
  ).toBeVisible();
});
