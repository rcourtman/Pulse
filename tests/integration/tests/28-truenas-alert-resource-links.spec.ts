import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = '/tmp/truenas-alert-resource-links.png';

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
      `truenas-alert-resource-links-${workerInfo.project.name}.json`,
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

test.describe('TrueNAS alert resource links', () => {
  test.setTimeout(180_000);

  test('keeps TrueNAS alert investigation on canonical resource handoff routes', async ({
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
              id: 'truenas-main',
              type: 'agent',
              name: 'truenas-main',
              displayName: 'TrueNAS Main',
              platformId: 'truenas-main',
              platformType: 'truenas',
              sourceType: 'hybrid',
              sources: ['agent', 'truenas'],
              status: 'online',
              lastSeen: '2026-03-30T10:00:00Z',
              canonicalIdentity: {
                displayName: 'TrueNAS Main',
                hostname: 'truenas-main',
                platformId: 'truenas-main',
              },
              platformData: {
                sources: ['agent', 'truenas'],
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

    await page.route('**/api/alerts/history**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          {
            id: 'truenas-alert-1',
            type: 'host-offline',
            level: 'critical',
            startTime: '2026-03-30T09:00:00Z',
            lastSeen: '2026-03-30T09:15:00Z',
            resourceId: 'truenas-main',
            resourceName: 'TrueNAS Main',
            message: 'TrueNAS Main is offline',
            acknowledged: false,
            node: 'truenas-main',
            nodeDisplayName: 'TrueNAS Main',
            metadata: {
              resourceType: 'agent',
            },
          },
        ]),
      });
    });

    await page.route('**/api/alerts/incidents**', async (route) => {
      const requestUrl = new URL(route.request().url());
      if (requestUrl.searchParams.get('resource_id') !== 'truenas-main') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(null),
        });
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          {
            id: 'incident-truenas-1',
            alertType: 'Host Offline',
            level: 'critical',
            status: 'open',
            acknowledged: false,
            openedAt: '2026-03-30T09:00:00Z',
            message: 'TrueNAS Main is offline',
            events: [
              {
                id: 'incident-event-1',
                type: 'opened',
                timestamp: '2026-03-30T09:00:00Z',
                summary: 'Alert opened',
              },
            ],
          },
        ]),
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

    await page.goto('/alerts/history', {
      waitUntil: 'domcontentloaded',
    });

    await expect(page.getByRole('heading', { name: 'Alert History' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Resource' })).toBeVisible();

    await page.getByRole('button', { name: 'Resource' }).click();

    await expect(page.getByRole('heading', { name: 'Resource incidents' })).toBeVisible();
    await expect(
      page.getByRole('link', { name: 'Open related infrastructure for TrueNAS Main' }),
    ).toHaveAttribute('href', '/infrastructure?resource=truenas-main');
    await expect(
      page.getByRole('link', { name: 'Open related workloads for TrueNAS Main' }),
    ).toHaveAttribute(
      'href',
      '/workloads?type=app-container&platform=truenas&agent=truenas-main',
    );
    await expect(
      page.getByRole('link', { name: 'Open related storage for TrueNAS Main' }),
    ).toHaveAttribute('href', '/storage?source=truenas&node=truenas-main');
    await expect(
      page.getByRole('link', { name: 'Open related recovery for TrueNAS Main' }),
    ).toHaveAttribute('href', '/recovery?platform=truenas&node=truenas-main');

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
