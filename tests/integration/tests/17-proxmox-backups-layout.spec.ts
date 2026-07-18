import { expect, test } from "@playwright/test";
import { ensureAuthenticated } from "./helpers";

const DESKTOP_VIEWPORT = { width: 1440, height: 900 };

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
    await page.goto("/proxmox/backups", { waitUntil: "domcontentloaded" });

    // The guest-centric Coverage view is the default whenever anything needs
    // attention; the day-activity strip lives in the By date view.
    await page.getByRole("button", { name: "By date" }).click();
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
    await expect(page).toHaveURL(/\/proxmox\/backups/);
    await expect(page.getByText(/^\d+ of \d+ backups$/).first()).toBeVisible();
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
    await page.goto("/proxmox/backups", { waitUntil: "domcontentloaded" });

    await page.getByRole("button", { name: "By date" }).click();

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
  });
});
