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
      preservedOverview: true,
    });
}

async function expectSummarySyncedReadoutCount(
  summary: import("@playwright/test").Locator,
  expectedCount: number,
): Promise<void> {
  await expect
    .poll(async () => summary.locator('[data-summary-sync-readout="true"]').count())
    .toBe(expectedCount);
}

async function expectOnlyOneSummaryHoverTooltip(
  page: import("@playwright/test").Page,
): Promise<void> {
  await expect
    .poll(async () =>
      page
        .locator('[data-sparkline-tooltip="true"], [data-density-map-tooltip="true"]')
        .count(),
    )
    .toBe(1);
}

async function hoverDensityMapUntilTooltipAppears(
  page: import("@playwright/test").Page,
  densityMap: import("@playwright/test").Locator,
): Promise<void> {
  const canvas = densityMap.locator("canvas").first();
  await expect(canvas).toBeVisible();
  const box = await canvas.boundingBox();
  expect(box).not.toBeNull();
  if (!box) {
    throw new Error("Density map canvas has no bounding box");
  }

  for (const point of [
    { x: 0.84, y: 0.2 },
    { x: 0.7, y: 0.35 },
    { x: 0.56, y: 0.5 },
  ]) {
    await page.mouse.move(
      box.x + box.width * point.x,
      box.y + box.height * point.y,
    );
    const tooltip = page.locator('[data-density-map-tooltip="true"]').last();
    if (await tooltip.isVisible().catch(() => false)) {
      return;
    }
  }

  throw new Error("Unable to activate a density-map tooltip");
}

async function expectSummaryHoverTimestampsAligned(
  summary: import("@playwright/test").Locator,
  expectedCount: number,
): Promise<void> {
  await expect
    .poll(async () =>
      summary.locator("[data-active-hover-timestamp]").evaluateAll((nodes) => {
        const timestamps = nodes
          .map((node) => node.getAttribute("data-active-hover-timestamp") || "")
          .filter(Boolean);
        return {
          count: timestamps.length,
          uniqueCount: new Set(timestamps).size,
        };
      }),
    )
    .toEqual({
      count: expectedCount,
      uniqueCount: 1,
    });
}

async function expectSparklineSourceCursorTracksLocalScrub(
  page: import("@playwright/test").Page,
  sparklineRoot: import("@playwright/test").Locator,
): Promise<void> {
  const surface = sparklineRoot.locator("svg").first();
  await expect(surface).toBeVisible();
  const box = await surface.boundingBox();
  expect(box).not.toBeNull();
  if (!box) {
    throw new Error("Sparkline surface has no bounding box");
  }

  const samples: Array<{ cursorX: number; lineX: number }> = [];
  for (const ratio of [0.15, 0.45, 0.75]) {
    await page.mouse.move(
      box.x + box.width * ratio,
      box.y + box.height * 0.45,
    );
    await expect
      .poll(async () =>
        sparklineRoot.evaluate((node) => {
          const activeCursor = Number.parseFloat(
            node.getAttribute("data-active-hover-cursor-x") || "NaN",
          );
          const svgCursor = Number.parseFloat(
            node
              .querySelector('line[stroke-dasharray="3 3"]')
              ?.getAttribute("x1") || "NaN",
          );
          return Number.isFinite(activeCursor) && Number.isFinite(svgCursor);
        }),
      )
      .toBe(true);

    const sample = await sparklineRoot.evaluate((node) => ({
      cursorX: Number.parseFloat(
        node.getAttribute("data-active-hover-cursor-x") || "NaN",
      ),
      lineX: Number.parseFloat(
        node.querySelector('line[stroke-dasharray="3 3"]')?.getAttribute("x1") ||
          "NaN",
      ),
      lineY1: Number.parseFloat(
        node.querySelector('line[stroke-dasharray="3 3"]')?.getAttribute("y1") ||
          "NaN",
      ),
      lineY2: Number.parseFloat(
        node.querySelector('line[stroke-dasharray="3 3"]')?.getAttribute("y2") ||
          "NaN",
      ),
    }));
    expect(sample.cursorX).toBeCloseTo(sample.lineX, 1);
    expect(sample.lineY1).toBe(0);
    expect(sample.lineY2).toBe(100);
    samples.push(sample);
  }

  expect(samples[0].lineX).toBeLessThan(samples[1].lineX);
  expect(samples[1].lineX).toBeLessThan(samples[2].lineX);
  await page.mouse.move(1, 1);
}

async function expectSparklineTooltipTracksPointer(
  page: import("@playwright/test").Page,
  sparklineRoot: import("@playwright/test").Locator,
): Promise<void> {
  const surface = sparklineRoot.locator("svg").first();
  await expect(surface).toBeVisible();
  const box = await surface.boundingBox();
  expect(box).not.toBeNull();
  if (!box) {
    throw new Error("Sparkline surface has no bounding box");
  }

  const tooltip = page.locator('[data-sparkline-tooltip="true"]').last();

  await page.mouse.move(box.x + box.width * 0.45, box.y + box.height * 0.2);
  await expect(tooltip).toBeVisible();
  const firstTop = await tooltip.evaluate((node) =>
    Number.parseFloat((node as HTMLElement).style.top || "NaN"),
  );

  await page.mouse.move(box.x + box.width * 0.45, box.y + box.height * 0.75);
  await expect(tooltip).toBeVisible();
  const secondTop = await tooltip.evaluate((node) =>
    Number.parseFloat((node as HTMLElement).style.top || "NaN"),
  );

  expect(secondTop).toBeGreaterThan(firstTop + 40);
  await page.mouse.move(1, 1);
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

async function readRenderedSeriesCounts(
  summary: import("@playwright/test").Locator,
): Promise<number[]> {
  return summary.locator("[data-rendered-series-count]").evaluateAll((nodes) =>
    nodes.map((node) =>
      Number.parseInt(node.getAttribute("data-rendered-series-count") || "0", 10),
    ),
  );
}

async function readSummaryCardHeights(
  summary: import("@playwright/test").Locator,
): Promise<number[]> {
  return summary.locator("[data-summary-card-state]").evaluateAll((nodes) =>
    nodes.map((node) => node.getBoundingClientRect().height),
  );
}

async function expectSummaryCardHeightsStableAcrossRowHover(
  page: import("@playwright/test").Page,
  summary: import("@playwright/test").Locator,
  rows: import("@playwright/test").Locator,
  sampleCount = 8,
  expectedCardCount = 4,
): Promise<void> {
  await expect
    .poll(async () => {
      const heights = await readSummaryCardHeights(summary);
      return (
        heights.length === expectedCardCount &&
        heights.every((height) => Number.isFinite(height) && height >= 180)
      );
    })
    .toBe(true);

  const baselineHeights = await readSummaryCardHeights(summary);
  expect(baselineHeights.length).toBeGreaterThan(0);

  const rowCount = await rows.count();
  let exercisedRows = 0;
  for (let index = 0; index < rowCount && exercisedRows < sampleCount; index += 1) {
    const row = rows.nth(index);
    if (!(await row.isVisible())) {
      continue;
    }
    await row.hover();
    await page.waitForTimeout(120);
    const currentHeights = await readSummaryCardHeights(summary);
    expect(currentHeights).toHaveLength(baselineHeights.length);
    currentHeights.forEach((height, cardIndex) => {
      expect(height).toBeCloseTo(baselineHeights[cardIndex], 0);
    });
    exercisedRows += 1;
  }

  expect(exercisedRows).toBeGreaterThan(0);
  await page.mouse.move(1, 1);
  const settledHeights = await readSummaryCardHeights(summary);
  settledHeights.forEach((height, cardIndex) => {
    expect(height).toBeCloseTo(baselineHeights[cardIndex], 0);
  });
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
    const infrastructureSparklines = infrastructureSummary.locator(
      "[data-active-series-display]",
    );
    await expectSparklineSourceCursorTracksLocalScrub(
      page,
      infrastructureSparklines.nth(0),
    );
    await expectSparklineTooltipTracksPointer(
      page,
      infrastructureSparklines.nth(0),
    );
    await expectSparklineSourceCursorTracksLocalScrub(
      page,
      infrastructureSparklines.nth(1),
    );
    await expectSparklineTooltipTracksPointer(
      page,
      infrastructureSparklines.nth(1),
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
    const workloadSparklines = workloadsSummary.locator(
      "[data-active-series-display]",
    );
    await expectSparklineSourceCursorTracksLocalScrub(
      page,
      workloadSparklines.nth(0),
    );
    await expectSparklineTooltipTracksPointer(
      page,
      workloadSparklines.nth(0),
    );
    await expectSparklineSourceCursorTracksLocalScrub(
      page,
      workloadSparklines.nth(1),
    );
    await expectSparklineTooltipTracksPointer(
      page,
      workloadSparklines.nth(1),
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
    expect(stickyBox?.y ?? Number.POSITIVE_INFINITY).toBeLessThanOrEqual(96);
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
    await expectSummaryHoverTimestampsAligned(infrastructureSummary, 4);
    await expectActiveIsolatedLineCards(infrastructureSummary, 2);
    await expectActiveDensityMapsPreserveOverview(
      infrastructureSummary,
      infrastructureChartId,
      2,
    );
    await expectSummarySyncedReadoutCount(infrastructureSummary, 3);
    await expectOnlyOneSummaryHoverTooltip(page);

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
    await expectSummaryHoverTimestampsAligned(workloadsSummary, 4);
    await expectActiveIsolatedLineCards(workloadsSummary, 2);
    await expectActiveDensityMapsPreserveOverview(
      workloadsSummary,
      workloadChartId,
      2,
    );
    await expectSummarySyncedReadoutCount(workloadsSummary, 3);
    await expectOnlyOneSummaryHoverTooltip(page);

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
    await expectSummaryHoverTimestampsAligned(storageSummary, 4);
    await expectActiveIsolatedLineCards(storageSummary, 3);
    await expectSummarySyncedReadoutCount(storageSummary, 2);
    await expectOnlyOneSummaryHoverTooltip(page);
  });

  test("keeps summary card geometry stable while row hover changes active series", async ({
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
    await expectSummaryCardHeightsStableAcrossRowHover(
      page,
      infrastructureSummary,
      page.locator("tr[data-summary-series-id]"),
    );

    await page.goto("/workloads", { waitUntil: "domcontentloaded" });
    const workloadsSummary = page.getByTestId("workloads-summary");
    await expect(workloadsSummary).toBeVisible();
    await expectSummaryCardHeightsStableAcrossRowHover(
      page,
      workloadsSummary,
      page.locator("tr[data-guest-id]"),
    );
  });

  test("keeps table coordination non-destructive and reversible", async ({
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
    const highlightedInfrastructureRow = page.locator(
      `tr[data-summary-series-id="${infrastructureChartId}"]`,
    );
    const infrastructureJumpButton = page.getByRole("button", {
      name: "Jump to row",
    });
    if (await highlightedInfrastructureRow.first().isVisible()) {
      await expect(highlightedInfrastructureRow.first()).toHaveAttribute(
        "data-summary-row-active",
        "true",
      );
    } else {
      await expect(infrastructureJumpButton).toBeVisible();
      await infrastructureJumpButton.click();
      await expect(highlightedInfrastructureRow.first()).toBeVisible();
    }

    await page.goto("/workloads", { waitUntil: "domcontentloaded" });
    const workloadsSummary = page.getByTestId("workloads-summary");
    await expect(workloadsSummary).toBeVisible();
    const focusedWorkloadRow = page.locator("tr[data-guest-id]").first();
    const focusedWorkloadId = await readSummarySeriesId(
      focusedWorkloadRow,
      "data-guest-id",
    );
    expect(focusedWorkloadId).not.toBe("");
    await focusedWorkloadRow.click();
    await scrollPrimaryViewportToBottom(page);
    const jumpButton = page.getByRole("button", { name: "Jump to row" });
    const focusedSummaryRow = page
      .locator(`tr[data-summary-series-id="${focusedWorkloadId}"]`)
      .first();
    if (await jumpButton.isVisible()) {
      await jumpButton.click();
      await expect(focusedSummaryRow).toBeVisible();
    } else {
      await expect(focusedSummaryRow).toBeVisible();
    }
  });

  test("treats workload group headers as first-class summary scope", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop runtime proof",
    );

    await ensureMockModeEnabled(page);

    await page.goto("/workloads", { waitUntil: "domcontentloaded" });
    const workloadsSummary = page.getByTestId("workloads-summary");
    await expect(workloadsSummary).toBeVisible();

    await expect
      .poll(async () => {
        const counts = await readRenderedSeriesCounts(workloadsSummary);
        return counts.length === 4 && counts.every((count) => count > 1);
      })
      .toBe(true);
    const resolvedBaselineCounts = await readRenderedSeriesCounts(workloadsSummary);

    const baselineVisibleRows = await page.locator("tr[data-guest-id]").count();
    const groupRows = page.locator("tr[data-summary-group-id]");
    const groupRowCount = await groupRows.count();
    expect(groupRowCount).toBeGreaterThan(0);

    let matchedGroupRow: import("@playwright/test").Locator | null = null;
    let matchedGroupSeriesCount = 0;
    for (let index = 0; index < groupRowCount; index += 1) {
      const row = groupRows.nth(index);
      if (!(await row.isVisible())) {
        continue;
      }
      const rawSeriesCount = await row.getAttribute("data-summary-group-series-count");
      const seriesCount = Number.parseInt(rawSeriesCount || "0", 10);
      if (!Number.isFinite(seriesCount) || seriesCount < 1) {
        continue;
      }
      if (resolvedBaselineCounts.every((count) => count <= seriesCount)) {
        continue;
      }

      matchedGroupRow = row;
      matchedGroupSeriesCount = seriesCount;
      break;
    }

    expect(matchedGroupRow).not.toBeNull();
    expect(matchedGroupSeriesCount).toBeGreaterThan(0);
    if (!matchedGroupRow) {
      return;
    }

    await matchedGroupRow.hover();
    await expect(matchedGroupRow).toHaveAttribute("data-summary-row-active", "true");
    await expect
      .poll(() => readRenderedSeriesCounts(workloadsSummary))
      .toEqual(new Array(4).fill(matchedGroupSeriesCount));
    await expect(page.locator("tr[data-guest-id]")).toHaveCount(baselineVisibleRows);

    await page.mouse.move(1, 1);
    await expect(matchedGroupRow).toHaveAttribute("data-summary-row-active", "false");
    await expect
      .poll(() => readRenderedSeriesCounts(workloadsSummary))
      .toEqual(resolvedBaselineCounts);
  });

  test("treats infrastructure cluster headers as first-class summary scope", async ({
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

    await expect
      .poll(async () => {
        const counts = await readRenderedSeriesCounts(infrastructureSummary);
        return counts.length === 4 && counts.every((count) => count > 1);
      })
      .toBe(true);
    const resolvedBaselineCounts = await readRenderedSeriesCounts(
      infrastructureSummary,
    );

    const baselineVisibleRows = await page.locator("tr[data-summary-series-id]").count();
    const groupRows = page.locator("tr[data-summary-group-id]");
    const groupRowCount = await groupRows.count();
    expect(groupRowCount).toBeGreaterThan(0);

    let matchedGroupRow: import("@playwright/test").Locator | null = null;
    let matchedGroupSeriesCount = 0;
    for (let index = 0; index < groupRowCount; index += 1) {
      const row = groupRows.nth(index);
      if (!(await row.isVisible())) {
        continue;
      }
      const rawSeriesCount = await row.getAttribute("data-summary-group-series-count");
      const seriesCount = Number.parseInt(rawSeriesCount || "0", 10);
      if (!Number.isFinite(seriesCount) || seriesCount < 1) {
        continue;
      }
      if (resolvedBaselineCounts.every((count) => count <= seriesCount)) {
        continue;
      }

      matchedGroupRow = row;
      matchedGroupSeriesCount = seriesCount;
      break;
    }

    expect(matchedGroupRow).not.toBeNull();
    expect(matchedGroupSeriesCount).toBeGreaterThan(0);
    if (!matchedGroupRow) {
      return;
    }

    await matchedGroupRow.hover();
    await expect(matchedGroupRow).toHaveAttribute("data-summary-row-active", "true");
    await expect
      .poll(() => readRenderedSeriesCounts(infrastructureSummary))
      .toEqual(new Array(4).fill(matchedGroupSeriesCount));
    await expect(page.locator("tr[data-summary-series-id]")).toHaveCount(
      baselineVisibleRows,
    );

    await page.mouse.move(1, 1);
    await expect(matchedGroupRow).toHaveAttribute("data-summary-row-active", "false");
    await expect
      .poll(() => readRenderedSeriesCounts(infrastructureSummary))
      .toEqual(resolvedBaselineCounts);
  });

  test("pins workload and infrastructure group scope into reversible route-backed focus", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop runtime proof",
    );

    await ensureMockModeEnabled(page);

    for (const surface of [
      {
        path: "/workloads",
        summaryTestId: "workloads-summary",
        scopeBarTestId: "workloads-summary-scope",
        tableRowSelector: "tr[data-guest-id]",
      },
      {
        path: "/infrastructure",
        summaryTestId: "infrastructure-summary",
        scopeBarTestId: "infrastructure-summary-scope",
        tableRowSelector: "tr[data-summary-series-id]",
      },
    ] as const) {
      await page.goto(surface.path, { waitUntil: "domcontentloaded" });
      const summary = page.getByTestId(surface.summaryTestId);
      const scopeBar = page.getByTestId(surface.scopeBarTestId);
      await expect(summary).toBeVisible();
      await expect(scopeBar).toContainText("All");

      await expect
        .poll(async () => {
          const counts = await readRenderedSeriesCounts(summary);
          return counts.length === 4 && counts.every((count) => count > 1);
        })
        .toBe(true);

      const baselineCounts = await readRenderedSeriesCounts(summary);
      const baselineVisibleRows = await page.locator(surface.tableRowSelector).count();
      const groupRows = page.locator("tr[data-summary-group-id]");
      const groupRowCount = await groupRows.count();
      expect(groupRowCount).toBeGreaterThan(0);

      let matchedGroupRow: import("@playwright/test").Locator | null = null;
      let matchedGroupSeriesCount = 0;
      let matchedGroupId = "";
      for (let index = 0; index < groupRowCount; index += 1) {
        const row = groupRows.nth(index);
        if (!(await row.isVisible())) {
          continue;
        }
        const rawSeriesCount = await row.getAttribute("data-summary-group-series-count");
        const seriesCount = Number.parseInt(rawSeriesCount || "0", 10);
        const groupId = (await row.getAttribute("data-summary-group-id")) || "";
        if (!Number.isFinite(seriesCount) || seriesCount < 1 || !groupId) {
          continue;
        }
        if (baselineCounts.every((count) => count <= seriesCount)) {
          continue;
        }
        matchedGroupRow = row;
        matchedGroupSeriesCount = seriesCount;
        matchedGroupId = groupId;
        break;
      }

      expect(matchedGroupRow).not.toBeNull();
      expect(matchedGroupSeriesCount).toBeGreaterThan(0);
      expect(matchedGroupId).not.toBe("");
      if (!matchedGroupRow) {
        return;
      }

      await matchedGroupRow.focus();
      await expect(scopeBar).toContainText("Preview");
      await page.keyboard.press("Escape");
      await expect(scopeBar).toContainText("All");

      await matchedGroupRow.focus();
      await expect(scopeBar).toContainText("Preview");
      await page.keyboard.press("Enter");
      await expect
        .poll(() => new URL(page.url()).searchParams.get("summaryGroup"))
        .toBe(matchedGroupId);
      await expect(matchedGroupRow).toHaveAttribute("aria-pressed", "true");
      await expect(scopeBar).toContainText("Pinned");
      await expect(
        scopeBar.getByRole("button", { name: "Reset pinned scope" }),
      ).toBeVisible();
      await expect
        .poll(() => readRenderedSeriesCounts(summary))
        .toEqual(new Array(4).fill(matchedGroupSeriesCount));
      await expect(page.locator(surface.tableRowSelector)).toHaveCount(
        baselineVisibleRows,
      );

      await page.mouse.move(1, 1);
      await expect
        .poll(() => readRenderedSeriesCounts(summary))
        .toEqual(new Array(4).fill(matchedGroupSeriesCount));

      await scopeBar.getByRole("button", { name: "Reset pinned scope" }).click();
      await expect
        .poll(() => new URL(page.url()).searchParams.get("summaryGroup"))
        .toBeNull();
      await expect(scopeBar).toContainText("All");
      await page.mouse.move(1, 1);
      await expect
        .poll(() => readRenderedSeriesCounts(summary))
        .toEqual(baselineCounts);
    }
  });

  test("pins storage pool-group scope into reversible route-backed focus without collapsing the table", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop runtime proof",
    );

    await ensureMockModeEnabled(page);

    await page.goto("/storage?group=node", { waitUntil: "domcontentloaded" });
    const storageSummary = page.getByTestId("storage-summary");
    const storageScopeBar = page.getByTestId("storage-summary-scope");
    await expect(storageSummary).toBeVisible();
    await expect(storageScopeBar).toContainText("All");

    await expect
      .poll(async () => {
        const counts = await readRenderedSeriesCounts(storageSummary);
        return counts.length >= 3 && counts.some((count) => count > 1);
      })
      .toBe(true);

    const baselineCounts = await readRenderedSeriesCounts(storageSummary);
    const baselineVisibleRows = await page.locator("tr[data-row-id]").count();
    const groupRows = page.locator("tr[data-summary-group-id]");
    const groupRowCount = await groupRows.count();
    expect(groupRowCount).toBeGreaterThan(0);

    let matchedGroupRow: import("@playwright/test").Locator | null = null;
    let matchedGroupId = "";
    for (let index = 0; index < groupRowCount; index += 1) {
      const row = groupRows.nth(index);
      if (!(await row.isVisible())) {
        continue;
      }
      const rawSeriesCount = await row.getAttribute("data-summary-group-series-count");
      const seriesCount = Number.parseInt(rawSeriesCount || "0", 10);
      const groupId = (await row.getAttribute("data-summary-group-id")) || "";
      if (!Number.isFinite(seriesCount) || seriesCount < 1 || !groupId) {
        continue;
      }
      if (baselineCounts.every((count) => count <= seriesCount)) {
        continue;
      }
      matchedGroupRow = row;
      matchedGroupId = groupId;
      break;
    }

    expect(matchedGroupRow).not.toBeNull();
    expect(matchedGroupId).not.toBe("");
    if (!matchedGroupRow) {
      return;
    }

    await matchedGroupRow.focus();
    await expect(storageScopeBar).toContainText("Preview");
    await page.keyboard.press("Escape");
    await expect(storageScopeBar).toContainText("All");

    await matchedGroupRow.focus();
    await expect(storageScopeBar).toContainText("Preview");
    await page.keyboard.press(" ");
    await expect
      .poll(() => new URL(page.url()).searchParams.get("summaryGroup"))
      .toBe(matchedGroupId);
    await expect(matchedGroupRow).toHaveAttribute("aria-pressed", "true");
    await expect(storageScopeBar).toContainText("Pinned");
    await expect(page.locator("tr[data-row-id]")).toHaveCount(baselineVisibleRows);

    await expect
      .poll(() => readRenderedSeriesCounts(storageSummary))
      .not.toEqual(baselineCounts);

    await page.mouse.move(1, 1);
    await expect(page.locator("tr[data-row-id]")).toHaveCount(baselineVisibleRows);

    await storageScopeBar
      .getByRole("button", { name: "Reset pinned scope" })
      .click();
    await expect
      .poll(() => new URL(page.url()).searchParams.get("summaryGroup"))
      .toBeNull();
    await expect(storageScopeBar).toContainText("All");
    await page.mouse.move(1, 1);
    await expect
      .poll(() => readRenderedSeriesCounts(storageSummary))
      .toEqual(baselineCounts);
  });

  test("keeps density-map detail inside the hover tooltip without extra chart chrome", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop runtime proof",
    );

    await ensureMockModeEnabled(page);

    await page.goto("/workloads", { waitUntil: "domcontentloaded" });
    const workloadsSummary = page.getByTestId("workloads-summary");
    await expect(workloadsSummary).toBeVisible();
    await expect(
      workloadsSummary.locator('[data-density-map-focus-detail="true"]'),
    ).toHaveCount(0);
    await expect(workloadsSummary).not.toContainText("Top activity overview");

    const firstDensityMap = workloadsSummary
      .locator('[data-summary-chart-kind="density-map"]')
      .first();
    await hoverDensityMapUntilTooltipAppears(page, firstDensityMap);

    const tooltip = page.locator('[data-density-map-tooltip="true"]').last();
    await expect(tooltip).toBeVisible();
    await expect(tooltip).toContainText("Current");
    await expect(tooltip).toContainText("Peak");
    await expect(
      tooltip.locator('[data-density-map-tooltip-sparkline="true"]'),
    ).toHaveCount(0);
  });
});
