import { expect, test, type Page } from "@playwright/test";
import { ensureAuthenticated } from "./helpers";

const DESKTOP_VIEWPORT = { width: 1440, height: 900 };

async function openProxmoxBackups(page: Page) {
  const proxmoxTab = page.getByRole("tab", {
    name: "Proxmox",
    exact: true,
  });
  await expect(proxmoxTab).toBeVisible({ timeout: 30_000 });
  await proxmoxTab.click();

  const sections = page.getByRole("navigation", {
    name: "Proxmox sections",
  });
  await expect(sections).toBeVisible({ timeout: 60_000 });
  const backupsLink = sections.getByRole("link", {
    name: "Backups",
    exact: true,
  });
  await expect(backupsLink).toHaveAttribute("href", "/proxmox/backups/date");
  await backupsLink.click();
  await expect(page).toHaveURL(/\/proxmox\/backups\/date$/);
}

// Layout guards for the Proxmox Backups section, which replaced the retired
// standalone /recovery surface. Runs against the mock-mode dataset; counts
// are asserted as shapes, not pinned values.
test.describe("Proxmox backups layout guards", () => {
  test.setTimeout(180_000);

  test("activity day selection filters the backups table in place", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only backups layout coverage",
    );

    await page.setViewportSize(DESKTOP_VIEWPORT);
    await ensureAuthenticated(page);

    // Existing bookmarks remain compatible, but the route is normalized to
    // the canonical route-segment view before the table is used.
    await page.goto("/proxmox/backups", { waitUntil: "domcontentloaded" });
    await expect(page).toHaveURL(/\/proxmox\/backups\/date$/, {
      timeout: 60_000,
    });
    await openProxmoxBackups(page);

    const byDateLink = page.getByRole("link", { name: "By date", exact: true });
    const coverageLink = page.getByRole("link", {
      name: "Coverage",
      exact: true,
    });
    await expect(byDateLink).toHaveAttribute("href", "/proxmox/backups/date");
    await expect(coverageLink).toHaveAttribute(
      "href",
      "/proxmox/backups/coverage",
    );
    await byDateLink.click();
    await expect(page).toHaveURL(/\/proxmox\/backups\/date$/);
    await expect(page.getByText("Backups per day").first()).toBeVisible();
    const dayButtons = page.getByRole("button", { name: /: \d+ backups?$/ });
    await expect.poll(() => dayButtons.count()).toBeGreaterThanOrEqual(7);

    const totalCopy = page.getByText(/^\d+ backups$/).first();
    await expect(totalCopy).toBeVisible();

    // Picking a day narrows the table without navigating away.
    const activeDay = page
      .getByRole("button", { name: /: [1-9]\d* backups?$/ })
      .last();
    await activeDay.click();
    await expect(page).toHaveURL(
      /\/proxmox\/backups\/date\?day=\d{4}-\d{2}-\d{2}$/,
    );
    await expect(page.getByText(/^\d+ of \d+ backups$/).first()).toBeVisible();

    const filteredScreenshotPath = testInfo.outputPath(
      "proxmox-backups-by-date-filtered.png",
    );
    await page.screenshot({ path: filteredScreenshotPath, fullPage: true });
    await testInfo.attach("proxmox-backups-by-date-filtered", {
      path: filteredScreenshotPath,
      contentType: "image/png",
    });
  });

  test("long-range activity keeps the page inside the horizontal viewport", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only backups layout coverage",
    );

    await page.setViewportSize(DESKTOP_VIEWPORT);
    await ensureAuthenticated(page);
    await openProxmoxBackups(page);

    const byDateLink = page.getByRole("link", { name: "By date", exact: true });
    await expect(byDateLink).toHaveAttribute("href", "/proxmox/backups/date");
    await byDateLink.click();
    await expect(page).toHaveURL(/\/proxmox\/backups\/date$/);

    await page
      .getByRole("group", { name: "Activity range" })
      .getByRole("button", { name: "1y" })
      .click();

    const dayButtons = page.getByRole("button", { name: /: \d+ backups?$/ });
    await expect.poll(() => dayButtons.count()).toBe(365);

    // A year of bars must stay contained: the chart may scroll internally but
    // the page itself must not overflow horizontally.
    const pageOverflow = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
    }));
    expect(pageOverflow.scrollWidth).toBeLessThanOrEqual(
      pageOverflow.clientWidth + 1,
    );

    // The PBS servers table keeps its trailing column inside its wrapper on
    // the default desktop column set.
    const serversTable = page
      .locator("div.overflow-x-auto")
      .filter({ has: page.locator('th:has-text("Backup server")') })
      .first();
    await expect(serversTable).toBeVisible();
    const dedupHeader = serversTable
      .locator("th")
      .filter({ hasText: /^Dedup$/ })
      .first();
    await expect(dedupHeader).toBeVisible();

    const wrapperBox = await serversTable.boundingBox();
    const dedupBox = await dedupHeader.boundingBox();
    expect(wrapperBox).toBeTruthy();
    expect(dedupBox).toBeTruthy();
    expect(dedupBox!.x + dedupBox!.width).toBeLessThanOrEqual(
      wrapperBox!.x + wrapperBox!.width + 1,
    );

    const yearScreenshotPath = testInfo.outputPath(
      "proxmox-backups-one-year-layout.png",
    );
    await page.screenshot({ path: yearScreenshotPath, fullPage: true });
    await testInfo.attach("proxmox-backups-one-year-layout", {
      path: yearScreenshotPath,
      contentType: "image/png",
    });
  });
});
