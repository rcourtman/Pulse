import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base, type Page } from "@playwright/test";

import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const offlineNode = {
  id: "proxmox:core-fabric:pve5",
  type: "agent",
  name: "pve5",
  canonicalIdentity: {
    displayName: "Disaster Recovery B",
    hostname: "pve5",
    platformId: "core-fabric",
  },
  sources: ["proxmox"],
  status: "offline",
  uptime: 2_419_200,
  lastSeen: "2026-07-24T08:00:00Z",
  metrics: {
    cpu: { percent: 47 },
    memory: { percent: 61, used: 61_000, total: 100_000 },
    disk: { percent: 72, used: 72_000, total: 100_000 },
  },
  proxmox: {
    instance: "core-fabric",
    clusterName: "Core Fabric",
    nodeName: "pve5",
    host: "https://pve5:8006",
    connectionHealth: "error",
    pveVersion: "7.4-18",
    temperature: 71,
  },
};

async function routeOfflineProxmoxInventory(page: Page) {
  // The browser assertion owns presentation, while the Go lifecycle tests own
  // membership reconciliation. Suppress live websocket inventory here and
  // provide one deterministic canonical REST snapshot so startup timing,
  // cache state, and unrelated mock-estate changes cannot weaken the UI proof.
  await page.routeWebSocket("**/ws*", () => {});
  await page.route("**/api/resources**", async (route) => {
    const requestUrl = new URL(route.request().url());
    if (requestUrl.pathname !== "/api/resources") {
      await route.continue();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: [offlineNode],
        meta: { page: 1, limit: 100, total: 1, totalPages: 1 },
        links: { next: null },
      }),
    });
  });
}

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
        `offline-proxmox-node-${workerInfo.project.name}.json`,
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

// Browser-level contract for the provider-first migration: a known but
// unreachable PVE node stays listed on its platform page instead of silently
// vanishing, and cached metrics are not presented as live.
test.describe("Offline Proxmox node visibility", () => {
  test.setTimeout(180_000);

  test("keeps an offline Proxmox node visible on desktop and mobile surfaces", async ({
    page,
  }) => {
    await routeOfflineProxmoxInventory(page);
    await page.goto("/proxmox", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("proxmox-page")).toBeVisible();

    const offlineRow = page
      .locator('[data-testid="proxmox-page"] tr')
      .filter({ hasText: "Disaster Recovery B" })
      .first();
    await expect(offlineRow).toBeVisible({ timeout: 60_000 });
    await offlineRow.scrollIntoViewIfNeeded();

    // Offline presentation: live metrics are dashed out, but the node keeps
    // its identity and management affordances.
    await expect(offlineRow).toContainText("Offline");
    await expect(offlineRow).toContainText("—");
    await expect(offlineRow).not.toContainText("71°C");
    await expect(offlineRow).not.toContainText("28d 0h");
    await expect(
      offlineRow.getByRole("link", {
        name: "Open web interface for Disaster Recovery B",
      }),
    ).toBeVisible();
    await expect(
      offlineRow.getByRole("button", {
        name: "Expand details for Disaster Recovery B",
      }),
    ).toBeVisible();
  });
});
