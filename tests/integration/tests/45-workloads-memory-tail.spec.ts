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
  memory?: MetricPoint[];
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
  'workloads-summary-1h-memory-tail.png',
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
      `workloads-memory-tail-${workerInfo.project.name}.json`,
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

async function dismissWhatsNewModal(page: import('@playwright/test').Page): Promise<void> {
  const modalTitle = page.getByText('Welcome to the New Navigation!');
  if (!(await modalTitle.isVisible().catch(() => false))) {
    return;
  }
  await page.getByRole('button', { name: "Let's go" }).click();
  await expect(modalTitle).toHaveCount(0);
}

function average(values: number[]): number {
  if (values.length === 0) return 0;
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function memoryTailDeltas(payload: WorkloadChartsResponse): number[] {
  return [
    ...Object.values(payload.data || {}).map((chartData) => chartData.memory || []),
    ...Object.values(payload.dockerData || {}).map((chartData) => chartData.memory || []),
  ]
    .map((points) => points.slice().sort((a, b) => a.timestamp - b.timestamp))
    .filter((points) => points.length >= 8)
    .map((points) => {
      const previous = points.slice(-8, -2).map((point) => point.value);
      const tail = points.slice(-2).map((point) => point.value);
      return Math.abs(average(tail) - average(previous));
    })
    .sort((a, b) => a - b);
}

function memoryAdjacentJumps(payload: WorkloadChartsResponse): number[] {
  return [
    ...Object.values(payload.data || {}).map((chartData) => chartData.memory || []),
    ...Object.values(payload.dockerData || {}).map((chartData) => chartData.memory || []),
  ]
    .map((points) => points.slice().sort((a, b) => a.timestamp - b.timestamp))
    .filter((points) => points.length >= 3)
    .map((points) => {
      let worst = 0;
      for (let index = 1; index < points.length; index++) {
        worst = Math.max(worst, Math.abs(points[index].value - points[index - 1].value));
      }
      return worst;
    })
    .sort((a, b) => a - b);
}

function percentile(sortedValues: number[], ratio: number): number {
  if (sortedValues.length === 0) return 0;
  const boundedRatio = Math.min(1, Math.max(0, ratio));
  const index = Math.min(
    sortedValues.length - 1,
    Math.floor((sortedValues.length - 1) * boundedRatio),
  );
  return sortedValues[index];
}

test.describe.serial('Workloads memory tail', () => {
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

  test('keeps 1h workload memory tails visually stable on the live page', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');

    await ensureMockModeEnabled(page);

    await page.goto('/workloads', { waitUntil: 'domcontentloaded' });
    await page.evaluate(() => window.localStorage.clear());

    const responsePromise = page.waitForResponse((response) => {
      const url = response.url();
      return response.request().method() === 'GET' &&
        url.includes('/api/charts/workloads?') &&
        url.includes('range=1h');
    });

    await page.reload({ waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('workloads-summary')).toBeVisible();
    await dismissWhatsNewModal(page);

    const response = await responsePromise;
    expect(response.ok()).toBeTruthy();

    const payload = (await response.json()) as WorkloadChartsResponse;
    const tailDeltas = memoryTailDeltas(payload);
    const adjacentJumps = memoryAdjacentJumps(payload);
    expect(percentile(tailDeltas, 0.95)).toBeLessThan(6);
    expect(tailDeltas[tailDeltas.length - 1]).toBeLessThan(8);
    expect(percentile(adjacentJumps, 0.95)).toBeLessThan(4);
    expect(adjacentJumps[adjacentJumps.length - 1]).toBeLessThan(6);

    fs.mkdirSync(path.dirname(WORKLOADS_SCREENSHOT_PATH), { recursive: true });
    await page.getByTestId('workloads-summary').screenshot({ path: WORKLOADS_SCREENSHOT_PATH });
  });
});
