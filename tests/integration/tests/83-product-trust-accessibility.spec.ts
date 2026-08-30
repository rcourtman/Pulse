import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import AxeBuilder from "@axe-core/playwright";
import { expect, test as base, type Page } from "@playwright/test";
import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const WCAG_TAGS = [
  "wcag2a",
  "wcag2aa",
  "wcag21a",
  "wcag21aa",
  "wcag22aa",
] as const;

const scanForWcagViolations = async (page: Page) => {
  const results = await new AxeBuilder({ page })
    .withTags([...WCAG_TAGS])
    .analyze();

  return results.violations.map((violation) => ({
    id: violation.id,
    impact: violation.impact,
    help: violation.help,
    targets: violation.nodes.map((node) => node.target.join(" ")),
  }));
};

type WorkerFixtures = { authStorageStatePath: string };
const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) =>
    use(authStorageStatePath),
  authStorageStatePath: [
    async ({ browser }, use, workerInfo) => {
      const storageStatePath = path.resolve(
        __dirname,
        "..",
        "..",
        "tmp",
        "playwright-auth",
        `product-trust-a11y-${workerInfo.project.name}.json`,
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

test("Actions remains named, directly reachable, keyboard accessible, and free of horizontal overflow at phone width", async ({
  page,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.route("**/api/actions?*", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ view: "pending", actions: [], count: 0 }),
    }),
  );
  await page.goto("/actions", { waitUntil: "domcontentloaded" });
  await expect(
    page.getByRole("heading", { name: "Actions", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("tablist", { name: "Action views" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Refresh actions" }),
  ).toBeVisible();
  await expect(page.locator('[data-tab-id="actions"]')).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(page.locator('[data-tab-id="ai"]')).not.toHaveAttribute(
    "aria-current",
    "page",
  );
  expect(await scanForWcagViolations(page)).toEqual([]);
  const overflow = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth,
  );
  expect(overflow).toBeFalsy();
  await page.keyboard.press("Tab");
  await expect(page.locator(":focus")).toBeVisible();
  await testInfo.attach("actions-phone-width", {
    body: await page.screenshot(),
    contentType: "image/png",
  });
});

test("representative authenticated surfaces have no automatically detectable WCAG A/AA violations", async ({
  page,
}) => {
  const surfaces = [
    { route: "/alerts/overview", heading: "Alerts Overview" },
    { route: "/settings/system-general", heading: "General" },
    { route: "/patrol", heading: "Patrol" },
  ] as const;

  for (const surface of surfaces) {
    await page.goto(surface.route, { waitUntil: "domcontentloaded" });
    await expect(
      page.getByRole("heading", { level: 1, name: surface.heading }),
    ).toBeVisible();
    expect(
      await scanForWcagViolations(page),
      `${surface.route} should have no automatically detectable WCAG A/AA violations`,
    ).toEqual([]);
  }
});

test("the logged-out entry surface has no automatically detectable WCAG A/AA violations", async ({
  browser,
}, testInfo) => {
  const context = await browser.newContext({
    baseURL: testInfo.project.use.baseURL,
  });
  const page = await context.newPage();

  try {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto("/", { waitUntil: "domcontentloaded" });
    const heading = page.getByRole("heading", { name: "Welcome to Pulse" });
    await expect(heading).toBeVisible();
    await expect(heading).toHaveCSS("animation-name", "none");
    await expect(page.locator("form").first()).toHaveCSS(
      "animation-name",
      "none",
    );
    expect(await scanForWcagViolations(page)).toEqual([]);
  } finally {
    await context.close();
  }
});
