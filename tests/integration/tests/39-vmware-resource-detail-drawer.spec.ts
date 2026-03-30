import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_PATH = '/tmp/vmware-resource-detail-drawer.png';
const RESOURCE_ID = 'vc-1:vm:vm-201';
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
      `vmware-resource-detail-drawer-${workerInfo.project.name}.json`,
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

test.describe('VMware resource detail drawer', () => {
  test.setTimeout(180_000);

  test('surfaces VMware read-only context through the shared drawer path', async ({ page }) => {
    let unexpectedVmwareApiCall: string | null = null;

    await page.route('**/api/vmware/**', async (route) => {
      unexpectedVmwareApiCall = route.request().url();
      await route.abort();
    });

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
                type: 'vm',
                name: 'app-01.lab.local',
                displayName: 'App 01',
                platformId: RESOURCE_ID,
                platformType: 'vmware-vsphere',
                sourceType: 'api',
                sources: ['vmware-vsphere'],
                status: 'running',
                lastSeen: '2026-03-30T18:25:00Z',
                canonicalIdentity: {
                  displayName: 'App 01',
                  hostname: 'app-01.lab.local',
                  platformId: RESOURCE_ID,
                },
                vmware: {
                  connectionId: 'vc-1',
                  connectionName: 'Lab VC',
                  vcenterHost: 'vc.lab.local',
                  managedObjectId: 'vm-201',
                  entityType: 'VirtualMachine',
                  overallStatus: 'green',
                  powerState: 'poweredOn',
                  datacenterName: 'Lab DC',
                  clusterName: 'Compute Cluster',
                  resourcePoolName: 'Production',
                  runtimeHostName: 'esxi-01.lab.local',
                  datastoreNames: ['shared-vsan'],
                  guestOsFamily: 'ubuntu64Guest',
                  guestHostname: 'app-01.lab.local',
                  guestIpAddresses: ['192.0.2.50'],
                  activeAlarmCount: 1,
                  activeAlarmSummary: 'Host fan degraded',
                  recentTaskCount: 1,
                  recentTaskSummary: 'Create snapshot (success)',
                  snapshotCount: 2,
                },
                platformData: {
                  sources: ['vmware-vsphere'],
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
          resource_name: 'App 01',
          resource_type: 'vm',
          health: {
            score: 88,
            grade: 'B',
            trend: 'stable',
            factors: [],
            prediction: 'VMware placement and signal context is available on the shared drawer.',
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
      `/infrastructure?source=vmware-vsphere&resource=${encodeURIComponent(RESOURCE_ID)}`,
      {
        waitUntil: 'domcontentloaded',
      },
    );

    const vmwareSection = page.getByTestId('resource-vmware-details-section');

    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await expect(page.locator('#infra-source-filter')).toHaveValue('vmware-vsphere');
    await expect(
      page.locator('div[title="App 01"]').filter({ hasText: 'App 01' }).first(),
    ).toBeVisible();
    await expect(vmwareSection).toBeVisible();
    await expect(vmwareSection).toContainText(
      'Lab VC · Read-only vCenter context · 2 snapshots · 1 alarm · 1 task',
    );

    await vmwareSection.getByRole('button', { name: 'Show vSphere' }).click();

    await expect(vmwareSection.getByText('State', { exact: true })).toBeVisible();
    await expect(vmwareSection.getByText('Placement', { exact: true })).toBeVisible();
    await expect(vmwareSection.getByText('Guest', { exact: true })).toBeVisible();
    await expect(vmwareSection.getByText('Signals', { exact: true })).toBeVisible();
    await expect(vmwareSection.getByText('vc.lab.local')).toBeVisible();
    await expect(vmwareSection.getByText('Compute Cluster')).toBeVisible();
    await expect(vmwareSection.getByText('esxi-01.lab.local')).toBeVisible();
    await expect(vmwareSection.getByText('ubuntu64Guest')).toBeVisible();
    await expect(vmwareSection.getByText('Create snapshot (success)')).toBeVisible();
    await expect(vmwareSection.getByText('Host fan degraded')).toBeVisible();
    await expect(vmwareSection.getByText('2 snapshots', { exact: true })).toBeVisible();

    expect(unexpectedVmwareApiCall).toBeNull();

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
