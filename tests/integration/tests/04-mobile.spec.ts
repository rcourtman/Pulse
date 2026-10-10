import { test, expect, devices } from "@playwright/test";
import {
  ensureAuthenticated,
  setMockMode,
  waitForDefaultMockRuntimeReady,
} from "./helpers";

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
  "/actions",
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

// Runs in the page against one inline drawer section and lists every value
// that is cut: content crossing a clipping box, or a value cell that ends in
// an ellipsis. The drawer is where a truncated table cell sends the reader
// for the full value, so neither is allowed there.
const findCutDrawerValues = (section: Element): string[] => {
  // Below lg the inline drawer's content shell clips anything past it.
  let shell: HTMLElement | null = section.parentElement;
  while (shell && window.getComputedStyle(shell).overflowX !== "clip") {
    shell = shell.parentElement;
  }
  if (!shell) return ["no clipping drawer content shell"];

  const offenders: string[] = [];
  const walker = document.createTreeWalker(
    section,
    NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT,
  );
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    let rect: DOMRect;
    let inlineLevel: boolean;
    if (node.nodeType === Node.TEXT_NODE) {
      if (!node.textContent?.trim()) continue;
      const range = document.createRange();
      range.selectNodeContents(node);
      rect = range.getBoundingClientRect();
      inlineLevel = true;
    } else {
      rect = (node as Element).getBoundingClientRect();
      inlineLevel = window
        .getComputedStyle(node as Element)
        .display.startsWith("inline");
    }
    const left = rect.left;
    let right = rect.right;
    // Inline content that truncates with an ellipsis ends inside the block
    // whose line it sits on; the value-cell check below reports that case.
    // The fixed-layout detail cell also declares an ellipsis, but that only
    // marks its own line: a block child such as a chip row crosses the cell
    // edge with no ellipsis and is silently cut.
    if (inlineLevel) {
      let line = node.parentElement;
      while (
        line &&
        line !== shell &&
        ["inline", "contents"].includes(window.getComputedStyle(line).display)
      ) {
        line = line.parentElement;
      }
      const lineStyle = line ? window.getComputedStyle(line) : null;
      if (
        line &&
        lineStyle?.textOverflow === "ellipsis" &&
        lineStyle.overflowX !== "visible" &&
        ["block", "inline-block", "table-cell", "list-item", "flow-root"].includes(
          lineStyle.display,
        )
      ) {
        right = Math.min(right, line.getBoundingClientRect().right);
      }
    }
    // Any clipping box on the way up (a chip, the detail cell, the table
    // wrapper, the drawer shell) hides whatever still crosses either edge.
    for (
      let box: HTMLElement | null = node.parentElement;
      box;
      box = box === shell ? null : box.parentElement
    ) {
      const style = window.getComputedStyle(box);
      if (style.overflowX === "visible" || style.display === "contents") {
        continue;
      }
      const edges = box.getBoundingClientRect();
      const past = Math.max(right - edges.right, edges.left - left);
      if (past > 1) {
        offenders.push(
          `${node.textContent?.trim().slice(0, 60)} (${Math.round(past)}px past ${box.tagName.toLowerCase()})`,
        );
        break;
      }
    }
  }

  for (const cell of Array.from(section.querySelectorAll("tr > td:last-child"))) {
    for (const element of [cell, ...Array.from(cell.querySelectorAll("*"))]) {
      const style = window.getComputedStyle(element);
      if (
        style.textOverflow === "ellipsis" &&
        style.overflowX !== "visible" &&
        element.scrollWidth > element.clientWidth + 1
      ) {
        offenders.push(
          `${element.textContent?.trim().slice(0, 60)} (ends in an ellipsis)`,
        );
        break;
      }
    }
  }
  return offenders;
};

test.describe("Mobile viewport flows", () => {
  test.beforeEach(async ({ page }) => {
    await ensureAuthenticated(page);
  });

  test("bottom nav bar is visible on mobile", async ({ page }) => {
    await page.goto("/infrastructure");
    await expect(page.locator("#root")).toBeVisible();

    const bottomNav = page.getByRole("navigation", { name: "Mobile navigation" });

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

    const nav = page.getByRole("navigation", { name: "Mobile navigation" });
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
    test.setTimeout(240_000);
    await setMockMode(page, true);
    await page.goto("/proxmox/workloads");

    const table = page.locator("table.workload-table--mobile");
    await expect(table).toBeVisible({ timeout: 30_000 });
    await expect(table).toHaveClass(/min-w-\[0px\]/);
    await expect(table.locator("xpath=..")).toHaveClass(/overflow-x-auto/);
  });

  test("utility destinations stay pinned in the fixed rail", async ({
    page,
  }) => {
    await page.goto("/proxmox/overview");

    // The mobile rail no longer scrolls a platform strip: one fixed rail holds
    // the platform switcher, the pinned utility destinations, and the More
    // trigger for everything else, so the whole rail must fit the viewport.
    const rail = page.locator('[data-mobile-nav-rail="fixed"]');
    await expect(rail).toBeVisible({ timeout: 30_000 });

    await expect
      .poll(
        () =>
          page.evaluate(() => {
            const element = document.querySelector<HTMLElement>(
              '[data-mobile-nav-rail="fixed"]',
            );
            if (!element) return null;
            return element.scrollWidth <= element.clientWidth + 1;
          }),
        { timeout: 30_000 },
      )
      .toBe(true);

    const viewportWidth = await getViewportWidth(page);
    for (const tabId of ["platform-switcher", "alerts", "ai", "more"]) {
      const destination = rail.locator(`[data-tab-id="${tabId}"]`);
      await expect(destination).toBeVisible();
      let box = await destination.boundingBox();
      await expect
        .poll(async () => {
          box = await destination.boundingBox();
          return box;
        })
        .not.toBeNull();
      expect(
        box,
        `${tabId} destination should have a layout box`,
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

  test("Drawer identity values show in full inside the drawer instead of being cut", async ({
    page,
  }) => {
    await waitForDefaultMockRuntimeReady(page);
    const requireRows = ["1", "true", "yes", "on"].includes(
      String(process.env.PULSE_E2E_REQUIRE_DEFAULT_MOCK_READY || "")
        .trim()
        .toLowerCase(),
    );

    // Identity values differ at their ends. Container drawers carry generated
    // IDs (app-container:<host>-<hash>), image tags and long label chips;
    // pods from one ReplicaSet differ only in their name suffix, primary ID
    // hash and the parent cluster ID. A phone cannot hover a title to read
    // the rest, so the drawer must show each value whole.
    const drawers = [
      {
        route: "/docker",
        row: "tr[data-docker-container-row]",
        sections: [
          "resource-identity-section",
          "resource-docker-container-section",
        ],
        identityLabel: "Aliases",
      },
      {
        route: "/kubernetes",
        row: "tr[data-kubernetes-pod-row]",
        sections: ["resource-identity-section"],
        identityLabel: "Parent",
      },
    ];
    let checked = 0;
    for (const drawer of drawers) {
      await page.goto(drawer.route);
      const row = page.locator(drawer.row).first();
      if (requireRows) {
        // CI runs the default mock estate, which always has containers and pods.
        await expect(row).toBeVisible({ timeout: 15_000 });
      } else {
        const rowVisible = await row
          .waitFor({ state: "visible", timeout: 15_000 })
          .then(
            () => true,
            () => false,
          );
        if (!rowVisible) continue;
      }
      // On phones the whole row is the disclosure target.
      await row.locator("td").first().click();
      await expect(
        row.getByRole("button", { name: /^Collapse details for / }),
      ).toHaveAttribute("aria-expanded", "true");
      if (requireRows) {
        // The default mock's first container and pod carry these long rows.
        await expect(
          page
            .locator('[data-testid="resource-identity-section"]')
            .first()
            .getByText(drawer.identityLabel, { exact: true }),
        ).toBeVisible();
      }

      for (const testId of drawer.sections) {
        const section = page.locator(`[data-testid="${testId}"]`).first();
        await expect(section).toBeVisible();
        const cutOff = await section.evaluate(findCutDrawerValues);
        expect(cutOff, `${drawer.route} ${testId}`).toEqual([]);
      }
      checked += 1;
    }
    if (checked === 0) {
      test.skip(true, "No container or pod rows available to expand");
    }
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

    const nav = page.getByRole("navigation", { name: "Mobile navigation" });
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
