import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test as base, expect } from "@playwright/test";
import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

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
        `truenas-disk-history-${workerInfo.project.name}.json`,
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

// TrueNAS disk history moved from the retired /storage route into the
// TrueNAS storage section's resource drawer. The mock dataset ships sdc on
// truenas-main with SMART failures, and the metrics store serves the disk
// series under both the serial and the disk:<node>:<device> composite key,
// which is what protects serial-less disks. (Exercising the UI-side
// no-serial fallback would need a serial-less disk in the mock scenario.)
test.describe("TrueNAS storage disk history", () => {
  test.setTimeout(180_000);

  test("separates shareable volume and physical-disk scopes", async ({
    page,
  }, testInfo) => {
    await page.goto("/truenas/storage", { waitUntil: "domcontentloaded" });
    if (testInfo.project.name.startsWith("mobile-")) {
      await page.getByRole("button", { name: "Filters" }).click();
    }
    const storageType = page.getByRole("group", { name: "Storage type" });
    await expect(storageType).toBeVisible({ timeout: 60_000 });

    await storageType.getByRole("button", { name: /^Physical disks,/ }).click();
    await expect(page).toHaveURL(/\/truenas\/storage\?kind=disks$/);
    await expect(
      storageType.getByRole("button", { name: /^Physical disks,/ }),
    ).toHaveAttribute("aria-pressed", "true");
    await expect(
      page.locator('tr[data-truenas-storage-kind="disk"]').first(),
    ).toBeVisible();
    const headers = page.locator("thead");
    await expect(headers).toContainText("Endurance");
    await expect(headers).toContainText("Temp");
    await expect(headers).toContainText(/Health|H/);
    await expect(
      page.locator('tr[data-truenas-storage-kind="pool"]'),
    ).toHaveCount(0);
    await testInfo.attach(`truenas-physical-disks-${testInfo.project.name}`, {
      body: await page.screenshot({ fullPage: true }),
      contentType: "image/png",
    });

    await page.reload({ waitUntil: "domcontentloaded" });
    if (testInfo.project.name.startsWith("mobile-")) {
      await page.getByRole("button", { name: /Filters/ }).click();
    }
    await expect(
      page
        .getByRole("group", { name: "Storage type" })
        .getByRole("button", { name: /^Physical disks,/ }),
    ).toHaveAttribute("aria-pressed", "true");

    await page
      .getByRole("group", { name: "Storage type" })
      .getByRole("button", { name: /^Volumes,/ })
      .click();
    await expect(page).toHaveURL(/\/truenas\/storage\?kind=volumes$/);
    await expect(
      page.locator('tr[data-truenas-storage-kind="pool"]').first(),
    ).toBeVisible();
    await expect(
      page.locator('tr[data-truenas-storage-kind="disk"]'),
    ).toHaveCount(0);
  });

  test("serves SMART temperature history from the storage drawer", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only storage drawer coverage",
    );

    const historyRequests: string[] = [];
    await page.route("**/api/metrics-store/history**", async (route) => {
      historyRequests.push(new URL(route.request().url()).search);
      await route.continue();
    });

    await page.goto("/truenas/storage", { waitUntil: "domcontentloaded" });
    const search = page.getByPlaceholder(
      "Search TrueNAS pools, datasets, or disks",
    );
    await expect(search).toBeVisible({ timeout: 60_000 });
    await search.fill("sdc");

    const diskRow = page.locator("tr").filter({ hasText: "sdc" }).first();
    await expect(diskRow).toBeVisible();
    await diskRow.getByRole("button").first().click();

    const drawer = page.getByRole("region", { name: "sdc" });
    await expect(drawer).toBeVisible();
    await expect(
      drawer.getByText(/Device \/dev\/sdc has SMART test/).first(),
    ).toBeVisible();

    await drawer.getByRole("tab", { name: "History" }).click();

    await expect
      .poll(() =>
        historyRequests.some((query) => {
          const params = new URLSearchParams(query);
          return params.get("resourceType") === "disk";
        }),
      )
      .toBe(true);

    const diskQuery = historyRequests.find((query) => {
      const params = new URLSearchParams(query);
      return params.get("resourceType") === "disk";
    })!;
    const resourceId = new URLSearchParams(diskQuery).get("resourceId") ?? "";
    // Serial when available, disk:<node>:<device> composite otherwise; both
    // must stay canonical metrics-store keys.
    expect(resourceId).toMatch(/^(WD-|disk:truenas-main:)/);
  });
});
