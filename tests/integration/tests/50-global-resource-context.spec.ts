import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base } from "@playwright/test";

import {
  createAuthenticatedStorageState,
  getMockMode,
  setMockMode,
} from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

let mockModeWasEnabled: boolean | null = null;

const escapeRegExp = (value: string): string =>
  value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

const test = base.extend<Record<string, never>, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => {
    await use(authStorageStatePath);
  },
  authStorageStatePath: [
    async ({ browser }, use, workerInfo) => {
      const storageStatePath = path.resolve(
        __dirname,
        "..",
        "..",
        "tmp",
        "playwright-auth",
        `global-resource-context-${workerInfo.project.name}.json`,
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

async function ensureMockModeEnabled(
  page: import("@playwright/test").Page,
): Promise<void> {
  const state = await getMockMode(page);
  if (mockModeWasEnabled === null) {
    mockModeWasEnabled = state.enabled;
  }
  if (!state.enabled) {
    await setMockMode(page, true);
  }
}

test.afterAll(async ({ browser }) => {
  if (mockModeWasEnabled === false) {
    const page = await browser.newPage();
    try {
      await setMockMode(page, false);
    } finally {
      await page.close();
    }
  }
  mockModeWasEnabled = null;
});

test("pins a resource context in infrastructure and carries it across platform views", async ({
  page,
}) => {
  await ensureMockModeEnabled(page);

  await page.goto("/infrastructure");
  await expect(page).toHaveURL(/\/infrastructure/);

  const pinButtons = page.getByRole("button", {
    name: /Set global context to/i,
  });
  await expect(pinButtons.first()).toBeAttached();

  const pinButton = pinButtons.first();
  await pinButton.scrollIntoViewIfNeeded();

  const pinButtonLabel = await pinButton.getAttribute("aria-label");
  expect(pinButtonLabel).toBeTruthy();
  const subjectLabel = pinButtonLabel?.replace(/^Set global context to\s+/i, "").trim() || "";
  expect(subjectLabel.length).toBeGreaterThan(0);

  const row = pinButton.locator("xpath=ancestor::tr[1]");
  const resourceId = await row.getAttribute("data-row-id");
  expect(resourceId).toBeTruthy();
  const escapedResourceId = escapeRegExp(resourceId || "");

  await pinButton.click();

  const contextBar = page.locator('[data-global-resource-context="true"]');
  await expect(contextBar).toContainText(subjectLabel);
  await expect(page).toHaveURL(new RegExp(`contextResource=${escapedResourceId}`));

  await page.getByRole("tab", { name: /Workloads/i }).click();
  await expect(page).toHaveURL(/\/workloads\?/);
  await expect(page).toHaveURL(new RegExp(`contextResource=${escapedResourceId}`));
  await expect(page).toHaveURL(/agent=/);
  await expect(contextBar).toContainText(subjectLabel);

  await page.getByRole("tab", { name: /^Storage$/ }).click();
  await expect(page).toHaveURL(/\/storage\?/);
  await expect(page).toHaveURL(new RegExp(`contextResource=${escapedResourceId}`));
  await expect(page).toHaveURL(new RegExp(`node=${escapedResourceId}`));
  await expect(page).toHaveURL(/source=proxmox-pve/);

  await page.getByRole("tab", { name: /^Recovery$/ }).click();
  await expect(page).toHaveURL(/\/recovery\?/);
  await expect(page).toHaveURL(new RegExp(`contextResource=${escapedResourceId}`));
  await expect(page).toHaveURL(new RegExp(`node=${escapedResourceId}`));
  await expect(page).toHaveURL(/platform=proxmox-pve/);

  await page.getByRole("button", { name: "Clear global context" }).click();
  await expect(contextBar).toHaveCount(0);
  await expect(page).toHaveURL(/\/recovery$/);
});
