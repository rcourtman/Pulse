import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
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
      `telemetry-disclosure-${workerInfo.project.name}.json`,
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

test.describe('Telemetry disclosure', () => {
  test.setTimeout(180_000);

  test('general settings opens the shipped privacy document', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only telemetry disclosure coverage');

    await page.goto('/settings/system-general', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings/, { timeout: 15_000 });

    const disclosureLink = page.getByRole('link', { name: 'Full details' }).first();
    await expect(disclosureLink).toHaveAttribute('href', '/docs/PRIVACY.md');

    const [popup] = await Promise.all([
      page.waitForEvent('popup'),
      disclosureLink.click(),
    ]);

    await popup.waitForLoadState('domcontentloaded');
    expect(new URL(popup.url()).pathname).toBe('/docs/PRIVACY.md');
    await expect(popup.locator('body')).toContainText('Pulse includes anonymous telemetry');
    await expect(popup.locator('body')).toContainText('What is NOT sent');
  });
});
