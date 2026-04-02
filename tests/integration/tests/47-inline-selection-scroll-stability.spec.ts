import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  expect,
  test as base,
  type Locator,
  type Page,
} from "@playwright/test";

import {
  apiRequest,
  createAuthenticatedStorageState,
  getMockMode,
  setMockMode,
} from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

type ResourceSummary = {
  name?: string;
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
        `inline-selection-scroll-stability-${workerInfo.project.name}.json`,
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

async function ensureMockModeEnabled(page: Page): Promise<void> {
  const state = await getMockMode(page);
  if (mockModeWasEnabled === null) {
    mockModeWasEnabled = state.enabled;
  }
  if (!state.enabled) {
    await setMockMode(page, true);
  }
}

async function fetchMockInfrastructureResourceName(
  page: Page,
): Promise<string> {
  const response = await apiRequest(
    page,
    "/api/resources?source=proxmox&limit=200&type=agent",
  );
  expect(response.ok()).toBeTruthy();
  const payload = (await response.json()) as { data?: ResourceSummary[] };
  const resources = Array.isArray(payload.data) ? payload.data : [];
  const chosen = resources.find((resource) => resource.name?.trim());
  if (!chosen?.name?.trim()) {
    throw new Error(
      "Expected at least one Proxmox mock infrastructure resource",
    );
  }
  return chosen.name.trim();
}

async function scrollSectionIntoView(
  page: Page,
  locator: Locator,
  offset = 180,
): Promise<number> {
  const top = await locator.evaluate(
    (element, sectionOffset) =>
      Math.max(
        0,
        window.scrollY + element.getBoundingClientRect().top - sectionOffset,
      ),
    offset,
  );
  await page.evaluate((nextTop) => window.scrollTo(0, nextTop), top);
  await page.waitForTimeout(150);
  return page.evaluate(() => window.scrollY);
}

async function positionElementNearViewportBottom(
  page: Page,
  locator: Locator,
  bottomInset = 96,
): Promise<number> {
  const targetTop = await locator.evaluate(
    (element, inset) =>
      Math.max(
        0,
        window.scrollY + element.getBoundingClientRect().top - (window.innerHeight - inset),
      ),
    bottomInset,
  );
  await page.evaluate((nextTop) => window.scrollTo(0, nextTop), targetTop);
  await page.waitForTimeout(150);
  return locator.evaluate((element) => element.getBoundingClientRect().top);
}

async function findLegacyScopedWorkloadRow(page: Page): Promise<Locator> {
  const rows = page.locator("tr[data-guest-id]");
  const rowCount = await rows.count();
  for (let index = 0; index < rowCount; index += 1) {
    const row = rows.nth(index);
    if (!(await row.isVisible())) {
      continue;
    }
    const guestId = ((await row.getAttribute("data-guest-id")) || "").trim();
    if (
      /^([^:]+):([^:]+):(\d+)$/.test(guestId) &&
      !guestId.startsWith("app-container:") &&
      !guestId.startsWith("pod:")
    ) {
      return row;
    }
  }
  throw new Error("Expected one visible legacy-scoped workload row");
}

test.describe.serial("Inline selection scroll stability", () => {
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

  test("keeps the infrastructure viewport stable when opening an inline resource drawer", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only drawer stability proof",
    );

    await ensureMockModeEnabled(page);
    const resourceName = await fetchMockInfrastructureResourceName(page);

    await page.goto("/infrastructure?source=proxmox-pve", {
      waitUntil: "domcontentloaded",
    });
    await expect(page.getByTestId("infrastructure-page")).toBeVisible();

    const table = page
      .locator('[data-testid="infrastructure-page"] table')
      .first();
    const beforeScroll = await scrollSectionIntoView(page, table);
    expect(beforeScroll).toBeGreaterThan(10);

    const row = table
      .locator("tbody tr")
      .filter({ hasText: resourceName })
      .first();
    await expect(row).toBeVisible();
    const beforeRowTop = await positionElementNearViewportBottom(page, row);
    expect(beforeRowTop).toBeGreaterThan(500);
    await row.click();

    await expect(page).toHaveURL(
      /\/infrastructure\?source=proxmox-pve&resource=/,
    );
    const detailRow = row.locator("xpath=following-sibling::tr[1]");
    await expect(
      page.getByRole("button", { name: /show access|hide access/i }),
    ).toBeVisible();
    await expect(detailRow).toBeVisible();
    await page.waitForTimeout(350);

    const viewportHeight = await page.evaluate(() => window.innerHeight);
    const afterRowTop = await row.evaluate((element) => element.getBoundingClientRect().top);
    const detailTop = await detailRow.evaluate((element) => element.getBoundingClientRect().top);

    expect(afterRowTop).toBeLessThan(beforeRowTop - 150);
    expect(afterRowTop).toBeLessThan(viewportHeight * 0.5);
    expect(detailTop).toBeLessThan(viewportHeight - 48);

    const afterScroll = await page.evaluate(() => window.scrollY);
    expect(afterScroll).toBeGreaterThanOrEqual(Math.max(10, beforeScroll - 60));
  });

  test("keeps the recovery viewport stable when selecting a protected item", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only recovery interaction proof",
    );

    await ensureMockModeEnabled(page);

    await page.goto("/recovery", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("recovery-page")).toBeVisible();

    const protectedItemsTable = page
      .locator('[data-testid="recovery-page"] table')
      .first();
    const beforeScroll = await scrollSectionIntoView(page, protectedItemsTable);
    expect(beforeScroll).toBeGreaterThan(10);

    const row = protectedItemsTable.locator("tbody tr").first();
    await expect(row).toBeVisible();
    await row.click();

    await expect(page).toHaveURL(/\/recovery\?rollupId=/);
    await expect(
      page.getByRole("tab", { name: /recovery events/i }),
    ).toHaveAttribute("aria-selected", "true");
    await expect(
      page.getByTestId("recovery-history-item-filter-trigger"),
    ).not.toContainText("Any Item");

    const afterScroll = await page.evaluate(() => window.scrollY);
    expect(afterScroll).toBeGreaterThanOrEqual(Math.max(10, beforeScroll - 60));
  });

  test("keeps the workloads viewport stable when opening an inline workload drawer", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only workload interaction proof",
    );

    await ensureMockModeEnabled(page);

    await page.goto("/workloads", { waitUntil: "domcontentloaded" });
    await expect(page.locator("tr[data-guest-id]").first()).toBeVisible();

    const row = await findLegacyScopedWorkloadRow(page);
    const beforeScroll = await scrollSectionIntoView(page, row);
    expect(beforeScroll).toBeGreaterThan(10);
    const beforeRowTop = await positionElementNearViewportBottom(page, row);
    expect(beforeRowTop).toBeGreaterThan(500);

    const workloadId = (await row.getAttribute("data-guest-id")) ?? "";
    expect(workloadId).not.toBe("");

    await row.click();

    await expect(page).toHaveURL(/\/workloads\?(?:.*&)?resource=/);
    {
      const openedUrl = new URL(page.url());
      expect(openedUrl.searchParams.get("resource")).toBe(workloadId);
      expect(openedUrl.searchParams.has("agent")).toBe(false);
    }
    const detailRow = row.locator("xpath=following-sibling::tr[1]");
    await expect(detailRow).toContainText(
      "Overview",
    );
    await page.waitForTimeout(350);

    const viewportHeight = await page.evaluate(() => window.innerHeight);
    const afterRowTop = await row.evaluate((element) => element.getBoundingClientRect().top);
    const detailTop = await detailRow.evaluate((element) => element.getBoundingClientRect().top);

    expect(afterRowTop).toBeLessThan(beforeRowTop - 150);
    expect(afterRowTop).toBeLessThan(viewportHeight * 0.5);
    expect(detailTop).toBeLessThan(viewportHeight - 48);

    const afterScroll = await page.evaluate(() => window.scrollY);
    expect(afterScroll).toBeGreaterThanOrEqual(Math.max(10, beforeScroll - 60));

    await row.click();
    await expect.poll(() => page.url()).not.toContain("resource=");
    {
      const closedUrl = new URL(page.url());
      expect(closedUrl.searchParams.has("resource")).toBe(false);
      expect(closedUrl.searchParams.has("agent")).toBe(false);
    }
  });

  test("supports keyboard open-close for workload rows without leaking filter state", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only keyboard interaction proof",
    );

    await ensureMockModeEnabled(page);

    await page.goto("/workloads", { waitUntil: "domcontentloaded" });
    const row = page.locator("tr[data-guest-id]").first();
    await expect(row).toBeVisible();
    const workloadId = (await row.getAttribute("data-guest-id")) ?? "";
    expect(workloadId).not.toBe("");

    const toggleButton = row.locator("button[aria-controls]").first();
    await expect(toggleButton).toBeVisible();
    await toggleButton.focus();
    await expect(toggleButton).toBeFocused();
    await page.keyboard.press("Enter");

    await expect(page).toHaveURL(/\/workloads\?(?:.*&)?resource=/);
    {
      const openedUrl = new URL(page.url());
      expect(openedUrl.searchParams.get("resource")).toBe(workloadId);
      expect(openedUrl.searchParams.has("agent")).toBe(false);
    }

    const detailRow = row.locator("xpath=following-sibling::tr[1]");
    await expect(detailRow).toContainText("Overview");

    await toggleButton.focus();
    await toggleButton.press("Space");
    await expect.poll(() => page.url()).not.toContain("resource=");
    {
      const closedUrl = new URL(page.url());
      expect(closedUrl.searchParams.has("resource")).toBe(false);
      expect(closedUrl.searchParams.has("agent")).toBe(false);
    }
  });

  test("reveals storage inline detail without hard-centering the selected row", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only storage interaction proof",
    );

    await ensureMockModeEnabled(page);

    await page.goto("/storage", { waitUntil: "domcontentloaded" });
    const row = page.locator("tr[data-summary-series-id]").first();
    await expect(row).toBeVisible();

    const beforeScroll = await scrollSectionIntoView(page, row);
    expect(beforeScroll).toBeGreaterThan(10);
    const beforeRowTop = await positionElementNearViewportBottom(page, row);
    expect(beforeRowTop).toBeGreaterThan(500);

    await row.click();

    const detailRow = row.locator("xpath=following-sibling::tr[1]");
    await expect(detailRow).toBeVisible();
    await page.waitForTimeout(350);

    const viewportHeight = await page.evaluate(() => window.innerHeight);
    const afterRowTop = await row.evaluate((element) => element.getBoundingClientRect().top);
    const detailTop = await detailRow.evaluate((element) => element.getBoundingClientRect().top);

    expect(afterRowTop).toBeLessThan(beforeRowTop - 150);
    expect(afterRowTop).toBeLessThan(viewportHeight * 0.42);
    expect(detailTop).toBeLessThan(viewportHeight - 48);
  });
});
