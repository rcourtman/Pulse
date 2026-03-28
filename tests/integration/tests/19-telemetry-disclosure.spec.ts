import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect, type Locator, type Page } from '@playwright/test';
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

async function expectPopupDoc(
  page: Page,
  link: Locator,
  pathname: string,
  expectedText: string,
) {
  const [popup] = await Promise.all([
    page.waitForEvent('popup'),
    link.click(),
  ]);

  await popup.waitForLoadState('domcontentloaded');
  expect(new URL(popup.url()).pathname).toBe(pathname);
  await expect(popup.locator('body')).toContainText(expectedText);
  await popup.close();
}

async function readTelemetryPreview(page: Page) {
  const preview = page.locator('pre[aria-label="Telemetry payload preview"]');
  await expect(preview).toBeVisible();
  return JSON.parse(await preview.textContent() ?? '{}') as {
    install_id: string;
    event: string;
  };
}

test.describe('Telemetry disclosure', () => {
  test.setTimeout(180_000);

  test('general settings opens the shipped privacy document', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only telemetry disclosure coverage');

    await page.goto('/settings/system-general', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings/, { timeout: 15_000 });

    const telemetrySummary = page.getByText(
      /rotating install ID, version, platform, resource counts, and feature flags/i,
    );
    await expect(telemetrySummary).toBeVisible();
    await expect(telemetrySummary).toContainText('Telemetry rows are retained for up to 90 days');
    await expect(telemetrySummary).toContainText(
      'IP addresses are not stored in telemetry rows',
    );

    const disclosureLink = page.getByRole('link', { name: 'Full details' }).first();
    await expect(disclosureLink).toHaveAttribute('href', '/docs/PRIVACY.md');
    await expectPopupDoc(
      page,
      disclosureLink,
      '/docs/PRIVACY.md',
      'Pulse includes anonymous telemetry',
    );
  });

  test('general settings lets operators preview and rotate the telemetry payload', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only telemetry disclosure coverage');

    await page.goto('/settings/system-general', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings/, { timeout: 15_000 });

    await page.getByRole('button', { name: 'Preview payload' }).click();

    const initialPreview = await readTelemetryPreview(page);
    expect(initialPreview.event).toBe('heartbeat');
    expect(initialPreview.install_id).toBeTruthy();

    page.once('dialog', (dialog) => dialog.accept());
    await page.getByRole('button', { name: 'Reset ID' }).click();

    await expect
      .poll(async () => {
        const refreshedPreview = await readTelemetryPreview(page);
        return refreshedPreview.install_id;
      })
      .not.toBe(initialPreview.install_id);
  });

  test('whats-new modal opens shipped privacy and documentation pages', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only telemetry disclosure coverage');

    await page.addInitScript(() => {
      localStorage.removeItem('pulse_whats_new_v2_shown');
    });

    await page.goto('/dashboard', { waitUntil: 'domcontentloaded' });

    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText('Welcome to the New Navigation!')).toBeVisible();
    await expect(
      dialog.getByText(/rotating install ID, version, platform, resource counts, and feature flags/i),
    ).toBeVisible();
    await expect(
      dialog.getByText(/IP addresses are not stored in telemetry rows/i),
    ).toBeVisible();

    const privacyLink = dialog.getByRole('link', { name: 'Full details' });
    await expect(privacyLink).toHaveAttribute('href', '/docs/PRIVACY.md');
    await expectPopupDoc(
      page,
      privacyLink,
      '/docs/PRIVACY.md',
      'Pulse includes anonymous telemetry',
    );

    const docsLink = dialog.getByRole('link', { name: 'Documentation' });
    await expect(docsLink).toHaveAttribute('href', '/docs/README.md');
    await expectPopupDoc(
      page,
      docsLink,
      '/docs/README.md',
      'Welcome to the Pulse documentation portal.',
    );
  });
});
