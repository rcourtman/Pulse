import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_PATH = '/tmp/truenas-infrastructure-route-filter.png';

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
      `truenas-infrastructure-route-filter-${workerInfo.project.name}.json`,
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

test.describe('TrueNAS infrastructure route filter', () => {
  test.setTimeout(180_000);

  test('keeps a route-owned TrueNAS source selection visible when current infrastructure data has no TrueNAS resources', async ({
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
              id: 'pve-main',
              type: 'agent',
              name: 'pve-main',
              displayName: 'PVE Main',
              platformId: 'pve-main',
              platformType: 'proxmox-pve',
              sourceType: 'hybrid',
              sources: ['agent', 'proxmox-pve'],
              status: 'online',
              lastSeen: '2026-03-29T22:00:00Z',
              canonicalIdentity: {
                displayName: 'PVE Main',
                hostname: 'pve-main',
                platformId: 'pve-main',
              },
              agent: {
                hostname: 'pve-main',
                platform: 'Debian',
                uptimeSeconds: 86400,
              },
              platformData: {
                sources: ['agent', 'proxmox-pve'],
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

    await page.goto('/infrastructure?source=truenas', {
      waitUntil: 'domcontentloaded',
    });

    await expect(page).toHaveURL(/\/infrastructure\?source=truenas/);
    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await expect(page.locator('#infra-source-filter')).toHaveValue('truenas');

    const sourceOptions = await page.locator('#infra-source-filter option').evaluateAll((options) =>
      options.map((option) => ({
        value: option.getAttribute('value'),
        label: option.textContent?.trim(),
      })),
    );
    expect(sourceOptions).toEqual([
      { value: '', label: 'All' },
      { value: 'proxmox-pve', label: 'PVE' },
      { value: 'agent', label: 'Agent' },
      { value: 'truenas', label: 'TrueNAS' },
    ]);

    await expect(page.getByText('No resources match filters')).toBeVisible();
    await expect(page.getByText('Try adjusting the search, source, or status filters.')).toBeVisible();

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
