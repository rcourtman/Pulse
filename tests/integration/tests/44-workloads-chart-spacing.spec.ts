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

type ChartData = {
  cpu?: MetricPoint[];
};

type WorkloadChartsResponse = {
  data?: Record<string, ChartData>;
  dockerData?: Record<string, ChartData>;
};

const WORKLOADS_SCREENSHOT_PATH = path.resolve(
  __dirname,
  '..',
  '..',
  'tmp',
  'workloads-table-7d-spacing.png',
);

let mockModeWasEnabled: boolean | null = null;

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
      `workloads-chart-spacing-${workerInfo.project.name}.json`,
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

async function ensureMockModeEnabled(page: import('@playwright/test').Page): Promise<void> {
  const state = await getMockMode(page);
  if (mockModeWasEnabled === null) {
    mockModeWasEnabled = state.enabled;
  }
  if (!state.enabled) {
    await setMockMode(page, true);
  }
}

function longestCPUSeries(payload: WorkloadChartsResponse): MetricPoint[] {
  const candidates = [
    ...Object.values(payload.data || {}).map((chartData) => chartData.cpu || []),
    ...Object.values(payload.dockerData || {}).map((chartData) => chartData.cpu || []),
  ];
  return candidates.sort((a, b) => b.length - a.length)[0] || [];
}

// The retired /workloads page carried a summary chart strip with 1h/7d range
// buttons; the workloads surface now lives on /proxmox with per-row trend
// sparklines and no page-level range switch. The sparkline path maps x by
// timestamp (workloadMetricHistoryModel), so rendering is time-proportional
// by construction and the payload is adaptively downsampled (dense tail,
// sparse seeded history). What the time-scaled renderer needs from the
// endpoint is coverage: the 7d payload must actually span the window and
// carry a fresh tail, which is what this spec asserts.
test.describe.serial('Workloads chart spacing', () => {
  test.setTimeout(180_000);

  test.afterAll(async ({ browser }) => {
    if (mockModeWasEnabled === null) {
      return;
    }

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

  test('serves full-window 7d workload history for the time-scaled sparklines', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');

    await ensureMockModeEnabled(page);

    await page.goto('/proxmox', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('workloads-table-surface')).toBeVisible({ timeout: 60_000 });

    let payload: WorkloadChartsResponse = {};
    await expect
      .poll(async () => {
        const response = await apiRequest(page, '/api/charts/workloads?range=7d');
        if (!response.ok()) return -1;
        payload = (await response.json()) as WorkloadChartsResponse;
        return longestCPUSeries(payload).length;
      }, { timeout: 60_000 })
      .toBeGreaterThan(40);

    const cpuSeries = longestCPUSeries(payload).slice().sort((a, b) => a.timestamp - b.timestamp);
    const spanMs = cpuSeries[cpuSeries.length - 1].timestamp - cpuSeries[0].timestamp;
    const dayMs = 24 * 60 * 60 * 1000;
    expect(spanMs, '7d history should cover most of the requested window').toBeGreaterThan(5 * dayMs);
    expect(
      Date.now() - cpuSeries[cpuSeries.length - 1].timestamp,
      '7d history should end at a fresh tail',
    ).toBeLessThan(60 * 60 * 1000);
    const midpoint = cpuSeries[0].timestamp + spanMs / 2;
    expect(
      cpuSeries.filter((point) => point.timestamp < midpoint).length,
      'history should not be compressed into the recent half of the window',
    ).toBeGreaterThan(5);

    fs.mkdirSync(path.dirname(WORKLOADS_SCREENSHOT_PATH), { recursive: true });
    await page.getByTestId('workloads-table-surface').screenshot({ path: WORKLOADS_SCREENSHOT_PATH });
  });
});
