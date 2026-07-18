import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = '/tmp/truenas-alert-thresholds.png';

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
      `truenas-alert-thresholds-${workerInfo.project.name}.json`,
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

// The thresholds page renders live websocket state, so REST stubs of
// /api/resources get overwritten by the next state frame. The assertions
// pin the mock scenario's TrueNAS fixture (truenas-main with pool tank,
// tank/* datasets, and physical disks) instead of stubbed payloads.
test.describe('TrueNAS alert thresholds', () => {
  test.setTimeout(180_000);

  // Desktop presentation spec. The mobile layout collapses the platform
  // scope filter into the FilterBar sheet, so the 'TrueNAS' scope button
  // this spec clicks does not exist there and the click falls through to
  // the platform nav tab; mobile coverage lives in 04-mobile.spec.ts.
  test.skip(({ isMobile }) => Boolean(isMobile), 'desktop-presentation spec');

  test('surfaces TrueNAS systems, pools, datasets, and disks under the TrueNAS thresholds scope', async ({
    page,
  }) => {
    // Fresh test backends boot with alerts deactivated, which gates the
    // thresholds route behind the Alerts Overview activation screen. The
    // config endpoint is REST-fed and stubable (same pattern as the Ceph
    // thresholds spec); entity data below still comes from live mock state.
    await page.route('**/api/alerts/config', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          enabled: true,
          activationState: 'active',
          overrides: {},
        }),
      });
    });

    await page.goto('/alerts/thresholds/infrastructure', {
      waitUntil: 'domcontentloaded',
    });

    await expect(page).toHaveURL(/\/alerts\/thresholds/);
    await expect(page.getByRole('heading', { name: 'Alert Thresholds' })).toBeVisible();

    // Threshold scopes are platform-first; TrueNAS entities live under
    // their own scope instead of the retired neutral groupings.
    await page.getByRole('button', { name: 'TrueNAS', exact: true }).click();
    await expect(page).toHaveURL(/\/alerts\/thresholds\/truenas/);
    await expect(page.getByRole('heading', { name: 'Systems' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Pools' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Datasets' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Disks' })).toBeVisible();
    await expect(page.getByText('truenas-main', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('tank', { exact: true }).first()).toBeVisible();

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
