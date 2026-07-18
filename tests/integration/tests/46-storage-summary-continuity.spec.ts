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

// The retired /storage page carried Pool Usage / Disk Temperature summary
// charts with 1h/7d range buttons; storage now lives on /proxmox/storage where
// per-pool history surfaces as the Growth column and row expansions. The
// multi-range history-continuity proof this spec exists for lives in the
// /api/storage-charts payload, so it is asserted directly. Point-count floors
// reflect what a fresh seeded backend guarantees (15-minute minute-tier over
// the last day, 2-hour hourly tier over the last week), not a long-lived dev
// store.
test.describe.serial('Storage summary chart continuity', () => {
  test.setTimeout(180_000);

  test('serves coherent storage histories across ranges on the live storage page', async ({
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

    // A busy box can push a cold storage-charts read past the 10s request
    // default; the payload itself is what matters, not its latency.
    const oneHourResponse = await apiRequest(page, '/api/storage-charts?range=60', {
      timeout: 60_000,
    });
    expect(oneHourResponse.ok()).toBeTruthy();
    const oneHourPayload = (await oneHourResponse.json()) as StorageChartsResponse;
    const oneHourPools = Object.values(oneHourPayload.pools ?? {}) as StorageSeries[];
    const oneHourDisks = Object.values(oneHourPayload.disks ?? {}) as DiskSeries[];
    expect(oneHourPools.length).toBeGreaterThan(0);
    expect(oneHourDisks.length).toBeGreaterThan(0);
    for (const pool of oneHourPools) {
      expect((pool.usage ?? []).length).toBeGreaterThanOrEqual(4);
      expect((pool.used ?? []).length).toBeGreaterThanOrEqual(4);
      expect((pool.avail ?? []).length).toBeGreaterThanOrEqual(4);
    }
    for (const disk of oneHourDisks) {
      expect((disk.temperature ?? []).length).toBeGreaterThanOrEqual(4);
    }

    const sevenDayResponse = await apiRequest(page, '/api/storage-charts?range=10080', {
      timeout: 60_000,
    });
    expect(sevenDayResponse.ok()).toBeTruthy();
    const sevenDayPayload = (await sevenDayResponse.json()) as StorageChartsResponse;
    const sevenDayPools = Object.values(sevenDayPayload.pools ?? {}) as StorageSeries[];
    const sevenDayDisks = Object.values(sevenDayPayload.disks ?? {}) as DiskSeries[];
    expect(sevenDayPools.length).toBeGreaterThan(0);
    expect(sevenDayDisks.length).toBeGreaterThan(0);
    for (const pool of sevenDayPools) {
      expect((pool.usage ?? []).length).toBeGreaterThanOrEqual(40);
      expect((pool.used ?? []).length).toBeGreaterThanOrEqual(40);
      expect((pool.avail ?? []).length).toBeGreaterThanOrEqual(40);
    }
    for (const disk of sevenDayDisks) {
      expect((disk.temperature ?? []).length).toBeGreaterThanOrEqual(40);
    }

    // The 7d window must actually be deeper history than the 1h window, and
    // seeded temperature history must stay continuous at the live tail.
    const longestOneHour = Math.max(...oneHourDisks.map((disk) => (disk.temperature ?? []).length));
    const longestSevenDay = Math.max(
      ...sevenDayDisks.map((disk) => (disk.temperature ?? []).length),
    );
    expect(longestSevenDay).toBeGreaterThan(longestOneHour);
    expect(worstTemperatureTailDelta(sevenDayPayload)).toBeLessThan(3);

    await page.getByTestId('storage-page').screenshot({
      path: path.resolve(ARTIFACTS_DIR, 'storage-page.png'),
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
