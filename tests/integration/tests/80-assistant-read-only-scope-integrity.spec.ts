import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test as base, expect, type Route } from "@playwright/test";
import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
type WorkerFixtures = { authStorageStatePath: string };

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) =>
    use(authStorageStatePath),
  authStorageStatePath: [
    async ({ browser }, use, workerInfo) => {
      const storageStatePath = path.resolve(
        __dirname,
        "..",
        "..",
        "tmp",
        "playwright-auth",
        `assistant-read-only-${workerInfo.project.name}.json`,
      );
      fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
      await createAuthenticatedStorageState(browser, storageStatePath);
      try {
        await use(storageStatePath);
      } finally {
        fs.rmSync(storageStatePath, { force: true });
      }
    },
    { scope: "worker" },
  ],
});

test("Assistant makes the read-only promise literal in the current browser build", async ({
  page,
}) => {
  const fulfillSettings = async (route: Route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        model: "openrouter:deepseek/deepseek-chat",
        control_level: "read_only",
      }),
    });
  };
  await page.route("**/api/ai/settings", fulfillSettings);
  await page.route("**/api/settings/ai", fulfillSettings);
  await page.route("**/api/ai/status", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: '{"running":true}',
    }),
  );
  await page.route("**/api/ai/models", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        models: [
          { id: "openrouter:deepseek/deepseek-chat", name: "DeepSeek Chat" },
        ],
      }),
    }),
  );
  await page.route("**/api/ai/sessions", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "[]" }),
  );

  await page.goto("/truenas/overview", {
    waitUntil: "domcontentloaded",
  });
  await page
    .getByRole("button", { name: "Ask Pulse Assistant about TrueNAS" })
    .click();

  const mode = page.getByRole("button", {
    name: "Assistant chat action mode: Read-only",
  });
  await expect(mode).toBeVisible();
  await mode.click();
  await expect(
    page.getByRole("menuitemradio", { name: /Read-only/ }),
  ).toHaveAttribute("aria-checked", "true");
  await expect(page.getByText("Observes only")).toBeVisible();

  await page.getByRole("button", { name: "Collapse Pulse Assistant" }).click();
  await page.goto("/settings/security/api", { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "New token" }).click();
  await page.getByText("Custom scopes", { exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Pulse Assistant chat" }),
  ).toHaveAttribute(
    "title",
    "Use interactive Pulse Assistant sessions, models, and read knowledge.",
  );
  await expect(
    page.getByRole("button", { name: "Pulse Intelligence actions" }),
  ).toHaveAttribute("title", /verification, history, and knowledge changes/);
});
