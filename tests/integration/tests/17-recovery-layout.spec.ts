import { expect, test, type Page } from "@playwright/test";

import { ensureAuthenticated } from "./helpers";

const DESKTOP_VIEWPORT = { width: 1280, height: 900 };
const RECOVERY_SUBJECT_LABEL = "Archive VM For Production Ledger Services";
const RECOVERY_DAY_MS = 24 * 60 * 60 * 1000;

const recoveryUtcDateKey = (date: Date): string => {
  const year = date.getUTCFullYear();
  const month = String(date.getUTCMonth() + 1).padStart(2, "0");
  const day = String(date.getUTCDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
};

const recoveryTransportDay = (date: Date, tzOffsetMinutes: number): number => {
  const shifted = new Date(date.getTime() + tzOffsetMinutes * 60 * 1000);
  return Date.UTC(
    shifted.getUTCFullYear(),
    shifted.getUTCMonth(),
    shifted.getUTCDate(),
  );
};

const buildRecoverySeries = (url: string) => {
  const requestUrl = new URL(url);
  const from = requestUrl.searchParams.get("from");
  const to = requestUrl.searchParams.get("to");
  if (!from || !to) return mockRecoveryData.series;

  const fromDate = new Date(from);
  const toDate = new Date(to);
  const parsedTzOffset = Number.parseInt(
    requestUrl.searchParams.get("tzOffsetMinutes") || "0",
    10,
  );
  const tzOffsetMinutes = Number.isFinite(parsedTzOffset)
    ? parsedTzOffset
    : 0;
  const fromDay = recoveryTransportDay(fromDate, tzOffsetMinutes);
  const toDay = recoveryTransportDay(toDate, tzOffsetMinutes);
  const daySpan = Math.floor((toDay - fromDay) / RECOVERY_DAY_MS) + 1;
  if (!Number.isFinite(daySpan) || daySpan < 1) return mockRecoveryData.series;

  const bucketCount = Math.min(daySpan, 365);

  return {
    data: Array.from({ length: bucketCount }, (_, index) => {
      const pointDate = new Date(fromDay + index * RECOVERY_DAY_MS);
      const day = recoveryUtcDateKey(pointDate);
      const remote =
        index === 0 ||
        index === bucketCount - 1 ||
        (bucketCount > 30 && index % 29 === 0) ||
        (bucketCount > 14 && index > bucketCount - 8)
          ? 1
          : 0;
      return {
        day,
        total: remote,
        snapshot: 0,
        local: 0,
        remote,
      };
    }),
  };
};

const mockRecoveryData = {
  rollups: {
    data: [
      {
        rollupId: "res:vm-archive-01",
        subjectResourceId: "vm-archive-01",
        subjectRef: {
          type: "proxmox-vm",
          namespace: "prod",
          name: "archive-ledger",
          id: "vm-archive-01",
          class: "cluster-a",
        },
        display: {
          subjectLabel: RECOVERY_SUBJECT_LABEL,
        },
        lastAttemptAt: "2026-03-24T04:03:04Z",
        lastSuccessAt: "2026-03-24T04:03:04Z",
        lastOutcome: "success",
        providers: ["proxmox-pbs"],
      },
    ],
    meta: { page: 1, limit: 500, total: 1, totalPages: 1 },
  },
  points: {
    data: [
      {
        id: "pbs-backup:archive-ledger-01",
        provider: "proxmox-pbs",
        kind: "backup",
        mode: "remote",
        outcome: "success",
        startedAt: "2026-03-24T04:02:12Z",
        completedAt: "2026-03-24T04:03:04Z",
        sizeBytes: 30546730222,
        verified: true,
        immutable: false,
        encrypted: true,
        entityId: "201",
        subjectResourceId: "vm-archive-01",
        subjectRef: {
          type: "proxmox-vm",
          namespace: "prod",
          name: "archive-ledger",
          id: "vm-archive-01",
          class: "cluster-a",
        },
        repositoryRef: {
          type: "proxmox-pbs-datastore",
          namespace: "pbs-prod",
          name: "vault-main",
          class: "cluster-a",
        },
        details: {
          summary:
            "Nightly immutable backup retained for compliance validation.",
        },
        display: {
          subjectLabel: RECOVERY_SUBJECT_LABEL,
          subjectType: "proxmox-vm",
          repositoryLabel: "pbs-prod/vault-main",
          detailsSummary:
            "Nightly immutable backup retained for compliance validation.",
        },
      },
    ],
    meta: { page: 1, limit: 500, total: 1, totalPages: 1 },
  },
  facets: {
    data: {
      clusters: [],
      nodesHosts: [],
      namespaces: [],
      hasSize: true,
      hasVerification: true,
      hasEntityId: false,
    },
  },
  series: {
    data: [{ day: "2026-03-24", total: 1, snapshot: 0, local: 0, remote: 1 }],
  },
};

async function mockRecoveryEndpoints(page: Page): Promise<void> {
  await page.route("**/api/recovery/rollups*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(mockRecoveryData.rollups),
    });
  });

  await page.route("**/api/recovery/points*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(mockRecoveryData.points),
    });
  });

  await page.route("**/api/recovery/facets*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(mockRecoveryData.facets),
    });
  });

  await page.route("**/api/recovery/series*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(buildRecoverySeries(route.request().url())),
    });
  });
}

test.describe("Recovery desktop layout guards", () => {
  test("history table keeps outcome visible within the desktop wrapper", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only recovery layout coverage",
    );

    await page.setViewportSize(DESKTOP_VIEWPORT);
    await ensureAuthenticated(page);
    await mockRecoveryEndpoints(page);

    await page.goto("/recovery", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("recovery-page")).toBeVisible();

    const protectedRow = page
      .locator('[data-testid="recovery-page"] table tbody tr')
      .first();
    await expect(protectedRow).toBeVisible();
    await protectedRow.click();

    await expect(
      page.getByTestId("recovery-history-item-filter-trigger"),
    ).toContainText(RECOVERY_SUBJECT_LABEL);
    await expect(
      page.getByText(/Showing 1 - 1 of 1 recovery points/i),
    ).toBeVisible();

    const historyWrapper = page
      .locator('[data-testid="recovery-page"] div.overflow-x-auto')
      .filter({ has: page.locator("table") })
      .last();
    await expect(historyWrapper).toBeVisible();

    const overflowMetrics = await historyWrapper.evaluate((el) => {
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

    expect(["auto", "scroll"]).toContain(overflowMetrics.overflowX);
    expect(
      overflowMetrics.wrapperScrollWidth,
      `Recovery history wrapper should fit the default desktop column set without horizontal scrolling (wrapper=${overflowMetrics.wrapperClientWidth}, table=${overflowMetrics.tableScrollWidth})`,
    ).toBeLessThanOrEqual(overflowMetrics.wrapperClientWidth + 1);

    const outcomeHeader = historyWrapper
      .locator("th")
      .filter({ hasText: /^Outcome$/ })
      .first();
    await expect(outcomeHeader).toBeVisible();

    const wrapperBox = await historyWrapper.boundingBox();
    const outcomeBox = await outcomeHeader.boundingBox();

    expect(wrapperBox, "Expected recovery history wrapper bounds").toBeTruthy();
    expect(outcomeBox, "Expected outcome column header bounds").toBeTruthy();

    const wrapperRight =
      (wrapperBox as { x: number; width: number }).x +
      (wrapperBox as { width: number }).width;
    const outcomeRight =
      (outcomeBox as { x: number; width: number }).x +
      (outcomeBox as { width: number }).width;
    expect(
      outcomeRight,
      `Recovery outcome header should stay inside the visible desktop wrapper (wrapperRight=${wrapperRight}, outcomeRight=${outcomeRight})`,
    ).toBeLessThanOrEqual(wrapperRight + 1);
  });

  test("long-range activity timeline stays readable and keeps selected day in the route", async ({
    page,
  }, testInfo) => {
    const seriesRequests: string[] = [];

    if (!testInfo.project.name.startsWith("mobile-")) {
      await page.setViewportSize(DESKTOP_VIEWPORT);
    }
    await ensureAuthenticated(page);
    await mockRecoveryEndpoints(page);
    await page.route("**/api/recovery/series*", async (route) => {
      seriesRequests.push(route.request().url());
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(buildRecoverySeries(route.request().url())),
      });
    });

    await page.goto("/recovery?view=events&range=365&node=pve-archive-01", {
      waitUntil: "domcontentloaded",
    });
    await expect(page.getByTestId("recovery-page")).toBeVisible();

    const chartScroll = page.getByTestId("recovery-activity-chart-scroll");
    const bars = page.getByTestId("recovery-activity-bars");
    await expect(chartScroll).toBeVisible();
    await expect(bars).toBeVisible();

    const chartMetrics = await chartScroll.evaluate((element) => {
      const scroller = element as HTMLElement;
      const labels = Array.from(
        scroller.querySelectorAll(".pointer-events-none span"),
      ).map((label) => {
        const rect = (label as HTMLElement).getBoundingClientRect();
        return { left: rect.left, right: rect.right, text: label.textContent };
      });
      const overlaps = labels.some((label, index) => {
        const next = labels[index + 1];
        return next ? label.right > next.left + 1 : false;
      });
      return {
        clientWidth: scroller.clientWidth,
        scrollWidth: scroller.scrollWidth,
        labelCount: labels.length,
        overlaps,
        firstLabel: labels[0]?.text || "",
        lastLabel: labels.at(-1)?.text || "",
      };
    });

    expect(chartMetrics.scrollWidth).toBeGreaterThan(chartMetrics.clientWidth);
    expect(chartMetrics.labelCount).toBeGreaterThan(0);
    expect(chartMetrics.labelCount).toBeLessThanOrEqual(16);
    expect(chartMetrics.overlaps).toBe(false);
    expect(chartMetrics.firstLabel).toBeTruthy();
    expect(chartMetrics.lastLabel).toBeTruthy();

    await expect.poll(() => seriesRequests.length).toBeGreaterThan(0);
    expect(
      seriesRequests.some(
        (url) =>
          url.includes("/api/recovery/series") &&
          url.includes("node=pve-archive-01") &&
          url.includes("tzOffsetMinutes="),
      ),
    ).toBe(true);

    const firstTimelineButton = bars.getByRole("button").first();
    await firstTimelineButton.click();
    await expect(page).toHaveURL(/\/recovery\?.*day=\d{4}-\d{2}-\d{2}/);

    await page.goto("/recovery?view=events&range=7&node=pve-archive-01", {
      waitUntil: "domcontentloaded",
    });
    await expect(page.getByTestId("recovery-page")).toBeVisible();
    await expect.poll(() => chartScroll.locator("button").count()).toBe(7);

    const shortRangeMetrics = await chartScroll.evaluate((element) => {
      const scroller = element as HTMLElement;
      const labels = Array.from(
        scroller.querySelectorAll(".pointer-events-none span"),
      ).map((label) => {
        const rect = (label as HTMLElement).getBoundingClientRect();
        return { left: rect.left, right: rect.right, text: label.textContent };
      });
      const overlaps = labels.some((label, index) => {
        const next = labels[index + 1];
        return next ? label.right > next.left + 1 : false;
      });
      return {
        clientWidth: scroller.clientWidth,
        scrollWidth: scroller.scrollWidth,
        labelCount: labels.length,
        buttonCount: scroller.querySelectorAll("button").length,
        overlaps,
      };
    });

    expect(shortRangeMetrics.scrollWidth).toBeLessThanOrEqual(
      shortRangeMetrics.clientWidth + 1,
    );
    expect(shortRangeMetrics.buttonCount).toBe(7);
    expect(shortRangeMetrics.labelCount).toBe(7);
    expect(shortRangeMetrics.overlaps).toBe(false);
  });
});
