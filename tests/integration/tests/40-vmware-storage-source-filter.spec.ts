import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_PATH = '/tmp/vmware-storage-source-filter.png';

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

// The retired shared /storage route (with its cross-platform source filter)
// was replaced by per-platform storage sections; vSphere datastores live on
// the Datastores tab of the vSphere platform page and render live websocket
// state from the mock scenario.
test.describe('VMware datastores platform section', () => {
  test.setTimeout(180_000);

  // Desktop presentation spec. The mobile layout renders platform tables
  // as cards, so the tr-row locators this spec asserts on do not exist
  // there; mobile coverage lives in 04-mobile.spec.ts.
  test.skip(({ isMobile }) => Boolean(isMobile), 'desktop-presentation spec');

  test('surfaces VMware datastores on the platform storage tab without backup semantics', async ({
    page,
  }) => {
    let unexpectedVmwareApiCall: string | null = null;

    await page.route('**/api/vmware/**', async (route) => {
      unexpectedVmwareApiCall = route.request().url();
      await route.abort();
    });

    await page.goto('/vmware/storage', { waitUntil: 'domcontentloaded' });

    await expect(page).toHaveURL(/\/vmware\/storage/);

    const storageTable = page.locator('table').first();
    // The mock scenario's nvme-primary datastore carries type, host, and
    // consumer context on the row. First websocket state frame can lag on a
    // freshly booted backend, so the first data assertion waits longer.
    const datastoreRow = page.locator('tr').filter({ hasText: 'nvme-primary' }).first();
    await expect(datastoreRow).toBeVisible({ timeout: 30_000 });
    await expect(page.getByRole('textbox', { name: 'Search vSphere datastores' })).toBeVisible();
    await expect(datastoreRow).toContainText('VMFS');
    await expect(datastoreRow).toContainText('esxi-01.lab.local');

    // Phase-1 VMware storage is read-only capacity context: no Proxmox-style
    // backup semantics may leak onto the datastore table.
    await expect(storageTable).not.toContainText('Backup Target');
    await expect(storageTable).not.toContainText('Protected');
    // TrueNAS entities stay on their own platform page.
    await expect(storageTable).not.toContainText('tank');

    expect(unexpectedVmwareApiCall).toBeNull();

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
