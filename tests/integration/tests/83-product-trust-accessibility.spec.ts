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

test("Home status wall is readable and motionless across desktop and phone widths", async ({
  page,
}, testInfo) => {
  const lastSeen = "2026-08-31T22:30:00Z";
  const resource = (
    id: string,
    name: string,
    type: string,
    sources: string[],
    verdict: string,
    reason?: { code: string; detail?: string },
  ) => ({
    id,
    name,
    type,
    sources,
    status:
      verdict === "off"
        ? "stopped"
        : verdict === "critical"
          ? "offline"
          : "online",
    lastSeen,
    health: { verdict, reasons: reason ? [reason] : [] },
  });
  const resources = [
    resource("node-down", "PVE node down", "agent", ["proxmox"], "critical", {
      code: "offline",
    }),
    resource("vm-warning", "Billing VM", "vm", ["proxmox"], "attention", {
      code: "warning_alert",
      detail: "memory",
    }),
    resource("agent-stale", "Remote agent", "agent", ["agent"], "stale", {
      code: "telemetry_stale",
      detail: "12m",
    }),
    resource(
      "container-off",
      "Batch worker",
      "app-container",
      ["docker"],
      "off",
      {
        code: "powered_off",
      },
    ),
    resource(
      "check-down",
      "Customer portal",
      "network-endpoint",
      ["availability"],
      "critical",
      { code: "availability_failed" },
    ),
    ...Array.from({ length: 65 }, (_, index) =>
      resource(
        `healthy-${index + 1}`,
        `Healthy VM ${index + 1}`,
        "vm",
        ["proxmox"],
        "ok",
      ),
    ),
  ];
  let resourceRequestsFail = false;
  await page.route("**/api/resources?*", (route) => {
    if (resourceRequestsFail) {
      return route.fulfill({
        status: 503,
        contentType: "application/json",
        body: JSON.stringify({ error: "resource refresh unavailable" }),
      });
    }
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: resources,
        meta: { totalPages: 1 },
        aggregations: { total: resources.length },
      }),
    });
  });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/home", { waitUntil: "domcontentloaded" });
  await expect(
    page.getByRole("heading", { level: 1, name: "Home" }),
  ).toBeVisible();
  await expect(
    page.getByRole("status").filter({ hasText: "3 need attention" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: /PVE node down: Critical/ }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Batch worker: Powered off. Powered off" }),
  ).toHaveAttribute("href", "/docker/overview?q=container-off");
  await expect(
    page.getByRole("link", {
      name: "Remote agent: Stale. Telemetry stale 12m",
    }),
  ).toHaveAttribute("href", "/standalone/machines?q=agent-stale");
  await expect(
    page.getByRole("link", {
      name: "Healthy VM 1: Healthy. Healthy",
      exact: true,
    }),
  ).toContainText("Healthy");
  const showAll = page.getByRole("button", { name: "Show all (5)" });
  await expect(showAll).toBeVisible();
  await showAll.click();
  await expect(page.getByRole("button", { name: "Show less" })).toBeVisible();

  resourceRequestsFail = true;
  await page.getByRole("button", { name: "Refresh fleet health" }).click();
  await expect(page.getByRole("alert")).toContainText(
    "Fleet health could not be refreshed",
  );
  await expect(
    page.getByRole("link", { name: /PVE node down: Critical/ }),
  ).toBeVisible();

  expect(await scanForWcagViolations(page)).toEqual([]);
  expect(await scanForUnexpectedReducedMotion(page)).toEqual([]);
  await testInfo.attach("home-desktop", {
    body: await page.screenshot({ fullPage: true }),
    contentType: "image/png",
  });

  await page.setViewportSize({ width: 390, height: 844 });
  expect(await scanForWcagViolations(page)).toEqual([]);
  expect(
    await page.evaluate(
      () =>
        document.documentElement.scrollWidth >
        document.documentElement.clientWidth,
    ),
  ).toBeFalsy();
  await testInfo.attach("home-phone", {
    body: await page.screenshot({ fullPage: true }),
    contentType: "image/png",
  });
});

test("representative authenticated surfaces have no automatically detectable WCAG A/AA violations", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
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
    expect(
      await scanForUnexpectedReducedMotion(page),
      `${surface.route} should complete non-essential motion immediately when reduced motion is requested`,
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
    expect(await scanForUnexpectedReducedMotion(page)).toEqual([]);
  } finally {
    await context.close();
  }
});
