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

// Mirrors computeStorageCapacityDelta / formatStorageCapacityDelta in
// frontend-modern/src/features/storageBackups/storageCapacityDeltaPresentation.ts.
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

async function firstVisibleGrowthRow(
  page: Page,
  payload: StorageChartsResponse,
): Promise<string> {
  for (const [seriesId, series] of Object.entries(payload.pools ?? {})) {
    if (growthLabelForPool(series) === '—') {
      continue;
    }
    const row = page.locator(`tr[data-summary-series-id="${seriesId}"]`);
    if ((await row.count()) > 0 && await row.first().isVisible()) {
      return seriesId;
    }
  }

  throw new Error('Expected at least one visible storage pool with used-capacity growth history.');
}

// The retired /storage page paired the Growth column with a 1h/7d range
// switch; on /proxmox/storage the column is fixed to a 24h window (the header
// is a sort control, not a range control). The proof: the rendered Growth
// (24h) label for a pool matches the delta derived from the same 24h
// used-capacity history the /api/storage-charts endpoint serves.
test.describe.serial('Storage growth column', () => {
  test.setTimeout(120_000);

  test('renders storage-history growth deltas for the 24h window', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');

    const mockMode = await getMockMode(page);
    if (!mockMode.enabled) {
      await setMockMode(page, true);
    }

    fs.mkdirSync(ARTIFACTS_DIR, { recursive: true });

    await page.goto('/proxmox/storage', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/proxmox\/storage/);
    await expect(page.getByTestId('storage-page')).toBeVisible({ timeout: 60_000 });
    await expect(
      page.getByRole('button', { name: 'Sort Growth (24h) column' }).first(),
    ).toBeVisible();
    await expect(page.locator('tr[data-summary-series-id]').first()).toBeVisible({
      timeout: 30_000,
    });

    const probeResponse = await apiRequest(page, '/api/storage-charts?range=1440');
    expect(probeResponse.ok()).toBeTruthy();
    const probePayload = (await probeResponse.json()) as StorageChartsResponse;
    const seriesId = await firstVisibleGrowthRow(page, probePayload);
    const growthCell = page
      .locator(`tr[data-summary-series-id="${seriesId}"]`)
      .locator('td')
      .last();

    // The live table refreshes its history on its own cadence, so recompute
    // the expectation from a fresh payload on every poll attempt instead of
    // racing a single snapshot against the render.
    await expect
      .poll(async () => {
        const response = await apiRequest(page, '/api/storage-charts?range=1440');
        if (!response.ok()) return 'payload-error';
        const payload = (await response.json()) as StorageChartsResponse;
        const expected = growthLabelForPool(payload.pools?.[seriesId]);
        const rendered = ((await growthCell.textContent()) ?? '').trim();
        return rendered === expected ? 'match' : `rendered=${rendered} expected=${expected}`;
      }, { timeout: 30_000 })
      .toBe('match');

    // Viewport-only: full-page screenshots can hang while the assistant panel
    // animates.
    await page.screenshot({
      path: path.resolve(ARTIFACTS_DIR, 'storage-growth-24h.png'),
    });
  });
});
