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
    slug: "dashboard",
    route: "/dashboard",
    title: "Dashboard",
    description:
      "Track infrastructure health, active risks, storage pressure, and recovery readiness from one overview.",
  },
  {
    slug: "workloads",
    route: "/workloads",
    title: "Workloads",
    description:
      "Inspect live workloads, filter by platform and status, and drill into compute, memory, and I/O posture.",
  },
  {
    slug: "infrastructure",
    route: "/infrastructure",
    title: "Infrastructure",
    description:
      "Inspect connected resources, filter by source and status, and drill into live health and capacity.",
  },
  {
    slug: "storage",
    route: "/storage",
    title: "Storage",
    description:
      "Review capacity, node health, pools, and storage pressure across connected clusters and devices.",
  },
  {
    slug: "recovery",
    route: "/recovery",
    title: "Recovery",
    description:
      "Review protected inventory, recent recovery activity, and restore posture across platforms.",
  },
  {
    slug: "alerts",
    route: "/alerts/overview",
    title: "Alerts Overview",
    description:
      "Review active incidents, confirm alert coverage, and control whether alerts are actively monitoring this install.",
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
    description:
      "Continuously verify infrastructure health, review findings, and control Patrol runtime behavior.",
  },
] as const;

const ALIGNED_PAGE_HEADER_ROUTES = [
  PAGE_HEADER_ROUTES[0],
  PAGE_HEADER_ROUTES[1],
  PAGE_HEADER_ROUTES[2],
  PAGE_HEADER_ROUTES[3],
  PAGE_HEADER_ROUTES[4],
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
      ).toBeVisible();
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

  test("keeps primary page headings vertically aligned", async ({ page }) => {
    let baselineY: number | null = null;

    for (const surface of ALIGNED_PAGE_HEADER_ROUTES) {
      await page.goto(surface.route, { waitUntil: "domcontentloaded" });

      const pageHeading = page.getByRole("heading", {
        level: 1,
        name: surface.title,
      });
      await expect(pageHeading).toBeVisible();

      const boundingBox = await pageHeading.boundingBox();
      expect(
        boundingBox,
        `${surface.route} should expose a measurable heading box`,
      ).not.toBeNull();

      if (baselineY === null) {
        baselineY = boundingBox!.y;
        continue;
      }

      expect(
        Math.abs(boundingBox!.y - baselineY),
        `${surface.route} should keep its top-level heading aligned with the other utility surfaces`,
      ).toBeLessThanOrEqual(1.5);
    }
  });
});
