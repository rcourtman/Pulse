import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = '/tmp/vmware-storage-source-filter.png';

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
      `vmware-storage-source-filter-${workerInfo.project.name}.json`,
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

test.describe('VMware storage source filter', () => {
  test.setTimeout(180_000);

  test('surfaces VMware datastores on the shared storage route without backup semantics', async ({
    page,
  }) => {
    let unexpectedVmwareApiCall: string | null = null;

    await page.route('**/api/vmware/**', async (route) => {
      unexpectedVmwareApiCall = route.request().url();
      await route.abort();
    });

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
              id: 'storage-vmware-1',
              type: 'storage',
              name: 'shared-vmfs',
              displayName: 'shared-vmfs',
              platformId: 'datastore-11',
              platformType: 'vmware-vsphere',
              sourceType: 'api',
              sources: ['vmware-vsphere'],
              status: 'online',
              lastSeen: '2026-03-30T20:00:00Z',
              canonicalIdentity: {
                displayName: 'shared-vmfs',
                platformId: 'datastore-11',
                primaryId: 'vmware:vc-1:datastore:datastore-11',
              },
              disk: {
                total: 4_000 * 1024 * 1024 * 1024,
                used: 1_500 * 1024 * 1024 * 1024,
                free: 2_500 * 1024 * 1024 * 1024,
                current: 37.5,
              },
              storage: {
                platform: 'vmware-vsphere',
                type: 'vmfs',
                topology: 'datastore',
                shared: true,
                nodes: ['esxi-01.lab.local', 'esxi-02.lab.local'],
              },
              vmware: {
                connectionId: 'vc-1',
                connectionName: 'Lab VC',
                vcenterHost: 'vc.lab.local',
                managedObjectId: 'datastore-11',
                entityType: 'datastore',
                datastoreType: 'VMFS',
                datastoreAccessible: true,
                multipleHostAccess: true,
                datastoreUrl: '/vmfs/volumes/shared-vmfs',
              },
              platformData: {
                sources: ['vmware-vsphere'],
              },
            },
            {
              id: 'storage-truenas-1',
              type: 'storage',
              name: 'tank',
              displayName: 'tank',
              parentId: 'truenas-main',
              parentName: 'truenas-main',
              platformId: 'truenas-1',
              platformType: 'truenas',
              sourceType: 'api',
              sources: ['truenas'],
              status: 'online',
              lastSeen: '2026-03-30T20:00:00Z',
              canonicalIdentity: {
                displayName: 'tank',
                platformId: 'truenas-1',
              },
              disk: {
                total: 2_000 * 1024 * 1024 * 1024,
                used: 1_000 * 1024 * 1024 * 1024,
                free: 1_000 * 1024 * 1024 * 1024,
                current: 50,
              },
              storage: {
                platform: 'truenas',
                type: 'zfs-pool',
                topology: 'pool',
                isZfs: true,
              },
              platformData: {
                sources: ['truenas'],
              },
            },
          ],
          meta: {
            page: 1,
            limit: 100,
            total: 2,
            totalPages: 1,
          },
        }),
      });
    });

    await page.goto('/storage?source=vmware-vsphere', {
      waitUntil: 'domcontentloaded',
    });

    await expect(page).toHaveURL(/\/storage\?source=vmware-vsphere/);
    await expect(page.locator('#storage-source-filter')).toHaveValue('vmware-vsphere');

    const sourceOptions = await page.locator('#storage-source-filter option').evaluateAll((options) =>
      options.map((option) => ({
        value: option.getAttribute('value'),
        label: option.textContent?.trim(),
      })),
    );
    expect(sourceOptions).toEqual([
      { value: 'all', label: 'All Sources' },
      { value: 'truenas', label: 'TrueNAS' },
      { value: 'vmware-vsphere', label: 'vSphere' },
    ]);

    const storageTable = page.locator('table').first();
    await expect(storageTable).toContainText('shared-vmfs');
    await expect(storageTable).toContainText('vSphere');
    await expect(storageTable).toContainText('Datastore');
    await expect(storageTable).toContainText('esxi-01.lab.local');
    await expect(storageTable).not.toContainText('tank');
    await expect(storageTable).not.toContainText('Backup Target');
    await expect(storageTable).not.toContainText('Protected');

    expect(unexpectedVmwareApiCall).toBeNull();

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
