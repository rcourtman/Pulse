import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base } from "@playwright/test";
import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
type WorkerFixtures = { authStorageStatePath: string };
const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => use(authStorageStatePath),
  authStorageStatePath: [async ({ browser }, use, workerInfo) => {
    const storageStatePath = path.resolve(__dirname, "..", "..", "tmp", "playwright-auth", `product-trust-a11y-${workerInfo.project.name}.json`);
    fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
    await createAuthenticatedStorageState(browser, storageStatePath);
    try { await use(storageStatePath); } finally { fs.rmSync(storageStatePath, { force: true }); }
  }, { scope: "worker" }],
});

test("Actions remains named, keyboard reachable, and free of horizontal overflow at phone width", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.route("**/api/actions?*", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ view: "pending", actions: [], count: 0 }) }));
  await page.goto("/actions", { waitUntil: "domcontentloaded" });
  await expect(page.getByRole("heading", { name: "Actions", exact: true })).toBeVisible();
  await expect(page.getByRole("tablist", { name: "Action views" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Refresh actions" })).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
  expect(overflow).toBeFalsy();
  await page.keyboard.press("Tab");
  await expect(page.locator(":focus")).toBeVisible();
  await testInfo.attach("actions-phone-width", { body: await page.screenshot(), contentType: "image/png" });
});
