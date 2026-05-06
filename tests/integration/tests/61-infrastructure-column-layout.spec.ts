import { expect, test as base, type Page } from '@playwright/test';

import { getMockMode, setMockMode } from './helpers';

type InfrastructureHeaderMetric = {
  text: string;
  width: number;
  attrWidth: number | null;
};

type InfrastructureTableMetric = {
  kind: 'host' | 'pbs' | 'pmg' | 'unknown';
  wrapperClientWidth: number;
  wrapperScrollWidth: number;
  tableScrollWidth: number;
  headers: InfrastructureHeaderMetric[];
};

type InfrastructureColumnLayout = {
  tables: InfrastructureTableMetric[];
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

async function readInfrastructureColumnLayout(page: Page): Promise<InfrastructureColumnLayout> {
  return page.evaluate(() => {
    const tables = Array.from(
      document.querySelectorAll<HTMLTableElement>('[data-testid="infrastructure-table-surface"] table'),
    );

    const classifyTable = (headers: string[]): InfrastructureTableMetric['kind'] => {
      if (headers.includes('CPU') && headers.includes('Net I/O')) return 'host';
      if (headers.includes('Datastores') && headers.includes('Jobs')) return 'pbs';
      if (headers.includes('Queue') && headers.includes('Deferred')) return 'pmg';
      return 'unknown';
    };

    return {
      tables: tables.map((table) => {
        const wrapper = table.closest<HTMLDivElement>('div.overflow-x-auto');
        const headers = Array.from(table.querySelectorAll<HTMLTableCellElement>('thead th')).map(
          (header) => {
            const rawAttrWidth = header.getAttribute('width');
            return {
              text: header.textContent?.replace(/\s+/g, ' ').trim() ?? '',
              width: Math.round(header.getBoundingClientRect().width),
              attrWidth:
                rawAttrWidth === null || Number.isNaN(Number(rawAttrWidth))
                  ? null
                  : Number(rawAttrWidth),
            };
          },
        );

        return {
          kind: classifyTable(headers.map((header) => header.text)),
          wrapperClientWidth: wrapper?.clientWidth ?? 0,
          wrapperScrollWidth: wrapper?.scrollWidth ?? 0,
          tableScrollWidth: table.scrollWidth,
          headers,
        };
      }),
    };
  });
}

test.describe.serial('Infrastructure column layout', () => {
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

  test('distributes full-width desktop space across infrastructure columns', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');

    await page.setViewportSize({ width: 1920, height: 1200 });
    await page.addInitScript(() => {
      localStorage.setItem('fullWidthMode', 'full-width');
    });

    await ensureMockModeEnabled(page);
    await page.goto('/infrastructure', { waitUntil: 'domcontentloaded' });
    await expect(page.locator('.pulse-shell--full-width').first()).toBeVisible();
    await expect(page.getByTestId('infrastructure-table-surface')).toBeVisible();
    await expect(page.locator('[data-testid="infrastructure-table-surface"] [data-summary-series-id]').first()).toBeVisible();
    await page.waitForTimeout(1000);

    const layout = await readInfrastructureColumnLayout(page);
    const hostTable = layout.tables.find((table) => table.kind === 'host');

    expect(hostTable, 'Expected the infrastructure host table to be visible in mock mode').toBeDefined();
    expect(layout.tables.length, 'Expected visible infrastructure tables to validate').toBeGreaterThan(0);

    for (const table of layout.tables) {
      const resourceHeader = table.headers[0];
      const fixedPeerHeaders = table.headers.slice(1).filter((header) => header.attrWidth !== null);
      const widestPeerWidth = Math.max(...fixedPeerHeaders.map((header) => header.width));

      expect(
        table.tableScrollWidth,
        `${table.kind} table should fit the full-width desktop shell without horizontal scrolling`,
      ).toBeLessThanOrEqual(table.wrapperClientWidth + 1);

      expect(
        table.wrapperScrollWidth,
        `${table.kind} wrapper should not need extra horizontal scroll room in full-width mode`,
      ).toBeLessThanOrEqual(table.wrapperClientWidth + 1);

      expect(
        fixedPeerHeaders.length,
        `Expected ${table.kind} table to expose fixed-width peer columns for validation`,
      ).toBeGreaterThan(0);

      expect(
        resourceHeader.width / widestPeerWidth,
        `${table.kind} resource column should not monopolize the full-width layout`,
      ).toBeLessThanOrEqual(1.8);

      const underExpandedPeers = fixedPeerHeaders.filter(
        (header) => header.width <= (header.attrWidth ?? 0) + 5,
      );

      expect(
        underExpandedPeers,
        `${table.kind} table should distribute extra full-width space across peer columns: ${JSON.stringify(table.headers)}`,
      ).toEqual([]);
    }
  });
});
