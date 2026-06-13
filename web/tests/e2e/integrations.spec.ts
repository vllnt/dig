import { expect, test } from "@playwright/test";

test("the homepage selector emits the exact install for the chosen agent", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(m.text());
  });

  await page.goto("/");
  const section = page.locator("#integrations");
  await section.scrollIntoViewIfNeeded();

  // Default is Claude Code — the plugin one-liner appears before any click.
  await expect(
    section.getByText("/plugin marketplace add vllnt/dig"),
  ).toBeVisible();

  // Pick "Any MCP client" → the MCP server config replaces the snippet.
  await section.getByRole("combobox", { name: "Select your agent" }).click();
  await page.getByRole("option", { name: "Any MCP client" }).click();
  await expect(section.getByText(/mcpServers/)).toBeVisible();

  expect(errors, "no console errors").toEqual([]);
});

test("a visitor browses the integrations hub and opens an agent guide", async ({
  page,
}) => {
  await page.goto("/integrations");
  await expect(
    page.getByRole("heading", { level: 1, name: "Drive dig from your agent" }),
  ).toBeVisible();

  await page.getByRole("link", { name: /Claude Code/ }).click();
  await expect(page).toHaveURL(/\/integrations\/claude-code$/);
  await expect(page.getByText("/plugin install dig@dig")).toBeVisible();
  await expect(page.getByText("dig_recall")).toBeVisible();
});

test("a WIP harness page shows the MCP fallback and a WIP badge", async ({
  page,
}) => {
  const response = await page.goto("/integrations/opencode");
  expect(response?.status()).toBeLessThan(400);

  await expect(
    page.getByRole("heading", { level: 1, name: /dig \+ opencode/i }),
  ).toBeVisible();
  await expect(page.getByText("WIP").first()).toBeVisible();
  await expect(page.getByText(/mcpServers/).first()).toBeVisible();
});

test("integration pages are crawlable via the sitemap", async ({ request }) => {
  const response = await request.get("/sitemap.xml");
  expect(response.status()).toBe(200);
  const body = await response.text();
  expect(body).toContain("/integrations");
  expect(body).toContain("/integrations/claude-code");
});

test("the install page lets you pick an agent and copy its install", async ({
  page,
}) => {
  await page.goto("/install");
  await expect(
    page.getByRole("heading", { name: "Drive dig from your agent" }),
  ).toBeVisible();
  await expect(
    page.getByText("/plugin marketplace add vllnt/dig"),
  ).toBeVisible();
});
