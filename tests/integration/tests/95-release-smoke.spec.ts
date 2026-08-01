import { expect, test, type Page } from "@playwright/test";
import { ensureAuthenticated } from "./helpers";

const DESKTOP_VIEWPORT = { width: 1440, height: 900 };

// Release smoke: the minimum bar every cut ships against, prereleases
// included. Each check asserts only that a primary surface renders real
// inventory from the mock dataset — no interaction sequences, no timing
// races — so a failure here always means the surface is broken, never that
// the test is flaky. v6.2.0-rc.5 shipped with an empty Proxmox workloads
// table and a crashing Thresholds page (#1663) while exactly these
// assertions were failing in non-gating tiers; this spec exists so that
// class of regression blocks the release instead.
//
// Keep this file interaction-free and assertion-light. Anything richer
// belongs in the numbered feature specs, not here.

async function expectNoErrorBoundary(page: Page) {
  await expect(
    page.getByText("This page couldn't load"),
    "an error boundary fired on a release-gating surface",
  ).toHaveCount(0);
}

test.describe("Release smoke", () => {
  test.setTimeout(180_000);

  test.beforeEach(async ({ page }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Release smoke gates on the desktop shell",
    );
    await page.setViewportSize(DESKTOP_VIEWPORT);
    await ensureAuthenticated(page);
  });

  test("Proxmox overview renders nodes and workloads", async ({ page }) => {
    await page.goto("/proxmox", { waitUntil: "domcontentloaded" });

    // Nodes and workloads travel different data paths (websocket state vs
    // /api/resources); rc.5 broke exactly one of them, so assert both.
    const nodeRows = page.locator("[data-proxmox-host-row]");
    await expect(nodeRows.first()).toBeVisible({ timeout: 60_000 });

    const workloadRows = page.locator("tr[data-guest-id]");
    await expect(workloadRows.first()).toBeVisible({ timeout: 60_000 });

    await expect(page.getByText("No Proxmox workloads")).toHaveCount(0);
    await expectNoErrorBoundary(page);
  });

  test("Docker page renders hosts and containers", async ({ page }) => {
    await page.goto("/docker", { waitUntil: "domcontentloaded" });

    await expect(
      page.locator("tr[data-docker-host-row]").first(),
    ).toBeVisible({ timeout: 60_000 });
    await expect(
      page.locator("tr[data-docker-container-row]").first(),
    ).toBeVisible({ timeout: 60_000 });
    await expectNoErrorBoundary(page);
  });

  test("Kubernetes page renders clusters and pods", async ({ page }) => {
    await page.goto("/kubernetes", { waitUntil: "domcontentloaded" });

    await expect(
      page.locator("tr[data-kubernetes-cluster-row]").first(),
    ).toBeVisible({ timeout: 60_000 });
    await expect(
      page.locator("tr[data-kubernetes-pod-row]").first(),
    ).toBeVisible({ timeout: 60_000 });
    await expectNoErrorBoundary(page);
  });

  test("Alert thresholds page renders guest sections", async ({ page }) => {
    await page.goto("/alerts/thresholds", { waitUntil: "domcontentloaded" });

    // The mock dataset always contains a freshly provisioned zero-used
    // guest filesystem (ensureFreshFilesystemFixture), so this render
    // covers the omitted-zero-numerics wire shape that crashed rc.5.
    await expect(
      page.getByText("Virtualization Hosts", { exact: false }).first(),
    ).toBeVisible({ timeout: 60_000 });
    await expectNoErrorBoundary(page);
  });
});
