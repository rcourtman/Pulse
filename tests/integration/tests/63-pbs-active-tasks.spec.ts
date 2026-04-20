import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ARTIFACTS_DIR = path.resolve(__dirname, '..', '..', 'tmp', 'pbs-active-tasks');
const RESOURCE_ID = 'pbs-active';
const RESOURCE_ID_ENCODED = encodeURIComponent(RESOURCE_ID);

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
      `pbs-active-tasks-${workerInfo.project.name}.json`,
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

test.describe('PBS active tasks', () => {
  test.setTimeout(180_000);

  test('surfaces running PBS tasks in the service table and shared detail drawer', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');
    fs.mkdirSync(ARTIFACTS_DIR, { recursive: true });

    await page.route('**/api/resources**', async (route) => {
      const requestUrl = new URL(route.request().url());

      if (requestUrl.pathname === '/api/resources') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: [
              {
                id: RESOURCE_ID,
                type: 'pbs',
                name: 'pbs-active',
                displayName: 'PBS Active',
                platformId: RESOURCE_ID,
                platformType: 'proxmox-pbs',
                sourceType: 'api',
                sources: ['pbs'],
                status: 'online',
                lastSeen: '2026-04-20T22:30:00Z',
                canonicalIdentity: {
                  displayName: 'PBS Active',
                  hostname: 'pbs-active.lab.local',
                  platformId: RESOURCE_ID,
                },
                pbs: {
                  instanceId: RESOURCE_ID,
                  hostname: 'pbs-active.lab.local',
                  version: '3.4-1',
                  uptimeSeconds: 86400,
                  datastoreCount: 2,
                  backupJobCount: 2,
                  syncJobCount: 1,
                  verifyJobCount: 1,
                  backupJobs: [
                    {
                      id: 'backup-nightly',
                      store: 'fast',
                      type: 'vm',
                      vmid: '100',
                      lastBackup: '2026-04-20T00:00:00Z',
                      nextRun: '2026-04-21T00:00:00Z',
                      status: 'running',
                      error: '',
                    },
                    {
                      id: 'backup-weekly',
                      store: 'archive',
                      type: 'ct',
                      vmid: '200',
                      lastBackup: '2026-04-19T00:00:00Z',
                      nextRun: '2026-04-26T00:00:00Z',
                      status: 'ok',
                      error: '',
                    },
                  ],
                  syncJobs: [
                    {
                      id: 'sync-remote',
                      store: 'fast',
                      remote: 'offsite',
                      status: 'queued',
                      lastSync: '2026-04-20T12:00:00Z',
                      nextRun: '2026-04-20T23:00:00Z',
                      error: '',
                    },
                  ],
                  verifyJobs: [
                    {
                      id: 'verify-1',
                      store: 'fast',
                      status: 'ok',
                      lastVerify: '2026-04-20T01:00:00Z',
                      nextRun: '2026-04-21T01:00:00Z',
                      error: '',
                    },
                  ],
                  connectionHealth: 'online',
                },
                platformData: {
                  sources: ['pbs'],
                  pbs: {
                    instanceId: RESOURCE_ID,
                    hostname: 'pbs-active.lab.local',
                    version: '3.4-1',
                    uptimeSeconds: 86400,
                    datastoreCount: 2,
                    backupJobCount: 2,
                    syncJobCount: 1,
                    verifyJobCount: 1,
                    backupJobs: [
                      {
                        id: 'backup-nightly',
                        store: 'fast',
                        type: 'vm',
                        vmid: '100',
                        lastBackup: '2026-04-20T00:00:00Z',
                        nextRun: '2026-04-21T00:00:00Z',
                        status: 'running',
                        error: '',
                      },
                      {
                        id: 'backup-weekly',
                        store: 'archive',
                        type: 'ct',
                        vmid: '200',
                        lastBackup: '2026-04-19T00:00:00Z',
                        nextRun: '2026-04-26T00:00:00Z',
                        status: 'ok',
                        error: '',
                      },
                    ],
                    syncJobs: [
                      {
                        id: 'sync-remote',
                        store: 'fast',
                        remote: 'offsite',
                        status: 'queued',
                        lastSync: '2026-04-20T12:00:00Z',
                        nextRun: '2026-04-20T23:00:00Z',
                        error: '',
                      },
                    ],
                    verifyJobs: [
                      {
                        id: 'verify-1',
                        store: 'fast',
                        status: 'ok',
                        lastVerify: '2026-04-20T01:00:00Z',
                        nextRun: '2026-04-21T01:00:00Z',
                        error: '',
                      },
                    ],
                    connectionHealth: 'online',
                  },
                },
              },
            ],
            meta: {
              page: 1,
              limit: 100,
              total: 1,
              totalPages: 1,
            },
          }),
        });
        return;
      }

      if (requestUrl.pathname === `/api/resources/${RESOURCE_ID_ENCODED}/facets`) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            capabilities: [],
            relationships: [],
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

    await page.route('**/api/ai/intelligence**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          resource_id: RESOURCE_ID,
          resource_name: 'PBS Active',
          resource_type: 'pbs',
          health: {
            score: 94,
            grade: 'A',
            trend: 'stable',
            factors: [],
            prediction: 'Pulse is surfacing active PBS backup activity through the shared infrastructure drawer.',
          },
          dependencies: [],
          dependents: [],
          correlations: [],
          recent_changes: [],
          note_count: 0,
        }),
      });
    });

    await page.goto(
      `/infrastructure?source=proxmox-pbs&resource=${encodeURIComponent(RESOURCE_ID)}`,
      {
        waitUntil: 'domcontentloaded',
      },
    );

    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await expect(page.getByText('PBS Services')).toBeVisible();
    await expect(page.getByText('Activity', { exact: true })).toBeVisible();

    const pbsRow = page.locator(`tr[data-summary-series-id="${RESOURCE_ID}"]`);
    await expect(pbsRow).toBeVisible();
    await expect(pbsRow.getByText('2 active', { exact: true })).toBeVisible();
    await expect(pbsRow.getByText('4 total', { exact: true })).toBeVisible();

    const serviceSection = page.getByTestId('resource-service-details-section');
    await expect(serviceSection).toContainText('2 datastores · 2 active tasks');
    await serviceSection.getByRole('button', { name: 'Show service' }).click();
    await expect(serviceSection.getByText('Active tasks', { exact: true })).toBeVisible();
    await serviceSection.getByText('2', { exact: true }).first().waitFor();

    await serviceSection.getByRole('button', { name: 'Show jobs' }).click();
    const activeTasks = page.getByTestId('pbs-active-tasks');
    await expect(activeTasks).toBeVisible();
    await expect(activeTasks.getByText('Backup backup-nightly')).toBeVisible();
    await expect(activeTasks.getByText('fast · VM 100')).toBeVisible();
    await expect(activeTasks.getByText('Running')).toBeVisible();
    await expect(activeTasks.getByText('Sync sync-remote')).toBeVisible();
    await expect(activeTasks.getByText('fast · Remote offsite')).toBeVisible();
    await expect(activeTasks.getByText('Queued')).toBeVisible();

    await page.screenshot({
      path: path.resolve(ARTIFACTS_DIR, 'pbs-active-tasks.png'),
      fullPage: true,
    });
  });
});
