import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base } from "@playwright/test";

import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const PAGE_HEADER_ROUTES = [
  {
    slug: "alerts",
    route: "/alerts/overview",
    title: "Alerts Overview",
    description:
      "Review active incidents and confirm current alert coverage across monitored resources.",
  },
  {
    slug: "settings",
    route: "/settings/system-general",
    title: "General",
    description: "Manage appearance, layout, and default monitoring cadence.",
  },
  {
    slug: "patrol",
    route: "/patrol",
    title: "Patrol",
    description: "Patrol checks your infrastructure and shows current issues.",
  },
] as const;

// The platform pages intentionally carry no page header (the platform tabs
// and section nav frame them), so header alignment is pinned across the
// utility surfaces that still render the shared PageHeader.
const ALIGNED_PAGE_HEADER_ROUTES = [
  PAGE_HEADER_ROUTES[0],
  PAGE_HEADER_ROUTES[2],
] as const;

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
        `page-header-consistency-${workerInfo.project.name}.json`,
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

test.describe("Top-level page header consistency", () => {
  test.setTimeout(180_000);

  for (const surface of PAGE_HEADER_ROUTES) {
    test(`renders the canonical header framing on ${surface.route}`, async ({
      page,
    }, testInfo) => {
      await page.goto(surface.route, { waitUntil: "domcontentloaded" });

      const pageHeading = page.getByRole("heading", {
        level: 1,
        name: surface.title,
      });
      await expect(
        pageHeading,
        `${surface.route} should render a single top-level heading`,
      ).toBeVisible();
      await expect(
        page.getByText(surface.description, { exact: true }).first(),
        `${surface.route} should render the canonical subheader copy`,
      ).toBeAttached();
      await expect(
        page.locator("h1"),
        `${surface.route} should not render duplicate page headings`,
      ).toHaveCount(1);

      const screenshotPath = testInfo.outputPath(`${surface.slug}.png`);
      await page.screenshot({ path: screenshotPath });
      console.log(
        `[page-header-consistency] screenshot ${surface.slug}: ${screenshotPath}`,
      );
    });
  }

  test("keeps primary page headings vertically aligned", async ({ page }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Phone headers use responsive framing and intentionally do not share desktop Y coordinates",
    );

    let baselineContentOffsetY: number | null = null;

    for (const surface of ALIGNED_PAGE_HEADER_ROUTES) {
      await page.goto(surface.route, { waitUntil: "domcontentloaded" });

      const pageHeading = page.getByRole("heading", {
        level: 1,
        name: surface.title,
      });
      await expect(pageHeading).toBeVisible();

      const headerFrame = page.locator("[data-page-header]").first();
      await expect(headerFrame).toBeVisible();
      const contentFrame = page.locator("#main .pulse-panel").first();
      await expect(contentFrame).toBeVisible();

      const [headerBoundingBox, contentBoundingBox] = await Promise.all([
        headerFrame.boundingBox(),
        contentFrame.boundingBox(),
      ]);
      expect(
        headerBoundingBox,
        `${surface.route} should expose a measurable canonical header frame`,
      ).not.toBeNull();
      expect(
        contentBoundingBox,
        `${surface.route} should expose a measurable canonical content frame`,
      ).not.toBeNull();

      // Shell-level banners can appear asynchronously between route visits.
      // Measure the shared PageHeader from the shared content frame so the
      // route contract stays strict without coupling it to transient chrome.
      const contentOffsetY = headerBoundingBox!.y - contentBoundingBox!.y;

      if (baselineContentOffsetY === null) {
        baselineContentOffsetY = contentOffsetY;
        continue;
      }

      expect(
        Math.abs(contentOffsetY - baselineContentOffsetY),
        `${surface.route} should keep its top-level heading aligned with the other utility surfaces`,
      ).toBeLessThanOrEqual(1.5);
    }
  });
});
