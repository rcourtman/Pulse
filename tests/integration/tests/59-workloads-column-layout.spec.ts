import { expect, test as base, type Page } from '@playwright/test';

import { getMockMode, setMockMode } from './helpers';

type ColumnHeaderMetric = {
  colId: string | null;
  text: string;
  width: number;
};

type ValueMetric = {
  text: string;
  clientWidth: number;
  scrollWidth: number;
  overflow: boolean;
};

type WorkloadsColumnLayout = {
  overflowX: string;
  wrapperClientWidth: number;
  wrapperScrollWidth: number;
  tableScrollWidth: number;
  headers: ColumnHeaderMetric[];
  typeWidths: number[];
  netIoWidths: number[];
  diskIoWidths: number[];
  ioValueSpans: Array<{
    columnId: 'netIo' | 'diskIo';
    values: ValueMetric[];
  }>;
};

let mockModeWasEnabled: boolean | null = null;

const HTTP_CREDENTIALS = {
  username: 'admin',
  password: 'adminadminadmin',
};

const test = base;

test.use({
  httpCredentials: HTTP_CREDENTIALS,
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

async function readWorkloadsColumnLayout(page: Page): Promise<WorkloadsColumnLayout> {
  return page.evaluate(() => {
    const surface = document.querySelector<HTMLElement>('[data-testid="workloads-table-surface"]');
    const wrapper = surface?.querySelector<HTMLDivElement>('div.overflow-x-auto') ?? null;
    const table = wrapper?.querySelector<HTMLTableElement>('table') ?? null;
    const style = wrapper ? window.getComputedStyle(wrapper) : null;
    const headers = Array.from(surface?.querySelectorAll<HTMLTableCellElement>('thead th') ?? []).map(
      (header) => ({
        colId: header.getAttribute('data-workload-col'),
        text: header.textContent?.replace(/\s+/g, ' ').trim() ?? '',
        width: Math.round(header.getBoundingClientRect().width),
      }),
    );
    const columnIndexById = new Map(headers.map((header, index) => [header.colId, index] as const));
    const rows = Array.from(surface?.querySelectorAll<HTMLTableRowElement>('tr[data-guest-id]') ?? []).slice(
      0,
      12,
    );

    const readColumnWidths = (columnId: string): number[] => {
      const columnIndex = columnIndexById.get(columnId);
      if (columnIndex === undefined) return [];
      return rows
        .map((row) => row.querySelectorAll<HTMLTableCellElement>('td')[columnIndex])
        .filter((cell): cell is HTMLTableCellElement => Boolean(cell))
        .map((cell) => Math.round(cell.getBoundingClientRect().width));
    };

    const ioValueSpans = (['netIo', 'diskIo'] as const).flatMap((columnId) => {
      const columnIndex = columnIndexById.get(columnId);
      if (columnIndex === undefined) return [];
      return rows
        .map((row) => row.querySelectorAll<HTMLTableCellElement>('td')[columnIndex])
        .filter((cell): cell is HTMLTableCellElement => Boolean(cell))
        .map((cell) => ({
          columnId,
          values: Array.from(cell.querySelectorAll<HTMLSpanElement>('span'))
            .map((span) => ({
              text: span.textContent?.trim() ?? '',
              clientWidth: span.clientWidth,
              scrollWidth: span.scrollWidth,
              overflow: span.scrollWidth > span.clientWidth,
            }))
            .filter((value) => /\d/.test(value.text)),
        }))
        .filter((entry) => entry.values.length > 0);
    });

    return {
      overflowX: style?.overflowX ?? '',
      wrapperClientWidth: wrapper?.clientWidth ?? 0,
      wrapperScrollWidth: wrapper?.scrollWidth ?? 0,
      tableScrollWidth: table?.scrollWidth ?? 0,
      headers,
      typeWidths: readColumnWidths('type'),
      netIoWidths: readColumnWidths('netIo'),
      diskIoWidths: readColumnWidths('diskIo'),
      ioValueSpans,
    };
  });
}

test.describe.serial('Workloads column layout', () => {
  test.afterAll(async ({ browser }) => {
    if (mockModeWasEnabled === null) {
      return;
    }

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

  test('keeps Type and I/O columns readable at 1440px', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');

    await page.setViewportSize({ width: 1440, height: 1200 });
    await page.addInitScript(() => {
      localStorage.setItem('pulse_whats_new_v2_shown', 'true');
    });

    await ensureMockModeEnabled(page);
    await page.goto('/workloads', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('workloads-table-surface')).toBeVisible();
    await expect(page.locator('tr[data-guest-id]').first()).toBeVisible();
    await page.waitForTimeout(1000);

    const layout = await readWorkloadsColumnLayout(page);
    const typeHeader = layout.headers.find((header) => header.colId === 'type');
    const netIoHeader = layout.headers.find((header) => header.colId === 'netIo');
    const diskIoHeader = layout.headers.find((header) => header.colId === 'diskIo');
    const overflowingValues = layout.ioValueSpans.flatMap((entry) =>
      entry.values
        .filter((value) => value.overflow)
        .map((value) => ({ columnId: entry.columnId, ...value })),
    );

    expect(typeHeader).toBeDefined();
    expect(netIoHeader).toBeDefined();
    expect(diskIoHeader).toBeDefined();

    expect(typeHeader?.width, 'Type column should stay compact on desktop workloads').toBeLessThanOrEqual(80);
    expect(typeHeader?.width, 'Type column should keep its canonical desktop width floor').toBeGreaterThanOrEqual(60);
    expect(netIoHeader?.width, 'Net I/O column should reserve enough width for both rates').toBeGreaterThanOrEqual(170);
    expect(diskIoHeader?.width, 'Disk I/O column should reserve enough width for both rates').toBeGreaterThanOrEqual(170);

    expect(layout.typeWidths.length, 'Expected visible workload rows with a Type cell').toBeGreaterThan(0);
    expect(Math.max(...layout.typeWidths), 'Type cells should not expand past the compact desktop contract').toBeLessThanOrEqual(80);
    expect(layout.netIoWidths.length, 'Expected visible workload rows with Net I/O cells').toBeGreaterThan(0);
    expect(layout.diskIoWidths.length, 'Expected visible workload rows with Disk I/O cells').toBeGreaterThan(0);
    expect(Math.min(...layout.netIoWidths), 'Net I/O cells should keep the widened desktop width').toBeGreaterThanOrEqual(170);
    expect(Math.min(...layout.diskIoWidths), 'Disk I/O cells should keep the widened desktop width').toBeGreaterThanOrEqual(170);

    expect(['auto', 'scroll']).toContain(layout.overflowX);
    expect(
      layout.tableScrollWidth,
      `Workloads table should fit the 1440px desktop shell without horizontal scrolling (wrapper=${layout.wrapperClientWidth}, table=${layout.tableScrollWidth})`,
    ).toBeLessThanOrEqual(layout.wrapperClientWidth + 1);

    expect(layout.ioValueSpans.length, 'Expected visible workload I/O values to validate').toBeGreaterThan(0);
    expect(
      overflowingValues,
      `Expected visible workload I/O values to render fully without ellipsis: ${JSON.stringify(overflowingValues)}`,
    ).toEqual([]);
  });
});
