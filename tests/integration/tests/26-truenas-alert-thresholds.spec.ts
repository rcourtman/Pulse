import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = '/tmp/truenas-alert-thresholds.png';

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
      `truenas-alert-thresholds-${workerInfo.project.name}.json`,
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

test.describe('TrueNAS alert thresholds', () => {
  test.setTimeout(180_000);

  test('routes TrueNAS through the neutral infrastructure and systems thresholds surfaces', async ({
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
              id: 'truenas-resource',
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
              identity: {
                hostname: 'truenas-main',
              },
              agent: {
                agentId: 'truenas-main',
                hostname: 'truenas-main',
                platform: 'TrueNAS SCALE',
                disks: [
                  {
                    mountpoint: '/mnt/tank',
                    type: 'zfs',
                    used: 50 * 1024 * 1024 * 1024,
                    total: 100 * 1024 * 1024 * 1024,
                  },
                ],
              },
              platformData: {
                sources: ['agent', 'truenas'],
                agent: {
                  agentId: 'truenas-main',
                  disks: [
                    {
                      mountpoint: '/mnt/tank',
                      type: 'zfs',
                      used: 50 * 1024 * 1024 * 1024,
                      total: 100 * 1024 * 1024 * 1024,
                    },
                  ],
                },
              },
            },
            {
              id: 'storage-truenas-tank',
              type: 'storage',
              name: 'tank',
              displayName: 'tank',
              parentId: 'truenas-resource',
              parentName: 'TrueNAS Main',
              platformId: 'truenas-storage-1',
              platformType: 'truenas',
              sourceType: 'api',
              sources: ['truenas'],
              status: 'online',
              lastSeen: '2026-03-29T22:00:00Z',
              canonicalIdentity: {
                displayName: 'tank',
                platformId: 'truenas-storage-1',
              },
              storage: {
                platform: 'truenas',
                type: 'zfs-pool',
                topology: 'pool',
                isZfs: true,
              },
              platformData: {
                node: 'truenas-main',
                instance: 'tank',
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
    });

    await page.route('**/api/alerts/config', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          enabled: true,
          activationState: 'active',
          overrides: {},
        }),
      });
    });

    await page.route('**/api/alerts/active', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      });
    });

    await page.route('**/api/notifications/email', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          enabled: false,
          provider: '',
          server: '',
          port: 587,
          username: '',
          password: '',
          from: '',
          to: [],
          tls: false,
          startTLS: false,
        }),
      });
    });

    await page.route('**/api/notifications/apprise', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          enabled: false,
        }),
      });
    });

    await page.goto('/alerts/thresholds/proxmox', {
      waitUntil: 'domcontentloaded',
    });

    await expect(page).toHaveURL(/\/alerts\/thresholds\/infrastructure/);
    await expect(page.getByRole('heading', { name: 'Alert Thresholds' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Infrastructure' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Storage Devices' })).toBeVisible();
    await expect(page.getByText('tank', { exact: true })).toBeVisible();

    await page.getByRole('button', { name: 'Systems' }).click();

    await expect(page).toHaveURL(/\/alerts\/thresholds\/systems/);
    const systemsTable = page.locator('table').first();
    const systemDisksSection = page.getByTestId('section-agentDisks');
    await expect(page.getByRole('heading', { name: 'Systems' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'System Disks' })).toBeVisible();
    await expect(systemsTable.getByText('TrueNAS Main', { exact: true })).toBeVisible();
    await expect(systemDisksSection.getByText('TrueNAS Main', { exact: true })).toBeVisible();
    await expect(systemDisksSection.getByText('/mnt/tank', { exact: true })).toBeVisible();

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
