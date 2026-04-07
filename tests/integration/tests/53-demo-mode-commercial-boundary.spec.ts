import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SECURITY_STATUS_DEMO_PAYLOAD = {
  hasAuthentication: true,
  hideLocalLogin: false,
  ssoProviders: [],
  sessionCapabilities: {
    demoMode: true,
  },
};

const DEMO_PUBLIC_RUNTIME_CAPABILITIES = {
  capabilities: ['ai_patrol', 'relay'],
  limits: [
    {
      key: 'max_monitored_systems',
      limit: 0,
      current: 0,
      state: 'ok',
    },
  ],
  hosted_mode: false,
  max_history_days: 90,
};

const authenticatedTest = base.extend<{}, WorkerFixtures>({
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
      `demo-mode-commercial-boundary-${workerInfo.project.name}.json`,
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

base.describe('Demo mode commercial boundary', () => {
  base.setTimeout(180_000);

  base('shows demo credentials on the login page from session capabilities', async ({
    page,
  }, testInfo) => {
    await page.route('**/api/state', async (route) => {
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'authentication required' }),
      });
    });

    await page.route('**/api/security/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(SECURITY_STATUS_DEMO_PAYLOAD),
      });
    });

    await page.goto('/', { waitUntil: 'domcontentloaded' });

    await expect(page.locator('input[name="username"]')).toBeVisible();
    await expect(page.getByText('Demo Mode', { exact: true })).toBeVisible();
    await expect(page.getByText('Login with', { exact: false })).toBeVisible();
    await expect(page.getByText('demo', { exact: true }).first()).toBeVisible();

    const screenshotPath = path.resolve(
      __dirname,
      '..',
      '..',
      'tmp',
      `demo-mode-login-${testInfo.project.name}.png`,
    );
    fs.mkdirSync(path.dirname(screenshotPath), { recursive: true });
    await page.screenshot({ path: screenshotPath, fullPage: true });
  });

  authenticatedTest(
    'hides billing surfaces and commercial prompts in the authenticated demo shell',
    async ({
    page,
  }, testInfo) => {
    let runtimeCapabilitiesRequests = 0;
    let entitlementsRequests = 0;
    let licenseStatusRequests = 0;
    let monitoredSystemLedgerRequests = 0;
    let billingStateRequests = 0;

    await page.addInitScript(() => {
      localStorage.setItem('pulse_whats_new_v2_shown', 'true');
    });

    await page.route('**/api/security/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(SECURITY_STATUS_DEMO_PAYLOAD),
      });
    });

    await page.route('**/api/license/runtime-capabilities', async (route) => {
      runtimeCapabilitiesRequests += 1;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(DEMO_PUBLIC_RUNTIME_CAPABILITIES),
      });
    });

    await page.route('**/api/license/entitlements', async (route) => {
      entitlementsRequests += 1;
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'not found' }),
      });
    });

    await page.route('**/api/license/status', async (route) => {
      licenseStatusRequests += 1;
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'not found' }),
      });
    });

    await page.route('**/api/license/monitored-system-ledger', async (route) => {
      monitoredSystemLedgerRequests += 1;
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'not found' }),
      });
    });

    await page.route('**/api/admin/orgs/**/billing-state', async (route) => {
      billingStateRequests += 1;
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'not found' }),
      });
    });

    await page.goto('/settings/system/billing', { waitUntil: 'domcontentloaded' });
    await page.waitForURL('**/settings/infrastructure/install', { timeout: 15_000 });

    await expect
      .poll(() => runtimeCapabilitiesRequests, {
        message: 'expected demo shell to load runtime capabilities',
      })
      .toBeGreaterThan(0);
    expect(entitlementsRequests, 'demo settings route should not read commercial entitlements').toBe(0);

    await expect(
      page.getByText('Demo instance with mock data (read-only)', { exact: true }),
    ).toBeVisible();
    await expect(page.getByRole('heading', { level: 1, name: 'Infrastructure Operations' })).toBeVisible();
    const settingsNavigation = page.locator('[aria-label="Settings navigation"]');
    await expect(settingsNavigation).toBeVisible();
    await expect(
      settingsNavigation.getByText('Pulse Pro', { exact: true }),
    ).toHaveCount(0);
    await expect(page.getByText('Pro Trial:', { exact: false })).toHaveCount(0);
    await expect(page.getByText('Monitored systems: 16/5', { exact: true })).toHaveCount(0);
    await expect(
      page
        .locator('[role="tab"]')
        .filter({ hasText: 'Settings' })
        .getByText('Pro', { exact: true }),
    ).toHaveCount(0);

    expect(licenseStatusRequests, 'demo settings route should not read license status').toBe(0);
    expect(
      monitoredSystemLedgerRequests,
      'demo settings route should not read the monitored-system ledger',
    ).toBe(0);
    expect(billingStateRequests, 'demo settings route should not read hosted billing state').toBe(0);

    const screenshotPath = path.resolve(
      __dirname,
      '..',
      '..',
      'tmp',
      `demo-mode-settings-${testInfo.project.name}.png`,
    );
    fs.mkdirSync(path.dirname(screenshotPath), { recursive: true });
    await page.screenshot({ path: screenshotPath, fullPage: true });
    },
  );

  authenticatedTest(
    'exposes only the runtime capability contract in the browser',
    async ({ page }) => {
      await page.route('**/api/security/status', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(SECURITY_STATUS_DEMO_PAYLOAD),
        });
      });

      await page.route('**/api/license/runtime-capabilities', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(DEMO_PUBLIC_RUNTIME_CAPABILITIES),
        });
      });

      const hiddenNotFound = async (route: string) => {
        await page.route(route, async (innerRoute) => {
          await innerRoute.fulfill({
            status: 404,
            contentType: 'application/json',
            body: JSON.stringify({ error: 'not found' }),
          });
        });
      };

      await hiddenNotFound('**/api/license/status');
      await hiddenNotFound('**/api/license/features');
      await hiddenNotFound('**/api/license/entitlements');
      await hiddenNotFound('**/api/license/activate');
      await hiddenNotFound('**/api/license/clear');
      await hiddenNotFound('**/api/license/trial/start');
      await hiddenNotFound('**/api/license/monitored-system-ledger');
      await hiddenNotFound('**/api/admin/orgs/**/billing-state');
      await hiddenNotFound('**/api/upgrade-metrics/**');
      await hiddenNotFound('**/auth/trial-activate');

      await page.goto('/settings/infrastructure/install', { waitUntil: 'domcontentloaded' });

      const responses = await page.evaluate(async () => {
        const probe = async (input: string, init?: RequestInit) => {
          const res = await fetch(input, {
            credentials: 'include',
            ...init,
          });
          let body: unknown = null;
          try {
            body = await res.json();
          } catch {
            body = null;
          }
          return { status: res.status, body };
        };

        return {
          runtimeCapabilities: await probe('/api/license/runtime-capabilities'),
          status: await probe('/api/license/status'),
          features: await probe('/api/license/features'),
          entitlements: await probe('/api/license/entitlements'),
          activate: await probe('/api/license/activate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ license_key: 'demo' }),
          }),
          clear: await probe('/api/license/clear', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({}),
          }),
          trialStart: await probe('/api/license/trial/start', { method: 'POST' }),
          ledger: await probe('/api/license/monitored-system-ledger'),
          billingState: await probe('/api/admin/orgs/default/billing-state'),
          upgradeMetrics: await probe('/api/upgrade-metrics/stats'),
          trialActivate: await probe('/auth/trial-activate'),
        };
      });

      expect(responses.runtimeCapabilities.status).toBe(200);
      expect(responses.runtimeCapabilities.body).toMatchObject({
        capabilities: ['ai_patrol', 'relay'],
        hosted_mode: false,
        max_history_days: 90,
      });
      expect(responses.runtimeCapabilities.body).not.toHaveProperty('licensed_email');
      expect(responses.runtimeCapabilities.body).not.toHaveProperty('tier');
      expect(responses.runtimeCapabilities.body).not.toHaveProperty('subscription_state');
      expect(responses.runtimeCapabilities.body).not.toHaveProperty('upgrade_reasons');
      expect((responses.runtimeCapabilities.body as { limits?: Array<{ key: string; current: number; limit: number; state: string }> }).limits).toEqual([
        { key: 'max_monitored_systems', limit: 0, current: 0, state: 'ok' },
      ]);

      expect(responses.entitlements.status).toBe(404);
      expect(responses.status.status).toBe(404);
      expect(responses.features.status).toBe(404);
      expect(responses.activate.status).toBe(404);
      expect(responses.clear.status).toBe(404);
      expect(responses.trialStart.status).toBe(404);
      expect(responses.ledger.status).toBe(404);
      expect(responses.billingState.status).toBe(404);
      expect(responses.upgradeMetrics.status).toBe(404);
      expect(responses.trialActivate.status).toBe(404);
    },
  );
});
