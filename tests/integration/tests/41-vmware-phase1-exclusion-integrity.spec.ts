import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';
import { installVmwareWorkloadResourceRoute } from './vmware-workload-fixture';

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
  }, testInfo) => {
    let unexpectedVmwareApiCall: string | null = null;
    let unexpectedRecoveryApiCall: string | null = null;

    await page.route('**/api/vmware/**', async (route) => {
      const url = new URL(route.request().url());
      if (
        route.request().method() === 'GET' &&
        url.pathname === '/api/vmware/connections'
      ) {
        await route.continue();
        return;
      }
      unexpectedVmwareApiCall = url.pathname;
      await route.abort();
    });

    await page.route('**/api/recovery/**', async (route) => {
      unexpectedRecoveryApiCall = route.request().url();
      await route.abort();
    });

    await installVmwareWorkloadResourceRoute(page);

    await page.goto('/vmware', { waitUntil: 'domcontentloaded' });
    await expect(page.locator('[data-testid="vmware-page"]')).toBeVisible();

    const expandButton = page.getByRole('button', { name: 'Expand warehouse-api-01' });
    await expect(expandButton).toBeVisible({ timeout: 30_000 });
    if (testInfo.project.name.startsWith('mobile-')) {
      // Compact rows expose a full name-cell touch target; their disclosure
      // button is screen-reader-only until focused from the keyboard.
      await page.locator('tr').filter({ has: expandButton }).locator('td').first().tap();
    } else {
      await expandButton.click();
    }

    const drawer = page.getByRole('region', { name: 'warehouse-api-01' });
    await expect(drawer).toBeVisible();

    // Coverage is evidence in Details; API-backed guests do not expose
    // the retired action-oriented onboarding link or native lifecycle controls.
    const details = drawer.getByTestId('guest-technical-details');
    await details.locator('summary').click();
    await expect(details).toHaveAttribute('open', '');
    await expect(details.getByText('Pulse coverage', { exact: true })).toBeVisible();
    await expect(details.getByText('Agent recommended', { exact: true })).toBeVisible();
    await expect(drawer.getByRole('link', { name: 'Add agent for AI actions' })).toHaveCount(0);

    await expect(page.getByRole('link', { name: /Open related recovery/i })).toHaveCount(0);
    await expect(page.getByRole('link', { name: /Open in Recovery/i })).toHaveCount(0);
    await expect(drawer.getByRole('button', { name: /^Restart$/ })).toHaveCount(0);
    await expect(drawer.getByRole('button', { name: /^Stop$/ })).toHaveCount(0);
    await expect(drawer.getByRole('button', { name: /^Shutdown$/ })).toHaveCount(0);

    expect(unexpectedVmwareApiCall).toBeNull();
    expect(unexpectedRecoveryApiCall).toBeNull();

    await page.screenshot({ path: testInfo.outputPath('vmware-exclusion-integrity.png'), fullPage: true });
  });
});
