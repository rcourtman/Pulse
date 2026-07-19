import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_PATH = '/tmp/vmware-phase1-exclusion-integrity.png';

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
      `vmware-phase1-exclusion-${workerInfo.project.name}.json`,
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

// Phase-1 VMware coverage is read-only vCenter context on the vSphere
// platform page. The guest drawer for an API-backed VM must not offer
// provider-local admin actions or recovery cross-links, and the browser
// must stay off vmware/recovery provider endpoints while rendering it.
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
      if (
        requestUrl.pathname !== '/api/resources' ||
        requestUrl.searchParams.get('type') !==
          'vm,system-container,app-container,pod'
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
              id: 'vmware:vc-mock-1:vm:vm-201',
              type: 'vm',
              name: 'warehouse-api-01',
              displayName: 'warehouse-api-01',
              status: 'online',
              lastSeen: '2026-07-19T12:00:00Z',
              sources: ['vmware'],
              platformScopes: ['vmware-vsphere'],
              platformType: 'vmware-vsphere',
              canonicalIdentity: {
                primaryId: 'vmware:vc-mock-1:vm:vm-201',
                displayName: 'warehouse-api-01',
                hostname: 'warehouse-api-01.internal',
                aliases: ['vm-201', 'warehouse-api-01'],
              },
              metrics: {
                cpu: { value: 18, percent: 18, unit: 'percent' },
                memory: {
                  used: 4_294_967_296,
                  total: 8_589_934_592,
                  percent: 50,
                  unit: 'bytes',
                },
                disk: {
                  used: 68_719_476_736,
                  total: 137_438_953_472,
                  percent: 50,
                  unit: 'bytes',
                },
              },
              vmware: {
                connectionId: 'vc-mock-1',
                connectionName: 'Lab vCenter',
                vcenterHost: 'vcsa.lab.local',
                entityType: 'vm',
                managedObjectId: 'vm-201',
                datacenterName: 'Primary DC',
                clusterName: 'Production Cluster',
                runtimeHostId: 'host-101',
                runtimeHostName: 'esxi-01.lab.local',
                powerState: 'POWERED_ON',
                guestHostname: 'warehouse-api-01.internal',
                guestIpAddresses: ['10.42.10.121'],
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

    await page.goto('/vmware', { waitUntil: 'domcontentloaded' });
    await expect(page.locator('[data-testid="vmware-page"]')).toBeVisible();

    // First websocket state frame can lag on a freshly booted backend.
    const expandButton = page.getByRole('button', { name: 'Expand warehouse-api-01' });
    await expect(expandButton).toBeVisible({ timeout: 30_000 });
    await expandButton.click();

    const drawer = page.getByRole('region', { name: 'warehouse-api-01' });
    await expect(drawer).toBeVisible();

    // The action surface points at agent onboarding instead of offering
    // native lifecycle controls for an API-backed VM.
    await expect(
      drawer.getByRole('link', { name: 'Add agent for AI actions' }),
    ).toBeVisible();

    await expect(page.getByRole('link', { name: /Open related recovery/i })).toHaveCount(0);
    await expect(page.getByRole('link', { name: /Open in Recovery/i })).toHaveCount(0);
    await expect(drawer.getByRole('button', { name: /^Restart$/ })).toHaveCount(0);
    await expect(drawer.getByRole('button', { name: /^Stop$/ })).toHaveCount(0);
    await expect(drawer.getByRole('button', { name: /^Shutdown$/ })).toHaveCount(0);

    expect(unexpectedVmwareApiCall).toBeNull();
    expect(unexpectedRecoveryApiCall).toBeNull();

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
