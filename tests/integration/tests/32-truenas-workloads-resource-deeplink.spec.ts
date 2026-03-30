import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = '/tmp/truenas-workloads-resource-deeplink.png';

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
      `truenas-workloads-resource-deeplink-${workerInfo.project.name}.json`,
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

test.describe('TrueNAS workloads resource deep links', () => {
  test.setTimeout(180_000);

  test('opens the canonical workload drawer without inventing a node scope', async ({ page }) => {
    await page.route('**/api/resources**', async (route) => {
      const requestUrl = new URL(route.request().url());
      if (
        requestUrl.pathname !== '/api/resources' ||
        requestUrl.searchParams.get('type') !== 'vm,system-container,app-container,pod'
      ) {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 'app-container:truenas-main:nextcloud',
              type: 'app-container',
              name: 'nextcloud',
              status: 'running',
              lastSeen: '2026-03-29T21:00:00Z',
              node: 'truenas-main',
              instance: 'truenas-main',
              sources: ['truenas'],
              parentName: 'truenas-main',
              metrics: {
                cpu: { value: 12, percent: 12 },
                memory: { total: 8 * 1024 * 1024 * 1024, used: 2 * 1024 * 1024 * 1024 },
                disk: { total: 100 * 1024 * 1024 * 1024, used: 40 * 1024 * 1024 * 1024 },
                netIn: { value: 2048 },
                netOut: { value: 1024 },
                diskRead: { value: 512 },
                diskWrite: { value: 256 },
              },
              docker: {
                containerId: 'ix-nextcloud',
                hostname: 'truenas-main',
                imageName: 'nextcloud:29',
                runtime: 'docker',
                hostSourceId: 'truenas-main',
              },
            },
            {
              id: 'cluster-a:pve1:101',
              type: 'vm',
              name: 'vm-101',
              status: 'running',
              lastSeen: '2026-03-29T21:00:00Z',
              node: 'pve1',
              instance: 'cluster-a',
              vmid: 101,
              sources: ['proxmox'],
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

    await page.goto(
      '/workloads?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud',
      {
        waitUntil: 'domcontentloaded',
      },
    );

    await page.waitForURL(
      /\/workloads\?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud/,
    );
    await expect(page.locator('#dashboard-type-filter')).toHaveValue('app-container');
    await expect(page.locator('#workloads-platform-filter')).toHaveValue('truenas');
    await expect(page.locator('#workloads-node-filter')).toHaveValue(/truenas-main/);
    await expect(
      page.locator('tr[data-guest-id="app-container:truenas-main:nextcloud"]'),
    ).toContainText('nextcloud');
    await expect(page.getByText('Open related infrastructure', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Overview' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Discovery' })).toHaveCount(0);

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
