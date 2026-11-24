import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base, type Page } from "@playwright/test";

import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Standalone VMs is a staged feature controlled by frontend-modern/src/utils/featureReleases.json.
// This spec verifies both sides of the release switch:
//
//   - the production build under test hides /standalone/vms and its Machines entry points
//     whenever the release list holds the feature back
//   - a development-marked page (featureReleaseTestBuild: true on /api/version, never sent
//     by any server) exercises the full /standalone/vms workflows
const featureReleases = JSON.parse(
  fs.readFileSync(
    path.resolve(
      __dirname,
      "..",
      "..",
      "..",
      "frontend-modern",
      "src",
      "utils",
      "featureReleases.json",
    ),
    "utf8",
  ),
) as Record<string, { stage: string; guides: string[] }>;
const STANDALONE_VMS_STAGE = featureReleases.standaloneVms?.stage;

// Must match FEATURE_RELEASE_TEST_BUILD_FIELD in frontend-modern/src/utils/featureReleases.ts.
const TEST_BUILD_FIELD = "featureReleaseTestBuild";

const VM_TABLE = '[data-testid="standalone-vms-table"]';
const VM_ROW = `${VM_TABLE} [data-standalone-vm-row]`;
const DETAIL_ROW = "[data-inline-detail-row]";

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
        `standalone-vms-${workerInfo.project.name}.json`,
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

async function asDevelopmentBuild(page: Page) {
  await page.route("**/api/version", async (route) => {
    const response = await route.fetch();
    const body = await response.json();
    await route.fulfill({
      response,
      json: { ...body, [TEST_BUILD_FIELD]: true },
    });
  });
}

async function waitForStandaloneVms(page: Page) {
  await expect(page.locator(VM_ROW).first()).toBeVisible({ timeout: 60_000 });
}

const isPhoneProject = (name: string) => name.startsWith("mobile-");

test.describe("Standalone VMs", () => {
  test.setTimeout(120_000);

  test("the build under test exposes standalone VMs only when the release list allows it", async ({
    page,
  }, testInfo) => {
    expect(["build", "preview", "released"]).toContain(STANDALONE_VMS_STAGE);

    const version = (await (
      await page.request.get("/api/version")
    ).json()) as Record<string, unknown>;
    expect(version[TEST_BUILD_FIELD]).toBeUndefined();

    const shown =
      STANDALONE_VMS_STAGE === "released" ||
      (STANDALONE_VMS_STAGE === "preview" && version.channel === "rc");

    if (shown) {
      await page.goto("/standalone/vms", { waitUntil: "domcontentloaded" });
      await waitForStandaloneVms(page);
      const previewTag = page
        .locator('[data-testid="standalone-page"]')
        .getByText("Preview", { exact: true });
      await expect(previewTag).toHaveCount(
        STANDALONE_VMS_STAGE === "preview" ? 1 : 0,
      );
      return;
    }

    // Held back in this build: direct route returns Page Not Found.
    await page.goto("/standalone/vms", { waitUntil: "domcontentloaded" });
    await expect(
      page.getByRole("heading", { level: 1, name: "Page Not Found" }),
    ).toBeVisible({ timeout: 60_000 });
    await expect(page.locator(VM_TABLE)).toHaveCount(0);

    // Machines hides the VMs tab and host VM count links.
    await page.goto("/standalone/machines", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("standalone-page")).toBeVisible({
      timeout: 60_000,
    });
    await expect(page.locator('a[href="/standalone/vms"]')).toHaveCount(0);
    await expect(
      page.locator("button", { hasText: /\b\d+\s+VMs?\b/i }),
    ).toHaveCount(0);

    if (!isPhoneProject(testInfo.project.name)) {
      await page.keyboard.press("Control+k");
      const dialog = page.getByRole("dialog", { name: "Command palette" });
      await expect(dialog).toBeVisible({ timeout: 10_000 });
      await dialog.locator("input").fill("standalone/vms");
      await expect(
        dialog.locator("#command-palette-results button"),
      ).toHaveCount(0);
      await page.keyboard.press("Escape");
      await expect(dialog).toBeHidden();
    }
  });

  test("a development build filters, host-scopes, searches, and opens/closes VM details by keyboard", async ({
    page,
  }, testInfo) => {
    test.skip(
      isPhoneProject(testInfo.project.name),
      "Desktop pointer and keyboard journey. Mobile layout is covered below.",
    );

    await asDevelopmentBuild(page);
    await page.goto("/standalone/machines", { waitUntil: "domcontentloaded" });

    // VMs tab and host VM counts appear on Machines in a development build.
    const vmsTab = page.locator('a[href="/standalone/vms"]');
    await expect(vmsTab).toBeVisible({ timeout: 60_000 });

    const hostVmButton = page
      .locator("button", { hasText: /\b\d+\s+VMs?\b/i })
      .first();
    await expect(hostVmButton).toBeVisible({ timeout: 60_000 });
    await hostVmButton.click();

    await expect(page).toHaveURL(/\/standalone\/vms\?.*host=/);
    await waitForStandaloneVms(page);
    const scopedCount = await page.locator(VM_ROW).count();
    expect(scopedCount).toBeGreaterThan(0);

    // Clearing host scope restores all standalone VMs and removes the host param.
    const showAllHosts = page.getByRole("button", { name: "Show all hosts" });
    await expect(showAllHosts).toBeVisible();
    await showAllHosts.click();
    await expect(page).toHaveURL(/\/standalone\/vms$/);
    await expect
      .poll(() => page.locator(VM_ROW).count(), { timeout: 15_000 })
      .toBeGreaterThan(scopedCount);
    const totalCount = await page.locator(VM_ROW).count();

    // Status filter pills persist through URL and reload.
    const attentionPill = page.getByRole("button", { name: /^Attention/i });
    await attentionPill.click();
    await expect(page).toHaveURL(/status=attention/);
    await expect(attentionPill).toHaveAttribute("aria-pressed", "true");
    const attentionCount = await page.locator(VM_ROW).count();
    expect(attentionCount).toBeGreaterThan(0);
    expect(attentionCount).toBeLessThan(totalCount);

    await page.reload({ waitUntil: "domcontentloaded" });
    await waitForStandaloneVms(page);
    await expect(
      page.getByRole("button", { name: /^Attention/i }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(await page.locator(VM_ROW).count()).toBe(attentionCount);

    // Search with no matches shows empty filter state and Reset filters recovers.
    const searchInput = page.getByPlaceholder(/Search VMs/i);
    await searchInput.fill("no-matching-standalone-vm-999");
    await expect(page.getByText("No VMs match current filters")).toBeVisible();
    await page.getByRole("button", { name: "Reset filters" }).click();
    await expect(page).toHaveURL(/\/standalone\/vms$/);
    await expect
      .poll(() => page.locator(VM_ROW).count(), { timeout: 15_000 })
      .toBe(totalCount);

    // Keyboard open and Escape dismiss restore focus to the row disclosure button.
    const firstDisclosure = page
      .locator(VM_ROW)
      .first()
      .locator("button[aria-controls]");
    const controlsId = (await firstDisclosure.getAttribute("aria-controls")) ?? "";
    expect(controlsId).toBeTruthy();

    await firstDisclosure.focus();
    await page.keyboard.press("Enter");
    await expect(firstDisclosure).toHaveAttribute("aria-expanded", "true");
    const detailRow = page.locator(DETAIL_ROW);
    await expect(detailRow).toBeVisible();

    const firstTab = detailRow.getByRole("tab").first();
    await firstTab.focus();
    await page.keyboard.press("Escape");
    await expect(detailRow).toHaveCount(0);
    await expect(firstDisclosure).toHaveAttribute("aria-expanded", "false");
    await expect
      .poll(() =>
        page.evaluate(
          () => document.activeElement?.getAttribute("aria-controls") ?? "",
        ),
      )
      .toBe(controlsId);
  });

  test("a development build renders cleanly on a mobile viewport without horizontal overflow", async ({
    page,
  }, testInfo) => {
    if (!isPhoneProject(testInfo.project.name)) {
      await page.setViewportSize({ width: 390, height: 844 });
    }

    await asDevelopmentBuild(page);
    await page.goto("/standalone/vms", { waitUntil: "domcontentloaded" });
    await waitForStandaloneVms(page);

    const layout = await page.evaluate(() => ({
      docOverflow:
        document.documentElement.scrollWidth - window.innerWidth,
      rows: document.querySelectorAll("[data-standalone-vm-row]").length,
    }));
    expect(layout.rows).toBeGreaterThan(0);
    expect(layout.docOverflow).toBeLessThanOrEqual(1);

    // Tapping or clicking a row opens its inline detail drawer without sideways overflow.
    const firstRow = page.locator(VM_ROW).first();
    if (isPhoneProject(testInfo.project.name)) {
      await firstRow.tap();
    } else {
      await firstRow.click();
    }
    await expect(page.locator(DETAIL_ROW)).toBeVisible();
    const drawerOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth - window.innerWidth,
    );
    expect(drawerOverflow).toBeLessThanOrEqual(1);

    const collapseBtn = page
      .locator(DETAIL_ROW)
      .getByRole("button", { name: /^Collapse /i });
    if (isPhoneProject(testInfo.project.name)) {
      await collapseBtn.tap();
    } else {
      await collapseBtn.click();
    }
    await expect(page.locator(DETAIL_ROW)).toHaveCount(0);
  });
});
