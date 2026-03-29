import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = '/tmp/truenas-storage-source-filter.png';

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
      `truenas-storage-source-filter-${workerInfo.project.name}.json`,
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

test.describe('TrueNAS storage source filter', () => {
  test.setTimeout(180_000);

  test('restores the canonical TrueNAS storage handoff on the storage route', async ({ page }) => {
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
              id: 'truenas-main',
              type: 'agent',
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
              agent: {
                hostname: 'truenas-main',
                platform: 'TrueNAS SCALE',
                uptimeSeconds: 86400,
              },
              platformData: {
                sources: ['agent', 'truenas'],
              },
            },
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
            {
              id: 'storage-truenas-display',
              type: 'storage',
              name: 'tank',
              displayName: 'tank',
              parentId: 'truenas-main',
              parentName: 'truenas-main',
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
              metrics: {
                disk: {
                  total: 2_000 * 1024 * 1024 * 1024,
                  used: 840 * 1024 * 1024 * 1024,
                  percent: 42,
                },
              },
              storage: {
                platform: 'truenas',
                type: 'zfs-pool',
                topology: 'pool',
                isZfs: true,
              },
              platformData: {
                sources: ['truenas'],
              },
            },
            {
              id: 'storage-pve-local',
              type: 'storage',
              name: 'local-zfs',
              displayName: 'local-zfs',
              parentId: 'pve-main',
              parentName: 'pve-main',
              platformId: 'cluster-a',
              platformType: 'proxmox-pve',
              sourceType: 'api',
              sources: ['proxmox-pve'],
              status: 'online',
              lastSeen: '2026-03-29T22:00:00Z',
              canonicalIdentity: {
                displayName: 'local-zfs',
                platformId: 'cluster-a',
              },
              metrics: {
                disk: {
                  total: 1_000 * 1024 * 1024 * 1024,
                  used: 400 * 1024 * 1024 * 1024,
                  percent: 40,
                },
              },
              storage: {
                platform: 'proxmox-pve',
                type: 'zfspool',
                topology: 'pool',
                isZfs: true,
              },
              platformData: {
                node: 'pve-main',
                sources: ['proxmox-pve'],
              },
            },
          ],
          meta: {
            page: 1,
            limit: 200,
            total: 4,
            totalPages: 1,
          },
        }),
      });
    });

    await page.goto('/storage?source=truenas&node=truenas-main', {
      waitUntil: 'domcontentloaded',
    });

    await expect(page).toHaveURL(/\/storage\?source=truenas&node=truenas-main/);
    await expect(page.locator('#storage-source-filter')).toHaveValue('truenas');
    await expect(page.getByLabel('Node')).toHaveValue('truenas-main');

    const sourceOptions = await page.locator('#storage-source-filter option').evaluateAll((options) =>
      options.map((option) => ({
        value: option.getAttribute('value'),
        label: option.textContent?.trim(),
      })),
    );
    expect(sourceOptions).toEqual([
      { value: 'all', label: 'All Sources' },
      { value: 'proxmox-pve', label: 'PVE' },
      { value: 'truenas', label: 'TrueNAS' },
    ]);

    const storageTable = page.locator('table').first();
    await expect(storageTable).toContainText('tank');
    await expect(storageTable).not.toContainText('local-zfs');

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
