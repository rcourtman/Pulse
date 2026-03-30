import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_PATH = '/tmp/vmware-phase1-exclusion-integrity.png';
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
      `vmware-phase1-exclusion-integrity-${workerInfo.project.name}.json`,
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

test.describe('VMware phase-1 exclusion integrity', () => {
  test.setTimeout(180_000);

  test('keeps VMware resources out of recovery and provider-local admin routes', async ({
    page,
  }) => {
    let unexpectedVmwareApiCall: string | null = null;
    let unexpectedRecoveryApiCall: string | null = null;

    await page.route('**/api/vmware/**', async (route) => {
      unexpectedVmwareApiCall = route.request().url();
      await route.abort();
    });

    await page.route('**/api/recovery/**', async (route) => {
      unexpectedRecoveryApiCall = route.request().url();
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
                lastSeen: '2026-03-30T21:10:00Z',
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
                  runtimeHostName: 'esxi-01.lab.local',
                  guestOsFamily: 'ubuntu64Guest',
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
            score: 90,
            grade: 'A',
            trend: 'stable',
            factors: [],
            prediction: 'Phase-1 VMware context remains read-only.',
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

    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await expect(page.getByTestId('resource-vmware-details-section')).toContainText(
      'Read-only vCenter context',
    );

    await page.getByRole('button', { name: 'Show access' }).click();

    await expect(page.getByRole('link', { name: /Open related recovery/i })).toHaveCount(0);
    await expect(page.getByRole('link', { name: /Open in Recovery/i })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /^Restart$/ })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /^Stop$/ })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /^Shutdown$/ })).toHaveCount(0);

    expect(unexpectedVmwareApiCall).toBeNull();
    expect(unexpectedRecoveryApiCall).toBeNull();

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
