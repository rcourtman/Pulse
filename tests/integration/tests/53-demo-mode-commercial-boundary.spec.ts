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

const DEMO_COMMERCIAL_ENTITLEMENTS = {
  capabilities: ['ai_patrol', 'relay'],
  limits: [
    {
      key: 'max_monitored_systems',
      limit: 5,
      current: 16,
      state: 'enforced',
    },
  ],
  subscription_state: 'trial',
  upgrade_reasons: [
    {
      key: 'max_monitored_systems',
      reason: 'Upgrade to monitor more systems.',
      action_url: '/settings/system/billing#plan',
    },
    {
      key: 'trial_banner',
      reason: 'Upgrade to keep Pro features active.',
      action_url: '/settings/system/billing#plan',
    },
  ],
  tier: 'pro',
  hosted_mode: false,
  trial_days_remaining: 7,
  trial_eligible: false,
  legacy_connections: {
    proxmox_nodes: 0,
    docker_hosts: 0,
    kubernetes_clusters: 0,
  },
  has_migration_gap: false,
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

    await page.route('**/api/license/entitlements', async (route) => {
      entitlementsRequests += 1;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(DEMO_COMMERCIAL_ENTITLEMENTS),
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
      .poll(() => entitlementsRequests, { message: 'expected demo shell to load entitlements' })
      .toBeGreaterThan(0);

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
});
