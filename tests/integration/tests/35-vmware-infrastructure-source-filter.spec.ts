import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_PATH = '/tmp/vmware-infrastructure-source-filter.png';

type WorkerFixtures = {
  authStorageStatePath: string;
};

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
      `vmware-infrastructure-source-filter-${workerInfo.project.name}.json`,
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

test.describe('VMware infrastructure source filter', () => {
  test.setTimeout(180_000);

  test('surfaces VMware resources under the canonical vSphere infrastructure filter', async ({
    page,
  }) => {
    await page.route('**/api/resources**', async (route) => {
      const requestUrl = new URL(route.request().url());
      if (requestUrl.pathname !== '/api/resources') {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 'vmware-host-1',
              type: 'agent',
              name: 'esxi-01.lab.local',
              displayName: 'ESXi 01',
              platformId: 'vmware-host-1',
              platformType: 'vmware-vsphere',
              sourceType: 'api',
              sources: ['vmware-vsphere'],
              status: 'online',
              lastSeen: '2026-03-30T17:00:00Z',
              canonicalIdentity: {
                displayName: 'ESXi 01',
                hostname: 'esxi-01.lab.local',
                platformId: 'vmware-host-1',
              },
              agent: {
                hostname: 'esxi-01.lab.local',
                platform: 'VMware ESXi',
                uptimeSeconds: 86400,
              },
              platformData: {
                sources: ['vmware-vsphere'],
              },
            },
          ],
          meta: {
            page: 1,
            limit: 200,
            total: 1,
            totalPages: 1,
          },
        }),
      });
    });

    await page.goto('/infrastructure?source=vmware-vsphere', {
      waitUntil: 'domcontentloaded',
    });

    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await expect(page).toHaveURL(/\/infrastructure\?source=vmware-vsphere/);
    await expect(page.locator('#infra-source-filter')).toHaveValue('vmware-vsphere');

    const sourceOptions = await page.locator('#infra-source-filter option').evaluateAll((options) =>
      options.map((option) => ({
        value: option.getAttribute('value'),
        label: option.textContent?.trim(),
      })),
    );
    expect(sourceOptions).toEqual([
      { value: '', label: 'All' },
      { value: 'vmware-vsphere', label: 'vSphere' },
    ]);

    await expect(page.getByText('ESXi 01')).toBeVisible();
    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
