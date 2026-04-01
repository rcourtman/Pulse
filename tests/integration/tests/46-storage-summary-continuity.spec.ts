import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

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
  usage?: MetricPoint[];
  used?: MetricPoint[];
  avail?: MetricPoint[];
};

type DiskSeries = {
  temperature?: MetricPoint[];
};

type StorageChartsResponse = {
  pools?: Record<string, StorageSeries>;
  disks?: Record<string, DiskSeries>;
};

const ARTIFACTS_DIR = path.resolve(__dirname, '..', '..', 'tmp', 'storage-summary-continuity');

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
      `storage-summary-continuity-${workerInfo.project.name}.json`,
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

function diskTemperatureSeries(payload: StorageChartsResponse): MetricPoint[][] {
  return Object.values(payload.disks ?? {})
    .map((disk) => disk.temperature ?? [])
    .map((points) => points.slice().sort((a, b) => a.timestamp - b.timestamp))
    .filter((points) => points.length >= 8);
}

function worstTemperatureTailDelta(payload: StorageChartsResponse): number {
  return diskTemperatureSeries(payload)
    .map((points) => {
      const previous = points.slice(-8, -2).map((point) => point.value);
      const tail = points.slice(-2).map((point) => point.value);
      return Math.abs(average(tail) - average(previous));
    })
    .reduce((worst, delta) => Math.max(worst, delta), 0);
}

test.describe.serial('Storage summary chart continuity', () => {
  test.setTimeout(180_000);

  test('renders coherent storage summary histories across the live storage page', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');

    const mockMode = await getMockMode(page);
    if (!mockMode.enabled) {
      await setMockMode(page, true);
    }

    fs.mkdirSync(ARTIFACTS_DIR, { recursive: true });

    await page.addInitScript(() => {
      localStorage.setItem('pulse_whats_new_v2_shown', 'true');
    });
    await page.goto('/storage', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/storage/);
    await expect(page.getByTestId('storage-summary')).toBeVisible();
    await expect(page.getByText('Pool Usage')).toBeVisible();
    await expect(page.getByText('Disk Temperature')).toBeVisible();

    await page.getByTestId('storage-summary').screenshot({
      path: path.resolve(ARTIFACTS_DIR, 'storage-summary-1h.png'),
    });

    const oneHourResponse = await apiRequest(page, '/api/storage-charts?range=60');
    expect(oneHourResponse.ok()).toBeTruthy();
    const oneHourPayload = (await oneHourResponse.json()) as StorageChartsResponse;
    const oneHourPools = Object.values(oneHourPayload.pools ?? {}) as StorageSeries[];
    const oneHourDisks = Object.values(oneHourPayload.disks ?? {}) as DiskSeries[];
    expect(oneHourPools.length).toBeGreaterThan(0);
    expect(oneHourDisks.length).toBeGreaterThan(0);
    for (const pool of oneHourPools) {
      expect((pool.usage ?? []).length).toBeGreaterThanOrEqual(30);
      expect((pool.used ?? []).length).toBeGreaterThanOrEqual(30);
      expect((pool.avail ?? []).length).toBeGreaterThanOrEqual(30);
    }
    for (const disk of oneHourDisks) {
      expect((disk.temperature ?? []).length).toBeGreaterThanOrEqual(30);
    }

    const sevenDayResponse = await apiRequest(page, '/api/storage-charts?range=10080');
    expect(sevenDayResponse.ok()).toBeTruthy();
    const sevenDayPayload = (await sevenDayResponse.json()) as StorageChartsResponse;
    const sevenDayPools = Object.values(sevenDayPayload.pools ?? {}) as StorageSeries[];
    const sevenDayDisks = Object.values(sevenDayPayload.disks ?? {}) as DiskSeries[];
    expect(sevenDayPools.length).toBeGreaterThan(0);
    expect(sevenDayDisks.length).toBeGreaterThan(0);
    for (const pool of sevenDayPools) {
      expect((pool.usage ?? []).length).toBeGreaterThanOrEqual(300);
      expect((pool.used ?? []).length).toBeGreaterThanOrEqual(300);
      expect((pool.avail ?? []).length).toBeGreaterThanOrEqual(300);
    }
    for (const disk of sevenDayDisks) {
      expect((disk.temperature ?? []).length).toBeGreaterThanOrEqual(300);
    }
    expect(worstTemperatureTailDelta(sevenDayPayload)).toBeLessThan(3);

    const sevenDayResponsePromise = page.waitForResponse((response) => {
      const url = response.url();
      return response.request().method() === 'GET' &&
        url.includes('/api/storage-charts?') &&
        url.includes('range=10080');
    });
    await page.getByRole('button', { name: '7d', exact: true }).click();
    await sevenDayResponsePromise;
    await page.waitForTimeout(1000);
    await page.getByTestId('storage-summary').screenshot({
      path: path.resolve(ARTIFACTS_DIR, 'storage-summary-7d.png'),
    });

    fs.writeFileSync(
      path.resolve(ARTIFACTS_DIR, 'storage-summary-1h.json'),
      JSON.stringify(oneHourPayload, null, 2),
    );
    fs.writeFileSync(
      path.resolve(ARTIFACTS_DIR, 'storage-summary-7d.json'),
      JSON.stringify(sevenDayPayload, null, 2),
    );
  });
});
