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

const unsafeUrlNode = {
  ...offlineNode,
  id: "proxmox:core-fabric:pve6",
  name: "pve6",
  customUrl: "javascript:alert(document.domain)",
  canonicalIdentity: {
    ...offlineNode.canonicalIdentity,
    displayName: "Unsafe Link Node",
    hostname: "pve6",
  },
  proxmox: {
    ...offlineNode.proxmox,
    nodeName: "pve6",
    host: "https://pve6:8006",
  },
};

const asRecord = (value: unknown): Record<string, unknown> =>
  value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};

function buildOfflineNodeFixture(
  resource: Record<string, unknown>,
  nodeName: "pve5" | "pve6",
) {
  const unsafe = nodeName === "pve6";
  const displayName = unsafe ? "Unsafe Link Node" : "Disaster Recovery B";
  const lastSeen = offlineNode.lastSeen;
  const proxmox = asRecord(resource.proxmox);
  const agent = asRecord(resource.agent);
  const sourceStatus = asRecord(resource.sourceStatus);

  return {
    ...resource,
    name: displayName,
    status: "offline",
    lastSeen,
    customUrl: unsafe ? unsafeUrlNode.customUrl : `https://${nodeName}:8006`,
    canonicalIdentity: {
      ...asRecord(resource.canonicalIdentity),
      displayName,
      hostname: nodeName,
      platformId: nodeName,
    },
    sourceStatus: {
      ...sourceStatus,
      agent: { status: "offline", lastSeen },
      proxmox: { status: "offline", lastSeen, error: "unreachable" },
    },
    metrics: offlineNode.metrics,
    uptime: offlineNode.uptime,
    proxmox: {
      ...proxmox,
      nodeName,
      nodeDisplayName: displayName,
      host: `https://${nodeName}:8006`,
      connectionHealth: "error",
      temperature: offlineNode.proxmox.temperature,
      uptime: offlineNode.uptime,
    },
    agent: {
      ...agent,
      hostname: nodeName,
      uptimeSeconds: offlineNode.uptime,
      temperature: offlineNode.proxmox.temperature,
    },
  };
}

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
    // Proxmox preloads the other section-specific resource queries. Returning
    // these agent rows for storage, backup, Ceph, and PMG queries violates the
    // API's type-filter contract and feeds the same identities into multiple
    // reactive caches. Only the overview query owns this fixture.
    const requestedTypes = new Set(
      (requestUrl.searchParams.get("type") ?? "").split(",").filter(Boolean),
    );
    if (
      ["agent", "vm", "system-container", "oci-container"].every((type) =>
        requestedTypes.has(type),
      )
    ) {
      const response = await route.fetch();
      const payload = asRecord(await response.json());
      const data = Array.isArray(payload.data)
        ? payload.data.flatMap((candidate) => {
            const resource = asRecord(candidate);
            const nodeName = asRecord(resource.proxmox).nodeName;
            return nodeName === "pve5" || nodeName === "pve6"
              ? [buildOfflineNodeFixture(resource, nodeName)]
              : [];
          })
        : [];
      await route.fulfill({
        status: response.status(),
        contentType: "application/json",
        body: JSON.stringify({
          ...payload,
          data,
          aggregations: { total: 2, byType: { agent: 2 } },
        }),
      });
      return;
    }
    const data: unknown[] = [];
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data,
        meta: {
          page: 1,
          limit: 100,
          total: data.length,
          totalPages: 1,
        },
        aggregations: {
          total: 2,
          byType: { agent: 2 },
        },
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
  }, testInfo) => {
    await page.context().route("https://**:8006/**", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "text/html",
        body: "<title>Proxmox VE</title>",
      });
    });
    await routeOfflineProxmoxInventory(page);
    await page.goto("/proxmox", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("proxmox-page")).toBeVisible();

    const offlineRow = page
      .locator('[data-testid="proxmox-page"] tr[data-proxmox-host-row]')
      .filter({ hasText: "Disaster Recovery B" })
      .first();
    await expect(offlineRow).toBeVisible({ timeout: 60_000 });
    await offlineRow.scrollIntoViewIfNeeded();

    // Offline presentation: live metrics are dashed out, but the node keeps
    // its identity and management affordances.
    await expect(offlineRow).toContainText("Disaster Recovery B");
    if (!testInfo.project.name.startsWith("mobile-")) {
      await expect(offlineRow).toContainText("Offline");
    }
    await expect(offlineRow).toContainText("—");
    await expect(offlineRow).not.toContainText("71°C");
    await expect(offlineRow).not.toContainText("28d 0h");
    if (!testInfo.project.name.startsWith("mobile-")) {
      const webInterfaceLink = offlineRow.getByRole("link", {
        name: "Open web interface for Disaster Recovery B",
      });
      await expect(webInterfaceLink).toBeVisible();
      await expect(webInterfaceLink).toHaveAttribute(
        "href",
        /^https:\/\/[^/]+:8006$/,
      );
      await expect(webInterfaceLink).toHaveAttribute("target", "_blank");
      await expect(webInterfaceLink).toHaveAttribute(
        "rel",
        "noopener noreferrer",
      );

      const linkBox = await webInterfaceLink.boundingBox();
      expect(
        linkBox,
        "Web-interface control should have a measurable hit target",
      ).not.toBeNull();
      expect(linkBox!.width).toBeGreaterThanOrEqual(24);
      expect(linkBox!.height).toBeGreaterThanOrEqual(24);

      // Opening the adjacent control must not bubble into the selectable row.
      await expect(offlineRow).toHaveAttribute("aria-expanded", "false");
      const webInterfaceUrl = await webInterfaceLink.getAttribute("href");
      expect(webInterfaceUrl).toBeTruthy();
      const popupPromise = page.context().waitForEvent("page");
      await webInterfaceLink.click();
      const popup = await popupPromise;
      await popup.waitForLoadState("domcontentloaded");
      expect(popup.url()).toBe(`${webInterfaceUrl}/`);
      await popup.close();
      await expect(offlineRow).toHaveAttribute("aria-expanded", "false");
    }

    const viewportContainment = await page.evaluate(() => ({
      clientWidth: document.documentElement.clientWidth,
      scrollWidth: document.documentElement.scrollWidth,
    }));
    expect(viewportContainment.scrollWidth).toBeLessThanOrEqual(
      viewportContainment.clientWidth + 1,
    );

    const unsafeRow = page
      .locator('[data-testid="proxmox-page"] tr[data-proxmox-host-row]')
      .filter({ hasText: "Unsafe Link Node" })
      .first();
    await expect(unsafeRow).toBeVisible();
    await expect(
      unsafeRow.getByRole("img", {
        name: "Web interface URL for Unsafe Link Node is invalid",
      }),
    ).toBeAttached();
    await expect(
      unsafeRow.getByRole("link", {
        name: "Open web interface for Unsafe Link Node",
      }),
    ).toHaveCount(0);

    await expect(
      offlineRow.getByRole("button", {
        name: "Expand details for Disaster Recovery B",
      }),
    ).toBeVisible();
  });
});
