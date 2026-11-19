import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base, type Page } from "@playwright/test";

import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// The map is a feature that is not in every build. Which builds show it is
// one line in the frontend's feature release list, and this spec reads that
// line, so it holds at every stage:
//
//   - the build under test, exactly as it would ship, shows the map only if
//     the list says the map is released;
//   - a development build shows it at any stage, and there the map's own
//     journeys run.
//
// The suite drives a production bundle, where a feature that is not released
// does not exist. The journeys below mark their own page as a development
// build by adding one field to that page's `/api/version` reply. No Pulse
// server sends that field, so no server can show a held feature, and every
// other spec still sees what ships.
const featureReleases = JSON.parse(
  fs.readFileSync(
    path.resolve(
      __dirname,
      "..",
      "..",
      "..",
      "frontend-modern",
      "src",
      "utils",
      "featureReleases.json",
    ),
    "utf8",
  ),
) as Record<string, { stage: string; guides: string[] }>;
const MAP_STAGE = featureReleases.infrastructureMap?.stage;

const MAP = 'svg[aria-label="Infrastructure map"]';
const TILE = `${MAP} [data-map-hit]`;
const HEADER_BUTTON = "infrastructure-map-header-button";
const FOCUS_RING = "rgb(59, 130, 246)";

type WorkerFixtures = {
  authStorageStatePath: string;
};

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
        `infrastructure-map-${workerInfo.project.name}.json`,
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

// Must match FEATURE_RELEASE_TEST_BUILD_FIELD in the frontend's
// utils/featureReleases.ts.
const TEST_BUILD_FIELD = "featureReleaseTestBuild";

async function asDevelopmentBuild(page: Page) {
  await page.route("**/api/version", async (route) => {
    const response = await route.fetch();
    const body = await response.json();
    await route.fulfill({
      response,
      json: { ...body, [TEST_BUILD_FIELD]: true },
    });
  });
}

// One tile, by its name, so a redraw of the live estate cannot hand a later
// step a different one.
async function pinGuest(page: Page) {
  const any = page
    .locator(`${MAP} a[data-map-hit][aria-label*=", LXC · "]`)
    .first();
  await expect(any).toBeVisible();
  const name = ((await any.getAttribute("aria-label")) ?? "").split(",")[0];
  const tile = page.locator(
    `${MAP} a[data-map-hit][aria-label^=${JSON.stringify(`${name},`)}]`,
  );
  const href = (await tile.first().getAttribute("href")) ?? "";
  expect(href).toMatch(/^\/proxmox\/overview\?resource=.+/);
  return { name, tile: tile.first(), href };
}

const pathOf = (page: Page) => {
  const url = new URL(page.url());
  return `${url.pathname}${url.search}`;
};

async function openMap(page: Page) {
  await page.goto("/map", { waitUntil: "domcontentloaded" });
  await expect(page.locator(TILE).first()).toBeVisible({ timeout: 60_000 });
}

const isPhoneProject = (name: string) => name.startsWith("mobile-");

test.describe("Infrastructure map", () => {
  test.setTimeout(120_000);

  test("the build under test shows the map exactly when the release list says a build of its kind does", async ({
    page,
  }) => {
    expect(["build", "preview", "released"]).toContain(MAP_STAGE);

    // What this build says it is. The test image takes its version from the
    // repository's VERSION file, so it is a Preview build while that names a
    // pre-release and a stable build once it does not. Nothing else a server
    // says about itself shows a held feature, and no server sends the mark.
    const version = (await (
      await page.request.get("/api/version")
    ).json()) as Record<string, unknown>;
    expect(version[TEST_BUILD_FIELD]).toBeUndefined();
    const shown =
      MAP_STAGE === "released" ||
      (MAP_STAGE === "preview" && version.channel === "rc");

    if (shown) {
      await openMap(page);
      const heading = page.getByRole("heading", { level: 1, name: "Map" });
      await expect(heading).toBeVisible();
      await expect(page.getByTestId(HEADER_BUTTON)).toBeVisible();
      // A feature in preview says so beside its title.
      const tag = page
        .locator('[data-testid="infrastructure-map"] [data-page-header]')
        .getByText("Preview", { exact: true });
      await expect(tag).toHaveCount(MAP_STAGE === "preview" ? 1 : 0);
      return;
    }

    // Held back: the address answers like any unknown one. That answer is
    // drawn only once the build has said what it is, so the checks after it
    // read a settled page.
    await page.goto("/map", { waitUntil: "domcontentloaded" });
    await expect(
      page.getByRole("heading", { level: 1, name: "Page Not Found" }),
    ).toBeVisible({ timeout: 60_000 });
    await expect(page.locator(MAP)).toHaveCount(0);
    await expect(page).not.toHaveTitle(/^Map\b/);
    await expect(page.getByTestId(HEADER_BUTTON)).toHaveCount(0);
    await expect(page.locator('a[href="/map"]')).toHaveCount(0);

    // Its guide is not part of what this build ships.
    await page.goto("/docs/MAP", { waitUntil: "domcontentloaded" });
    await expect(
      page.getByText(
        "That document is not part of this installation's shipped documentation set.",
      ),
    ).toBeVisible({ timeout: 30_000 });

    // The docs index does not point to it.
    await page.goto("/docs/README", { waitUntil: "domcontentloaded" });
    await expect(page.locator("h2").first()).toBeVisible({ timeout: 30_000 });
    await expect(page.locator('a[href*="/docs/MAP"]')).toHaveCount(0);
  });

  test("a development build reaches the map from the header and draws the estate", async ({
    page,
  }, testInfo) => {
    await asDevelopmentBuild(page);
    await page.goto("/proxmox/overview", { waitUntil: "domcontentloaded" });

    const button = page.getByTestId(HEADER_BUTTON);
    await expect(button).toBeVisible({ timeout: 60_000 });
    await button.click();
    await expect(page).toHaveURL(/\/map$/);
    await expect(page.locator(TILE).first()).toBeVisible({ timeout: 60_000 });
    await expect(
      page.getByRole("heading", { level: 1, name: "Map" }),
    ).toBeVisible();
    await expect(page).toHaveTitle(/^Map\b/);

    // The mock estate is far more than a handful of tiles, and every one
    // says what it is.
    const tiles = page.locator(TILE);
    expect(await tiles.count()).toBeGreaterThan(20);
    const unlabelled = await tiles.evaluateAll(
      (elements) =>
        elements.filter(
          (element) => !(element.getAttribute("aria-label") ?? "").trim(),
        ).length,
    );
    expect(unlabelled).toBe(0);

    // The legend is on screen, clear of the navigation bar a window that is
    // not wide keeps at its foot.
    const legend = page.getByRole("list", { name: "Legend" });
    await expect(legend).toBeVisible();
    const clear = await page.evaluate(() => {
      const legendBox = document
        .querySelector('ul[aria-label="Legend"]')!
        .getBoundingClientRect();
      const nav = document.querySelector("nav.fixed.bottom-0");
      const navBox = nav?.getBoundingClientRect();
      const floor =
        navBox && navBox.height > 0 ? navBox.top : window.innerHeight;
      return { bottom: legendBox.bottom, floor };
    });
    if (!isPhoneProject(testInfo.project.name)) {
      expect(clear.bottom).toBeLessThanOrEqual(clear.floor);
    }

    await expect(
      page.getByRole("link", { name: "How the map works" }),
    ).toHaveAttribute("href", "/docs/MAP");
  });

  test("keeps the legend above the navigation bar between a tablet and a desktop", async ({
    page,
  }, testInfo) => {
    test.skip(
      isPhoneProject(testInfo.project.name),
      "A phone project cannot be resized to a 1024px window",
    );
    await page.setViewportSize({ width: 1024, height: 768 });
    await asDevelopmentBuild(page);
    await openMap(page);

    const nav = page.locator("nav.fixed.bottom-0");
    await expect(nav).toBeVisible();
    await expect
      .poll(
        async () =>
          page.evaluate(() => {
            const legend = document
              .querySelector('ul[aria-label="Legend"]')
              ?.getBoundingClientRect();
            const bar = document
              .querySelector("nav.fixed.bottom-0")
              ?.getBoundingClientRect();
            return legend && bar ? Math.round(bar.top - legend.bottom) : -1;
          }),
        {
          message:
            "the legend should end above the navigation bar once the map has fitted",
          timeout: 15_000,
        },
      )
      .toBeGreaterThanOrEqual(0);
  });

  test("names can be hidden, and a tile says what it is and opens its own page", async ({
    page,
  }, testInfo) => {
    test.skip(
      isPhoneProject(testInfo.project.name),
      "Hover and a one-click open are pointer journeys. The phone journey is below",
    );
    await asDevelopmentBuild(page);
    await openMap(page);

    // Names on, so there is something to hide.
    await page
      .getByRole("group", { name: "Map detail" })
      .getByRole("button", { name: "Names" })
      .click();
    const addresses = () =>
      page
        .locator(`${MAP} text`)
        .evaluateAll((texts) =>
          texts
            .map((text) => text.textContent ?? "")
            .filter((text) => /\b\d{1,3}(?:\.\d{1,3}){3}\b/.test(text)),
        );
    await expect.poll(addresses).not.toHaveLength(0);

    const hide = page.getByRole("button", { name: "Hide names" });
    await hide.click();
    await expect(hide).toHaveAttribute("aria-pressed", "true");
    await expect.poll(addresses).toHaveLength(0);
    const labelled = await page
      .locator(TILE)
      .evaluateAll((tiles) =>
        tiles
          .map((tile) => tile.getAttribute("aria-label") ?? "")
          .filter((label) => /\b\d{1,3}(?:\.\d{1,3}){3}\b/.test(label)),
      );
    expect(labelled).toEqual([]);
    await hide.click();
    await expect(hide).toHaveAttribute("aria-pressed", "false");

    // Pointing at a tile says what it is.
    const { name, tile: guest, href } = await pinGuest(page);
    await guest.hover();
    const card = page.locator('[data-tooltip-portal="true"]');
    await expect(card).toBeVisible();
    await expect(card).toContainText(name);

    // Clicking it opens that guest, not merely some guest.
    await guest.click();
    await expect.poll(() => pathOf(page), { timeout: 30_000 }).toBe(href);
  });

  test("the unmonitored gateway leads to the check form and back to the map", async ({
    page,
  }, testInfo) => {
    test.skip(
      isPhoneProject(testInfo.project.name),
      "Covered once, with a pointer",
    );
    await asDevelopmentBuild(page);
    await openMap(page);

    const gateway = page
      .locator(`${MAP} a[data-map-hit][aria-label$="add a check"]`)
      .first();
    await expect(gateway).toBeVisible();
    const address = ((await gateway.getAttribute("aria-label")) ?? "").split(
      ",",
    )[0];
    expect(address).toMatch(/^\d{1,3}(\.\d{1,3}){3}$/);

    await gateway.click();
    await expect(page).toHaveURL(/add=target/, { timeout: 30_000 });
    expect(new URL(page.url()).searchParams.get("from")).toBe("map");
    expect(new URL(page.url()).searchParams.get("address")).toBe(address);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible({ timeout: 30_000 });
    await expect(dialog).toContainText("Add router check");

    await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(page).toHaveURL(/\/map$/, { timeout: 30_000 });
    await expect(page.locator(TILE).first()).toBeVisible({ timeout: 60_000 });
  });

  test("a keyboard reaches a tile, sees where it is, and opens it", async ({
    page,
  }, testInfo) => {
    test.skip(
      isPhoneProject(testInfo.project.name),
      "A phone project has no keyboard",
    );
    await asDevelopmentBuild(page);
    await openMap(page);

    await page.getByRole("button", { name: "Save image" }).focus();
    await page.keyboard.press("Tab");

    const focused = page.locator(`${MAP} :focus`);
    await expect(focused).toHaveAttribute("aria-label", /.+/);
    // The ring is on the rect inside the link. Read once the colour has settled.
    await expect
      .poll(async () =>
        focused.evaluate((element) => {
          const rect =
            element.tagName.toLowerCase() === "a"
              ? element.querySelector("rect")
              : element;
          return rect ? getComputedStyle(rect).stroke : "";
        }),
      )
      .toBe(FOCUS_RING);
    await expect(page.locator('[data-tooltip-portal="true"]')).toBeVisible();

    // Walk on to a tile that is a link, and open it: Enter goes where that
    // tile points.
    for (let step = 0; step < 6; step += 1) {
      if ((await focused.getAttribute("href")) !== null) break;
      await page.keyboard.press("Tab");
    }
    const target = (await focused.getAttribute("href")) ?? "";
    expect(target).toMatch(/^\//);
    await page.keyboard.press("Enter");
    await expect
      .poll(() => pathOf(page).startsWith(target.split("?")[0]), {
        timeout: 30_000,
      })
      .toBe(true);
  });

  test("saves the map as an image file", async ({ page }, testInfo) => {
    test.skip(
      isPhoneProject(testInfo.project.name),
      "The file is the same on a phone",
    );
    await asDevelopmentBuild(page);
    await openMap(page);

    const [download] = await Promise.all([
      page.waitForEvent("download", { timeout: 60_000 }),
      page.getByRole("button", { name: "Save image" }).click(),
    ]);
    expect(download.suggestedFilename()).toMatch(
      /^pulse-map-\d{4}-\d{2}-\d{2}\.(png|svg)$/,
    );
    const saved = fs.readFileSync(await download.path());
    expect(saved.length).toBeGreaterThan(10_000);
    if (download.suggestedFilename().endsWith(".png")) {
      // A real PNG, not a page of text with an image's name.
      expect(saved.subarray(0, 8).toString("hex")).toBe("89504e470d0a1a0a");
    } else {
      expect(saved.subarray(0, 200).toString("utf8")).toContain("<svg");
    }
  });

  test("on a phone the first tap reads a tile and the second opens it", async ({
    page,
  }, testInfo) => {
    test.skip(
      !isPhoneProject(testInfo.project.name),
      "A touch journey. Pointer journeys are above",
    );
    await asDevelopmentBuild(page);
    await openMap(page);

    // A phone gets the map at full size in one column, not shrunk to one
    // screen: icons alone to begin with, every tile big enough to tap, and
    // nothing wider than the window.
    await expect(
      page
        .getByRole("group", { name: "Map detail" })
        .getByRole("button", { name: "Compact" }),
    ).toHaveAttribute("aria-pressed", "true");
    const drawn = await page.evaluate((selector) => {
      const svg = document.querySelector(selector)!;
      const box = svg.getBoundingClientRect();
      const tiles = [...svg.querySelectorAll("a[data-map-hit] rect")].map(
        (rect) => rect.getBoundingClientRect().width,
      );
      return {
        smallestTile: Math.min(...tiles),
        width: box.width,
        height: box.height,
        window: { width: window.innerWidth, height: window.innerHeight },
        sideways:
          document.documentElement.scrollWidth > window.innerWidth,
        outside: [
          ...svg.querySelectorAll("rect, text, circle, path, line, image"),
        ]
          .map((node) => node.getBoundingClientRect())
          .filter(
            (at) =>
              (at.width > 0 || at.height > 0) &&
              (at.left < box.left - 1 || at.right > box.right + 1),
          )
          .map((at) => `${Math.round(at.left)}..${Math.round(at.right)}`),
      };
    }, MAP);
    expect(drawn.smallestTile).toBeGreaterThanOrEqual(36);
    expect(drawn.width).toBeLessThanOrEqual(drawn.window.width);
    expect(drawn.height).toBeGreaterThan(drawn.window.height);
    expect(drawn.sideways).toBe(false);
    // Nothing is drawn past either side of the drawing, where it would be
    // cut off, and the page's last line ends above the navigation bar.
    expect(drawn.outside).toEqual([]);
    const foot = await page.evaluate(() => {
      // The app scrolls inside its shell, not the window.
      for (const scroller of [
        document.querySelector(".app-scroll-shell"),
        document.scrollingElement,
      ]) {
        if (scroller) scroller.scrollTop = scroller.scrollHeight;
      }
      const legend = document
        .querySelector('[aria-label="Legend"]')!
        .getBoundingClientRect();
      const bar = Number.parseFloat(
        getComputedStyle(document.documentElement).getPropertyValue(
          "--pulse-mobile-nav-height",
        ),
      );
      return {
        legendBottom: legend.bottom,
        barTop: window.innerHeight - (Number.isFinite(bar) ? bar : 0),
      };
    });
    expect(foot.legendBottom).toBeLessThanOrEqual(foot.barTop);

    const { name, tile: guest, href } = await pinGuest(page);
    await guest.scrollIntoViewIfNeeded();
    await guest.tap();
    await expect(page.getByText("Tap again to open")).toBeVisible();
    await expect(page.getByText(name).first()).toBeVisible();
    await expect(page).toHaveURL(/\/map$/);

    await guest.tap();
    await expect.poll(() => pathOf(page), { timeout: 30_000 }).toBe(href);
  });

  test("the guide opens from the map in a development build", async ({
    page,
  }, testInfo) => {
    test.skip(
      isPhoneProject(testInfo.project.name),
      "Covered once, with a pointer",
    );
    await asDevelopmentBuild(page);
    await openMap(page);

    await page.getByRole("link", { name: "How the map works" }).click();
    await expect(page).toHaveURL(/\/docs\/MAP$/);
    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "See everything on one map",
      }),
    ).toBeVisible({ timeout: 30_000 });
  });
});
