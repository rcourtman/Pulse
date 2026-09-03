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

// The drawer tab strip (Overview | History | Discovery) marks the active tab
// with aria-selected, so read that instead of panel order.
async function readGuestDrawerActiveTab(detailRow: Locator): Promise<string> {
  const buttons = detailRow.locator("[aria-selected]");
  const active = await buttons.evaluateAll((nodes) => {
    const selected = nodes.find(
      (node) => node.getAttribute("aria-selected") === "true",
    );
    return selected?.textContent?.trim().toLowerCase() ?? "unknown";
  });
  return active;
}

function issue1611Container(
  vmid: number,
  name: string,
  status: "online" | "warning",
  availability = false,
) {
  return {
    id: `lab-node-a-${vmid}`,
    type: "system-container",
    name,
    status,
    lastSeen: "2026-07-24T08:00:00Z",
    vmid,
    node: "node-a",
    instance: "lab",
    sources: ["proxmox", ...(availability ? ["availability"] : [])],
    platformScopes: ["proxmox-pve"],
    metrics: {
      cpu: { percent: 0.12 },
      memory: { used: 1024, total: 4096, percent: 25 },
      disk: { used: 2048, total: 8192, percent: 25 },
    },
    proxmox: {
      runtimeStatus: "running",
      nodeName: "node-a",
      instance: "lab",
      vmid,
      cpus: 2,
      uptime: 3600,
    },
    ...(availability
      ? {
          availability: {
            targetId: `probe-${vmid}`,
            protocol: "icmp",
            enabled: true,
            available: true,
          },
        }
      : {}),
  };
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

    // Workloads are owned by the Proxmox workloads sub-route; preserve the
    // canonical type/platform scope while exercising its polling behavior.
    await page.goto("/proxmox/workloads?type=vm&platform=proxmox-pve", {
      waitUntil: "domcontentloaded",
    });

    const rows = page.locator("tr[data-guest-id]");
    await expect(rows.first()).toBeVisible({ timeout: 60_000 });

    const row = rows.first();
    const guestId = (await row.getAttribute("data-guest-id")) ?? "";
    expect(guestId).not.toBe("");

    const beforeRowTop = await positionElementNearViewportBottom(page, row);
    expect(beforeRowTop).toBeGreaterThan(500);

    await row.click();

    // Row selection no longer writes a ?resource= deep link; the drawer row
    // itself is the selection contract.
    const detailRow = page.locator(`tr[data-inline-detail-for="${guestId}"]`);
    await expect(detailRow).toBeVisible();

    // The drawer tab strip exposes proper tab roles.
    const discoveryButton = detailRow.getByRole("tab", {
      name: "Discovery",
      exact: true,
    });
    await expect(discoveryButton).toBeVisible();
    await discoveryButton.click();

    await expect
      .poll(() => readGuestDrawerActiveTab(detailRow))
      .toBe("discovery");

    const beforePollScrollTop = await readPrimaryViewportScrollTop(page);

    await page.waitForTimeout(7_500);

    await expect(detailRow).toBeVisible();
    await expect
      .poll(() => readGuestDrawerActiveTab(detailRow), { timeout: 15_000 })
      .toBe("discovery");

    const afterPollScrollTop = await readPrimaryViewportScrollTop(page);
    expect(afterPollScrollTop).toBeGreaterThanOrEqual(
      Math.max(10, beforePollScrollTop - 80),
    );
  });

  test("retains LXC rows through warning health projections and removes a confirmed deletion", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only workload refresh proof",
    );

    await ensureMockModeEnabled(page);

    let workloadRequests = 0;
    let staleProjectionResponses = 0;
    let deletionResponses = 0;
    let publishDeletion = false;

    // The REST sequence is the contract under test. Prevent live inventory
    // frames from replacing it while polling is exercised.
    await page.routeWebSocket("**/ws*", () => {});

    await page.route("**/api/resources?**", async (route) => {
      const url = new URL(route.request().url());
      if (
        url.pathname !== "/api/resources" ||
        url.searchParams.get("type") !== "vm,system-container,app-container,pod"
      ) {
        await route.continue();
        return;
      }

      workloadRequests += 1;
      let data;
      if (workloadRequests === 1) {
        data = [
          issue1611Container(101, "lxc-alpha", "online"),
          issue1611Container(102, "lxc-beta", "online", true),
          issue1611Container(103, "lxc-gamma", "online"),
        ];
      } else if (!publishDeletion) {
        staleProjectionResponses += 1;
        data = [
          issue1611Container(101, "lxc-alpha", "warning"),
          issue1611Container(102, "lxc-beta", "online", true),
          issue1611Container(103, "lxc-gamma", "warning"),
        ];
      } else {
        deletionResponses += 1;
        data = [
          issue1611Container(101, "lxc-alpha", "online"),
          issue1611Container(102, "lxc-beta", "online", true),
        ];
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data,
          meta: { page: 1, limit: 200, total: data.length, totalPages: 1 },
        }),
      });
    });

    await page.goto(
      "/proxmox/workloads?type=system-container&platform=proxmox-pve",
      { waitUntil: "domcontentloaded" },
    );

    const rows = page.locator("tr[data-guest-id]");
    await expect(rows).toHaveCount(3, { timeout: 60_000 });
    await expect(rows.first()).toBeVisible();

    await page.locator("th").filter({ hasText: "Name" }).last().click();
    const retainedRow = rows.filter({ hasText: "lxc-beta" });
    await retainedRow.click();
    const detailRow = page.locator(
      'tr[data-inline-detail-for="lab:node-a:102"]',
    );
    await expect(detailRow).toBeVisible();

    await expect
      .poll(() => staleProjectionResponses, { timeout: 15_000 })
      .toBeGreaterThan(0);
    await expect(rows).toHaveCount(3);
    await expect(rows.filter({ hasText: "lxc-alpha" })).toBeVisible();
    await expect(rows.filter({ hasText: "lxc-gamma" })).toBeVisible();
    await expect(detailRow).toBeVisible();

    publishDeletion = true;
    await expect
      .poll(() => deletionResponses, { timeout: 15_000 })
      .toBeGreaterThan(0);
    await expect(rows).toHaveCount(2);
    await expect(rows.filter({ hasText: "lxc-gamma" })).toHaveCount(0);
    await expect(detailRow).toBeVisible();
  });

  test("does not request vertical scrolling when an off-screen Backups drawer changes or closes", async ({
    page,
  }, testInfo) => {
    await ensureMockModeEnabled(page);

    await page.goto("/proxmox/backups", { waitUntil: "domcontentloaded" });
    const serversTable = page.locator('[data-proxmox-backups-table="servers"]');
    await expect(serversTable).toBeVisible({ timeout: 60_000 });

    const disclosure = serversTable.locator("button[aria-controls]").first();
    const summaryRow = disclosure.locator("xpath=ancestor::tr");
    await summaryRow.click();

    const detailRow = serversTable.locator(
      "tr[data-inline-platform-resource-detail-for]",
    );
    await expect(detailRow).toBeVisible();
    const manageTab = detailRow.getByRole("tab", {
      name: "Manage",
      exact: true,
    });
    const overviewTab = detailRow.getByRole("tab", {
      name: "Overview",
      exact: true,
    });
    const closeButton = detailRow.getByRole("button", {
      name: /^Collapse .* details$/,
    });
    const viewportAnchor = page.getByText("Backup health", { exact: true });
    await expect(viewportAnchor).toBeVisible();

    await page.screenshot({
      path: testInfo.outputPath("pbs-detail-overview.png"),
    });
    await manageTab.click();
    await expect(manageTab).toHaveAttribute("aria-selected", "true");
    await page.screenshot({
      path: testInfo.outputPath("pbs-detail-manage.png"),
    });
    await overviewTab.click();
    await expect(overviewTab).toHaveAttribute("aria-selected", "true");

    // Put the section beneath the expanded drawer in view so the drawer tabs
    // are above the viewport, matching an operator reading backup evidence
    // below a server row. Tab selection must only move its horizontal rail.
    const nextScrollTop = await viewportAnchor.evaluate((element) => {
      const shell = document.querySelector<HTMLElement>(".app-scroll-shell");
      if (shell && shell.contains(element)) {
        const shellRect = shell.getBoundingClientRect();
        return Math.max(
          0,
          shell.scrollTop +
            element.getBoundingClientRect().top -
            shellRect.top -
            180,
        );
      }
      return Math.max(
        0,
        window.scrollY + element.getBoundingClientRect().top - 180,
      );
    });
    await page.evaluate((top) => {
      const shell = document.querySelector<HTMLElement>(".app-scroll-shell");
      if (shell) shell.scrollTop = top;
      else window.scrollTo(0, top);
    }, nextScrollTop);
    await page.waitForTimeout(150);

    await page.evaluate(() => {
      const state = window as Window & {
        __pulseScrollIntoViewCalls?: number;
      };
      state.__pulseScrollIntoViewCalls = 0;
      Element.prototype.scrollIntoView = () => {
        state.__pulseScrollIntoViewCalls =
          (state.__pulseScrollIntoViewCalls ?? 0) + 1;
      };
    });
    await manageTab.evaluate((element) => (element as HTMLElement).click());
    await expect(manageTab).toHaveAttribute("aria-selected", "true");
    await page.waitForTimeout(150);
    expect(
      await page.evaluate(
        () =>
          (window as Window & { __pulseScrollIntoViewCalls?: number })
            .__pulseScrollIntoViewCalls ?? 0,
      ),
    ).toBe(0);

    await page.screenshot({
      path: testInfo.outputPath("pbs-detail-anchor.png"),
    });

    // Closing from focused drawer content returns focus to the disclosure,
    // but that restoration must not scroll the summary row back into view.
    await manageTab.evaluate((element) =>
      (element as HTMLElement).focus({ preventScroll: true }),
    );
    await disclosure.evaluate((element) => {
      const state = window as Window & {
        __pulseDisclosureFocusOptions?: FocusOptions;
      };
      const disclosureElement = element as HTMLElement;
      const originalFocus = disclosureElement.focus.bind(disclosureElement);
      disclosureElement.focus = (options?: FocusOptions) => {
        state.__pulseDisclosureFocusOptions = options;
        originalFocus(options);
      };
    });
    await closeButton.evaluate((element) => (element as HTMLElement).click());
    await expect(detailRow).toHaveCount(0);
    await page.waitForTimeout(150);
    expect(
      await page.evaluate(
        () =>
          (
            window as Window & {
              __pulseDisclosureFocusOptions?: FocusOptions;
            }
          ).__pulseDisclosureFocusOptions,
      ),
    ).toEqual({ preventScroll: true });
    if (!testInfo.project.name.startsWith("mobile-")) {
      await expect(disclosure).toBeFocused();
    }
  });
});
