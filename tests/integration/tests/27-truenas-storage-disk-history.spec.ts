import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = '/tmp/truenas-storage-disk-history.png';

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
      `truenas-storage-disk-history-${workerInfo.project.name}.json`,
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

test.describe('TrueNAS storage disk history', () => {
  test.setTimeout(180_000);

  test('uses the canonical disk metrics target for TrueNAS disk history even without serial or WWN', async ({
    page,
  }) => {
    const historyRequests: string[] = [];

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
              id: 'disk-truenas-sda',
              type: 'physical_disk',
              name: 'disk-truenas-sda',
              displayName: 'disk-truenas-sda',
              parentId: 'truenas-main',
              parentName: 'truenas-main',
              platformId: 'truenas-main',
              platformType: 'truenas',
              sourceType: 'api',
              sources: ['truenas'],
              status: 'online',
              lastSeen: '2026-03-29T22:00:00Z',
              metricsTarget: {
                resourceType: 'disk',
                resourceId: 'disk:truenas-main:sda',
              },
              canonicalIdentity: {
                displayName: 'disk-truenas-sda',
                hostname: 'truenas-main',
                platformId: 'truenas-main',
              },
              physicalDisk: {
                devPath: '/dev/sda',
                model: 'Seagate IronWolf',
                serial: '',
                wwn: '',
                diskType: 'hdd',
                sizeBytes: 4_000 * 1024 * 1024 * 1024,
                health: 'PASSED',
                temperature: 41,
                smart: {},
              },
              platformData: {
                sources: ['truenas'],
                physicalDisk: {
                  serial: '',
                  wwn: '',
                },
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

    await page.route('**/api/storage-charts**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          pools: {},
          disks: {},
          stats: {
            oldestDataTimestamp: Date.parse('2026-03-29T20:00:00Z'),
          },
        }),
      });
    });

    await page.route('**/api/metrics-store/history**', async (route) => {
      const requestUrl = new URL(route.request().url());
      historyRequests.push(requestUrl.search);

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          resourceType: requestUrl.searchParams.get('resourceType') || 'disk',
          resourceId: requestUrl.searchParams.get('resourceId') || '',
          metric: requestUrl.searchParams.get('metric') || 'smart_temp',
          range: requestUrl.searchParams.get('range') || '24h',
          start: Date.parse('2026-03-29T20:00:00Z'),
          end: Date.parse('2026-03-29T22:00:00Z'),
          points: [
            {
              timestamp: Date.parse('2026-03-29T20:30:00Z'),
              value: 39,
              min: 39,
              max: 39,
            },
            {
              timestamp: Date.parse('2026-03-29T21:30:00Z'),
              value: 41,
              min: 41,
              max: 41,
            },
          ],
          source: 'store',
        }),
      });
    });

    await page.goto('/storage?tab=disks&source=truenas&node=truenas-main', {
      waitUntil: 'domcontentloaded',
    });

    await expect(page).toHaveURL(/\/storage\?tab=disks&source=truenas&node=truenas-main/);
    await expect(page.getByRole('tab', { name: 'Physical Disks' })).toHaveAttribute(
      'aria-selected',
      'true',
    );

    await page.getByRole('button', { name: 'Toggle details for Seagate IronWolf' }).click();

    await expect(page.getByText('Historical disk charts are unavailable', { exact: false })).toHaveCount(0);
    await expect(page.getByText('Temperature').first()).toBeVisible();
    await expect
      .poll(() =>
        historyRequests.some((query) => {
          const params = new URLSearchParams(query);
          return (
            params.get('resourceType') === 'disk' &&
            params.get('resourceId') === 'disk:truenas-main:sda' &&
            params.get('metric') === 'smart_temp'
          );
        }),
      )
      .toBe(true);

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
