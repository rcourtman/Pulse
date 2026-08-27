import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { expect, test as base } from '@playwright/test';

import { restartManagedLocalBackend } from '../scripts/managed-local-backend.mjs';
import { readRuntimeState } from '../scripts/runtime-state.mjs';
import { apiRequest, createAuthenticatedStorageState, ensureAuthenticated } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => use(authStorageStatePath),
  authStorageStatePath: [
    async ({ browser }, use, workerInfo) => {
      const storageStatePath = path.resolve(
        __dirname,
        '..',
        '..',
        'tmp',
        'playwright-auth',
        `alert-history-real-backend-${workerInfo.project.name}.json`,
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

const qualificationEnabled = ['1', 'true', 'yes', 'on'].includes(
  String(process.env.PULSE_E2E_ALERT_HISTORY_QUALIFICATION || '')
    .trim()
    .toLowerCase(),
);

const LEGACY_ALERT_ID = 'legacy-history-qualification';
const ACTIVE_ALERT_ID = 'active-overlay-qualification';

type HistoryAlert = {
  id: string;
  resourceName: string;
  node?: string;
  nodeDisplayName?: string;
  message: string;
  type: string;
  startTime: string;
  lastSeen: string;
};

async function readHistory(page: import('@playwright/test').Page): Promise<HistoryAlert[]> {
  const response = await apiRequest(page, '/api/alerts/history?limit=0');
  expect(response.ok(), `history API returned ${response.status()}`).toBeTruthy();
  return (await response.json()) as HistoryAlert[];
}

test.describe('Real backend alert history qualification', () => {
  test.skip(!qualificationEnabled, 'Run through npm run test:alerts:qualification');
  test.setTimeout(240_000);

  test('imports once, renders exact fields, survives restart, and preserves clear tombstones', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name !== 'chromium', 'The isolated qualification runner owns Chromium');

    const runtimeState = await readRuntimeState();
    expect(runtimeState?.managedLocalBackend).toBe(true);
    expect(runtimeState?.dataDir).toBeTruthy();
    expect(runtimeState?.logPath).toBeTruthy();
    const alertsDir = path.join(runtimeState!.dataDir, 'alerts');
    const importedPath = path.join(alertsDir, 'alert-history.json.imported');
    const sourcePath = path.join(alertsDir, 'alert-history.json');

    expect(fs.existsSync(importedPath), 'legacy history source must retire after import').toBe(true);
    expect(fs.existsSync(sourcePath), 'retired legacy history source must leave the load path').toBe(false);
    expect(fs.readFileSync(runtimeState!.logPath, 'utf8')).toContain(
      'legacy alert history imported into the event log; JSON history files retired',
    );

    await page.goto('/alerts/history', { waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('heading', { name: 'Alert History' })).toBeVisible();
    const legacyRow = page.locator('tr').filter({ hasText: 'Legacy Qualification VM' }).first();
    await expect(legacyRow).toContainText('Legacy history import rendered from SQLite');
    await expect(legacyRow).toContainText('CPU');
    await expect(legacyRow).toContainText('Qualification Node');
    await expect(legacyRow).toContainText('30m');
    await expect(
      page.locator('table.alert-history-responsive-table').getByText('Active Overlay Node').first(),
    ).toBeVisible();

    let history = await readHistory(page);
    expect(history.filter((alert) => alert.id === LEGACY_ALERT_ID)).toHaveLength(1);
    expect(history.filter((alert) => alert.id === ACTIVE_ALERT_ID)).toHaveLength(1);
    const importedAlert = history.find((alert) => alert.id === LEGACY_ALERT_ID);
    expect(importedAlert).toMatchObject({
      resourceName: 'Legacy Qualification VM',
      node: 'qualification-node',
      nodeDisplayName: 'Qualification Node',
      message: 'Legacy history import rendered from SQLite',
      type: 'cpu',
    });
    expect(Date.parse(importedAlert!.lastSeen) - Date.parse(importedAlert!.startTime)).toBe(30 * 60_000);

    await restartManagedLocalBackend();
    await ensureAuthenticated(page);
    await page.goto('/alerts/history', { waitUntil: 'domcontentloaded' });
    await expect(
      page.locator('table.alert-history-responsive-table').getByText('Legacy Qualification VM'),
    ).toBeVisible();
    history = await readHistory(page);
    expect(
      history.filter((alert) => alert.id === LEGACY_ALERT_ID),
      'restart must not duplicate the imported occurrence',
    ).toHaveLength(1);

    page.once('dialog', (dialog) => dialog.accept());
    await page.getByRole('button', { name: 'Clear All History' }).click();
    await expect(page.getByText('Legacy Qualification VM')).toHaveCount(0);
    await expect(
      page.locator('table.alert-history-responsive-table').getByText('Active Overlay Node').first(),
    ).toBeVisible();
    history = await readHistory(page);
    expect(history.map((alert) => alert.id)).not.toContain(LEGACY_ALERT_ID);
    expect(history.map((alert) => alert.id)).toContain(ACTIVE_ALERT_ID);

    await restartManagedLocalBackend();
    await ensureAuthenticated(page);
    await page.goto('/alerts/history', { waitUntil: 'domcontentloaded' });
    await expect(page.getByText('Legacy Qualification VM')).toHaveCount(0);
    await expect(
      page.locator('table.alert-history-responsive-table').getByText('Active Overlay Node').first(),
    ).toBeVisible();
    history = await readHistory(page);
    expect(history.map((alert) => alert.id)).not.toContain(LEGACY_ALERT_ID);
    expect(history.filter((alert) => alert.id === ACTIVE_ALERT_ID)).toHaveLength(1);

    await testInfo.attach('real-backend-alert-history-final.json', {
      body: Buffer.from(JSON.stringify(history, null, 2)),
      contentType: 'application/json',
    });
  });
});
