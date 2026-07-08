import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

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

// The mock scenario deterministically forces pve5 (Disaster Recovery B)
// offline, which is exactly the state this guard exists for: an offline
// node must stay listed on its platform page instead of silently vanishing
// from the fleet view. Platform pages render websocket state, so the guard
// asserts against the mock dataset rather than REST stubs.
test.describe('Offline Proxmox node visibility', () => {
  test.setTimeout(180_000);

  test('keeps an offline Proxmox node visible on the Proxmox platform surface', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');

    await page.goto('/proxmox', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('proxmox-page')).toBeVisible();

    const offlineRow = page
      .locator('[data-testid="proxmox-page"] tr')
      .filter({ hasText: 'Disaster Recovery B' })
      .first();
    await offlineRow.scrollIntoViewIfNeeded();
    await expect(offlineRow).toBeVisible();

    // Offline presentation: live metrics are dashed out, but the node keeps
    // its identity and management affordances.
    await expect(offlineRow).toContainText('—');
    await expect(
      offlineRow.getByRole('link', { name: 'Open web interface for Disaster Recovery B' }),
    ).toBeVisible();
    await expect(
      offlineRow.getByRole('button', { name: 'Expand details for Disaster Recovery B' }),
    ).toBeVisible();
  });
});
