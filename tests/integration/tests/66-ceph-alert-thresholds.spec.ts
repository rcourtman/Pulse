import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

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
      `ceph-alert-thresholds-${workerInfo.project.name}.json`,
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

test.use({ serviceWorkers: 'block' });

test.describe('Ceph alert thresholds', () => {
  test.setTimeout(180_000);

  test('keeps shared Ceph storage visible and editable on the live thresholds surface', async ({
    page,
  }) => {
    await page.addInitScript(() => {
      class FakeWebSocket {
        static CONNECTING = 0;
        static OPEN = 1;
        static CLOSING = 2;
        static CLOSED = 3;

        readonly url: string;
        readyState = FakeWebSocket.CLOSED;
        onopen: ((event: Event) => void) | null = null;
        onclose:
          | ((event: { code?: number; reason?: string; wasClean?: boolean }) => void)
          | null = null;
        onerror: ((event: Event) => void) | null = null;
        onmessage: ((event: MessageEvent) => void) | null = null;

        constructor(url: string) {
          this.url = url;
          queueMicrotask(() => {
            this.onclose?.({ code: 1006, reason: 'e2e websocket disabled', wasClean: false });
          });
        }

        close() {
          this.readyState = FakeWebSocket.CLOSED;
        }

        send() {}

        addEventListener() {}

        removeEventListener() {}
      }

      // @ts-expect-error Playwright init script runs in the browser context.
      window.WebSocket = FakeWebSocket;
    });

    await page.context().route('**/api/resources**', async (route) => {
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
              id: 'Main-pve1',
              type: 'agent',
              name: 'pve1',
              displayName: 'pve1',
              platformId: 'Main',
              platformType: 'proxmox-pve',
              sourceType: 'api',
              sources: ['proxmox'],
              status: 'online',
              canonicalIdentity: {
                displayName: 'pve1',
                hostname: 'pve1',
                platformId: 'Main',
              },
              proxmox: {
                node: 'pve1',
                instance: 'Main',
              },
              platformData: {
                node: 'pve1',
                instance: 'Main',
                clusterName: 'Main',
                isClusterMember: true,
                sources: ['proxmox'],
              },
            },
            {
              id: 'Main-cluster-ceph-pool',
              type: 'storage',
              name: 'ceph-pool',
              displayName: 'ceph-pool',
              platformId: 'Main',
              platformType: 'proxmox-pve',
              sourceType: 'api',
              sources: ['proxmox'],
              status: 'available',
              canonicalIdentity: {
                displayName: 'ceph-pool',
                platformId: 'Main-cluster-ceph-pool',
              },
              storage: {
                type: 'rbd',
                shared: true,
                isCeph: true,
                nodes: ['pve1', 'pve2'],
              },
              proxmox: {
                node: 'cluster',
                instance: 'Main',
              },
              platformData: {
                node: 'cluster',
                instance: 'Main',
                sources: ['proxmox'],
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

    await page.context().route('**/api/alerts/config', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          enabled: true,
          activationState: 'active',
          storageDefault: {
            trigger: 85,
            clear: 80,
          },
          overrides: {
            'Main-pve1-ceph-pool': {
              usage: {
                trigger: 92,
                clear: 82,
              },
            },
          },
        }),
      });
    });

    await page.context().route('**/api/alerts/active', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      });
    });

    await page.context().route('**/api/notifications/email', async (route) => {
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

    await page.context().route('**/api/notifications/apprise', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          enabled: false,
        }),
      });
    });

    await page.goto('/alerts/thresholds/infrastructure', {
      waitUntil: 'domcontentloaded',
    });

    await expect(page).toHaveURL(/\/alerts\/thresholds\/infrastructure/);
    await expect(page.getByRole('heading', { name: 'Alert Thresholds' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Storage Devices' })).toBeVisible();
    await expect(page.getByText('ceph-pool', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Revert to defaults for ceph-pool' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Edit thresholds for ceph-pool' })).toBeVisible();
  });
});
