import fs from 'node:fs';
import path from 'node:path';
import { expect, test as base, type Locator, type Page, type TestInfo } from '@playwright/test';

import {
  ensureAuthenticated,
  getMockMode,
  setMockMode,
  waitForDefaultMockRuntimeReady,
} from './helpers';

// Interaction stability: layout must not move while a person hovers, scrubs
// or watches live readouts. This is the hover counterpart of "table rows stay
// single-line": a readout may change its own text, but never the position or
// size of the structure around it.
//
// The probe is a measurement rig, not a screenshot judgement. For every
// hovered region it records the boxes of the surrounding structural elements
// (the hovered element itself, each ancestor up to the surface, and the
// ancestors' neighbours), steps the pointer across the region, and compares
// every step with the box taken before the pointer arrived. A layout-shift
// PerformanceObserver runs alongside: pointer movement does not set
// hadRecentInput, so shifts caused by hover readouts are reported in full.
//
// PULSE_INTERACTION_STABILITY_REPORT=<path> appends one JSON line per surface
// so a sweep over main can be read as a list instead of a test log.

const HTTP_CREDENTIALS = {
  username: 'admin',
  password: 'adminadminadmin',
};

// Boxes are compared to the pre-hover box. A full pixel absorbs subpixel
// rounding; anything larger is a visible jump.
const BOX_TOLERANCE_PX = 1;
// Layout-shift score allowed per probed region and phase. Hover readouts
// that only change their own text score 0.
const LAYOUT_SHIFT_TOLERANCE = 0.001;
const CHART_SWEEP_STEPS = 40;
const ROW_SWEEP_STEPS = 30;
const LIVE_WATCH_MS = 8_000;
const LIVE_WATCH_INTERVAL_MS = 500;
const SIBLING_WINDOW = 2;
const MAX_FINDINGS_PER_SURFACE = 40;

type Box = { x: number; y: number; w: number; h: number };
type Snapshot = Record<string, Box | null>;

type Finding = {
  surface: string;
  phase: string;
  step: number;
  landmark: string;
  delta: { dx: number; dy: number; dw: number; dh: number } | 'detached';
  readout?: string;
  readoutBefore?: string;
  // Steps in the phase at which the box differed from the reference, and the
  // distinct heights it took: several heights mean it bounced.
  movedSteps?: number;
  heights?: number[];
};

type ShiftSummary = {
  score: number;
  sources: string[];
};

type RegionResult = {
  surface: string;
  findings: Finding[];
  shifts: { phase: string; summary: ShiftSummary }[];
};

type RegisterOptions = {
  // Positions are measured relative to this ancestor of the hovered element,
  // so a live update that moves the whole surface (a row above re-sorting)
  // is not blamed on the hover. The scope's own size is still a landmark.
  scopeSelector: string;
  // Extra landmarks resolved with querySelectorAll from the scope, or from
  // the closest table when prefixed with "table:".
  extraSelectors?: string[];
  // Text of this selector (from the scope) is captured with each finding.
  readoutSelector?: string;
};

let mockModeWasEnabled: boolean | null = null;

const test = base;
test.use({ httpCredentials: HTTP_CREDENTIALS });

async function ensureMockModeEnabled(page: Page): Promise<void> {
  await ensureAuthenticated(page);
  const state = await getMockMode(page);
  if (mockModeWasEnabled === null) {
    mockModeWasEnabled = state.enabled;
  }
  if (!state.enabled) {
    await setMockMode(page, true);
  } else {
    await waitForDefaultMockRuntimeReady(page);
  }
}

// Installed before any page script so the observer sees every shift.
function installLayoutShiftObserver(): void {
  type ShiftWindow = Window & {
    __pulseLayoutShifts?: { t: number; value: number; sources: string[] }[];
  };
  const target = window as ShiftWindow;
  if (target.__pulseLayoutShifts) return;
  target.__pulseLayoutShifts = [];
  // Names the shifted node by its own class, the nearest test id and, inside
  // a table, the column header above it, plus how far it moved.
  const describe = (source: {
    node?: Node | null;
    previousRect?: DOMRectReadOnly;
    currentRect?: DOMRectReadOnly;
  }): string => {
    const node = source.node;
    const element = node instanceof Element ? node : (node?.parentElement ?? null);
    if (!element) return node?.nodeName?.toLowerCase() ?? 'unknown';
    const classes =
      typeof element.className === 'string' ? element.className.trim().split(/\s+/) : [];
    let name = `${node instanceof Element ? '' : '#text in '}${element.tagName.toLowerCase()}${
      classes.length ? `.${classes.slice(0, 3).join('.')}` : ''
    }`;
    const testId = element.closest('[data-testid]')?.getAttribute('data-testid');
    if (testId) name += ` in [${testId}]`;
    const cell = element.closest('td, th') as HTMLTableCellElement | null;
    const headerRow = cell?.closest('table')?.tHead?.rows;
    if (cell && headerRow?.length) {
      const header = headerRow[headerRow.length - 1].cells[cell.cellIndex];
      name += ` in column "${(header?.textContent ?? '').trim().slice(0, 24) || cell.cellIndex}"`;
    }
    const before = source.previousRect;
    const after = source.currentRect;
    if (before && after) {
      const moves = [
        ['dx', after.x - before.x],
        ['dy', after.y - before.y],
        ['dw', after.width - before.width],
        ['dh', after.height - before.height],
      ]
        .filter(([, value]) => Math.abs(value as number) >= 0.5)
        .map(([key, value]) => `${key}=${Math.round((value as number) * 10) / 10}`);
      if (moves.length) name += ` (${moves.join(' ')})`;
    }
    return name;
  };
  // Tooltips and popovers float above the page and follow the pointer by
  // design; their movement is the overlay, not the layout under it.
  const isOverlay = (node: Node | null | undefined): boolean => {
    for (
      let element = node instanceof Element ? node : (node?.parentElement ?? null);
      element;
      element = element.parentElement
    ) {
      if (element.getAttribute('role') === 'tooltip') return true;
      if (getComputedStyle(element).position === 'fixed') return true;
    }
    return false;
  };
  try {
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries() as (PerformanceEntry & {
        value: number;
        sources?: {
          node?: Node | null;
          previousRect?: DOMRectReadOnly;
          currentRect?: DOMRectReadOnly;
        }[];
      })[]) {
        const sources = entry.sources ?? [];
        const inFlow = sources.filter((source) => !isOverlay(source.node));
        // An entry whose every source is an overlay is dropped; a mixed entry
        // keeps its score but names only the in-flow nodes.
        if (sources.length > 0 && inFlow.length === 0) continue;
        target.__pulseLayoutShifts!.push({
          t: entry.startTime,
          value: entry.value,
          sources: inFlow.map(describe),
        });
      }
    }).observe({ type: 'layout-shift', buffered: true });
  } catch {
    // Browsers without layout-shift entries still get the box comparison.
  }
}

async function nextFrames(page: Page): Promise<void> {
  await page.evaluate(
    () =>
      new Promise<void>((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
      ),
  );
}

async function shiftMark(page: Page): Promise<number> {
  return page.evaluate(() => performance.now());
}

async function readShifts(page: Page, from: number, to = Infinity): Promise<ShiftSummary> {
  return page.evaluate(
    ([start, end]) => {
      const entries =
        (
          window as Window & {
            __pulseLayoutShifts?: { t: number; value: number; sources: string[] }[];
          }
        ).__pulseLayoutShifts ?? [];
      const recent = entries.filter((entry) => entry.t >= start && entry.t < end);
      return {
        score: recent.reduce((total, entry) => total + entry.value, 0),
        sources: Array.from(new Set(recent.flatMap((entry) => entry.sources))).slice(0, 8),
      };
    },
    [from, to] as const,
  );
}

// Registers the structural landmarks around `target` in the page and returns
// their names. Landmarks are block-level boxes; inline text whose width
// follows its value is the readout itself, not structure.
async function registerLandmarks(target: Locator, options: RegisterOptions): Promise<string[]> {
  return target.evaluate(
    (element, args) => {
      type Registry = {
        scope: Element;
        entries: { name: string; el: Element; relative: boolean }[];
        readoutSelector?: string;
      };
      const scope = element.closest(args.scopeSelector);
      if (!scope) throw new Error(`No scope ${args.scopeSelector} around the hovered element`);

      const label = (el: Element): string => {
        const testId = el.getAttribute('data-testid');
        const group = el.getAttribute('data-history-group');
        const firstClass =
          typeof el.className === 'string' ? (el.className.trim().split(/\s+/)[0] ?? '') : '';
        const index = el.parentElement ? Array.from(el.parentElement.children).indexOf(el) : 0;
        if (testId) return `${el.tagName.toLowerCase()}[${testId}${group ? `:${group}` : ''}]`;
        return `${el.tagName.toLowerCase()}${firstClass ? `.${firstClass}` : ''}:${index}`;
      };
      const isStructural = (el: Element): boolean => {
        const style = getComputedStyle(el);
        if (style.position === 'absolute' || style.position === 'fixed') return false;
        if (style.display === 'none' || style.display === 'contents') return false;
        if (style.display.startsWith('inline')) return false;
        const rect = el.getBoundingClientRect();
        return rect.width > 2 && rect.height > 2;
      };

      const entries: Registry['entries'] = [];
      const seen = new Set<Element>();
      const add = (el: Element, name: string, relative = true) => {
        if (seen.has(el)) return;
        seen.add(el);
        entries.push({ name, el, relative });
      };

      add(element, `hovered ${label(element)}`);
      let depth = 0;
      let current: Element | null = element;
      while (current && current !== scope) {
        const parent: Element | null = current.parentElement;
        if (!parent) break;
        const siblings = Array.from(parent.children);
        const index = siblings.indexOf(current);
        for (
          let i = Math.max(0, index - args.siblingWindow);
          i <= Math.min(siblings.length - 1, index + args.siblingWindow);
          i += 1
        ) {
          const sibling = siblings[i];
          if (sibling !== current && isStructural(sibling)) {
            add(sibling, `${label(sibling)} beside ${label(current)}`);
          }
        }
        depth += 1;
        if (parent !== scope && isStructural(parent))
          add(parent, `ancestor ${depth} ${label(parent)}`);
        current = parent;
      }
      add(scope, `scope ${label(scope)}`);

      for (const selector of args.extraSelectors) {
        const fromTable = selector.startsWith('table:');
        const root = fromTable ? scope.closest('table') : scope;
        if (!root) continue;
        const query = fromTable ? selector.slice('table:'.length) : selector;
        for (const extra of Array.from(root.querySelectorAll(query))) {
          if (isStructural(extra)) {
            // Table-level landmarks (header, neighbouring rows) are absolute:
            // nothing on the page should move them while a pointer moves.
            add(extra, `${fromTable ? 'table' : 'scope'} ${query} ${label(extra)}`, !fromTable);
          }
        }
      }

      (window as Window & { __pulseStability?: Registry }).__pulseStability = {
        scope,
        entries,
        readoutSelector: args.readoutSelector,
      };
      return entries.map((entry) => entry.name);
    },
    {
      scopeSelector: options.scopeSelector,
      extraSelectors: options.extraSelectors ?? [],
      readoutSelector: options.readoutSelector,
      siblingWindow: SIBLING_WINDOW,
    },
  );
}

async function snapshot(page: Page): Promise<{ boxes: Snapshot; readout: string }> {
  return page.evaluate(() => {
    const registry = (
      window as Window & {
        __pulseStability?: {
          scope: Element;
          entries: { name: string; el: Element; relative: boolean }[];
          readoutSelector?: string;
        };
      }
    ).__pulseStability;
    if (!registry) throw new Error('Landmarks were not registered');
    const origin = registry.scope.getBoundingClientRect();
    const boxes: Record<string, { x: number; y: number; w: number; h: number } | null> = {};
    for (const entry of registry.entries) {
      if (!entry.el.isConnected) {
        boxes[entry.name] = null;
        continue;
      }
      const rect = entry.el.getBoundingClientRect();
      const isScope = entry.el === registry.scope;
      boxes[entry.name] = {
        x: isScope ? 0 : rect.left - (entry.relative ? origin.left : 0),
        y: isScope ? 0 : rect.top - (entry.relative ? origin.top : 0),
        w: rect.width,
        h: rect.height,
      };
    }
    const readout = registry.readoutSelector
      ? Array.from(registry.scope.querySelectorAll(registry.readoutSelector))
          .map((node) => ((node as HTMLElement).innerText ?? '').replace(/\s+/g, ' ').trim())
          .filter(Boolean)
          .join(' | ')
      : '';
    return { boxes, readout };
  });
}

function compare(
  surface: string,
  phase: string,
  step: number,
  baseline: { boxes: Snapshot; readout: string },
  current: { boxes: Snapshot; readout: string },
  findings: Finding[],
  reported: Map<string, Finding>,
): void {
  for (const [landmark, before] of Object.entries(baseline.boxes)) {
    if (!before) continue;
    const after = current.boxes[landmark];
    const key = `${phase}:${landmark}`;
    const earlier = reported.get(key);
    if (!after) {
      if (earlier) continue;
      const finding: Finding = {
        surface,
        phase,
        step,
        landmark,
        delta: 'detached',
        readout: current.readout,
        readoutBefore: baseline.readout,
      };
      reported.set(key, finding);
      findings.push(finding);
      continue;
    }
    const delta = {
      dx: Math.round((after.x - before.x) * 10) / 10,
      dy: Math.round((after.y - before.y) * 10) / 10,
      dw: Math.round((after.w - before.w) * 10) / 10,
      dh: Math.round((after.h - before.h) * 10) / 10,
    };
    if (!Object.values(delta).some((value) => Math.abs(value) > BOX_TOLERANCE_PX)) continue;
    if (earlier) {
      // Later steps only count: a box that keeps changing size from point to
      // point is the bounce a person sees, a single jump is a toggle.
      earlier.movedSteps = (earlier.movedSteps ?? 1) + 1;
      const size = Math.round(after.h);
      if (!earlier.heights!.includes(size)) earlier.heights!.push(size);
      continue;
    }
    // Reported once per phase, at the first step it moved.
    const finding: Finding = {
      surface,
      phase,
      step,
      landmark,
      delta,
      readout: current.readout,
      readoutBefore: baseline.readout,
      movedSteps: 1,
      heights: [Math.round(before.h), Math.round(after.h)],
    };
    reported.set(key, finding);
    findings.push(finding);
  }
}

async function parkPointer(page: Page): Promise<void> {
  // Bottom-right corner of the viewport: outside every probed region and
  // clear of the navigation rail.
  const viewport = page.viewportSize() ?? { width: 1440, height: 900 };
  await page.mouse.move(viewport.width - 2, viewport.height - 2);
  await nextFrames(page);
}

type SweepPath = { x: number; y: number }[];
type PageBox = { x: number; y: number; width: number; height: number };

function horizontalPath(box: PageBox, steps: number) {
  const inset = Math.min(2, box.width / 4);
  return Array.from({ length: steps }, (_, index) => ({
    x: box.x + inset + ((box.width - 2 * inset) * index) / Math.max(1, steps - 1),
    y: box.y + box.height / 2,
  }));
}

function verticalPath(box: PageBox, xRatio: number, steps: number) {
  return Array.from({ length: steps }, (_, index) => ({
    x: box.x + box.width * xRatio,
    y: box.y + 2 + ((box.height - 4) * index) / Math.max(1, steps - 1),
  }));
}

// Hover sweep, pointer leave, then a stationary dwell through live updates.
async function probeRegion(
  page: Page,
  surface: string,
  target: Locator,
  options: RegisterOptions & { paths: (box: PageBox) => SweepPath[]; dwell?: boolean },
): Promise<RegionResult> {
  const findings: Finding[] = [];
  const reported = new Map<string, Finding>();
  await target.scrollIntoViewIfNeeded();
  await parkPointer(page);
  // Paths are planned after the scroll, from where the region now sits.
  const box = await target.boundingBox();
  if (!box) throw new Error(`${surface} has no box to sweep`);
  const paths = options.paths(box);
  await registerLandmarks(target, options);
  const baseline = await snapshot(page);
  const shiftsFrom = await shiftMark(page);

  let step = 0;
  for (const sweep of paths) {
    for (const point of sweep) {
      await page.mouse.move(point.x, point.y);
      await nextFrames(page);
      compare(surface, 'hover sweep', step, baseline, await snapshot(page), findings, reported);
      step += 1;
    }
  }

  const sweepEnded = await shiftMark(page);
  const shifts: RegionResult['shifts'] = [
    { phase: 'hover sweep', summary: await readShifts(page, shiftsFrom, sweepEnded) },
  ];

  if (options.dwell) {
    const middle = paths[0][Math.floor(paths[0].length / 2)];
    await page.mouse.move(middle.x, middle.y);
    await nextFrames(page);
    // Compared with the box under the pointer, not the pre-hover box: the
    // dwell asks whether live readouts move the layout, not whether hover did.
    const held = await snapshot(page);
    const ticks = Math.ceil(LIVE_WATCH_MS / LIVE_WATCH_INTERVAL_MS);
    for (let tick = 1; tick <= ticks; tick += 1) {
      await page.waitForTimeout(LIVE_WATCH_INTERVAL_MS);
      compare(
        surface,
        'hover dwell (live updates)',
        tick,
        held,
        await snapshot(page),
        findings,
        reported,
      );
    }
    shifts.push({
      phase: 'hover dwell (live updates)',
      summary: await readShifts(page, sweepEnded),
    });
  }

  const leaving = await shiftMark(page);
  await parkPointer(page);
  compare(surface, 'pointer left', 0, baseline, await snapshot(page), findings, reported);
  shifts.push({ phase: 'pointer left', summary: await readShifts(page, leaving) });
  return { surface, findings, shifts };
}

function formatFindings(findings: Finding[]): string {
  return findings
    .slice(0, MAX_FINDINGS_PER_SURFACE)
    .map((finding) => {
      const delta =
        finding.delta === 'detached'
          ? 'detached'
          : Object.entries(finding.delta)
              .filter(([, value]) => value !== 0)
              .map(([key, value]) => `${key}=${value}`)
              .join(' ');
      const readout =
        finding.readout && finding.readout !== finding.readoutBefore
          ? ` | readout "${finding.readoutBefore ?? ''}" -> "${finding.readout}"`
          : finding.readout
            ? ` | readout "${finding.readout}"`
            : '';
      const moved = finding.movedSteps
        ? ` | moved on ${finding.movedSteps} step(s), heights ${finding.heights!.join('/')}px`
        : '';
      return `${finding.surface} | ${finding.phase} step ${finding.step} | ${finding.landmark} | ${delta}${moved}${readout}`;
    })
    .join('\n');
}

async function finishSurface(
  testInfo: TestInfo,
  surface: string,
  results: RegionResult[],
): Promise<void> {
  const findings = results.flatMap((result) => result.findings);
  for (const result of results) {
    for (const { phase, summary } of result.shifts) {
      if (summary.score <= LAYOUT_SHIFT_TOLERANCE) continue;
      findings.push({
        surface: result.surface,
        phase: `${phase} layout-shift`,
        step: 0,
        landmark: `score ${summary.score.toFixed(4)} from ${summary.sources.join('; ') || 'unattributed nodes'}`,
        delta: { dx: 0, dy: 0, dw: 0, dh: 0 },
      });
    }
  }
  const reportPath = String(process.env.PULSE_INTERACTION_STABILITY_REPORT || '').trim();
  if (reportPath) {
    // Appended per surface: a failed test restarts the worker, so module
    // state does not survive to an afterAll hook.
    fs.mkdirSync(path.dirname(path.resolve(reportPath)), { recursive: true });
    fs.appendFileSync(
      path.resolve(reportPath),
      `${JSON.stringify({ surface, findings, shifts: results.map((r) => ({ surface: r.surface, shifts: r.shifts })) })}\n`,
    );
  }
  await testInfo.attach(`${surface}.json`, {
    body: JSON.stringify({ surface, findings, results }, null, 2),
    contentType: 'application/json',
  });
  expect(
    findings,
    `Layout moved while interacting with ${surface}:\n${formatFindings(findings)}`,
  ).toEqual([]);
}

async function openProxmoxOverview(page: Page): Promise<void> {
  await page.goto('/proxmox/overview', { waitUntil: 'domcontentloaded' });
  await expect(page.locator('tr[data-guest-id]').first()).toBeVisible({ timeout: 60_000 });
}

async function waitForHistoryCharts(drawer: Locator): Promise<Locator> {
  const plots = drawer.getByTestId('guest-history-plot');
  await expect(plots.first()).toBeVisible({ timeout: 30_000 });
  await expect(drawer.locator('path[data-history-series]').first()).toBeAttached({
    timeout: 30_000,
  });
  // Let the lazy chunk, the first read and the scale settle before measuring.
  await drawer.page().waitForTimeout(1_000);
  return plots;
}

async function probeDrawerHistory(
  page: Page,
  surface: string,
  drawerSelector: string,
): Promise<RegionResult[]> {
  const drawer = page.locator(drawerSelector);
  await expect(drawer).toBeVisible();
  const results: RegionResult[] = [];
  // The history panel is a lazy chunk: its range picker exists only once the
  // charts do.
  await waitForHistoryCharts(drawer);
  const rangeControl = drawer.getByTestId('guest-history-range-control');
  const ranges =
    (await rangeControl.count()) > 0
      ? await rangeControl
          .first()
          .evaluate((select: HTMLSelectElement) =>
            Array.from(select.options).map((option) => option.value),
          )
      : [''];
  // The default range plus the deepest one: bucketed history is where peak
  // labels appear.
  const deepest = ranges.includes('7d') ? '7d' : ranges[ranges.length - 1];
  const probeRanges = Array.from(new Set([ranges[0], deepest]));
  for (const range of probeRanges) {
    if (range && (await rangeControl.count()) > 0) {
      await rangeControl.first().selectOption(range);
    }
    const plots = await waitForHistoryCharts(drawer);
    const count = await plots.count();
    for (let index = 0; index < count; index += 1) {
      const plot = plots.nth(index);
      if (!(await plot.isVisible())) continue;
      const group =
        (await plot.evaluate(
          (svg) => svg.closest('[data-history-group]')?.getAttribute('data-history-group') ?? '',
        )) || String(index);
      results.push(
        await probeRegion(page, `${surface} ${range || 'default'} ${group} chart`, plot, {
          scopeSelector: drawerSelector,
          extraSelectors: ['table::scope > thead', `table::scope > tbody > ${drawerSelector} + tr`],
          readoutSelector: `[data-history-group="${group}"] > div:first-child`,
          paths: (box) => [horizontalPath(box, CHART_SWEEP_STEPS)],
          dwell: index === 0,
        }),
      );
      // Re-measure from the new pointer-free state for the next chart.
      await parkPointer(page);
    }
  }
  return results;
}

// Not serial: each surface reports on its own, so one offender does not hide
// the next.
test.describe('Interaction stability', () => {
  test.setTimeout(300_000);

  test.beforeEach(async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== 'chromium', 'Pointer hover is a desktop interaction');
    await page.addInitScript(installLayoutShiftObserver);
    await ensureMockModeEnabled(page);
  });

  test.afterAll(async ({ browser }) => {
    if (mockModeWasEnabled === null) return;
    const context = await browser.newContext({ httpCredentials: HTTP_CREDENTIALS });
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

  // Drawers open from the keyboard: live updates keep rows re-rendering,
  // which a pointer click's stability wait can mistake for movement.
  for (const width of [1440, 1280]) {
    test(`guest drawer history charts hold their layout while scrubbed at ${width}px`, async ({
      page,
    }, testInfo) => {
      await page.setViewportSize({ width, height: 900 });
      await openProxmoxOverview(page);
      const row = page.locator('tr[data-guest-id]').first();
      const guestId = (await row.getAttribute('data-guest-id')) ?? '';
      expect(guestId).not.toBe('');
      await row.locator('button[aria-controls]').first().focus();
      await page.keyboard.press('Enter');
      const results = await probeDrawerHistory(
        page,
        `guest drawer ${width}px`,
        `tr[data-inline-detail-for="${guestId}"]`,
      );
      expect(results.length, 'probed at least one history chart').toBeGreaterThan(0);
      await finishSurface(testInfo, `guest-drawer-history-${width}`, results);
    });
  }

  test('node drawer history charts hold their layout while scrubbed', async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await openProxmoxOverview(page);
    const row = page.locator('tr[data-proxmox-host-row]').first();
    await expect(row).toBeVisible({ timeout: 30_000 });
    const nodeId = (await row.getAttribute('data-proxmox-host-row')) ?? '';
    await row.locator('button[aria-controls^="proxmox-host-drawer-"]').first().focus();
    await page.keyboard.press('Enter');
    const results = await probeDrawerHistory(
      page,
      'node drawer 1440px',
      `tr[data-inline-detail-for="${nodeId}"]`,
    );
    expect(results.length, 'probed at least one history chart').toBeGreaterThan(0);
    await finishSurface(testInfo, 'node-drawer-history-1440', results);
  });

  test('workload row sparklines hold the table while hovered', async ({ page }, testInfo) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    // Rows draw bars by default; Trends swaps every metric cell to a sparkline.
    await page.addInitScript(() =>
      localStorage.setItem('workloadsMetricDisplayMode', 'sparklines'),
    );
    await openProxmoxOverview(page);
    const sparklines = page.locator('tr[data-guest-id] [data-testid="metric-mini-sparkline"] svg');
    await expect(sparklines.first()).toBeVisible({ timeout: 30_000 });
    const results: RegionResult[] = [];
    const count = Math.min(await sparklines.count(), 6);
    for (let index = 0; index < count; index += 1) {
      const sparkline = sparklines.nth(index);
      if (!(await sparkline.isVisible())) continue;
      results.push(
        await probeRegion(page, `workload sparkline ${index}`, sparkline, {
          scopeSelector: 'table',
          extraSelectors: ['table::scope > thead'],
          paths: (box) => [horizontalPath(box, CHART_SWEEP_STEPS)],
          dwell: index === 0,
        }),
      );
    }
    expect(results.length, 'probed at least one sparkline').toBeGreaterThan(0);
    await finishSurface(testInfo, 'workload-row-sparklines', results);
  });

  for (const route of [
    '/proxmox/overview',
    '/proxmox/storage',
    '/docker',
    '/kubernetes',
    '/standalone',
  ]) {
    test(`platform table rows on ${route} hold their height while hovered`, async ({
      page,
    }, testInfo) => {
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto(route, { waitUntil: 'domcontentloaded' });
      const body = page
        .locator('table > tbody')
        .filter({ has: page.locator('tr'), visible: true })
        .first();
      const visible = await body
        .waitFor({ state: 'visible', timeout: 30_000 })
        .then(() => true)
        .catch(() => false);
      test.skip(!visible, `${route} rendered no table in this estate`);
      await page.waitForTimeout(1_000);
      const viewportHeight = page.viewportSize()?.height ?? 900;
      const results = [
        await probeRegion(page, `${route} rows`, body, {
          scopeSelector: 'table',
          extraSelectors: ['table::scope > thead', 'table::scope > tbody > tr:nth-child(-n+12)'],
          // The first screen of rows: the region a person sweeps past on the
          // way to something else.
          paths: (box) => {
            const top = Math.max(box.y, 8);
            const bottom = Math.min(box.y + box.height, top + 420, viewportHeight - 8);
            const region = { ...box, y: top, height: Math.max(0, bottom - top) };
            return [0.15, 0.5, 0.85].map((ratio) => verticalPath(region, ratio, ROW_SWEEP_STEPS));
          },
          dwell: true,
        }),
      ];
      await finishSurface(testInfo, `rows${route.replace(/\//g, '-')}`, results);
    });
  }
});
