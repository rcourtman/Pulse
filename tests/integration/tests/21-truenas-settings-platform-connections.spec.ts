import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = path.resolve(
  __dirname,
  '..',
  '..',
  'tmp',
  'truenas-settings-platform-connections.png',
);
const OPERATIONS_SCREENSHOT_PATH = path.resolve(
  __dirname,
  '..',
  '..',
  'tmp',
  'truenas-settings-operations-summary.png',
);

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
      `truenas-settings-platform-connections-${workerInfo.project.name}.json`,
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

test.describe('TrueNAS platform connections settings', () => {
  test.setTimeout(180_000);

  test('renders the platform-connections workspace with the TrueNAS integration shell', async ({
    page,
  }) => {
    await page.route('**/api/truenas/connections', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          {
            id: 'truenas-1',
            name: 'Tower NAS',
            host: 'tower.local',
            port: 443,
            apiKey: '********',
            useHttps: true,
            insecureSkipVerify: false,
            fingerprint: '',
            enabled: true,
          },
          {
            id: 'truenas-2',
            name: 'Backup Vault',
            host: 'vault.local',
            port: 443,
            username: 'admin',
            password: '********',
            useHttps: true,
            insecureSkipVerify: true,
            fingerprint: 'sha256:example',
            enabled: false,
          },
        ]),
      });
    });

    await page.goto('/settings/infrastructure/platforms/truenas', {
      waitUntil: 'domcontentloaded',
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/truenas/, {
      timeout: 15_000,
    });

    await expect(
      page.getByRole('heading', { level: 1, name: 'Infrastructure Operations' }),
    ).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Platform connections' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await expect(page.getByRole('tab', { name: 'TrueNAS' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await expect(page.getByRole('tab', { name: 'Proxmox' })).toHaveAttribute(
      'aria-selected',
      'false',
    );

    await expect(page.getByText('TrueNAS platform integration')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Add TrueNAS connection' })).toBeVisible();
    await expect(page.getByText('Tower NAS')).toBeVisible();
    await expect(page.getByText('Backup Vault')).toBeVisible();
    await expect(page.getByText('API key auth')).toBeVisible();
    await expect(page.getByText('Username/password auth')).toBeVisible();

    fs.mkdirSync(path.dirname(SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });

  test('counts TrueNAS alongside the other platform connections on the operations summary', async ({
    page,
  }) => {
    await page.route('**/api/config/nodes', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          {
            id: 'pve-1',
            type: 'pve',
            name: 'pve-main',
            host: 'pve-main.local',
            user: 'root@pam',
            verifySSL: true,
            monitorVMs: true,
            monitorContainers: true,
            monitorStorage: true,
            monitorBackups: true,
            monitorPhysicalDisks: true,
            status: 'connected',
          },
          {
            id: 'pbs-1',
            type: 'pbs',
            name: 'pbs-main',
            host: 'pbs-main.local',
            user: 'root@pam',
            verifySSL: true,
            monitorDatastores: true,
            monitorSyncJobs: true,
            monitorVerifyJobs: true,
            monitorPruneJobs: true,
            monitorGarbageJobs: true,
            status: 'connected',
          },
          {
            id: 'pbs-2',
            type: 'pbs',
            name: 'pbs-vault',
            host: 'pbs-vault.local',
            user: 'root@pam',
            verifySSL: true,
            monitorDatastores: true,
            monitorSyncJobs: true,
            monitorVerifyJobs: true,
            monitorPruneJobs: true,
            monitorGarbageJobs: true,
            status: 'connected',
          },
          {
            id: 'pmg-1',
            type: 'pmg',
            name: 'pmg-main',
            host: 'pmg-main.local',
            user: 'root@pam',
            verifySSL: true,
            monitorMailStats: true,
            monitorQueues: true,
            monitorQuarantine: true,
            monitorDomainStats: true,
            status: 'connected',
          },
        ]),
      });
    });

    await page.route('**/api/truenas/connections', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          {
            id: 'truenas-1',
            name: 'Tower NAS',
            host: 'tower.local',
            port: 443,
            apiKey: '********',
            useHttps: true,
            insecureSkipVerify: false,
            enabled: true,
          },
          {
            id: 'truenas-2',
            name: 'Backup Vault',
            host: 'vault.local',
            port: 443,
            apiKey: '********',
            useHttps: true,
            insecureSkipVerify: false,
            enabled: true,
          },
        ]),
      });
    });

    await page.goto('/settings/infrastructure/operations', {
      waitUntil: 'domcontentloaded',
    });
    await page.waitForURL(/\/settings\/infrastructure\/operations/, {
      timeout: 15_000,
    });

    await expect(
      page.getByRole('heading', { level: 1, name: 'Infrastructure Operations' }),
    ).toBeVisible();
    await expect(page.getByRole('heading', { level: 3, name: 'Platform connections' })).toBeVisible();
    await expect(page.getByTestId('platform-connections-pve')).toContainText('1');
    await expect(page.getByTestId('platform-connections-pbs')).toContainText('2');
    await expect(page.getByTestId('platform-connections-pmg')).toContainText('1');
    await expect(page.getByTestId('platform-connections-truenas')).toContainText('2');
    await expect(page.getByTestId('platform-connections-truenas')).toContainText(
      'API-backed NAS connections',
    );

    fs.mkdirSync(path.dirname(OPERATIONS_SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: OPERATIONS_SCREENSHOT_PATH, fullPage: true });
  });
});
