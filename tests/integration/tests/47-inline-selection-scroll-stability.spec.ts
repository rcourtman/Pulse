import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  expect,
  test as base,
  type Locator,
  type Page,
} from "@playwright/test";

import {
  createAuthenticatedStorageState,
  getMockMode,
  setMockMode,
} from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

type LayoutObserverOptions = {
  rowSelector: string;
  detailSelector: string;
  // Whether the row's inline detail must be showing (after opening) or gone
  // (after closing) before the layout can count as settled.
  detailOpen: boolean;
  // Opening near the bottom edge: a layout where the row has not risen more
  // than 150 px above this parked top is not a result yet.
  revealFrom?: number;
  // Closing: the row must never move more than 2 px from this top.
  holdRowAt?: number;
  // Draining only: movement after settling restarts settling, not a failure.
  restartOnMovement?: boolean;
};

type ObservedLayout = {
  rowTop: number | null;
  detailTop: number | null;
  viewportTop: number;
  viewportBottom: number;
  viewportHeight: number;
};

type LayoutObservation = {
  settled: ObservedLayout | null;
  violations: string[];
};

let mockModeWasEnabled: boolean | null = null;

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
        `inline-selection-scroll-stability-${workerInfo.project.name}.json`,
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

async function ensureMockModeEnabled(page: Page): Promise<void> {
  const state = await getMockMode(page);
  if (mockModeWasEnabled === null) {
    mockModeWasEnabled = state.enabled;
  }
  if (!state.enabled) {
    await setMockMode(page, true);
  }
}

async function readPrimaryViewportScrollTop(page: Page): Promise<number> {
  return page.evaluate(() => {
    const shell = document.querySelector<HTMLElement>(".app-scroll-shell");
    return shell ? shell.scrollTop : window.scrollY;
  });
}

async function positionElementNearViewportBottom(
  page: Page,
  locator: Locator,
  bottomInset = 96,
): Promise<number> {
  const targetTop = await locator.evaluate(
    (element, inset) =>
      (() => {
        const shell = document.querySelector<HTMLElement>(".app-scroll-shell");
        if (shell && shell.contains(element)) {
          const shellRect = shell.getBoundingClientRect();
          return Math.max(
            0,
            shell.scrollTop +
              element.getBoundingClientRect().top -
              shellRect.top -
              (shell.clientHeight - inset),
          );
        }
        return Math.max(
          0,
          window.scrollY +
            element.getBoundingClientRect().top -
            (window.innerHeight - inset),
        );
      })(),
    bottomInset,
  );
  await page.evaluate((nextTop) => {
    const shell = document.querySelector<HTMLElement>(".app-scroll-shell");
    if (shell) {
      shell.scrollTop = nextTop;
      return;
    }
    window.scrollTo(0, nextTop);
  }, targetTop);
  await page.waitForTimeout(150);
  return locator.evaluate((element) => element.getBoundingClientRect().top);
}

// Starts one in-page observer that samples the row, its inline detail and the
// scroll shell on every animation frame from before the action until a
// verdict. An element that is absent, renders no box or is not visible counts
// as missing. Positions compare with a 1 px tolerance against a fixed
// reference, so drift cannot accumulate. The layout settles once every frame
// in a 400 ms window agrees (a frame gap over 100 ms, a busy main thread,
// restarts the window). The settled layout must then hold for a further
// 1.2 s, which outlasts the longest the shared reveal keeps working on a
// focus change (summaryTableFocus.ts), ending with at least ten frames after
// the last gap.
//
// Reveal observers also catch a second scroll. The reveal is one smooth
// movement: once the row has moved 20 px or more and no sample has seen it
// move by over 1 px for 200 ms of wall time, that position is the rest
// position, and any later excursion beyond 8 px from it is a late jump, even
// one that lands inside the accepted band or returns. Close observers must not scroll at all, so a
// `scroll` event on the shell, which the browser delivers even for a move that
// is undone before the next frame, fails them as well. Every violation is
// latched and final, nothing is retried, and a timer enforces the 30 s
// deadline whether or not frames keep arriving.
//
// The model assumes ordinary frame cadence. Two things it cannot see: a jump
// that happens and is undone entirely inside one starved frame gap (about
// 150 ms or more), and a jump within about a quarter second of the reveal
// ending when a starved frame hid that ending, which reads as part of the
// reveal. The ten-frames-after-a-gap rule only stops starved stretches from
// counting towards a pass.
async function startLayoutObserver(
  page: Page,
  options: LayoutObserverOptions,
): Promise<void> {
  await page.evaluate((observerOptions) => {
    type Layout = {
      rowTop: number | null;
      detailTop: number | null;
      viewportTop: number;
      viewportBottom: number;
      viewportHeight: number;
    };
    const visibleTop = (selector: string): number | null => {
      const element = document.querySelector<HTMLElement>(selector);
      if (!element) return null;
      const rect = element.getBoundingClientRect();
      const visible =
        rect.width > 0 &&
        rect.height > 0 &&
        getComputedStyle(element).visibility === "visible";
      return visible ? rect.top : null;
    };
    const read = (): Layout => {
      const shell = document.querySelector<HTMLElement>(".app-scroll-shell");
      const shellRect = shell?.getBoundingClientRect();
      return {
        rowTop: visibleTop(observerOptions.rowSelector),
        detailTop: visibleTop(observerOptions.detailSelector),
        viewportTop: shellRect ? shellRect.top : 0,
        viewportBottom: shellRect ? shellRect.bottom : window.innerHeight,
        viewportHeight: shell ? shell.clientHeight : window.innerHeight,
      };
    };
    const near = (left: number | null, right: number | null) =>
      left === null || right === null
        ? left === right
        : Math.abs(left - right) <= 1;
    const same = (left: Layout, right: Layout) =>
      near(left.rowTop, right.rowTop) &&
      near(left.detailTop, right.detailTop) &&
      near(left.viewportTop, right.viewportTop) &&
      near(left.viewportHeight, right.viewportHeight);
    const isResult = (layout: Layout) =>
      layout.rowTop !== null &&
      (observerOptions.detailOpen
        ? layout.detailTop !== null
        : layout.detailTop === null) &&
      (observerOptions.revealFrom === undefined ||
        layout.rowTop < observerOptions.revealFrom - 150);

    const violations: string[] = [];
    let candidate: { layout: Layout; since: number } | null = null;
    let settled: Layout | null = null;
    let settledAt = 0;
    let framesSinceGap = 0;
    let lastLayout: Layout | null = null;
    // The row starts parked, so the first sample is measured against that.
    let previousRowTop: number | null = observerOptions.revealFrom ?? null;
    let scrollPhase: "before" | "moving" | "rested" = "before";
    let movedInPhase = 0;
    let lastMovementAt = performance.now();
    let restTop = 0;
    let finished = false;
    const startedAt = performance.now();
    let previousFrameAt = startedAt;
    // Frame statistics, reported when the observer runs out of time so that a starved
    // run can be told apart from a layout that really never held still.
    let frameCount = 0;
    let longGaps = 0;
    let maxGap = 0;
    const frameStats = () =>
      `frames=${frameCount} gapsOver100ms=${longGaps} maxGapMs=${Math.round(maxGap)}`;
    const scrollTarget: EventTarget =
      document.querySelector<HTMLElement>(".app-scroll-shell") ?? window;

    const done = new Promise<{ settled: Layout | null; violations: string[] }>(
      (resolve) => {
        const onScroll = () => {
          violations.push("the shell scrolled while the row was closing");
          finish();
        };
        const finish = () => {
          if (finished) return;
          finished = true;
          window.clearTimeout(deadlineTimer);
          scrollTarget.removeEventListener("scroll", onScroll);
          resolve({ settled, violations });
        };
        const deadlineTimer = window.setTimeout(() => {
          violations.push(
            settled
              ? `settled layout was not observed for the whole hold; ${frameStats()}`
              : `layout never settled; last ${JSON.stringify(lastLayout)}; ${frameStats()}`,
          );
          finish();
        }, 30_000);
        if (observerOptions.holdRowAt !== undefined) {
          scrollTarget.addEventListener("scroll", onScroll, { passive: true });
        }

        const step = (now: number) => {
          if (finished) return;
          const frameGap = now - previousFrameAt;
          previousFrameAt = now;
          frameCount += 1;
          if (frameGap > 100) longGaps += 1;
          maxGap = Math.max(maxGap, frameGap);
          const layout = read();
          lastLayout = layout;

          const holdRowAt = observerOptions.holdRowAt;
          if (
            holdRowAt !== undefined &&
            (layout.rowTop === null || Math.abs(layout.rowTop - holdRowAt) > 2)
          ) {
            violations.push(`row moved from ${holdRowAt} to ${layout.rowTop}`);
            finish();
            return;
          }

          if (observerOptions.revealFrom !== undefined) {
            if (scrollPhase === "rested") {
              if (
                layout.rowTop === null ||
                Math.abs(layout.rowTop - restTop) > 8
              ) {
                violations.push(
                  `row moved after it had come to rest: ${restTop} -> ${layout.rowTop}`,
                );
                finish();
                return;
              }
            } else if (layout.rowTop !== null) {
              if (
                previousRowTop !== null &&
                Math.abs(layout.rowTop - previousRowTop) > 1
              ) {
                movedInPhase += Math.abs(layout.rowTop - previousRowTop);
                // The move happened somewhere since the previous sample; date
                // it to the start of that interval, so a long gap cannot push
                // the end of the reveal later than it really was.
                lastMovementAt = now - frameGap;
                if (movedInPhase >= 20) scrollPhase = "moving";
              } else if (now - lastMovementAt >= 200) {
                if (scrollPhase === "moving") {
                  scrollPhase = "rested";
                  restTop = layout.rowTop;
                } else {
                  movedInPhase = 0;
                }
              }
            }
            if (layout.rowTop !== null) previousRowTop = layout.rowTop;
          }

          if (settled && !same(layout, settled)) {
            if (!observerOptions.restartOnMovement) {
              violations.push(
                `layout moved after settling: ${JSON.stringify(settled)} -> ${JSON.stringify(layout)}`,
              );
              finish();
              return;
            }
            settled = null;
            candidate = null;
          }

          if (settled) {
            framesSinceGap = frameGap > 100 ? 0 : framesSinceGap + 1;
            if (now - settledAt >= 1_200 && framesSinceGap >= 10) {
              finish();
              return;
            }
          } else if (
            !isResult(layout) ||
            frameGap > 100 ||
            !candidate ||
            !same(layout, candidate.layout)
          ) {
            candidate = isResult(layout) ? { layout, since: now } : null;
          } else if (now - candidate.since >= 400) {
            settled = candidate.layout;
            settledAt = now;
            framesSinceGap = 0;
          }
          requestAnimationFrame(step);
        };
        requestAnimationFrame(step);
      },
    );
    (
      window as unknown as { __pulseLayoutObserver?: Promise<unknown> }
    ).__pulseLayoutObserver = done;
  }, options);
}

async function finishLayoutObserver(page: Page): Promise<LayoutObservation> {
  return page.evaluate(async () => {
    const host = window as unknown as {
      __pulseLayoutObserver?: Promise<LayoutObservation>;
    };
    const observer = host.__pulseLayoutObserver;
    delete host.__pulseLayoutObserver;
    return observer
      ? await observer
      : { settled: null, violations: ["layout observer was not running"] };
  });
}

// Opening a row near the bottom edge must bring its inline detail into view
// by lifting the row to an anchor in the upper part of the scroll shell: not
// pinned to the top edge, not hard-centred, not left where it was. The
// observer only settles on a row that has risen at least 150 px from where it
// was parked, so "left where it was" already fails there.
function expectInlineDetailRevealedBelowTop(
  observation: LayoutObservation,
): number {
  expect(observation.violations).toEqual([]);
  const { rowTop, detailTop, viewportTop, viewportBottom, viewportHeight } =
    observation.settled!;
  expect(rowTop!).toBeGreaterThan(viewportTop + 96);
  expect(rowTop!).toBeLessThan(viewportTop + viewportHeight * 0.42);
  expect(detailTop!).toBeGreaterThan(rowTop!);
  expect(detailTop!).toBeLessThan(viewportBottom - 48);
  return rowTop!;
}

// Row expansion is local state: it must not touch the URL or the history at
// all. Record every same-document navigation the page starts from here on
// (pushState and replaceState included, whatever URL they write), so a write
// that is later undone, or that restores the same URL, still fails the test.
async function startUrlWriteProbe(page: Page): Promise<void> {
  await page.evaluate(() => {
    type NavigateEvent = Event & { destination: { url: string } };
    const host = window as unknown as {
      navigation?: EventTarget;
      __pulseInlineSelectionUrlWrites?: { writes: string[]; stop: () => void };
    };
    const navigation = host.navigation;
    if (!navigation) {
      throw new Error("Navigation API unavailable");
    }
    const writes: string[] = [];
    const record = (event: Event) => {
      writes.push((event as NavigateEvent).destination.url);
    };
    navigation.addEventListener("navigate", record);
    host.__pulseInlineSelectionUrlWrites = {
      writes,
      stop: () => navigation.removeEventListener("navigate", record),
    };
  });
}

async function stopUrlWriteProbe(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const host = window as unknown as {
      __pulseInlineSelectionUrlWrites?: { writes: string[]; stop: () => void };
    };
    const probe = host.__pulseInlineSelectionUrlWrites;
    probe?.stop();
    delete host.__pulseInlineSelectionUrlWrites;
    return probe ? [...probe.writes] : ["URL write probe was not running"];
  });
}

// Returns the first rendered row that starts below the first screen, so that
// parking it near the bottom edge needs a genuinely scrolled shell. The row is
// re-located by its own id because windowed tables re-render as they scroll,
// which would move a positional nth() match to a different row; ids that
// appear on more than one row are skipped so the selector names one row.
async function findRowBelowFirstScreen(
  rows: Locator,
  idAttribute: string,
): Promise<{ id: string; selector: string; row: Locator }> {
  const id = await rows.evaluateAll((elements, attribute) => {
    const shell = document.querySelector<HTMLElement>(".app-scroll-shell");
    const viewportTop = shell ? shell.getBoundingClientRect().top : 0;
    const scrollTop = shell ? shell.scrollTop : window.scrollY;
    const viewportHeight = shell ? shell.clientHeight : window.innerHeight;
    const idCounts = new Map<string, number>();
    for (const element of elements) {
      const value = element.getAttribute(attribute) ?? "";
      idCounts.set(value, (idCounts.get(value) ?? 0) + 1);
    }
    const match = elements.find((element) => {
      const rect = element.getBoundingClientRect();
      const value = element.getAttribute(attribute) ?? "";
      return (
        value !== "" &&
        idCounts.get(value) === 1 &&
        rect.height > 0 &&
        rect.top - viewportTop + scrollTop > viewportHeight
      );
    });
    return match?.getAttribute(attribute) ?? "";
  }, idAttribute);
  expect(id, "a row starts below the first screen").not.toBe("");
  const selector = `tr[${idAttribute}="${id}"]`;
  return { id, selector, row: rows.page().locator(selector) };
}

test.describe.serial("Inline selection scroll stability", () => {
  test.setTimeout(180_000);

  test.afterAll(async ({ browser }) => {
    if (mockModeWasEnabled === null) return;

    const context = await browser.newContext();
    const page = await context.newPage();
    try {
      const current = await getMockMode(page);
      if (current.enabled !== mockModeWasEnabled) {
        await setMockMode(page, mockModeWasEnabled);
      }
    } finally {
      await context.close();
    }
  });

  test("reveals workload inline detail without hard-centering the selected row", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only workload interaction proof",
    );

    await ensureMockModeEnabled(page);

    await page.goto("/proxmox/overview", { waitUntil: "domcontentloaded" });
    const rows = page.locator("tr[data-guest-id]");
    await expect(rows.first()).toBeVisible({ timeout: 60_000 });

    const {
      id: workloadId,
      selector: rowSelector,
      row,
    } = await findRowBelowFirstScreen(rows, "data-guest-id");
    const beforeRowTop = await positionElementNearViewportBottom(page, row);
    expect(await readPrimaryViewportScrollTop(page)).toBeGreaterThan(10);
    expect(beforeRowTop).toBeGreaterThan(500);

    const detailSelector = `[data-inline-detail-for="${workloadId}"]`;
    const detailRow = page.locator(detailSelector);
    await startUrlWriteProbe(page);
    await startLayoutObserver(page, {
      rowSelector,
      detailSelector,
      detailOpen: true,
      revealFrom: beforeRowTop,
    });
    await row.click();

    await expect(detailRow).toBeVisible();
    await expect(row.locator("button[aria-controls]").first()).toHaveAttribute(
      "aria-expanded",
      "true",
    );
    const openRowTop = expectInlineDetailRevealedBelowTop(
      await finishLayoutObserver(page),
    );

    // Collapsing holds the row where the operator left it.
    await startLayoutObserver(page, {
      rowSelector,
      detailSelector,
      detailOpen: false,
      holdRowAt: openRowTop,
    });
    await row.click();
    await expect(detailRow).toHaveCount(0);
    expect((await finishLayoutObserver(page)).violations).toEqual([]);
    expect(await stopUrlWriteProbe(page)).toEqual([]);
  });

  test("supports keyboard open-close for workload rows without leaking filter state", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only keyboard interaction proof",
    );

    await ensureMockModeEnabled(page);

    await page.goto("/proxmox/overview", { waitUntil: "domcontentloaded" });
    const firstRow = page.locator("tr[data-guest-id]").first();
    await expect(firstRow).toBeVisible({ timeout: 60_000 });
    const workloadId = (await firstRow.getAttribute("data-guest-id")) ?? "";
    expect(workloadId).not.toBe("");
    // Pin the row by id: the reveal scrolls, and windowing can re-render.
    const rowSelector = `tr[data-guest-id="${workloadId}"]`;
    const row = page.locator(rowSelector);
    const detailSelector = `[data-inline-detail-for="${workloadId}"]`;
    const detailRow = page.locator(detailSelector);

    const toggleButton = row.locator("button[aria-controls]").first();
    await expect(toggleButton).toBeVisible();
    const controlsId = (await toggleButton.getAttribute("aria-controls")) ?? "";
    expect(controlsId).not.toBe("");

    await startUrlWriteProbe(page);
    await toggleButton.focus();
    await expect(toggleButton).toBeFocused();
    // This test owns keyboard toggling, not the reveal geometry, so the open
    // observer only waits for the layout to settle.
    await startLayoutObserver(page, {
      rowSelector,
      detailSelector,
      detailOpen: true,
      restartOnMovement: true,
    });
    await page.keyboard.press("Enter");

    await expect(toggleButton).toHaveAttribute("aria-expanded", "true");
    await expect(detailRow).toBeVisible();
    await expect(detailRow.locator(`[id="${controlsId}"]`)).toHaveCount(1);
    const opened = await finishLayoutObserver(page);
    expect(opened.violations).toEqual([]);

    // Collapsing from the keyboard holds the row in place too.
    await toggleButton.focus();
    await startLayoutObserver(page, {
      rowSelector,
      detailSelector,
      detailOpen: false,
      holdRowAt: opened.settled!.rowTop!,
    });
    await toggleButton.press("Space");
    await expect(toggleButton).toHaveAttribute("aria-expanded", "false");
    await expect(detailRow).toHaveCount(0);
    expect((await finishLayoutObserver(page)).violations).toEqual([]);
    expect(await stopUrlWriteProbe(page)).toEqual([]);
  });

  test("reveals storage inline detail without hard-centering the selected row", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only storage interaction proof",
    );

    await ensureMockModeEnabled(page);

    // The default Priority sort breaks ties on live utilisation, so a mock
    // update can reorder pools mid-check. Host order (ties by name) is static.
    await page.goto("/proxmox/storage?sort=host&order=asc", {
      waitUntil: "domcontentloaded",
    });
    const rows = page.locator("tr[data-summary-series-id]");
    await expect(rows.first()).toBeVisible({ timeout: 60_000 });

    const {
      id: seriesId,
      selector: rowSelector,
      row,
    } = await findRowBelowFirstScreen(rows, "data-summary-series-id");
    const beforeRowTop = await positionElementNearViewportBottom(page, row);
    expect(await readPrimaryViewportScrollTop(page)).toBeGreaterThan(10);
    expect(beforeRowTop).toBeGreaterThan(500);

    const detailSelector = `[data-inline-detail-for="${seriesId}"]`;
    await startUrlWriteProbe(page);
    await startLayoutObserver(page, {
      rowSelector,
      detailSelector,
      detailOpen: true,
      revealFrom: beforeRowTop,
    });
    await row.click();

    await expect(page.locator(detailSelector)).toBeVisible();
    expectInlineDetailRevealedBelowTop(await finishLayoutObserver(page));
    expect(await stopUrlWriteProbe(page)).toEqual([]);
  });
});
