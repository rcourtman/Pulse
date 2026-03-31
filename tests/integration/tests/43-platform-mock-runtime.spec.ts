import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState, getMockMode, setMockMode } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const TRUENAS_SCREENSHOT_PATH = path.resolve(
  __dirname,
  '..',
  '..',
  'tmp',
  'platform-mock-runtime-truenas.png',
);
const VMWARE_SCREENSHOT_PATH = path.resolve(
  __dirname,
  '..',
  '..',
  'tmp',
  'platform-mock-runtime-vmware.png',
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
      `platform-mock-runtime-${workerInfo.project.name}.json`,
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

test.describe.serial('Platform mock runtime', () => {
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

  test('renders TrueNAS and VMware mock data from the live runtime', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');

    await ensureMockModeEnabled(page);

    await page.goto('/settings/infrastructure/platforms/truenas', {
      waitUntil: 'domcontentloaded',
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/truenas/, {
      timeout: 15_000,
    });
    await expect(page.getByText('Archive NAS')).toBeVisible();
    await expect(page.getByText('3 pools')).toBeVisible();
    await expect(page.getByText('5 datasets')).toBeVisible();
    fs.mkdirSync(path.dirname(TRUENAS_SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: TRUENAS_SCREENSHOT_PATH, fullPage: true });

    await page.goto('/infrastructure?source=truenas', {
      waitUntil: 'domcontentloaded',
    });
    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await expect(page.locator('#infra-source-filter')).toHaveValue('truenas');
    await expect(page.getByText('truenas-main').first()).toBeVisible();

    await page.goto('/settings/infrastructure/platforms/vmware', {
      waitUntil: 'domcontentloaded',
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/vmware/, {
      timeout: 15_000,
    });
    await expect(page.getByText('Lab vCenter')).toBeVisible();
    await expect(page.getByText('2 hosts')).toBeVisible();
    await expect(page.getByText('3 vms')).toBeVisible();
    await expect(page.getByText('2 datastores')).toBeVisible();
    fs.mkdirSync(path.dirname(VMWARE_SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: VMWARE_SCREENSHOT_PATH, fullPage: true });

    await page.goto('/infrastructure?source=vmware-vsphere', {
      waitUntil: 'domcontentloaded',
    });
    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await expect(page.locator('#infra-source-filter')).toHaveValue('vmware-vsphere');
    await expect(page.getByText('esxi-01.lab.local').first()).toBeVisible();
  });
});
