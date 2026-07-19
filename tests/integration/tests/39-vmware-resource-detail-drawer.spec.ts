import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';
import { installVmwareWorkloadResourceRoute } from './vmware-workload-fixture';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_PATH = '/tmp/vmware-resource-detail-drawer.png';

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

// The retired /infrastructure resource deep-link was replaced by the vSphere
// platform page: expanding a VM row opens the shared guest drawer, which
// carries the read-only vCenter placement context from the canonical resource
// contract.
test.describe('VMware resource detail drawer', () => {
  test.setTimeout(180_000);

  test('surfaces VMware read-only context through the shared guest drawer', async ({ page }) => {
    let unexpectedVmwareApiCall: string | null = null;

    await page.route('**/api/vmware/**', async (route) => {
      unexpectedVmwareApiCall = route.request().url();
      await route.abort();
    });
    await installVmwareWorkloadResourceRoute(page);

    await page.goto('/vmware', { waitUntil: 'domcontentloaded' });
    await expect(page.locator('[data-testid="vmware-page"]')).toBeVisible();

    const expandButton = page.getByRole('button', { name: 'Expand warehouse-api-01' });
    await expect(expandButton).toBeVisible({ timeout: 30_000 });
    await expandButton.click();

    const drawer = page.getByRole('region', { name: 'warehouse-api-01' });
    await expect(drawer).toBeVisible();
    await expect(
      drawer.getByRole('heading', { level: 2, name: 'warehouse-api-01' }),
    ).toBeVisible();

    // Placement context comes from the vCenter inventory, rendered read-only.
    await expect(drawer.getByRole('heading', { name: 'vSphere' })).toBeVisible();
    await expect(drawer.getByText('Lab vCenter', { exact: true })).toBeVisible();
    await expect(drawer.getByText('Primary DC', { exact: true })).toBeVisible();
    await expect(drawer.getByText('Production Cluster', { exact: true })).toBeVisible();

    // Host placement and guest identity surface alongside.
    await expect(drawer.getByText('esxi-01.lab.local')).toBeVisible();
    await expect(drawer.getByRole('heading', { name: 'Guest Info' })).toBeVisible();

    // Phase-1 VMware stays on shared read paths: the browser never talks to
    // provider-local vmware endpoints for drawer context.
    expect(unexpectedVmwareApiCall).toBeNull();

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
