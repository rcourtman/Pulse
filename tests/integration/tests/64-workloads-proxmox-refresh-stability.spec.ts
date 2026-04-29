import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base, type Locator, type Page } from "@playwright/test";

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
        `workloads-proxmox-refresh-stability-${workerInfo.project.name}.json`,
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

async function readPrimaryViewportScrollTop(page: Page): Promise<number> {
  return page.evaluate(() => {
    const shell = document.querySelector<HTMLElement>(".app-scroll-shell");
    return shell ? shell.scrollTop : window.scrollY;
  });
}

async function positionElementNearViewportBottom(
  page: Page,
  locator: Locator,
  bottomInset = 96,
): Promise<number> {
  const targetTop = await locator.evaluate(
    (element, inset) =>
      (() => {
        const shell = document.querySelector<HTMLElement>(".app-scroll-shell");
        if (shell && shell.contains(element)) {
          const shellRect = shell.getBoundingClientRect();
          return Math.max(
            0,
            shell.scrollTop +
              element.getBoundingClientRect().top -
              shellRect.top -
              (shell.clientHeight - inset),
          );
        }
        return Math.max(
          0,
          window.scrollY +
            element.getBoundingClientRect().top -
            (window.innerHeight - inset),
        );
      })(),
    bottomInset,
  );
  await page.evaluate((nextTop) => {
    const shell = document.querySelector<HTMLElement>(".app-scroll-shell");
    if (shell) {
      shell.scrollTop = nextTop;
      return;
    }
    window.scrollTo(0, nextTop);
  }, targetTop);
  await page.waitForTimeout(150);
  return locator.evaluate((element) => element.getBoundingClientRect().top);
}

async function readGuestDrawerActiveTab(
  detailRow: Locator,
): Promise<"overview" | "discovery" | "unknown"> {
  const panels = detailRow.locator('[style*="overflow-anchor"]');
  const visiblePanelIndex = await panels.evaluateAll((nodes) =>
    nodes.findIndex((node) => !node.classList.contains("hidden")),
  );
  if (visiblePanelIndex === 0) return "overview";
  if (visiblePanelIndex === 1) return "discovery";
  return "unknown";
}

test.describe.serial("Workloads Proxmox refresh stability", () => {
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

  test("keeps an expanded Proxmox VM on the Discovery tab across workload polling", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only workload refresh proof",
    );

    await ensureMockModeEnabled(page);
    await page.addInitScript(() => {
      localStorage.setItem("pulse_whats_new_v2_shown", "true");
    });

    await page.goto("/workloads?type=vm&platform=proxmox-pve", {
      waitUntil: "domcontentloaded",
    });

    const rows = page.locator("tr[data-guest-id]");
    await expect(rows.first()).toBeVisible();

    const row = rows.first();
    const guestId = (await row.getAttribute("data-guest-id")) ?? "";
    expect(guestId).not.toBe("");

    const beforeRowTop = await positionElementNearViewportBottom(page, row);
    expect(beforeRowTop).toBeGreaterThan(500);

    await row.click();

    await expect
      .poll(() => new URL(page.url()).searchParams.get("resource"))
      .toBe(guestId);

    const detailRow = page.locator(`tr[data-inline-detail-for="${guestId}"]`);
    await expect(detailRow).toBeVisible();

    const discoveryButton = detailRow.getByRole("button", {
      name: "Discovery",
      exact: true,
    });
    await expect(discoveryButton).toBeVisible();
    await discoveryButton.click();

    await expect.poll(() => readGuestDrawerActiveTab(detailRow)).toBe("discovery");

    const beforePollScrollTop = await readPrimaryViewportScrollTop(page);

    await page.waitForTimeout(7_500);

    await expect(detailRow).toBeVisible();
    await expect
      .poll(() => new URL(page.url()).searchParams.get("resource"))
      .toBe(guestId);
    await expect
      .poll(() => readGuestDrawerActiveTab(detailRow), { timeout: 15_000 })
      .toBe("discovery");

    const afterPollScrollTop = await readPrimaryViewportScrollTop(page);
    expect(afterPollScrollTop).toBeGreaterThanOrEqual(
      Math.max(10, beforePollScrollTop - 80),
    );
  });
});
