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
      summary
        .locator("[data-highlight-series-id]")
        .evaluateAll(
          (nodes, expectedId) =>
            nodes.filter(
              (node) =>
                node.getAttribute("data-highlight-series-id") === expectedId &&
                node.getAttribute("data-highlight-series-active") === "true",
            ).length,
          resourceId,
        ),
    )
    .toBe(expectedCount);
}

async function readSummaryHighlightCount(
  summary: import("@playwright/test").Locator,
  resourceId: string,
): Promise<number> {
  return summary
    .locator("[data-highlight-series-id]")
    .evaluateAll(
      (nodes, expectedId) =>
        nodes.filter(
          (node) =>
            node.getAttribute("data-highlight-series-id") === expectedId &&
            node.getAttribute("data-highlight-series-active") === "true",
        ).length,
      resourceId,
    );
}

async function hoverRowUntilSummaryHighlights(
  page: import("@playwright/test").Page,
  rows: import("@playwright/test").Locator,
  summary: import("@playwright/test").Locator,
  fallbackAttribute: string,
  expectedCount: number,
): Promise<{ index: number; resourceId: string }> {
  const rowCount = await rows.count();
  for (let index = 0; index < rowCount; index += 1) {
    const row = rows.nth(index);
    if (!(await row.isVisible())) {
      continue;
    }
    const resourceId = await readSummarySeriesId(row, fallbackAttribute);
    if (!resourceId) {
      continue;
    }
    await row.hover();
    try {
      await expect
        .poll(() => readSummaryHighlightCount(summary, resourceId), {
          timeout: 1_500,
        })
        .toBe(expectedCount);
      return { index, resourceId };
    } catch {
      await page.mouse.move(1, 1);
    }
  }

  throw new Error("Unable to find a row that maps to summary highlight state");
}

async function dispatchRowHover(
  row: import("@playwright/test").Locator,
): Promise<void> {
  await row.dispatchEvent("mouseenter");
  await row.dispatchEvent("mouseover");
  await row.dispatchEvent("focusin");
}

async function hoverSummaryChartUntilActiveId(
  page: import("@playwright/test").Page,
  summary: import("@playwright/test").Locator,
  chartRoot: import("@playwright/test").Locator,
): Promise<string> {
  const surface = chartRoot.locator("svg, canvas").first();
  await expect(surface).toBeVisible();
  const box = await surface.boundingBox();
  expect(box).not.toBeNull();
  if (!box) {
    throw new Error("Chart surface has no bounding box");
  }

  const probePoints = [
    { x: 0.88, y: 0.25 },
    { x: 0.88, y: 0.5 },
    { x: 0.72, y: 0.25 },
    { x: 0.72, y: 0.5 },
    { x: 0.56, y: 0.25 },
    { x: 0.56, y: 0.5 },
  ];

  for (const point of probePoints) {
    await page.mouse.move(
      box.x + box.width * point.x,
      box.y + box.height * point.y,
    );
    const activeIds = await summary
      .locator("[data-highlight-series-id]")
      .evaluateAll((nodes) =>
        Array.from(
          new Set(
            nodes
              .filter(
                (node) =>
                  node.getAttribute("data-highlight-series-active") === "true",
              )
              .map(
                (node) => node.getAttribute("data-highlight-series-id") || "",
              )
              .filter(Boolean),
          ),
        ),
      );
    if (activeIds.length === 1) {
      return activeIds[0];
    }
  }

  throw new Error("Unable to activate a summary chart hover state");
}

async function expectActiveIsolatedLineCards(
  summary: import("@playwright/test").Locator,
  expectedCount: number,
): Promise<void> {
  await expect
    .poll(async () =>
      summary
        .locator('[data-active-series-display="isolate"]')
        .evaluateAll(
          (nodes) =>
            nodes.filter(
              (node) =>
                node.getAttribute("data-highlight-series-active") === "true" &&
                node.getAttribute("data-rendered-series-count") === "1",
            ).length,
        ),
    )
    .toBe(expectedCount);
}

async function expectActiveDensityMapsPreserveOverview(
  summary: import("@playwright/test").Locator,
  resourceId: string,
  expectedCount: number,
): Promise<void> {
  await expect
    .poll(async () =>
      summary
        .locator('[data-summary-chart-kind="density-map"]')
        .evaluateAll((nodes, expectedId) => {
          const activeNodes = nodes.filter(
            (node) =>
              node.getAttribute("data-highlight-series-id") === expectedId &&
              node.getAttribute("data-highlight-series-active") === "true",
          );
          return {
            activeCount: activeNodes.length,
            focusDetailCount: activeNodes.filter((node) => {
              const detail = node.querySelector(
                '[data-density-map-focus-detail="true"]',
              );
              return (
                detail?.getAttribute("data-density-map-focus-series-id") ===
                expectedId
              );
            }).length,
            preservedOverview: activeNodes.every(
              (node) =>
                Number(node.getAttribute("data-rendered-series-count") || "0") >
                1,
            ),
          };
        }, resourceId),
    )
    .toEqual({
      activeCount: expectedCount,
      focusDetailCount: expectedCount,
      preservedOverview: true,
    });
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
    const infrastructureRows = page.locator(
      '[data-testid="infrastructure-page"] tr[data-row-id]',
    );
    const firstInfrastructureRow = infrastructureRows.first();
    await expect(firstInfrastructureRow).toBeVisible();
    const { index: infrastructureRowIndex, resourceId: infrastructureRowId } =
      await hoverRowUntilSummaryHighlights(
        page,
        infrastructureRows,
        infrastructureSummary,
        "data-row-id",
        4,
      );
    const infrastructureRow = infrastructureRows.nth(infrastructureRowIndex);
    expect(infrastructureRowId).not.toBe("");
    await expectActiveIsolatedLineCards(infrastructureSummary, 2);
    await infrastructureRow.click();
    await infrastructureSummary.hover();
    await expectSummaryHighlightCount(
      infrastructureSummary,
      infrastructureRowId,
      4,
    );
    await expectActiveIsolatedLineCards(infrastructureSummary, 2);
    await expectActiveDensityMapsPreserveOverview(
      infrastructureSummary,
      infrastructureRowId,
      2,
    );

    await page.goto("/workloads", { waitUntil: "domcontentloaded" });
    const workloadsSummary = page.getByTestId("workloads-summary");
    await expect(workloadsSummary).toBeVisible();
    const workloadRows = page.locator("tr[data-guest-id]");
    const firstWorkloadRow = workloadRows.first();
    await expect(firstWorkloadRow).toBeVisible();
    const { resourceId: workloadRowId } = await hoverRowUntilSummaryHighlights(
      page,
      workloadRows,
      workloadsSummary,
      "data-guest-id",
      4,
    );
    expect(workloadRowId).not.toBe("");
    await expectActiveIsolatedLineCards(workloadsSummary, 2);
    await expectActiveDensityMapsPreserveOverview(
      workloadsSummary,
      workloadRowId,
      2,
    );

    const vmwareWorkloadRow = page
      .locator('tr[data-guest-id^="vm-"]', { hasText: "warehouse-api-01" })
      .first();
    await expect(vmwareWorkloadRow).toBeVisible();
    const vmwareWorkloadId = await readSummarySeriesId(
      vmwareWorkloadRow,
      "data-guest-id",
    );
    expect(vmwareWorkloadId).not.toBe("");
    await dispatchRowHover(vmwareWorkloadRow);
    await expectSummaryHighlightCount(workloadsSummary, vmwareWorkloadId, 4);
    await expectActiveIsolatedLineCards(workloadsSummary, 2);
    await expectActiveDensityMapsPreserveOverview(
      workloadsSummary,
      vmwareWorkloadId,
      2,
    );

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

    const storagePoolRows = page.locator("tr[data-row-id]");
    const firstStoragePoolRow = storagePoolRows.first();
    await expect(firstStoragePoolRow).toBeVisible();
    const { resourceId: storagePoolRowId } =
      await hoverRowUntilSummaryHighlights(
        page,
        storagePoolRows,
        storageSummary,
        "data-row-id",
        3,
      );
    expect(storagePoolRowId).not.toBe("");
    await expectActiveIsolatedLineCards(storageSummary, 3);

    await page.getByRole("tab", { name: "Physical Disks" }).click();
    const storageDiskRows = page.locator("tr[data-row-id]");
    const firstStorageDiskRow = storageDiskRows.first();
    await expect(firstStorageDiskRow).toBeVisible();
    const { resourceId: storageDiskRowId } =
      await hoverRowUntilSummaryHighlights(
        page,
        storageDiskRows,
        storageSummary,
        "data-row-id",
        1,
      );
    expect(storageDiskRowId).not.toBe("");
    await expectActiveIsolatedLineCards(storageSummary, 1);

    await scrollPrimaryViewportToBottom(page);
    await page.waitForTimeout(250);
    const stickyBox = await stickyStorageSummary.boundingBox();
    expect(stickyBox).not.toBeNull();
    expect(stickyBox?.y ?? Number.POSITIVE_INFINITY).toBeLessThanOrEqual(24);
  });

  test("synchronizes chart hover across summary cards on infrastructure, workloads, and storage", async ({
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
    const infrastructureChartId = await hoverSummaryChartUntilActiveId(
      page,
      infrastructureSummary,
      infrastructureSummary
        .locator('[data-active-series-display="isolate"]')
        .first(),
    );
    expect(infrastructureChartId).not.toBe("");
    await expectSummaryHighlightCount(
      infrastructureSummary,
      infrastructureChartId,
      4,
    );
    await expectActiveIsolatedLineCards(infrastructureSummary, 2);
    await expectActiveDensityMapsPreserveOverview(
      infrastructureSummary,
      infrastructureChartId,
      2,
    );

    await page.goto("/workloads", { waitUntil: "domcontentloaded" });
    const workloadsSummary = page.getByTestId("workloads-summary");
    await expect(workloadsSummary).toBeVisible();
    const workloadChartId = await hoverSummaryChartUntilActiveId(
      page,
      workloadsSummary,
      workloadsSummary
        .locator('[data-active-series-display="isolate"]')
        .first(),
    );
    expect(workloadChartId).not.toBe("");
    await expectSummaryHighlightCount(workloadsSummary, workloadChartId, 4);
    await expectActiveIsolatedLineCards(workloadsSummary, 2);
    await expectActiveDensityMapsPreserveOverview(
      workloadsSummary,
      workloadChartId,
      2,
    );

    await page.goto("/storage", { waitUntil: "domcontentloaded" });
    const storageSummary = page.getByTestId("storage-summary");
    await expect(storageSummary).toBeVisible();
    const storageChartId = await hoverSummaryChartUntilActiveId(
      page,
      storageSummary,
      storageSummary.locator('[data-active-series-display="isolate"]').first(),
    );
    expect(storageChartId).not.toBe("");
    await expectSummaryHighlightCount(storageSummary, storageChartId, 3);
    await expectActiveIsolatedLineCards(storageSummary, 3);
  });
});
