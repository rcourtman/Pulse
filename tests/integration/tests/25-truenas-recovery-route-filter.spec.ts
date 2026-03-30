import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_PATH = '/tmp/truenas-recovery-route-filter.png';

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
      `truenas-recovery-route-filter-${workerInfo.project.name}.json`,
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

test.describe('TrueNAS recovery route filters', () => {
  test.setTimeout(180_000);

  test('keeps route-owned platform and node filters visible while recovery data warms', async ({
    page,
  }) => {
    const recoveryRequests: string[] = [];
    let releaseRecoveryResponses: (() => void) | null = null;
    const recoveryResponseGate = new Promise<void>((resolve) => {
      releaseRecoveryResponses = resolve;
    });

    await page.route('**/api/resources**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 'truenas-main',
              type: 'truenas',
              name: 'tower',
              displayName: 'TrueNAS Tower',
              platformId: 'truenas-main',
              platformType: 'truenas',
              sourceType: 'api',
              status: 'online',
              lastSeen: '2026-03-29T22:00:00Z',
              platformData: {
                sources: ['truenas'],
              },
            },
          ],
          meta: { page: 1, limit: 200, total: 1, totalPages: 1 },
        }),
      });
    });

    await page.route('**/api/recovery/rollups*', async (route) => {
      recoveryRequests.push(route.request().url());
      await recoveryResponseGate;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              rollupId: 'ext:truenas-1',
              itemRef: { type: 'truenas-dataset', name: 'tank/apps', id: 'tank/apps' },
              display: { itemType: 'dataset', subjectLabel: 'tank/apps', nodeHostLabel: 'tower' },
              lastAttemptAt: '2026-03-29T09:00:00.000Z',
              lastSuccessAt: '2026-03-29T09:00:00.000Z',
              lastOutcome: 'success',
              platforms: ['truenas'],
            },
          ],
          meta: { page: 1, limit: 500, total: 1, totalPages: 1 },
        }),
      });
    });

    await page.route('**/api/recovery/points*', async (route) => {
      recoveryRequests.push(route.request().url());
      await recoveryResponseGate;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 'truenas-point-1',
              platform: 'truenas',
              kind: 'snapshot',
              mode: 'snapshot',
              outcome: 'success',
              completedAt: '2026-03-29T09:00:00.000Z',
              node: 'tower',
              itemRef: {
                type: 'truenas-dataset',
                name: 'tank/apps',
                id: 'tank/apps',
              },
              display: {
                itemType: 'dataset',
                subjectType: 'truenas-dataset',
                subjectLabel: 'tank/apps',
                nodeHostLabel: 'tower',
              },
            },
          ],
          meta: { page: 1, limit: 200, total: 1, totalPages: 1 },
        }),
      });
    });

    await page.route('**/api/recovery/facets*', async (route) => {
      recoveryRequests.push(route.request().url());
      await recoveryResponseGate;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            clusters: [],
            nodesAgents: ['tower'],
            namespaces: [],
            itemTypes: ['dataset'],
            hasSize: false,
            hasVerification: false,
            hasEntityId: false,
          },
        }),
      });
    });

    await page.route('**/api/recovery/series*', async (route) => {
      recoveryRequests.push(route.request().url());
      await recoveryResponseGate;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [{ day: '2026-03-29', total: 1, snapshot: 1, local: 0, remote: 0 }],
        }),
      });
    });

    await page.goto('/recovery?view=events&platform=truenas&node=tower', {
      waitUntil: 'domcontentloaded',
    });

    await expect(page).toHaveURL(/\/recovery\?view=events&platform=truenas&node=tower/);
    await expect.poll(() => recoveryRequests.length).toBeGreaterThan(0);

    releaseRecoveryResponses?.();

    await expect(page.getByTestId('recovery-page')).toBeVisible();
    await expect(page.getByLabel('Platform')).toHaveValue('truenas');
    await expect(page.getByText('Host / Agent')).toBeVisible();
    await expect(page.getByText('tower')).toBeVisible();
    await expect(page.getByText('tank/apps')).toBeVisible();

    expect(
      recoveryRequests.some(
        (url) => url.includes('/api/recovery/rollups') && url.includes('platform=truenas'),
      ),
    ).toBe(true);
    expect(
      recoveryRequests.some(
        (url) =>
          url.includes('/api/recovery/points') &&
          url.includes('platform=truenas') &&
          url.includes('node=tower'),
      ),
    ).toBe(true);
    expect(
      recoveryRequests.some(
        (url) =>
          url.includes('/api/recovery/facets') &&
          url.includes('platform=truenas') &&
          url.includes('node=tower'),
      ),
    ).toBe(true);

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
