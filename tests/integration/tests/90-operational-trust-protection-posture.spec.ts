import { expect, test } from "@playwright/test";
import { ensureAuthenticated } from "./helpers";

const DESKTOP_VIEWPORT = { width: 1440, height: 900 };

test.describe("Operational trust protection posture", () => {
  test.setTimeout(180_000);

  test("uses one bounded posture query and keeps evidence in the workload drill-down", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "The desktop journey proves the full evidence drill-down",
    );

    const postureRequests: string[] = [];
    page.on("request", (request) => {
      if (request.url().includes("/api/recovery/postures")) {
        postureRequests.push(request.url());
      }
    });

    await page.setViewportSize(DESKTOP_VIEWPORT);
    await ensureAuthenticated(page);
    await page.goto("/proxmox/backups", { waitUntil: "domcontentloaded" });
    const coverageButton = page.getByRole("button", { name: "Coverage" });
    await expect(coverageButton).toBeVisible({ timeout: 60_000 });
    await coverageButton.click();

    const coverageTable = page
      .locator("div.overflow-x-auto")
      .filter({ has: page.locator('th:has-text("Posture")') })
      .first();
    await expect(coverageTable).toBeVisible();
    await expect.poll(() => postureRequests.length).toBe(1);

    const postureURL = new URL(postureRequests[0]);
    const requestedResourceIDs = postureURL.searchParams.getAll("resourceId");
    expect(requestedResourceIDs.length).toBeGreaterThan(0);
    expect(requestedResourceIDs.length).toBeLessThanOrEqual(200);

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
    await expect(detailRow).toContainText(
      /Protected:|Attention:|Unprotected:|Unknown:/,
    );
    await expect(detailRow).toContainText(
      /Restore evidence|No restore evidence has been discovered/,
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
    const coverageButton = page.getByRole("button", { name: "Coverage" });
    await expect(coverageButton).toBeVisible({ timeout: 60_000 });
    await coverageButton.click();
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
