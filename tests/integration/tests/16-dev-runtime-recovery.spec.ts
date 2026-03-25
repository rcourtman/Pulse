import { expect, test } from '@playwright/test';

import { ensureAuthenticated } from './helpers';
import {
  killManagedDevRuntimeOwnerProcess,
  restartManagedDevRuntimeBackend,
} from '../scripts/managed-dev-runtime.mjs';

const truthy = (value: string | undefined) =>
  ['1', 'true', 'yes', 'on'].includes(String(value || '').trim().toLowerCase());

test.describe.serial('Managed dev runtime recovery', () => {
  test('browser shell distinguishes stream-only reconnect from total backend loss', async ({
    page,
  }, testInfo) => {
    test.setTimeout(120_000);
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only managed dev runtime coverage');
    test.skip(!truthy(process.env.PULSE_E2E_USE_HOT_DEV), 'Runs only against the managed hot-dev runtime');

    let blockNextSocket = false;
    await page.routeWebSocket('/ws', async (ws) => {
      if (blockNextSocket) {
        blockNextSocket = false;
        await ws.close({ code: 1013, reason: 'test-stream-interruption' });
        return;
      }

      ws.connectToServer();
    });

    await ensureAuthenticated(page);
    await page.goto('/infrastructure', { waitUntil: 'domcontentloaded' });

    const mainContent = page.locator('main, [role="main"], #root > div').first();
    await expect(mainContent).toBeVisible();
    await expect(
      page.getByRole('status', { name: 'Backend and live data stream are connected.' }),
    ).toBeVisible({ timeout: 20_000 });

    blockNextSocket = true;
    await page.evaluate(() => {
      window.__pulseWsStore?.reconnect();
    });

    await expect
      .poll(
        async () => {
          const labels = await page.locator('[role="status"]').evaluateAll((nodes) =>
            nodes.map((node) => node.getAttribute('aria-label') || ''),
          );
          return labels.find((label) =>
            label === 'Backend is healthy. Live updates are reconnecting.' ||
            label === 'Backend is healthy, but the live data stream is not connected.',
          ) || '';
        },
        {
          timeout: 30_000,
          message: 'expected stream-only reconnect state while backend health remained available',
        },
      )
      .not.toBe('');

    await expect(mainContent).toBeVisible();
    await expect(page.getByRole('status')).not.toHaveAttribute(
      'aria-label',
      'Backend and live data stream are unavailable.',
    );

    await page.evaluate(() => {
      window.__pulseWsStore?.reconnect();
    });

    await expect(
      page.getByRole('status', { name: 'Backend and live data stream are connected.' }),
    ).toBeVisible({ timeout: 30_000 });
    await expect
      .poll(async () => (await page.request.get('/api/health')).status(), {
        timeout: 15_000,
        message: 'expected backend health to stay available during stream-only reconnect coverage',
      })
      .toBe(200);
  });

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

    const restartResult = await restartManagedDevRuntimeBackend();

    expect(
      restartResult.afterPid,
      'expected managed backend bounce to replace the backend listener process',
    ).not.toBe(restartResult.beforePid);

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

  test('browser shell recovers after the managed hot-dev owner process is killed', async ({
    page,
  }, testInfo) => {
    test.setTimeout(180_000);
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only managed dev runtime coverage');
    test.skip(!truthy(process.env.PULSE_E2E_USE_HOT_DEV), 'Runs only against the managed hot-dev runtime');

    await ensureAuthenticated(page);
    await page.goto('/infrastructure', { waitUntil: 'domcontentloaded' });

    const mainContent = page.locator('main, [role="main"], #root > div').first();
    await expect(mainContent).toBeVisible();
    await expect(
      page.getByRole('status', { name: 'Backend and live data stream are connected.' }),
    ).toBeVisible({ timeout: 20_000 });

    const restartResult = await killManagedDevRuntimeOwnerProcess();

    expect(
      restartResult.afterOwnerPid,
      'expected managed owner-process recovery to replace the hot-dev owner process',
    ).not.toBe(restartResult.beforeOwnerPid);

    await expect(
      page.getByRole('status', { name: 'Backend and live data stream are connected.' }),
    ).toBeVisible({ timeout: 60_000 });
    await expect
      .poll(async () => (await page.request.get('/api/health')).status(), {
        timeout: 60_000,
        message: 'expected proxied backend health to recover after supervised owner-process death',
      })
      .toBe(200);
  });
});
