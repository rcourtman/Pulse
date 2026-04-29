import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ARTIFACTS_DIR = path.resolve(__dirname, '..', '..', 'tmp', 'offline-proxmox-node-visibility');

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
      `offline-proxmox-node-${workerInfo.project.name}.json`,
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

const OFFLINE_RESOURCE = {
  id: 'offline-pve',
  type: 'agent',
  name: 'pve-offline',
  displayName: 'PVE Offline',
  platformId: 'offline-pve',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  sources: ['proxmox-pve'],
  status: 'offline',
  lastSeen: '2026-04-21T20:00:00Z',
  canonicalIdentity: {
    displayName: 'PVE Offline',
    hostname: 'pve-offline.lab.local',
    platformId: 'offline-pve',
  },
  agent: {
    hostname: 'pve-offline.lab.local',
    platform: 'Proxmox VE',
    uptimeSeconds: 0,
    memory: { total: 0, used: 0, free: 0, usage: 0 },
    disks: [],
    networkInterfaces: [],
    raid: [],
  },
  platformData: {
    sources: ['proxmox-pve'],
    proxmox: {
      instance: 'homelab',
      nodeName: 'pve-offline',
      clusterName: 'homelab',
      connectionHealth: 'error',
      pveVersion: '8.3-1',
      kernelVersion: '6.8.12-9-pve',
      uptime: 0,
      cpuInfo: {
        model: 'Intel Xeon',
        cores: 8,
        sockets: 1,
        mhz: '2400',
      },
      memory: { total: 0, used: 0, free: 0, usage: 0 },
      disk: { total: 0, used: 0, free: 0, usage: 0 },
      loadAverage: [],
      linkedAgentId: '',
    },
  },
} as const;

const EMPTY_INFRASTRUCTURE_CHARTS = {
  nodeData: {},
  agentData: {},
  dockerHostData: {},
  timestamp: Date.now(),
  stats: {
    oldestDataTimestamp: 0,
    range: '1h',
    rangeSeconds: 3600,
    pointCounts: {
      total: 0,
      nodes: 0,
      agents: 0,
      dockerHosts: 0,
    },
  },
} as const;

const EMPTY_STORAGE_SUMMARY_TREND = {
  capacity: [],
  stats: {
    oldestDataTimestamp: 0,
    range: '24h',
    rangeSeconds: 86400,
    pointCounts: {
      total: 0,
      storage: 0,
    },
  },
} as const;

const EMPTY_RECOVERY_ROLLUPS = {
  data: [],
  meta: {
    page: 1,
    limit: 500,
    total: 0,
    totalPages: 1,
  },
} as const;

test.describe('Offline Proxmox node visibility', () => {
  test.setTimeout(180_000);

  test('keeps an offline Proxmox node visible on the infrastructure surface', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');
    fs.mkdirSync(ARTIFACTS_DIR, { recursive: true });

    await page.addInitScript(() => {
      localStorage.setItem('pulse_whats_new_v2_shown', 'true');
    });

    await page.route('**/api/resources**', async (route) => {
      const requestUrl = new URL(route.request().url());

      if (requestUrl.pathname === '/api/resources') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: [OFFLINE_RESOURCE],
            meta: {
              page: 1,
              limit: 200,
              total: 1,
              totalPages: 1,
            },
          }),
        });
        return;
      }

      await route.continue();
    });

    await page.route('**/api/charts/infrastructure**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(EMPTY_INFRASTRUCTURE_CHARTS),
      });
    });

    await page.route('**/api/charts/storage-summary**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(EMPTY_STORAGE_SUMMARY_TREND),
      });
    });

    await page.route('**/api/recovery/rollups**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(EMPTY_RECOVERY_ROLLUPS),
      });
    });

    await page.goto('/infrastructure?source=proxmox-pve', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('infrastructure-page')).toBeVisible();

    const infrastructureTable = page.locator('[data-testid="infrastructure-page"] table').first();
    await expect(infrastructureTable).toContainText('PVE Offline');
    await expect(infrastructureTable).toContainText('Offline');
  });
});
