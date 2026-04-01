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
        `summary-hover-selection-${workerInfo.project.name}.json`,
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

async function expectSummaryHighlightCount(
  summary: import("@playwright/test").Locator,
  resourceId: string,
  expectedCount: number,
): Promise<void> {
  await expect
    .poll(async () =>
      summary.locator("[data-highlight-series-id]").evaluateAll((nodes, expectedId) =>
        nodes.filter(
          (node) =>
            node.getAttribute("data-highlight-series-id") === expectedId &&
            node.getAttribute("data-highlight-series-active") === "true",
        ).length,
      resourceId),
    )
    .toBe(expectedCount);
}

async function readSummarySeriesId(
  row: import("@playwright/test").Locator,
  fallbackAttribute: string,
): Promise<string> {
  return (
    (await row.getAttribute("data-summary-series-id")) ??
    (await row.getAttribute(fallbackAttribute)) ??
    ""
  );
}

async function scrollPrimaryViewportToBottom(
  page: import("@playwright/test").Page,
): Promise<void> {
  await page.evaluate(() => {
    const shell = document.querySelector<HTMLElement>(".app-scroll-shell");
    if (shell) {
      shell.scrollTo({ top: shell.scrollHeight });
      return;
    }
    window.scrollTo(0, document.body.scrollHeight);
  });
}

test.describe.serial("Summary hover selection", () => {
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

  test("keeps summary chart hover selection coherent across infrastructure, workloads, and storage", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop runtime proof",
    );

    await ensureMockModeEnabled(page);

    await page.goto("/infrastructure", { waitUntil: "domcontentloaded" });
    const infrastructureSummary = page.getByTestId("infrastructure-summary");
    await expect(infrastructureSummary).toBeVisible();
    const infrastructureRow = page
      .locator('[data-testid="infrastructure-page"] tr[data-row-id]')
      .first();
    await expect(infrastructureRow).toBeVisible();
    const infrastructureRowId =
      (await infrastructureRow.getAttribute("data-row-id")) ?? "";
    expect(infrastructureRowId).not.toBe("");
    await infrastructureRow.hover();
    await expectSummaryHighlightCount(
      infrastructureSummary,
      infrastructureRowId,
      4,
    );

    await page.goto("/workloads", { waitUntil: "domcontentloaded" });
    const workloadsSummary = page.getByTestId("workloads-summary");
    await expect(workloadsSummary).toBeVisible();
    const workloadRow = page.locator("tr[data-guest-id]").first();
    await expect(workloadRow).toBeVisible();
    const workloadRowId =
      (await workloadRow.getAttribute("data-guest-id")) ?? "";
    expect(workloadRowId).not.toBe("");
    await workloadRow.hover();
    await expectSummaryHighlightCount(workloadsSummary, workloadRowId, 4);

    await page.goto("/storage", { waitUntil: "domcontentloaded" });
    const storageSummary = page.getByTestId("storage-summary");
    await expect(storageSummary).toBeVisible();
    const stickyStorageSummary = storageSummary.locator(
      'xpath=ancestor::*[@data-sticky-summary="true"][1]',
    );
    await expect(stickyStorageSummary).toHaveAttribute(
      "data-sticky-summary-desktop-only",
      "false",
    );

    const storagePoolRow = page.locator("tr[data-row-id]").first();
    await expect(storagePoolRow).toBeVisible();
    const storagePoolRowId = await readSummarySeriesId(
      storagePoolRow,
      "data-row-id",
    );
    expect(storagePoolRowId).not.toBe("");
    await storagePoolRow.hover();
    await expectSummaryHighlightCount(storageSummary, storagePoolRowId, 3);

    await page.getByRole("tab", { name: "Physical Disks" }).click();
    const storageDiskRow = page.locator("tr[data-row-id]").first();
    await expect(storageDiskRow).toBeVisible();
    const storageDiskRowId = await readSummarySeriesId(
      storageDiskRow,
      "data-row-id",
    );
    expect(storageDiskRowId).not.toBe("");
    await storageDiskRow.hover();
    await expectSummaryHighlightCount(storageSummary, storageDiskRowId, 1);

    await scrollPrimaryViewportToBottom(page);
    await page.waitForTimeout(250);
    const stickyBox = await stickyStorageSummary.boundingBox();
    expect(stickyBox).not.toBeNull();
    expect(stickyBox?.y ?? Number.POSITIVE_INFINITY).toBeLessThanOrEqual(24);
  });
});
