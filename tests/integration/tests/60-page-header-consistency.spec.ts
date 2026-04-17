import { test, expect } from "@playwright/test";
import { readRuntimeState } from "./runtime-defaults";

const PAGE_HEADER_ROUTES = [
  {
    slug: "operations",
    route: "/operations",
    title: "Operations",
    description:
      "Run diagnostics, review generated reports, and inspect system logs without leaving the app.",
  },
  {
    slug: "alerts",
    route: "/alerts/overview",
    title: "Alerts Overview",
    description:
      "Review active incidents, confirm alert coverage, and control whether alerts are actively monitoring this install.",
  },
  {
    slug: "patrol",
    route: "/patrol",
    title: "Patrol",
    description:
      "Continuously verify infrastructure health, review findings, and control Patrol runtime behavior.",
  },
] as const;

const PRIMARY_API_TOKEN =
  process.env.PULSE_E2E_PRIMARY_API_TOKEN?.trim() ||
  (typeof readRuntimeState()?.primaryAPIToken === "string"
    ? readRuntimeState()!.primaryAPIToken!.trim()
    : "");

test.skip(
  PRIMARY_API_TOKEN === "",
  "Top-level header browser proof requires a runtime API token.",
);

test.describe("Top-level page header consistency", () => {
  test.setTimeout(180_000);

  test.beforeEach(async ({ page }) => {
    await page.addInitScript((token: string) => {
      sessionStorage.setItem(
        "pulse_auth",
        JSON.stringify({
          type: "token",
          value: token,
        }),
      );
      sessionStorage.setItem("pulse_auth_user", "admin");
      localStorage.setItem("pulse_whats_new_v2_shown", "true");
    }, PRIMARY_API_TOKEN);
  });

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
});
