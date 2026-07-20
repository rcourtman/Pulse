import { test, expect, devices } from "@playwright/test";
import { ensureAuthenticated } from "./helpers";

const getViewportWidth = async (
  page: import("@playwright/test").Page,
): Promise<number> => {
  const size = page.viewportSize();
  if (size) return size.width;
  return await page.evaluate(() => window.innerWidth);
};

const MOBILE_VIEWPORTS = [
  { width: 320, height: 568, label: "compact phone" },
  { width: 390, height: 844, label: "modern phone" },
] as const;

const CANONICAL_MOBILE_SURFACE_ROUTES = [
  "/",
  "/preview/setup-complete",
  "/route-that-does-not-exist",
  "/proxmox/overview",
  "/proxmox/storage",
  "/proxmox/replication",
  "/proxmox/backups",
  "/proxmox/ceph",
  "/proxmox/mail",
  "/docker/overview",
  "/docker/images",
  "/docker/storage",
  "/docker/networks",
  "/docker/swarm",
  "/kubernetes/overview",
  "/kubernetes/nodes",
  "/kubernetes/workloads",
  "/kubernetes/services",
  "/kubernetes/storage",
  "/kubernetes/configuration",
  "/kubernetes/events",
  "/truenas/overview",
  "/truenas/storage",
  "/truenas/services",
  "/truenas/apps",
  "/truenas/vms",
  "/truenas/shares",
  "/truenas/protection",
  "/vmware/overview",
  "/vmware/storage",
  "/vmware/networks",
  "/vmware/health",
  "/vmware/activity",
  "/standalone/machines",
  "/standalone/availability",
  "/alerts/overview",
  "/alerts/thresholds",
  "/alerts/notifications",
  "/alerts/schedule",
  "/alerts/history",
  "/patrol",
] as const;

const gotoMobileRoute = async (
  page: import("@playwright/test").Page,
  route: string,
): Promise<void> => {
  let lastError: unknown;
  for (let attempt = 0; attempt < 3; attempt += 1) {
    try {
      await page.goto(route, { waitUntil: "domcontentloaded" });
      return;
    } catch (error) {
      lastError = error;
      await page.waitForTimeout(250 * (attempt + 1));
    }
  }
  throw lastError;
};

test.describe("Mobile viewport flows", () => {
  test.beforeEach(async ({ page }) => {
    await ensureAuthenticated(page);
  });

  test("bottom nav bar is visible on mobile", async ({ page }) => {
    await page.goto("/infrastructure");
    await expect(page.locator("#root")).toBeVisible();

    const bottomNav = page.getByRole("tablist", { name: "Mobile navigation" });

    // Wait for the nav to mount before evaluating (evaluateAll does not auto-wait;
    // WebKit can be slower to render SolidJS components than Chromium).
    await bottomNav.first().waitFor({ state: "attached", timeout: 10000 });

    const visibleCount = await bottomNav.evaluateAll((els) => {
      const isVisible = (el: Element) => {
        const style = window.getComputedStyle(el as HTMLElement);
        if (
          style.display === "none" ||
          style.visibility === "hidden" ||
          style.opacity === "0"
        )
          return false;
        const rect = (el as HTMLElement).getBoundingClientRect();
        return rect.width > 0 && rect.height > 0;
      };
      return els.filter(isVisible).length;
    });

    expect(
      visibleCount,
      "Expected a visible mobile bottom nav",
    ).toBeGreaterThan(0);
  });

  test("MobileNavBar has safe-area padding on nav", async ({ page }) => {
    await page.goto("/infrastructure");
    await expect(page.locator("#root")).toBeVisible();

    const nav = page.getByRole("tablist", { name: "Mobile navigation" });
    await expect(nav).toBeVisible();

    // Verify the safe-area CSS class is applied to the nav. The computed padding-bottom
    // value is 0 in headless Chromium (no notch), but the pb-safe class must be present.
    const hasSafeClass = await nav.evaluate((el: HTMLElement) =>
      el.classList.contains("pb-safe"),
    );
    expect(
      hasSafeClass,
      "Expected nav to have pb-safe class for safe-area-inset-bottom",
    ).toBeTruthy();
  });

  test("Platform filter bar does not overflow horizontally", async ({
    page,
  }) => {
    await page.goto("/docker/overview");

    await expect(page.getByPlaceholder("Search containers")).toBeVisible();

    // On mobile the full filter controls are hidden behind a toggle; only the
    // search bar + Filters button row should be visible. Check the overall page
    // body does not overflow horizontally.
    const viewportWidth = await getViewportWidth(page);
    const bodyScrollWidth = await page.evaluate(
      () => document.body.scrollWidth,
    );
    expect(
      bodyScrollWidth,
      "Platform page body must not overflow horizontally",
    ).toBeLessThanOrEqual(viewportWidth + 1);
  });

  test("platform section navigation stays on one scrollable row and reveals the active tab", async ({
    page,
  }) => {
    await page.goto("/truenas/protection");

    const navigation = page.getByRole("navigation", {
      name: "TrueNAS sections",
    });
    await expect(navigation).toBeVisible({ timeout: 30_000 });
    const activeTab = navigation.getByRole("link", { name: "Protection" });
    await expect(activeTab).toHaveAttribute("aria-current", "page");

    const geometry = await navigation.evaluate((element) => {
      const active = element.querySelector<HTMLElement>(
        '[aria-current="page"]',
      );
      const navBox = element.getBoundingClientRect();
      const activeBox = active?.getBoundingClientRect();
      return {
        height: navBox.height,
        clientWidth: element.clientWidth,
        scrollWidth: element.scrollWidth,
        overflowX: window.getComputedStyle(element).overflowX,
        activeLeft: activeBox?.left ?? -1,
        activeRight: activeBox?.right ?? Number.POSITIVE_INFINITY,
        navLeft: navBox.left,
        navRight: navBox.right,
      };
    });

    expect(["auto", "scroll"]).toContain(geometry.overflowX);
    expect(geometry.scrollWidth).toBeGreaterThan(geometry.clientWidth);
    expect(geometry.height).toBeLessThanOrEqual(42);
    expect(geometry.activeLeft).toBeGreaterThanOrEqual(geometry.navLeft - 1);
    expect(geometry.activeRight).toBeLessThanOrEqual(geometry.navRight + 1);
  });

  test("shared Workloads table preserves its mobile width and scroll contract", async ({
    page,
  }) => {
    await page.goto("/proxmox/workloads");

    const table = page.locator("table.workload-table--mobile");
    await expect(table).toBeVisible({ timeout: 30_000 });
    await expect(table).toHaveClass(/min-w-\[36rem\]/);
    await expect(table.locator("xpath=..")).toHaveClass(/overflow-x-auto/);
  });

  test("utility destinations stay pinned beside the scrollable platform rail", async ({
    page,
  }) => {
    await page.goto("/proxmox/overview");

    const primaryRail = page.locator('[data-mobile-nav-rail="primary"]');
    const utilityRail = page.locator('[data-mobile-nav-rail="utility"]');
    await expect(primaryRail).toBeVisible({ timeout: 30_000 });
    await expect(utilityRail).toBeVisible({ timeout: 30_000 });

    await expect
      .poll(
        () =>
          page.evaluate(() => {
            const element = document.querySelector<HTMLElement>(
              '[data-mobile-nav-rail="primary"]',
            );
            if (!element) return null;
            return {
              hasScrollableOverflow: ["auto", "scroll"].includes(
                window.getComputedStyle(element).overflowX,
              ),
              preservesRailWidth: element.scrollWidth >= element.clientWidth,
            };
          }),
        { timeout: 30_000 },
      )
      .toEqual({
        hasScrollableOverflow: true,
        preservesRailWidth: true,
      });

    const viewportWidth = await getViewportWidth(page);
    for (const tabId of ["alerts", "ai", "settings"]) {
      const button = utilityRail.locator(`button[data-tab-id="${tabId}"]`);
      await expect(button).toBeVisible();
      const box = await button.boundingBox();
      expect(
        box,
        `${tabId} utility destination should have a layout box`,
      ).toBeTruthy();
      expect((box?.x ?? 0) + (box?.width ?? 0)).toBeLessThanOrEqual(
        viewportWidth + 1,
      );
    }
  });

  test("canonical product routes keep mobile overflow contained and dense tables readable", async ({
    page,
  }) => {
    test.setTimeout(240_000);

    for (const viewport of MOBILE_VIEWPORTS) {
      await page.setViewportSize(viewport);

      for (const route of CANONICAL_MOBILE_SURFACE_ROUTES) {
        await gotoMobileRoute(page, route);
        await expect(
          page.locator("#root"),
          `${route} should render the app surface on a ${viewport.label}`,
        ).toBeVisible();

        const viewportWidth = await getViewportWidth(page);
        const layout = await page.evaluate(() => ({
          bodyWidth: document.body.scrollWidth,
          documentWidth: document.documentElement.scrollWidth,
        }));
        expect(
          Math.max(layout.bodyWidth, layout.documentWidth),
          `${route} must contain horizontal overflow inside the owning control or table shell on a ${viewport.label}`,
        ).toBeLessThanOrEqual(viewportWidth + 1);

        const denseTables = await page
          .locator("#root div.overflow-x-auto")
          .filter({ has: page.locator("table") })
          .evaluateAll((wrappers) =>
            wrappers
              .map((wrapper) => {
                const table = wrapper.querySelector("table");
                const headerCount =
                  table?.querySelectorAll("thead th").length ?? 0;
                return {
                  headerCount,
                  tableWidth: table?.scrollWidth ?? 0,
                  wrapperWidth: wrapper.clientWidth,
                };
              })
              .filter((table) => table.headerCount >= 4),
          );

        for (const table of denseTables) {
          expect(
            table.tableWidth,
            `${route} should scroll dense tables instead of compressing their columns on a ${viewport.label}`,
          ).toBeGreaterThan(table.wrapperWidth);
        }
      }
    }
  });

  test("Platform table wrapper enables horizontal overflow when needed", async ({
    page,
  }) => {
    await page.goto("/proxmox/overview");

    const tableWrapper = page
      .locator("div.overflow-x-auto")
      .filter({ has: page.locator("table") })
      .first();
    const wrapperVisible = await tableWrapper.isVisible().catch(() => false);
    if (!wrapperVisible) {
      test.skip(
        true,
        "No platform table rendered (no resources or table not present)",
      );
    }

    const overflowBehavior = await tableWrapper.evaluate((el) => {
      const wrapper = el as HTMLElement;
      const table = wrapper.querySelector("table") as HTMLElement | null;
      const style = window.getComputedStyle(wrapper);
      return {
        overflowX: style.overflowX,
        wrapperClientWidth: wrapper.clientWidth,
        wrapperScrollWidth: wrapper.scrollWidth,
        tableScrollWidth: table?.scrollWidth ?? 0,
      };
    });

    // In v6 some datasets fit cleanly on mobile after column/layout optimizations.
    // The contract is that the wrapper is configured to allow horizontal scrolling
    // if content exceeds available width.
    expect(["auto", "scroll"]).toContain(overflowBehavior.overflowX);
    expect(overflowBehavior.wrapperClientWidth).toBeGreaterThan(0);
    expect(overflowBehavior.tableScrollWidth).toBeGreaterThan(0);
  });

  test("Tapping a resource disclosure opens its mobile detail state", async ({
    page,
  }) => {
    await page.goto("/proxmox/overview");

    const disclosure = page
      .getByRole("button", { name: /^Expand details for / })
      .first();
    const disclosureVisible = await disclosure.isVisible().catch(() => false);
    if (!disclosureVisible) {
      test.skip(true, "No resource rows available to expand");
    }

    const expandedLabel = (
      await disclosure.getAttribute("aria-label")
    )?.replace(/^Expand /, "Collapse ");
    expect(expandedLabel).toBeTruthy();
    const disclosureBox = await disclosure.boundingBox();
    expect(disclosureBox?.width ?? 0).toBeGreaterThanOrEqual(40);
    expect(disclosureBox?.height ?? 0).toBeGreaterThanOrEqual(40);
    await disclosure.click();

    await expect(
      page.getByRole("button", { name: expandedLabel! }),
    ).toBeVisible();

    const detailCellId = await page
      .getByRole("button", { name: expandedLabel! })
      .getAttribute("aria-controls");
    expect(detailCellId).toBeTruthy();
    const detailContent = page.locator(`[id="${detailCellId}"] > div`).first();
    await expect(detailContent).toBeVisible();
    const detailGeometry = await detailContent.evaluate((element) => {
      let scrollContainer: HTMLElement | null = element.parentElement;
      while (scrollContainer) {
        const overflowX = window.getComputedStyle(scrollContainer).overflowX;
        if (overflowX === "auto" || overflowX === "scroll") break;
        scrollContainer = scrollContainer.parentElement;
      }
      const detailBox = element.getBoundingClientRect();
      const wrapperBox = scrollContainer?.getBoundingClientRect();
      return {
        detailLeft: detailBox.left,
        detailRight: detailBox.right,
        detailWidth: detailBox.width,
        wrapperLeft: wrapperBox?.left ?? 0,
        wrapperRight: wrapperBox?.right ?? 0,
        wrapperWidth: wrapperBox?.width ?? 0,
      };
    });
    expect(detailGeometry.detailWidth).toBeLessThanOrEqual(
      detailGeometry.wrapperWidth + 1,
    );
    expect(detailGeometry.detailLeft).toBeGreaterThanOrEqual(
      detailGeometry.wrapperLeft - 1,
    );
    expect(detailGeometry.detailRight).toBeLessThanOrEqual(
      detailGeometry.wrapperRight + 1,
    );
  });

  test("Infrastructure landing loads without horizontal overflow at mobile viewport", async ({
    page,
  }) => {
    await page.goto("/infrastructure");
    await expect(page.locator("#root")).toBeVisible();

    const viewportWidth = await getViewportWidth(page);
    const bodyScrollWidth = await page.evaluate(
      () => document.body.scrollWidth,
    );
    expect(bodyScrollWidth).toBeLessThanOrEqual(viewportWidth + 1);
  });

  test("AI assistant button is visible above nav bar", async ({ page }) => {
    await page.goto("/infrastructure");
    await expect(page.locator("#root")).toBeVisible();

    const nav = page.getByRole("tablist", { name: "Mobile navigation" });
    await expect(nav).toBeVisible();

    const aiButton = page.getByRole("button", {
      name: /Ask Pulse Assistant about/,
    });
    await expect(aiButton).toBeVisible();

    const navBox = await nav.boundingBox();
    const aiBox = await aiButton.boundingBox();

    expect(navBox, "Expected nav bounding box").toBeTruthy();
    expect(aiBox, "Expected AI button bounding box").toBeTruthy();

    const navTop = (navBox as { y: number }).y;
    const aiBottom =
      (aiBox as { y: number; height: number }).y +
      (aiBox as { height: number }).height;
    expect(aiBottom).toBeLessThanOrEqual(navTop + 1);
  });
});
