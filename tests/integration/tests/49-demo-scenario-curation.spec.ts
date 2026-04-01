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

const test = base.extend<{}, WorkerFixtures>({
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
        `demo-scenario-curation-${workerInfo.project.name}.json`,
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

async function waitForAppRoute(
  page: import("@playwright/test").Page,
): Promise<void> {
  await page.waitForLoadState("domcontentloaded");
  await page.waitForFunction(
    () => {
      const root = document.getElementById("root");
      return root !== null && root.children.length > 0;
    },
    undefined,
    { timeout: 20_000 },
  );
}

async function tableRowsText(
  page: import("@playwright/test").Page,
): Promise<string[]> {
  return page
    .locator("table tbody tr")
    .evaluateAll((rows) =>
      rows.map((row) => row.innerText.replace(/\s+/g, " ").trim()),
    );
}

async function expectSomeRowContains(
  page: import("@playwright/test").Page,
  text: string,
): Promise<void> {
  await expect
    .poll(async () => {
      const rows = await tableRowsText(page);
      return rows.some((row) => row.includes(text));
    })
    .toBe(true);
}

async function expectNoRowContains(
  page: import("@playwright/test").Page,
  text: string,
): Promise<void> {
  await expect
    .poll(async () => {
      const rows = await tableRowsText(page);
      return rows.some((row) => row.includes(text));
    })
    .toBe(false);
}

test.describe.serial("Demo scenario curation", () => {
  test.setTimeout(180_000);

  test.afterAll(async ({ browser }) => {
    if (mockModeWasEnabled === null) return;

    const context = await browser.newContext();
    const page = await context.newPage();
    try {
      const current = await getMockMode(page);
      if (current.enabled !== mockModeWasEnabled) {
        await setMockMode(page, mockModeWasEnabled);
      }
    } finally {
      await context.close();
    }
  });

  test("renders the curated demo estate across infrastructure, workloads, storage, and recovery", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop runtime proof",
    );

    await ensureMockModeEnabled(page);

    await page.goto("/infrastructure", { waitUntil: "domcontentloaded" });
    await waitForAppRoute(page);
    await expect(page.getByTestId("infrastructure-summary")).toBeVisible();
    await expectSomeRowContains(page, "West Production A");
    await expectSomeRowContains(page, "Prod Euw1 K8s 01");
    await expectNoRowContains(page, "mock-cluster");

    await page.goto("/workloads", { waitUntil: "domcontentloaded" });
    await waitForAppRoute(page);
    await expect(page.getByTestId("workloads-summary")).toBeVisible();
    await expectSomeRowContains(page, "customer-portal");
    await expectSomeRowContains(page, "backup-coordinator");
    await expectSomeRowContains(page, "checkout-web-01");

    await page.goto("/storage", { waitUntil: "domcontentloaded" });
    await waitForAppRoute(page);
    await expect(page.getByTestId("storage-summary")).toBeVisible();
    await expectSomeRowContains(page, "shared-backup-fabric");
    await expectSomeRowContains(page, "west-a-service-pool");
    await expectNoRowContains(page, "service-pool");

    await page.goto("/recovery", { waitUntil: "domcontentloaded" });
    await waitForAppRoute(page);
    await expectSomeRowContains(page, "checkout-web-01");
    await expectSomeRowContains(page, "production/monitoring/prometheus-pvc");
  });
});
