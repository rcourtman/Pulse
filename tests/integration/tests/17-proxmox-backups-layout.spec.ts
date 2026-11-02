import { expect, test, type Page } from "@playwright/test";
import {
  ensureAuthenticated,
  getMockMode,
  primaryNavigationLink,
} from "./helpers";

const DESKTOP_VIEWPORT = { width: 1440, height: 900 };

const truthy = (value: string | undefined) =>
  ["1", "true", "yes", "on"].includes(
    String(value || "")
      .trim()
      .toLowerCase(),
  );

// The guards read the mock-mode estate: PBS servers and a populated backup
// calendar. The Core E2E harness runs mock mode, but `npm run dev:verify`
// attaches to the managed hot-dev runtime in whatever mode it was left in.
// There a real-mode runtime skips the guards instead of switching modes in
// place, because Monitor.SetMockMode clears the runtime's active alerts and
// in-memory history on every switch.
async function skipUnlessManagedRuntimeIsMock(page: Page) {
  if (!truthy(process.env.PULSE_E2E_USE_HOT_DEV)) {
    return;
  }
  const { enabled } = await getMockMode(page);
  test.skip(
    !enabled,
    "Needs the mock-mode dataset; run `npm run mock:on` before `npm run dev:verify`",
  );
}

async function openProxmoxBackups(page: Page) {
  const proxmoxTab = primaryNavigationLink(page, "Proxmox");
  await expect(proxmoxTab).toBeVisible({ timeout: 30_000 });
  await proxmoxTab.click();

  const sections = page.getByRole("navigation", {
    name: "Proxmox sections",
  });
  await expect(sections).toBeVisible({ timeout: 60_000 });
  await sections.getByRole("link", { name: "Backups", exact: true }).click();
  await expect(page).toHaveURL(/\/proxmox\/backups\/date$/);
  // The route commits before the Proxmox snapshot finishes loading. CI can
  // take more than a minute while all E2E shards share the runner, so wait for
  // the destination surface rather than the URL alone.
  await expect(page.getByText(/^\d+ backup entries$/).first()).toBeVisible({
    timeout: 120_000,
  });
}

// Layout guards for the Proxmox Backups section, which replaced the retired
// standalone /recovery surface. Runs against the mock-mode dataset; counts
// are asserted as shapes, not pinned values.
test.describe("Proxmox backups layout guards", () => {
  test.setTimeout(180_000);

  test("the Backups page stays inside the horizontal viewport", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only backups layout coverage",
    );

    await page.setViewportSize(DESKTOP_VIEWPORT);
    await ensureAuthenticated(page);
    await skipUnlessManagedRuntimeIsMock(page);
    await openProxmoxBackups(page);

    // The page itself must not overflow horizontally.
    const pageOverflow = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
    }));
    expect(pageOverflow.scrollWidth).toBeLessThanOrEqual(
      pageOverflow.clientWidth + 1,
    );

    // The PBS servers table keeps its trailing column inside its wrapper on
    // the default desktop column set.
    // Overview remains mounted for fast tab restoration and now owns the same
    // shared PBS table. Scope to the active Backups composition so its hidden
    // Overview sibling cannot satisfy this layout guard first.
    const serversTable = page
      .locator('[data-proxmox-backups-table="servers"]:visible')
      .locator("div.overflow-x-auto");
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
