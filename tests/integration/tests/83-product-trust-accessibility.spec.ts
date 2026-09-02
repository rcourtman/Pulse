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

const scanForUnexpectedReducedMotion = async (page: Page) =>
  page.evaluate(() => {
    const parseTimeList = (value: string) =>
      value.split(",").map((entry) => {
        const time = entry.trim();
        if (time.endsWith("ms")) return Number.parseFloat(time) / 1000;
        if (time.endsWith("s")) return Number.parseFloat(time);
        return 0;
      });
    const describe = (element: Element) => {
      const id = element.id ? `#${element.id}` : "";
      const classes = Array.from(element.classList)
        .slice(0, 3)
        .map((name) => `.${name}`)
        .join("");
      return `${element.tagName.toLowerCase()}${id}${classes}`;
    };

    return Array.from(document.querySelectorAll("*")).flatMap((element) => {
      const style = getComputedStyle(element);
      const animationSeconds = parseTimeList(style.animationDuration);
      const transitionSeconds = parseTimeList(style.transitionDuration);
      const hasRepeatedAnimation = style.animationIterationCount
        .split(",")
        .some(
          (count) =>
            count.trim() === "infinite" || Number.parseFloat(count) > 1,
        );
      const hasVisibleMotion =
        animationSeconds.some((seconds) => seconds > 0.001) ||
        transitionSeconds.some((seconds) => seconds > 0.001) ||
        hasRepeatedAnimation ||
        style.scrollBehavior === "smooth";

      return hasVisibleMotion
        ? [
            {
              target: describe(element),
              animationDuration: style.animationDuration,
              animationIterationCount: style.animationIterationCount,
              transitionDuration: style.transitionDuration,
              scrollBehavior: style.scrollBehavior,
            },
          ]
        : [];
    });
  });

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
  // Scan the settled surface: the update banner slides in over 300ms and
  // axe would otherwise sample its controls mid-fade and report blended
  // colours as contrast failures.
  await page.emulateMedia({ reducedMotion: "reduce" });
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

  const skipLink = page.getByRole("link", { name: "Skip to main content" });
  await page.evaluate(() => {
    document.body.tabIndex = -1;
    document.body.focus();
    document.body.removeAttribute("tabindex");
  });
  await page.keyboard.press("Tab");
  await expect(skipLink).toBeFocused();
  await expect(skipLink).toBeVisible();
  await page.keyboard.press("Enter");
  await expect(page.locator("#main")).toBeFocused();

  await testInfo.attach("actions-phone-width", {
    body: await page.screenshot(),
    contentType: "image/png",
  });
});

test("representative authenticated surfaces have no automatically detectable WCAG A/AA violations", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  const surfaces = [
    { route: "/alerts/overview", heading: "Alerts Overview" },
    { route: "/settings/infrastructure", heading: "Infrastructure" },
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
    expect(
      await scanForUnexpectedReducedMotion(page),
      `${surface.route} should complete non-essential motion immediately when reduced motion is requested`,
    ).toEqual([]);

    if (surface.route === "/settings/infrastructure") {
      const addButton = page.getByRole("button", {
        name: "Add infrastructure",
      });
      await addButton.click();
      const dialog = page.getByRole("dialog", { name: "Add infrastructure" });
      await expect(dialog).toHaveAccessibleDescription(
        "Choose the system, device, host, or service you want Pulse to monitor.",
      );
      await expect(page.locator(":focus")).toHaveAttribute(
        "aria-label",
        "Close add infrastructure dialog",
      );
      expect(await scanForWcagViolations(page)).toEqual([]);
      await page.keyboard.press("Escape");
      await expect(dialog).toBeHidden();
      await expect(addButton).toBeFocused();
    }
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
    expect(await scanForUnexpectedReducedMotion(page)).toEqual([]);
  } finally {
    await context.close();
  }
});
