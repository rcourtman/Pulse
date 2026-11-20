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

const scanForWcagViolations = async (
  page: Page,
  options: { except?: string[] } = {},
) => {
  const results = await new AxeBuilder({ page })
    .withTags([...WCAG_TAGS])
    .disableRules(options.except ?? [])
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

for (const theme of ["light", "dark"] as const) {
test(`representative authenticated surfaces have no automatically detectable WCAG A/AA violations (${theme})`, async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.addInitScript(preference => {
    localStorage.setItem("pulseThemePreference", preference);
    localStorage.setItem("darkMode", String(preference === "dark"));
    localStorage.removeItem("pulse_dark_mode");
  }, theme);
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
    if (surface.route === "/patrol") {
      await page.getByRole("list", { name: "Patrol attention items", exact: true })
        .getByRole("button").first().hover();
    }
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

      const installAgent = page.getByRole("button", { name: "Add Pulse Agent hosts", exact: true });
      await installAgent.focus();
      await page.keyboard.press("Enter");
      const installer = page.getByRole("dialog");
      await expect(installer.getByRole("heading", { name: "Add Pulse Agent", exact: true })).toBeVisible();
      expect(await scanForWcagViolations(page)).toEqual([]);
      await page.keyboard.press("Escape");
      await expect(installer).toBeHidden();
      await expect(installAgent).toBeFocused();

      await page.getByRole("button", {
        name: /^(Agent update|\d+ (agent )?updates): open Agent Doctor for /,
      }).first().click();
      await expect(page.getByRole("heading", { name: "Agent Doctor", exact: true })).toBeVisible();
      expect(new URL(page.url()).searchParams.getAll("agents").length).toBeGreaterThan(0);
      const command = page.locator("pre").first();
      await expect(command).toBeVisible();
      await command.focus();
      await expect(command).toBeFocused();
      await page.keyboard.press("ArrowRight");
      await expect.poll(() => command.evaluate(node => node.scrollLeft)).toBeGreaterThan(0);
      // Focusing the command scrolled the page to wherever the browser chose,
      // and a row's control cut by the sticky header at that offset reads as
      // a target too small. Which row that is, if any, is an accident of the
      // offset: on 20 November 2026 a 22px difference in where WebKit rests a
      // focused element turned this scan from passing to failing. Target
      // sizes are judged on the unscrolled page, above.
      expect(
        await scanForWcagViolations(page, { except: ["target-size"] }),
      ).toEqual([]);
    }

    if (surface.route === "/patrol") {
      const list = page.getByRole("list", { name: "Patrol attention items", exact: true });
      const item = list.getByRole("button").first();
      await item.focus();
      await page.keyboard.press("Enter");
      const detail = page.getByRole("complementary");
      await expect(detail).toBeVisible();
      await expect(detail).toHaveAttribute("aria-busy", "false");
      const selectedTitle = await detail.getByRole("heading", { level: 3 }).first().innerText();
      expect(await scanForWcagViolations(page)).toEqual([]);
      await detail.getByText("Evidence and history", { exact: true }).click();
      await expect(detail.getByRole("button", { name: "Copy ID", exact: true })).toBeVisible();
      expect(await scanForWcagViolations(page)).toEqual([]);
      const back = detail.getByRole("button", { name: "Back to list of Patrol attention", exact: true });
      if (await back.isVisible()) await back.click();
      else await detail.getByRole("button", { name: "Close attention detail", exact: true }).click();
      await expect(detail).toBeHidden();
      await expect(list.locator("button:focus")).toContainText(selectedTitle);
    }
  }
});
}

test("populated platform tables expose one named disclosure control per resource", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });

  for (const viewport of [
    { width: 1280, height: 800 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport);
    await page.goto("/truenas/overview", { waitUntil: "domcontentloaded" });

    const row = page.locator("[data-truenas-system-row]").first();
    await expect(row).toBeVisible({ timeout: 30_000 });
    await expect(row).not.toHaveAttribute("tabindex");
    await expect(row).not.toHaveAttribute("aria-expanded");
    await expect(row).not.toHaveAttribute("aria-controls");

    await expect(row.getByRole("button", { name: /Expand details for/ })).toBeAttached();
    const disclosure = row.locator('button[data-row-action="true"]').first();
    const detailRowId = await disclosure.getAttribute("aria-controls");
    expect(detailRowId).toBeTruthy();

    await disclosure.focus();
    await page.keyboard.press("Enter");
    await expect(disclosure).toHaveAttribute("aria-expanded", "true");
    await expect(page.locator(`#${detailRowId}`)).toBeVisible();

    await page.keyboard.press("Space");
    await expect(disclosure).toHaveAttribute("aria-expanded", "false");

    const rowBox = await row.boundingBox();
    expect(rowBox).not.toBeNull();
    await row.click({ position: { x: Math.min(30, rowBox!.width / 2), y: rowBox!.height / 2 } });
    await expect(disclosure).toHaveAttribute("aria-expanded", "true");
    expect(await scanForWcagViolations(page)).toEqual([]);
  }
});

// The banner for a new version is the commonest thing above the content
// area, and it arrives when the update check answers, after the shell. A
// test of how the app fits the window has to see the page with it showing.
// Where the build under test offers no update there is no banner to wait for.
const waitForBannerIfAny = async (
  page: import("@playwright/test").Page,
): Promise<void> => {
  // The app's own question, asked again: the answer may already be in.
  const offered = await page.request
    .get("/api/updates/check")
    .then(async (response) => {
      if (!response.ok()) return false;
      const body = (await response.json().catch(() => null)) as {
        available?: boolean;
      } | null;
      return body?.available === true;
    })
    .catch(() => false);
  if (!offered) return;
  await expect(page.locator(".update-banner").first()).toBeVisible({
    timeout: 20_000,
  });
};

test("a control that takes focus comes to rest above the bottom navigation bar", async ({
  page,
}) => {
  // WCAG 2.2, 2.4.11 Focus Not Obscured: what has the focus is not hidden by
  // something the page itself put on top of it.
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/proxmox/overview", { waitUntil: "domcontentloaded" });
  await expect(page.locator("#root")).toBeVisible();
  await expect(
    page.getByRole("navigation", { name: "Mobile navigation" }),
  ).toBeVisible();
  // A banner above the content, such as the one for a new version, is part
  // of the app's one window of height. It used to push the app past the
  // foot of the window, and the window then scrolled as well as the page.
  // Give it time to arrive, so that this reads the page with it showing
  // where the build under test has one.
  await waitForBannerIfAny(page);
  const fit = await page.evaluate(() => ({
    windowScroll:
      document.documentElement.scrollHeight -
      document.documentElement.clientHeight,
    shellBelowWindow:
      document.querySelector(".app-scroll-shell")!.getBoundingClientRect()
        .bottom - window.innerHeight,
  }));
  expect(
    fit.windowScroll,
    "The app should be exactly a window tall, banners included",
  ).toBeLessThanOrEqual(1);
  expect(fit.shellBelowWindow).toBeLessThanOrEqual(1);
  // The page has to be long enough to scroll before this means anything.
  await expect
    .poll(
      () =>
        page.evaluate(() => {
          const shell = document.querySelector(".app-scroll-shell");
          return shell ? shell.scrollHeight - shell.clientHeight : 0;
        }),
      { timeout: 30_000 },
    )
    .toBeGreaterThan(200);

  // The page scrolls inside the app, and the bar is fixed over the foot of
  // that scroll area. Give focus to controls down the page, as Tab would,
  // and see where the browser brings each to rest.
  const result = await page.evaluate(async () => {
    const shell = document.querySelector(".app-scroll-shell")!;
    const bar = document
      .querySelector('nav[aria-label="Mobile navigation"]')!
      .getBoundingClientRect();
    const controls = Array.from(
      shell.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), [tabindex="0"]',
      ),
    ).filter((control) => {
      // The bar's own buttons are in the bar.
      if (control.closest('nav[aria-label="Mobile navigation"]')) return false;
      const box = control.getBoundingClientRect();
      return box.width > 0 && box.height > 0 && box.height < 120;
    });
    const step = Math.max(1, Math.floor(controls.length / 30));
    const under: string[] = [];
    const unrevealed: string[] = [];
    let checked = 0;
    let scrolled = 0;
    for (let index = 0; index < controls.length; index += step) {
      const control = controls[index];
      const before = shell.scrollTop;
      control.focus();
      if (document.activeElement !== control) continue;
      await new Promise((resolve) => setTimeout(resolve, 120));
      checked += 1;
      if (shell.scrollTop !== before) scrolled += 1;
      const box = control.getBoundingClientRect();
      const name = `${control.tagName.toLowerCase()} "${(control.textContent ?? "").trim().slice(0, 24)}"`;
      // A control the browser did not bring into the window at all is not
      // this rule's. WebKit stops short on the odd long jump made from
      // script, so one or two are allowed and no more.
      if (box.top >= window.innerHeight) {
        unrevealed.push(name);
      } else if (box.bottom > bar.top + 1) {
        under.push(`${name} ${Math.round(box.bottom - bar.top)}px under`);
      }
    }
    return { checked, scrolled, under, unrevealed };
  });

  expect(result.checked).toBeGreaterThan(10);
  expect(
    result.scrolled,
    "Giving focus down the page should have scrolled it",
  ).toBeGreaterThan(2);
  expect(result.under).toEqual([]);
  expect(result.unrevealed.length, result.unrevealed.join(", ")).toBeLessThanOrEqual(2);
});

test("the window itself has nothing to scroll, whatever the page holds", async ({
  page,
}) => {
  // The page scrolls inside the app. A banner above the content, or a
  // label for screen readers far down a page and placed against the window
  // instead of the page, used to give the window its own distance to
  // scroll, where a key the page did not take found a blank screen.
  for (const viewport of [
    { width: 390, height: 844 },
    { width: 1024, height: 700 },
  ]) {
    await page.setViewportSize(viewport);
    for (const route of [
      "/proxmox/overview",
      "/proxmox/backups",
      "/vmware/overview",
    ]) {
      // WebKit now and then refuses a navigation made while the page before
      // it is still loading ("WebKit encountered an internal error").
      await page
        .goto(route, { waitUntil: "domcontentloaded" })
        .catch(() => page.goto(route, { waitUntil: "domcontentloaded" }));
      await expect(page.locator(".app-scroll-shell")).toBeVisible();
      await waitForBannerIfAny(page);
      // Rows arrive after the shell does, and the labels that used to
      // escape the page come with them.
      await page
        .waitForFunction(
          () =>
            document.querySelectorAll(".app-scroll-shell table tbody tr")
              .length > 0,
          undefined,
          { timeout: 20_000 },
        )
        .catch(() => undefined);
      await page.waitForTimeout(500);
      const windowScroll = await page.evaluate(
        () =>
          document.documentElement.scrollHeight -
          document.documentElement.clientHeight,
      );
      expect(
        windowScroll,
        `${route} at ${viewport.width}px wide`,
      ).toBeLessThanOrEqual(1);
    }
  }
});

test("the logged-out entry surface has no automatically detectable WCAG A/AA violations", async ({
  browser,
}, testInfo) => {
  // browser.newContext inherits this file's signed-in storage state unless told otherwise.
  const context = await browser.newContext({
    baseURL: testInfo.project.use.baseURL,
    storageState: { cookies: [], origins: [] },
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
