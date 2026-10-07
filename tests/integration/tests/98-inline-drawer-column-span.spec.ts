import { expect, test as base, type Locator, type Page } from '@playwright/test';

import {
  ensureAuthenticated,
  getMockMode,
  setMockMode,
  waitForDefaultMockRuntimeReady,
} from './helpers';

let mockModeWasEnabled: boolean | null = null;

const HTTP_CREDENTIALS = {
  username: 'admin',
  password: 'adminadminadmin',
};

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

async function readLayout(drawer: Locator) {
  return drawer.evaluate((row: HTMLTableRowElement) => {
    const table = row.closest('table')!;
    // Only the outer table's own header row: drawer content may hold tables.
    const headerRows = table.tHead!.rows;
    const headers = Array.from(headerRows[headerRows.length - 1].cells).filter(
      (cell) => getComputedStyle(cell).display !== 'none' && cell.getClientRects().length > 0,
    );
    return {
      headerSpan: headers.reduce((total, cell) => total + cell.colSpan, 0),
      drawerSpan: row.cells[0].colSpan,
      drawerRight: row.cells[0].getBoundingClientRect().right,
      headerRight: headers[headers.length - 1].getBoundingClientRect().right,
      tableRight: table.getBoundingClientRect().right,
    };
  });
}

async function expectAligned(drawer: Locator, headerSpan: number): Promise<void> {
  await expect(async () => {
    const layout = await readLayout(drawer);
    expect(layout.headerSpan, 'Visible header span').toBe(headerSpan);
    expect(layout.drawerSpan, 'Drawer colspan matches visible headers').toBe(layout.headerSpan);
    expect(
      Math.abs(layout.drawerRight - layout.tableRight),
      `Drawer reaches table right edge: ${JSON.stringify(layout)}`,
    ).toBeLessThanOrEqual(1);
    expect(
      Math.abs(layout.headerRight - layout.tableRight),
      `Last visible header reaches table right edge: ${JSON.stringify(layout)}`,
    ).toBeLessThanOrEqual(1);
    expect(
      Math.abs(layout.drawerRight - layout.headerRight),
      'Drawer and header right edges',
    ).toBeLessThanOrEqual(1);
  }).toPass({ timeout: 10_000 });
}

test.describe.serial('Inline machine drawer column span', () => {
  test.setTimeout(240_000);

  test.afterAll(async ({ browser }) => {
    if (mockModeWasEnabled === null) return;

    const context = await browser.newContext({
      httpCredentials: HTTP_CREDENTIALS,
    });
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

  test('resynchronizes a mounted drawer after hiding and restoring System', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name !== 'chromium', 'Chromium fixed-table layout regression');
    await page.setViewportSize({ width: 1440, height: 900 });
    await ensureMockModeEnabled(page);
    await page.goto('/standalone', { waitUntil: 'domcontentloaded' });

    const surface = page.getByTestId('standalone-page');
    const summary = surface
      .getByRole('row')
      .filter({
        has: page.locator('button[aria-controls^="agents-machine-drawer"]'),
      })
      .first();
    const disclosure = summary.locator('button[aria-controls^="agents-machine-drawer"]');
    await expect(disclosure).toBeVisible({ timeout: 60_000 });
    await summary.getByRole('cell').nth(1).click();

    const drawer = surface.locator('tr[data-inline-platform-resource-detail-for]');
    await expect(drawer).toHaveCount(1);
    await expect(drawer).toBeVisible();
    const originalRow = (await drawer.elementHandle())!;
    await originalRow.evaluate((row) => row.setAttribute('data-column-span-test', 'original'));
    const focusTarget = (await drawer
      .getByRole('button', { name: /^Collapse .* details$/ })
      .elementHandle())!;
    await focusTarget.evaluate((button: HTMLElement) => button.focus({ preventScroll: true }));

    const scrollHandle = await drawer.evaluateHandle((row) => {
      return (
        Array.from(row.querySelectorAll<HTMLElement>('*')).find((element) => {
          const style = getComputedStyle(element);
          return (
            element.getClientRects().length > 0 &&
            ((/^(auto|scroll)$/.test(style.overflowY) &&
              element.scrollHeight > element.clientHeight) ||
              (/^(auto|scroll)$/.test(style.overflowX) &&
                element.scrollWidth > element.clientWidth))
          );
        }) ?? null
      );
    });
    const scrollElement = scrollHandle.asElement();
    const scrollOffset = scrollElement
      ? await scrollElement.evaluate((element) => {
          element.scrollTop = Math.min(32, element.scrollHeight - element.clientHeight);
          element.scrollLeft = Math.min(32, element.scrollWidth - element.clientWidth);
          return { top: element.scrollTop, left: element.scrollLeft };
        })
      : null;

    const expectContinuity = async () => {
      expect(
        await originalRow.evaluate(
          (row) =>
            row.isConnected && document.querySelector('[data-column-span-test="original"]') === row,
        ),
        'Original drawer row stays mounted',
      ).toBe(true);
      expect(
        await focusTarget.evaluate(
          (button) =>
            document.activeElement === button &&
            button.closest('tr')?.hasAttribute('data-column-span-test'),
        ),
        'Original interactive element keeps focus inside the drawer',
      ).toBe(true);
      if (scrollElement && scrollOffset) {
        const current = await scrollElement.evaluate((element) => ({
          connected: element.isConnected,
          top: element.scrollTop,
          left: element.scrollLeft,
        }));
        expect(current.connected, 'Original inner scroll container stays mounted').toBe(true);
        expect(current.top, 'Inner vertical scroll survives column changes').toBe(scrollOffset.top);
        expect(current.left, 'Inner horizontal scroll survives column changes').toBe(
          scrollOffset.left,
        );
      }
    };

    const initialSpan = (await readLayout(drawer)).headerSpan;
    const view = surface.getByRole('button', { name: 'View', exact: true });
    const preferences = surface.getByRole('region', {
      name: 'View preferences',
      exact: true,
    });

    for (const visible of [false, true]) {
      await test.step(
        visible
          ? 'Show System after the reduced table has laid out'
          : 'Hide System with the drawer open',
        async () => {
          await view.click();
          await preferences.getByRole('button', { name: /^Columns\b/ }).click();
          const system = preferences.getByRole('checkbox', {
            name: 'System',
            exact: true,
          });
          await expect(system).toBeChecked({ checked: !visible });
          await focusTarget.evaluate((button: HTMLElement) =>
            button.focus({ preventScroll: true }),
          );
          // Native checkbox activation preserves drawer focus during the column mutation.
          await system.evaluate((checkbox: HTMLInputElement) => checkbox.click());
          await expect(system).toBeChecked({ checked: visible });
          await drawer.evaluate(
            (row) =>
              new Promise<void>((resolve) => {
                // Force the reduced table to lay out before restoring the column.
                void row.getBoundingClientRect().width;
                requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
              }),
          );
          await expectAligned(drawer, initialSpan - (visible ? 0 : 1));
          await expectContinuity();
          await view.click();
          await expect(preferences).toBeHidden();
          await focusTarget.evaluate((button: HTMLElement) =>
            button.focus({ preventScroll: true }),
          );
          await expectContinuity();
        },
      );
    }
  });
});
