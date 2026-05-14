import { expect, test as base, type Page } from "@playwright/test";

import {
  apiRequest,
  ensureAuthenticated,
  getMockMode,
  setMockMode,
} from "./helpers";

type APIResource = {
  id: string;
  type?: string;
  name?: string;
  sources?: string[];
  parentId?: string;
  parentName?: string;
  proxmox?: {
    nodeName?: string;
    instance?: string;
    vmid?: number;
  };
};

const test = base;

let mockModeWasEnabled: boolean | null = null;

const isProxmoxResource = (resource: APIResource): boolean =>
  Boolean(resource.proxmox) ||
  (resource.sources ?? []).some((source) => source === "proxmox");

const isProxmoxWorkload = (resource: APIResource): boolean =>
  (resource.type === "vm" || resource.type === "system-container") &&
  isProxmoxResource(resource);

const cssAttrValue = (value: string): string =>
  value.replace(/\\/g, "\\\\").replace(/"/g, '\\"');

async function ensureMockModeEnabled(page: Page): Promise<void> {
  const state = await getMockMode(page);
  if (mockModeWasEnabled === null) {
    mockModeWasEnabled = state.enabled;
  }
  if (!state.enabled) {
    await setMockMode(page, true);
  }
}

async function readResources(page: Page): Promise<APIResource[]> {
  const response = await apiRequest(
    page,
    "/api/resources?type=agent,vm,system-container&page=1&limit=500",
  );
  expect(
    response.ok(),
    `Expected /api/resources to succeed, got ${response.status()}`,
  ).toBe(true);
  const payload = (await response.json()) as { data?: APIResource[] };
  return payload.data ?? [];
}

function buildProxmoxWorkloadRouteId(resource: APIResource): string {
  const instance = (resource.proxmox?.instance ?? "").trim();
  const node = (resource.proxmox?.nodeName ?? "").trim();
  const vmid = resource.proxmox?.vmid ?? 0;
  expect(
    instance,
    `Expected Proxmox workload ${resource.id} to expose an instance`,
  ).not.toBe("");
  expect(
    node,
    `Expected Proxmox workload ${resource.id} to expose a node name`,
  ).not.toBe("");
  expect(
    vmid,
    `Expected Proxmox workload ${resource.id} to expose a VMID`,
  ).toBeGreaterThan(0);
  return `${instance}:${node}:${vmid}`;
}

function buildProxmoxNodeFilterValue(resource: APIResource): string {
  const instance = (resource.proxmox?.instance ?? "").trim();
  const node = (resource.proxmox?.nodeName ?? "").trim();
  expect(
    instance,
    `Expected Proxmox workload ${resource.id} to expose an instance`,
  ).not.toBe("");
  expect(
    node,
    `Expected Proxmox workload ${resource.id} to expose a node name`,
  ).not.toBe("");
  return `${instance}-${node}`;
}

function isWorkloadsResourcesRequest(requestUrl: URL): boolean {
  if (requestUrl.pathname !== "/api/resources") return false;
  const typeParam = requestUrl.searchParams.get("type") ?? "";
  return (
    typeParam.includes("vm") &&
    typeParam.includes("system-container") &&
    typeParam.includes("app-container")
  );
}

async function assertSurfaceDoesNotBlank(
  page: Page,
  testId: string,
  durationMs: number,
): Promise<void> {
  const result = await page.evaluate(
    async ({ durationMs: sampleDurationMs, testId: targetTestId }) => {
      const failures: Array<{
        elapsedMs: number;
        bodyText: string;
        rootChildren: number;
      }> = [];
      const startedAt = performance.now();
      let samples = 0;

      while (performance.now() - startedAt < sampleDurationMs) {
        const root = document.getElementById("root");
        const target = document.querySelector<HTMLElement>(
          `[data-testid="${targetTestId}"]`,
        );
        const rect = target?.getBoundingClientRect();
        const targetVisible = Boolean(
          rect && rect.width > 0 && rect.height > 0,
        );
        const rootChildren = root?.childElementCount ?? 0;

        if (!root || rootChildren === 0 || !target || !targetVisible) {
          failures.push({
            elapsedMs: Math.round(performance.now() - startedAt),
            bodyText: (document.body?.innerText ?? "")
              .replace(/\s+/g, " ")
              .trim()
              .slice(0, 120),
            rootChildren,
          });
        }

        samples += 1;
        await new Promise((resolve) => window.setTimeout(resolve, 250));
      }

      return { failures, samples };
    },
    { durationMs, testId },
  );

  expect(
    result.samples,
    "Expected browser stability sampling to run",
  ).toBeGreaterThan(0);
  expect(
    result.failures,
    `Expected ${testId} to stay mounted and visible without a blank route flash`,
  ).toEqual([]);
}

test.describe.serial("Infrastructure and workloads resource coherence", () => {
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

  test("keeps a Proxmox workload visible from its Infrastructure parent through scoped Workloads routing", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop runtime proof",
    );

    await ensureAuthenticated(page);
    await ensureMockModeEnabled(page);

    const resources = await readResources(page);
    const agents = resources.filter((resource) => resource.type === "agent");
    const workload = resources.find(
      (resource) =>
        isProxmoxWorkload(resource) &&
        Boolean(resource.parentId) &&
        Boolean(resource.proxmox?.nodeName) &&
        Boolean(resource.proxmox?.instance) &&
        Boolean(resource.proxmox?.vmid),
    );

    expect(
      workload,
      "Expected mock/runtime data to include a parented Proxmox workload",
    ).toBeDefined();
    const selectedWorkload = workload as APIResource;
    const parent = agents.find(
      (resource) => resource.id === selectedWorkload.parentId,
    );

    expect(
      parent,
      `Expected Proxmox workload ${selectedWorkload.id} to resolve to one Infrastructure parent`,
    ).toBeDefined();

    const selectedParent = parent as APIResource;
    const duplicateParentIds = agents.filter(
      (resource) => resource.id === selectedParent.id,
    );
    expect(
      duplicateParentIds,
      `Expected parent ${selectedParent.id} to appear once in the resource API`,
    ).toHaveLength(1);

    const parentName = (
      selectedWorkload.parentName ||
      selectedParent.name ||
      selectedParent.id
    ).trim();
    const nodeName = (selectedWorkload.proxmox?.nodeName ?? "").trim();
    const workloadRouteId = buildProxmoxWorkloadRouteId(selectedWorkload);
    const nodeFilterValue = buildProxmoxNodeFilterValue(selectedWorkload);

    await page.goto(
      `/infrastructure?source=proxmox-pve&q=${encodeURIComponent(nodeName)}`,
      {
        waitUntil: "domcontentloaded",
      },
    );

    await expect(page.getByTestId("infrastructure-page")).toBeVisible();
    await expect(
      page.getByTestId("infrastructure-table-surface"),
    ).toContainText(parentName);

    const parentRows = page.locator(
      `[data-testid="infrastructure-table-surface"] [data-summary-series-id="${cssAttrValue(selectedParent.id)}"]`,
    );
    await expect(
      parentRows,
      `Expected Infrastructure to render parent ${parentName} exactly once`,
    ).toHaveCount(1);

    await assertSurfaceDoesNotBlank(
      page,
      "infrastructure-table-surface",
      3_000,
    );
    await expect(
      parentRows,
      `Expected Infrastructure parent ${parentName} to remain visible`,
    ).toHaveCount(1);

    await page.goto(
      `/workloads?type=${encodeURIComponent(selectedWorkload.type ?? "")}` +
        `&platform=proxmox-pve&agent=${encodeURIComponent(nodeFilterValue)}` +
        `&resource=${encodeURIComponent(workloadRouteId)}`,
      { waitUntil: "domcontentloaded" },
    );

    await expect(page.getByTestId("workloads-table-surface")).toBeVisible();
    await expect
      .poll(() => new URL(page.url()).searchParams.get("platform"))
      .toBe("proxmox-pve");
    await expect
      .poll(() => new URL(page.url()).searchParams.get("agent"))
      .toBe(nodeFilterValue);
    await expect(
      page.getByRole("button", { name: /Platform:\s*PVE/ }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: new RegExp(`Node:\\s*${nodeName}`) }),
    ).toBeVisible();

    const workloadRows = page.locator(
      `tr[data-guest-id="${cssAttrValue(workloadRouteId)}"]`,
    );
    await expect(
      workloadRows,
      `Expected Workloads to render ${selectedWorkload.name || workloadRouteId} from parent ${parentName}`,
    ).toHaveCount(1);

    await assertSurfaceDoesNotBlank(page, "workloads-table-surface", 5_000);
    await expect(workloadRows).toHaveCount(1);
  });

  test("keeps Workloads mounted with the last good inventory when resource refresh fails", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop runtime proof",
    );

    await ensureAuthenticated(page);
    await ensureMockModeEnabled(page);

    let workloadResourceRequests = 0;
    await page.route("**/api/resources**", async (route) => {
      const requestUrl = new URL(route.request().url());
      if (!isWorkloadsResourcesRequest(requestUrl)) {
        await route.continue();
        return;
      }

      workloadResourceRequests += 1;
      if (workloadResourceRequests === 1) {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 503,
        contentType: "application/json",
        body: JSON.stringify({
          error: "simulated transient workload inventory gap",
        }),
      });
    });

    await page.goto("/workloads", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("workloads-table-surface")).toBeVisible();

    const firstWorkloadRow = page.locator("tr[data-guest-id]").first();
    await expect(firstWorkloadRow).toBeVisible();
    const firstGuestId = await firstWorkloadRow.getAttribute("data-guest-id");
    expect(firstGuestId, "Expected first workload row to expose a stable guest id").toBeTruthy();

    await expect
      .poll(() => workloadResourceRequests, {
        message: "Expected the Workloads resource poll to retry after the first render",
        timeout: 8_000,
      })
      .toBeGreaterThanOrEqual(2);

    await assertSurfaceDoesNotBlank(page, "workloads-table-surface", 2_000);
    await expect(
      page.locator(`tr[data-guest-id="${cssAttrValue(firstGuestId ?? "")}"]`),
    ).toHaveCount(1);
    await expect(page.getByText("Loading view...")).toHaveCount(0);
    await expect(page.getByRole("heading", { name: "Welcome to Pulse" })).toHaveCount(0);
  });
});
