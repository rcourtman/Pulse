import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState, getMockMode, setMockMode } from './helpers';

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
  'workloads-summary-7d-spacing.png',
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

function average(values: number[]): number {
  if (values.length === 0) return 0;
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function longestCPUSeries(payload: WorkloadChartsResponse): MetricPoint[] {
  const candidates = [
    ...Object.values(payload.data || {}).map((chartData) => chartData.cpu || []),
    ...Object.values(payload.dockerData || {}).map((chartData) => chartData.cpu || []),
  ];
  return candidates.sort((a, b) => b.length - a.length)[0] || [];
}

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

  test('keeps 7d workload charts time-proportional on the live page', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');

    await ensureMockModeEnabled(page);

    await page.goto('/workloads', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('workloads-summary')).toBeVisible();

    const responsePromise = page.waitForResponse((response) => {
      const url = response.url();
      return response.request().method() === 'GET' &&
        url.includes('/api/charts/workloads?') &&
        url.includes('range=7d');
    });

    await page.getByRole('button', { name: '7d', exact: true }).click();
    const response = await responsePromise;
    expect(response.ok()).toBeTruthy();

    const payload = (await response.json()) as WorkloadChartsResponse;
    const cpuSeries = longestCPUSeries(payload);
    expect(cpuSeries.length).toBeGreaterThan(40);

    const deltas = cpuSeries
      .slice(1)
      .map((point, index) => point.timestamp - cpuSeries[index].timestamp)
      .filter((delta) => delta > 0);
    const firstAverage = average(deltas.slice(0, 10));
    const lastAverage = average(deltas.slice(-10));

    expect(lastAverage).toBeGreaterThan(firstAverage * 0.5);
    expect(lastAverage).toBeLessThan(firstAverage * 1.5);

    fs.mkdirSync(path.dirname(WORKLOADS_SCREENSHOT_PATH), { recursive: true });
    await page.getByTestId('workloads-summary').screenshot({ path: WORKLOADS_SCREENSHOT_PATH });
  });
});
