import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = '/tmp/truenas-infrastructure-storage-recovery-links.png';

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
      `truenas-infrastructure-storage-links-${workerInfo.project.name}.json`,
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

test.describe('TrueNAS infrastructure storage and recovery links', () => {
  test.setTimeout(180_000);

  test('surfaces canonical workloads, storage, and recovery links for top-level TrueNAS systems', async ({
    page,
  }) => {
    await page.route('**/api/resources**', async (route) => {
      const requestUrl = new URL(route.request().url());

      if (requestUrl.pathname === '/api/resources') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: [
              {
                id: 'truenas-main',
                type: 'truenas',
                name: 'truenas-main',
                displayName: 'TrueNAS Main',
                platformId: 'truenas-main',
                platformType: 'truenas',
                sourceType: 'hybrid',
                sources: ['agent', 'truenas'],
                status: 'online',
                lastSeen: '2026-03-29T22:00:00Z',
                canonicalIdentity: {
                  displayName: 'TrueNAS Main',
                  hostname: 'truenas-main',
                  platformId: 'truenas-main',
                },
                platformData: {
                  sources: ['agent', 'truenas'],
                },
              },
              {
                id: 'storage-truenas-display',
                type: 'storage',
                name: 'tank',
                displayName: 'tank',
                parentId: 'truenas-main',
                parentName: 'TrueNAS Main',
                platformId: 'truenas-1',
                platformType: 'truenas',
                sourceType: 'api',
                sources: ['truenas'],
                status: 'online',
                lastSeen: '2026-03-29T22:00:00Z',
                canonicalIdentity: {
                  displayName: 'tank',
                  platformId: 'truenas-1',
                },
                storage: {
                  platform: 'truenas',
                  type: 'zfs-pool',
                  topology: 'pool',
                },
                platformData: {
                  sources: ['truenas'],
                },
              },
            ],
            meta: {
              page: 1,
              limit: 200,
              total: 2,
              totalPages: 1,
            },
          }),
        });
        return;
      }

      if (requestUrl.pathname === '/api/resources/truenas-main/facets') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            recentChanges: [],
            counts: {
              recentChanges: 0,
            },
          }),
        });
        return;
      }

      await route.continue();
    });

    await page.goto('/infrastructure?source=truenas&resource=truenas-main', {
      waitUntil: 'domcontentloaded',
    });

    await expect(page).toHaveURL(/\/infrastructure\?source=truenas&resource=truenas-main/);
    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await page.getByRole('button', { name: 'List' }).click();

    const showAccessButton = page.getByRole('button', { name: 'Show access' });
    await expect(showAccessButton).toBeVisible();
    await showAccessButton.click();

    const workloadsLink = page.getByRole('link', { name: 'Open related workloads for TrueNAS Main' });
    const storageLink = page.getByRole('link', { name: 'Open related storage for TrueNAS Main' });
    const recoveryLink = page.getByRole('link', { name: 'Open related recovery for TrueNAS Main' });

    await expect(workloadsLink).toHaveAttribute(
      'href',
      '/workloads?type=app-container&platform=truenas&agent=truenas-main',
    );
    await expect(storageLink).toHaveAttribute(
      'href',
      '/storage?source=truenas&node=truenas-main',
    );
    await expect(recoveryLink).toHaveAttribute(
      'href',
      '/recovery?platform=truenas&node=truenas-main',
    );

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
