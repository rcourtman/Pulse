import { expect, test } from '@playwright/test';

import { ensureAuthenticated } from './helpers';
import { restartManagedDevRuntimeBackend } from '../scripts/managed-dev-runtime.mjs';

const truthy = (value: string | undefined) =>
  ['1', 'true', 'yes', 'on'].includes(String(value || '').trim().toLowerCase());

test.describe.serial('Managed dev runtime recovery', () => {
  test('browser shell stays usable and recovers after a managed backend bounce', async ({
    page,
  }, testInfo) => {
    test.setTimeout(120_000);
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only managed dev runtime coverage');
    test.skip(!truthy(process.env.PULSE_E2E_USE_HOT_DEV), 'Runs only against the managed hot-dev runtime');

    await ensureAuthenticated(page);
    await page.goto('/infrastructure', { waitUntil: 'domcontentloaded' });

    const mainContent = page.locator('main, [role="main"], #root > div').first();
    await expect(mainContent).toBeVisible();
    await expect(
      page.getByRole('status', { name: 'Backend and live data stream are connected.' }),
    ).toBeVisible({ timeout: 20_000 });

    const restartPromise = restartManagedDevRuntimeBackend();

    await expect
      .poll(
        async () => {
          const labels = await page.locator('[role="status"]').evaluateAll((nodes) =>
            nodes.map((node) => node.getAttribute('aria-label') || ''),
          );
          return labels.find((label) =>
            label === 'Backend and live data stream are unavailable.' ||
            label === 'Attempting to reconnect to the backend and live data stream.',
          ) || '';
        },
        {
          timeout: 30_000,
          message: 'expected the browser shell to report a real backend outage during restart',
        },
      )
      .not.toBe('');

    await expect(mainContent).toBeVisible();

    await restartPromise;

    await expect(
      page.getByRole('status', { name: 'Backend and live data stream are connected.' }),
    ).toBeVisible({ timeout: 30_000 });
    await expect
      .poll(async () => (await page.request.get('/api/health')).status(), {
        timeout: 30_000,
        message: 'expected proxied backend health to recover after managed backend restart',
      })
      .toBe(200);
  });
});
