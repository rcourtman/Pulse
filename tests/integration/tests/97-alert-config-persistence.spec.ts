import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { expect, test as base } from '@playwright/test';

import { restartManagedLocalBackend } from '../scripts/managed-local-backend.mjs';
import { readRuntimeState } from '../scripts/runtime-state.mjs';
import {
  apiRequest,
  createAuthenticatedStorageState,
  ensureAuthenticated,
} from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) =>
    use(authStorageStatePath),
  authStorageStatePath: [
    async ({ browser }, use, workerInfo) => {
      const storageStatePath = path.resolve(
        __dirname,
        '..',
        '..',
        'tmp',
        'playwright-auth',
        `alert-config-persistence-${workerInfo.project.name}.json`,
      );
      fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
      await createAuthenticatedStorageState(browser, storageStatePath);
      try {
        await use(storageStatePath);
      } finally {
        fs.rmSync(storageStatePath, { force: true });
      }
    },
    { scope: 'worker' },
  ],
});

// No HTTP or WebSocket interception: this owns an isolated managed backend.
test('saves recovery intent through the UI and preserves it across reload and restart', async ({ page }, testInfo) => {
  test.skip(process.env.PULSE_E2E_ALERT_CONFIG_PERSISTENCE !== '1', 'Requires an owned managed backend');
  test.skip(testInfo.project.name !== 'chromium', 'One worker owns backend restart');
  test.setTimeout(240_000);
  const runtime = await readRuntimeState();
  expect(runtime?.managedLocalBackend).toBe(true);
  expect(process.env.PULSE_MOCK_MODE).toBe('false');
  const readConfig = async () => {
    const response = await apiRequest(page, '/api/alerts/config');
    expect(response.ok(), `config GET: ${response.status()}`).toBe(true);
    return response.json();
  };
  await page.goto('/alerts/overview', { waitUntil: 'domcontentloaded' });
  const original = await readConfig();
  // Activate only the disposable instance; no destinations or resources are seeded.
  const setup = await apiRequest(page, '/api/alerts/config', {
    method: 'PUT', data: { ...original, enabled: true, activationState: 'active',
      schedule: { ...original.schedule, notifyOnResolve: true } },
  });
  expect(setup.ok(), `config setup: ${setup.status()}`).toBe(true);
  await page.goto('/alerts/schedule', { waitUntil: 'domcontentloaded' });
  const recovery = page.getByRole('button', { name: /^Recovery notifications/ });
  // Persist false, not the default true, so a reset cannot satisfy restart proof.
  const initial = true;
  await expect(recovery).toHaveAttribute('aria-pressed', String(initial));
  await recovery.click();
  await expect(recovery).toHaveAttribute('aria-pressed', String(!initial));
  // A staged edit must not already have altered the server.
  expect((await readConfig()).schedule.notifyOnResolve).toBe(initial);
  // Reload must discard the unsaved edit rather than restore browser-only state.
  await page.reload({ waitUntil: 'domcontentloaded' });
  await expect(recovery).toHaveAttribute('aria-pressed', String(initial));
  expect((await readConfig()).schedule.notifyOnResolve).toBe(initial);
  await recovery.click();
  await expect(recovery).toHaveAttribute('aria-pressed', String(!initial));
  const saved = page.waitForResponse(response =>
    new URL(response.url()).pathname === '/api/alerts/config' && response.request().method() === 'PUT');
  await page.getByRole('button', { name: 'Save Changes', exact: true }).click();
  expect((await saved).ok()).toBe(true);
  const expected = { enabled: true, activationState: 'active', schedule: { notifyOnResolve: !initial } };
  expect(await readConfig()).toMatchObject(expected);
  await page.reload({ waitUntil: 'domcontentloaded' });
  await expect(recovery).toHaveAttribute('aria-pressed', String(!initial));
  expect(await readConfig()).toMatchObject(expected);
  await restartManagedLocalBackend();
  await ensureAuthenticated(page);
  await page.goto('/alerts/schedule', { waitUntil: 'domcontentloaded' });
  await expect(recovery).toHaveAttribute('aria-pressed', String(!initial));
  expect(await readConfig()).toMatchObject(expected);
  await testInfo.attach('saved-intent-proof.json', {
    body: Buffer.from(JSON.stringify({ expected, unsavedEditDiscardedOnReload: true,
      reload: true, backendRestart: true,
      mockedAlertEndpoints: false, destinationReceiptProven: false })),
    contentType: 'application/json',
  });
});
