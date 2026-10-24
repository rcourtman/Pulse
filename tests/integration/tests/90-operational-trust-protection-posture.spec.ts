import { expect, test, type Page } from "@playwright/test";
import { apiRequest, ensureAuthenticated, primaryNavigationLink } from "./helpers";

const DESKTOP_VIEWPORT = { width: 1440, height: 900 };
const POSTURE_BATCH_SIZE = 200;

async function proxmoxWorkloadResourceIDs(page: Page): Promise<string[]> {
  const ids: string[] = [];
  let totalPages = 1;
  let total = 0;
  for (let pageNumber = 1; pageNumber <= totalPages; pageNumber += 1) {
    const params = new URLSearchParams({
      type: "vm,system-container,oci-container",
      source: "proxmox",
      page: String(pageNumber),
      limit: String(POSTURE_BATCH_SIZE),
    });
    const response = await apiRequest(page, `/api/resources?${params}`);
    expect(response.ok(), "The fixture workload inventory must be readable").toBe(true);
    const payload = (await response.json()) as {
      data: { id: string }[];
      meta: { page: number; total: number; totalPages: number };
    };
    expect(payload.meta.page).toBe(pageNumber);
    expect(Number.isInteger(payload.meta.totalPages)).toBe(true);
    expect(payload.meta.totalPages).toBeGreaterThan(0);
    if (pageNumber === 1) {
      totalPages = payload.meta.totalPages;
      total = payload.meta.total;
    }
    expect(payload.meta.totalPages).toBe(totalPages);
    expect(payload.meta.total).toBe(total);
    for (const resource of payload.data) {
      expect(resource.id.trim()).not.toBe("");
      ids.push(resource.id);
    }
  }
  expect(ids.length).toBeGreaterThan(0);
  expect(ids).toHaveLength(total);
  expect(new Set(ids).size, "The inventory must not repeat a workload").toBe(total);
  return ids.sort((left, right) => left.localeCompare(right));
}

async function openDesktopProxmoxBackups(page: Page) {
  const proxmoxTab = primaryNavigationLink(page, "Proxmox");
  await expect(proxmoxTab).toBeVisible({ timeout: 30_000 });
  await proxmoxTab.click();

  const sections = page.getByRole("navigation", {
    name: "Proxmox sections",
  });
  await expect(sections).toBeVisible({ timeout: 60_000 });
  await sections.getByRole("link", { name: "Backups", exact: true }).click();
  await expect(page).toHaveURL(/\/proxmox\/backups\/date$/);
}

test.describe("Operational trust protection posture", () => {
  test.setTimeout(180_000);

  test("uses bounded posture batches and keeps evidence in the workload drill-down", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "The desktop journey proves the full evidence drill-down",
    );

    const postureRequests: string[] = [];
    const postureResponses: Promise<{
      data: { subjectResourceId: string; explanation: string }[];
    }>[] = [];
    page.on("request", (request) => {
      if (request.url().includes("/api/recovery/postures")) {
        postureRequests.push(request.url());
      }
    });
    page.on("response", (response) => {
      if (new URL(response.url()).pathname === "/api/recovery/postures") {
        postureResponses.push(response.json());
      }
    });

    await page.setViewportSize(DESKTOP_VIEWPORT);
    await ensureAuthenticated(page);
    const workloadIDs = await proxmoxWorkloadResourceIDs(page);
    await openDesktopProxmoxBackups(page);
    const coverageLink = page
      .getByRole("navigation", { name: "Backup views" })
      .getByRole("link", { name: "Coverage", exact: true });
    await expect(coverageLink).toBeVisible({ timeout: 60_000 });
    await expect(coverageLink).toHaveAttribute(
      "href",
      "/proxmox/backups/coverage",
    );
    await coverageLink.click();
    await expect(page).toHaveURL(/\/proxmox\/backups\/coverage$/);

    const coverageTable = page
      .locator("div.overflow-x-auto")
      .filter({ has: page.locator('th:has-text("Posture")') })
      .first();
    await expect(coverageTable).toBeVisible();
    // The default mock estate now exceeds one 200-workload batch. Assert the
    // exact complete partition, not one request for every fleet size (or a
    // relaxed upper bound that would allow missing or repeated subjects).
    const expectedBatchCount = Math.ceil(workloadIDs.length / POSTURE_BATCH_SIZE);
    await expect.poll(() => postureRequests.length).toBe(expectedBatchCount);
    const requestedIDs: string[] = [];
    for (const request of postureRequests) {
      const postureURL = new URL(request);
      expect(postureURL.searchParams.get("limit")).toBe(String(POSTURE_BATCH_SIZE));
      const batchIDs = postureURL.searchParams.getAll("resourceId");
      expect(batchIDs.length).toBeGreaterThan(0);
      expect(batchIDs.length).toBeLessThanOrEqual(POSTURE_BATCH_SIZE);
      expect(new Set(batchIDs).size).toBe(batchIDs.length);
      requestedIDs.push(...batchIDs);
    }
    expect(requestedIDs.sort((left, right) => left.localeCompare(right))).toEqual(workloadIDs);

    // The default monitor stays compact. Explanation and provider/restore
    // evidence appear only after the operator asks for the row's details.
    await expect(coverageTable.getByText("Provider evidence")).toHaveCount(0);
    const detailToggle = coverageTable
      .locator('button[aria-label^="Expand details for"]')
      .first();
    await expect(detailToggle).toBeVisible();
    await detailToggle.click();

    const detailRow = coverageTable.locator("[data-inline-detail-for]").first();
    await expect(detailRow).toBeVisible();
    const rowKey = await detailRow.getAttribute("data-inline-detail-for");
    expect(rowKey).toMatch(/^resource:/);
    const subjectID = rowKey!.slice("resource:".length);
    const posture = (await Promise.all(postureResponses))
      .flatMap((response) => response.data)
      .find((entry) => entry.subjectResourceId === subjectID);
    // Explanations are server-owned plain language, not a client-added
    // "Protected:" prefix. Prove that the selected subject's actual response
    // reaches its detail rather than merely accepting any non-empty copy.
    expect(posture?.explanation.trim()).toBeTruthy();
    await expect(detailRow).toContainText(posture!.explanation);
    await expect(detailRow).toContainText(
      /Restore evidence|Restore-point history is not confirmed complete\. Missing entries do not mean no backups exist\.|No restore points are visible in the current list\. Missing entries do not mean no backups exist\.|No restore points reported for this workload in the readable history\./,
    );
  });

  test("contains wide evidence tables on a phone-sized viewport", async ({
    page,
  }, testInfo) => {
    test.skip(
      !testInfo.project.name.startsWith("mobile-"),
      "The mobile projects own responsive containment",
    );

    await ensureAuthenticated(page);
    await page.goto("/proxmox/backups", { waitUntil: "domcontentloaded" });
    const coverageLink = page
      .getByRole("navigation", { name: "Backup views" })
      .getByRole("link", { name: "Coverage", exact: true });
    await expect(coverageLink).toBeVisible({ timeout: 60_000 });
    await expect(coverageLink).toHaveAttribute(
      "href",
      "/proxmox/backups/coverage",
    );
    await coverageLink.click();
    await expect(page).toHaveURL(/\/proxmox\/backups\/coverage$/);
    await expect(page.locator('th:has-text("Posture")').first()).toBeVisible();

    const viewport = await page.evaluate(() => ({
      bodyScrollWidth: document.body.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
    }));
    expect(viewport.bodyScrollWidth).toBeLessThanOrEqual(
      viewport.clientWidth + 1,
    );
  });
});
