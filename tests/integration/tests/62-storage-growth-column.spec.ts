import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base, type Page } from '@playwright/test';

import {
  apiRequest,
  createAuthenticatedStorageState,
  getMockMode,
  setMockMode,
} from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

type MetricPoint = {
  timestamp: number;
  value: number;
};

type StorageSeries = {
  used?: MetricPoint[];
};

type StorageChartsResponse = {
  pools?: Record<string, StorageSeries>;
};

const ARTIFACTS_DIR = path.resolve(__dirname, '..', '..', 'tmp', 'storage-growth-column');
const STORAGE_POOL_GROWTH_CELL_SELECTOR = 'td:nth-child(8)';

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => {
    await use(authStorageStatePath);
  },
  authStorageStatePath: [async ({ browser }, use, workerInfo) => {
    const storageStatePath = path.resolve(
      __dirname,
      '..',
      '..',
      'tmp',
      'playwright-auth',
      `storage-growth-column-${workerInfo.project.name}.json`,
    );
    fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
    await createAuthenticatedStorageState(browser, storageStatePath);
    try {
      await use(storageStatePath);
    } finally {
      fs.rmSync(storageStatePath, { force: true });
    }
  }, { scope: 'worker' }],
});

function average(values: number[]): number {
  if (values.length === 0) return 0;
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes < 0) return '0 B';

  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const unitIndex = Math.floor(Math.log(bytes) / Math.log(k));
  const value = bytes / Math.pow(k, unitIndex);

  let precision: number;
  if (value < 10) precision = 2;
  else if (value < 100) precision = 1;
  else precision = 0;

  return `${value.toFixed(precision)} ${sizes[unitIndex]}`;
}

function computeStorageCapacityDelta(points: MetricPoint[]): number | null {
  const normalized = points
    .filter((point) => Number.isFinite(point.timestamp) && Number.isFinite(point.value))
    .slice()
    .sort((left, right) => left.timestamp - right.timestamp);

  if (normalized.length < 2) {
    return null;
  }

  const sampleWindowSize =
    normalized.length >= 4
      ? Math.max(2, Math.floor(normalized.length * 0.25))
      : 1;
  const startAverage = average(normalized.slice(0, sampleWindowSize).map((point) => point.value));
  const endAverage = average(normalized.slice(normalized.length - sampleWindowSize).map((point) => point.value));
  const delta = endAverage - startAverage;
  if (!Number.isFinite(delta)) {
    return null;
  }
  if (Math.abs(delta) < 1) {
    return 0;
  }
  return delta;
}

function formatStorageCapacityDelta(deltaBytes: number | null): string {
  if (deltaBytes === null) {
    return '—';
  }
  if (deltaBytes === 0) {
    return '0 B';
  }
  const sign = deltaBytes > 0 ? '+' : '-';
  return `${sign}${formatBytes(Math.abs(deltaBytes))}`;
}

function growthLabelForPool(series: StorageSeries | undefined): string {
  return formatStorageCapacityDelta(computeStorageCapacityDelta(series?.used ?? []));
}

async function firstVisibleGrowthExpectation(
  page: Page,
  payload: StorageChartsResponse,
): Promise<{ seriesId: string; label: string }> {
  for (const [seriesId, series] of Object.entries(payload.pools ?? {})) {
    const label = growthLabelForPool(series);
    if (label === '—') {
      continue;
    }
    const row = page.locator(`tr[data-summary-series-id="${seriesId}"]`);
    if ((await row.count()) > 0 && await row.first().isVisible()) {
      return { seriesId, label };
    }
  }

  throw new Error('Expected at least one visible storage pool growth label derived from used-capacity history.');
}

test.describe.serial('Storage growth column', () => {
  test.setTimeout(120_000);

  test('renders shared storage-history growth deltas for 24h and 7d', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');

    const mockMode = await getMockMode(page);
    if (!mockMode.enabled) {
      await setMockMode(page, true);
    }

    fs.mkdirSync(ARTIFACTS_DIR, { recursive: true });

    await page.goto('/storage', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/storage/);
    await expect(page.getByTestId('storage-summary')).toBeVisible();

    const poolsTable = page.locator('table').first();
    await expect(poolsTable.getByText('Growth (24h)', { exact: true })).toBeVisible();

    const defaultResponse = await apiRequest(page, '/api/storage-charts?range=1440');
    expect(defaultResponse.ok()).toBeTruthy();
    const defaultPayload = (await defaultResponse.json()) as StorageChartsResponse;
    const defaultGrowth = await firstVisibleGrowthExpectation(page, defaultPayload);
    const defaultGrowthCell = page.locator(
      `tr[data-summary-series-id="${defaultGrowth.seriesId}"] ${STORAGE_POOL_GROWTH_CELL_SELECTOR}`,
    );
    await expect(defaultGrowthCell).toHaveText(defaultGrowth.label);

    await page.screenshot({
      path: path.resolve(ARTIFACTS_DIR, 'storage-growth-24h.png'),
      fullPage: true,
    });

    const sevenDayPayloadPromise = apiRequest(page, '/api/storage-charts?range=10080');
    const sevenDayResponsePromise = page.waitForResponse((response) => {
      const url = response.url();
      return response.request().method() === 'GET' &&
        url.includes('/api/storage-charts?') &&
        url.includes('range=10080');
    });
    await page.getByRole('button', { name: '7d', exact: true }).click();
    const sevenDayResponse = await sevenDayPayloadPromise;
    expect(sevenDayResponse.ok()).toBeTruthy();
    const sevenDayPayload = (await sevenDayResponse.json()) as StorageChartsResponse;
    await sevenDayResponsePromise;

    await expect(poolsTable.getByText('Growth (7d)', { exact: true })).toBeVisible();
    const sevenDayGrowthCell = page.locator(
      `tr[data-summary-series-id="${defaultGrowth.seriesId}"] ${STORAGE_POOL_GROWTH_CELL_SELECTOR}`,
    );
    await expect(sevenDayGrowthCell).toHaveText(
      growthLabelForPool(sevenDayPayload.pools?.[defaultGrowth.seriesId]),
    );

    await page.screenshot({
      path: path.resolve(ARTIFACTS_DIR, 'storage-growth-7d.png'),
      fullPage: true,
    });
  });
});
