import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base, type Page } from '@playwright/test';
import { createAuthenticatedStorageState, ensureAuthenticated, trackBrowserRequests } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

type BrowserProbeResult = {
  status: number;
  body: unknown;
};

const HIDDEN_DEMO_COMMERCIAL_RESPONSE_KEYS = [
  'commercialPosture',
  'entitlements',
  'status',
  'features',
  'activate',
  'clear',
  'trialStart',
  'ledger',
  'ledgerPreview',
  'billingState',
  'upgradeMetrics',
  'truenasDraftPreview',
  'truenasSavedPreview',
  'vmwareDraftPreview',
  'vmwareSavedPreview',
  'checkoutStart',
  'trialActivate',
] as const;

type HiddenDemoCommercialResponseKey = typeof HIDDEN_DEMO_COMMERCIAL_RESPONSE_KEYS[number];

type DemoCommercialBoundaryResponses = Record<
  'runtimeCapabilities' | HiddenDemoCommercialResponseKey,
  BrowserProbeResult
>;

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

const truthy = (value: unknown): boolean =>
  ['1', 'true', 'yes', 'on'].includes(String(value ?? '').trim().toLowerCase());

const integratedManagedDemoRuntime =
  truthy(process.env.PULSE_E2E_USE_LOCAL_BACKEND) &&
  truthy(process.env.DEMO_MODE) &&
  truthy(process.env.PULSE_MOCK_MODE);

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

async function probeDemoCommercialBoundaryFromBrowser(
  page: Page,
): Promise<DemoCommercialBoundaryResponses> {
  return page.evaluate(async () => {
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
    const postEmptyJSON = {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    };

    return {
      runtimeCapabilities: await probe('/api/license/runtime-capabilities'),
      status: await probe('/api/license/status'),
      features: await probe('/api/license/features'),
      commercialPosture: await probe('/api/license/commercial-posture'),
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
      ledgerPreview: await probe('/api/license/monitored-system-ledger/preview', postEmptyJSON),
      billingState: await probe('/api/admin/orgs/default/billing-state'),
      upgradeMetrics: await probe('/api/upgrade-metrics/stats'),
      truenasDraftPreview: await probe('/api/truenas/connections/preview', postEmptyJSON),
      truenasSavedPreview: await probe('/api/truenas/connections/conn-1/preview', postEmptyJSON),
      vmwareDraftPreview: await probe('/api/vmware/connections/preview', postEmptyJSON),
      vmwareSavedPreview: await probe('/api/vmware/connections/conn-1/preview', postEmptyJSON),
      checkoutStart: await probe('/auth/license-purchase-start'),
      trialActivate: await probe('/auth/trial-activate'),
    };
  });
}

function expectPublicDemoRuntimeCapabilities(response: BrowserProbeResult): void {
  expect(response.status).toBe(200);
  expect(response.body).toMatchObject({ hosted_mode: false });
  expect((response.body as { capabilities?: string[] }).capabilities).toEqual(
    expect.arrayContaining(['ai_patrol', 'relay']),
  );
  expect((response.body as { max_history_days?: number }).max_history_days).toEqual(expect.any(Number));
  expect((response.body as { max_history_days?: number }).max_history_days).toBeGreaterThan(0);
  expect(response.body).not.toHaveProperty('licensed_email');
  expect(response.body).not.toHaveProperty('tier');
  expect(response.body).not.toHaveProperty('subscription_state');
  expect(response.body).not.toHaveProperty('upgrade_reasons');
  const limits = (response.body as { limits?: Array<{ key: string; current: number; limit: number; state: string }> }).limits ?? [];
  expect(limits).toContainEqual({ key: 'max_monitored_systems', limit: 0, current: 0, state: 'ok' });
  for (const limit of limits) {
    expect(limit.limit, `expected ${limit.key} demo limit to be redacted`).toBe(0);
    expect(limit.current, `expected ${limit.key} demo current usage to be redacted`).toBe(0);
    expect(limit.state, `expected ${limit.key} demo limit state to be harmless`).toBe('ok');
  }
}

function expectHiddenDemoCommercialResponses(
  responses: DemoCommercialBoundaryResponses,
): void {
  for (const key of HIDDEN_DEMO_COMMERCIAL_RESPONSE_KEYS) {
    expect(responses[key].status, `expected ${key} to stay hidden in demo mode`).toBe(404);
  }
}

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
    let commercialPostureRequests = 0;
    let entitlementsRequests = 0;
    let licenseStatusRequests = 0;
    let monitoredSystemLedgerRequests = 0;
    let billingStateRequests = 0;
    let checkoutStartRequests = 0;

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

    await page.route('**/api/license/commercial-posture', async (route) => {
      commercialPostureRequests += 1;
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'not found' }),
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

    await page.route('**/api/license/monitored-system-ledger**', async (route) => {
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

    await page.route('**/auth/license-purchase-start**', async (route) => {
      checkoutStartRequests += 1;
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'not found' }),
      });
    });

    await page.goto('/settings/system/billing/usage?details=counting-rules', {
      waitUntil: 'domcontentloaded',
    });
    await page.waitForURL('**/settings/infrastructure/install', { timeout: 15_000 });

    await expect
      .poll(() => runtimeCapabilitiesRequests, {
        message: 'expected demo shell to load runtime capabilities',
      })
      .toBeGreaterThan(0);
    expect(commercialPostureRequests, 'demo settings route should not read commercial posture').toBe(0);
    expect(entitlementsRequests, 'demo settings route should not read commercial entitlements').toBe(0);

    await expect(
      page.getByText('Demo instance with mock data (read-only)', { exact: true }),
    ).toBeVisible();
    await expect(page.getByRole('heading', { level: 1, name: 'Infrastructure Operations' })).toBeVisible();
    const settingsNavigation = page.locator('[aria-label="Settings navigation"]');
    await expect(settingsNavigation).toBeVisible();
    await expect(page.getByText('Default Organization', { exact: true })).toHaveCount(0);
    await expect(settingsNavigation.getByText('Organization', { exact: true })).toHaveCount(0);
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
    expect(checkoutStartRequests, 'demo settings route should not trigger hosted checkout start').toBe(0);

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
      await hiddenNotFound('**/api/license/commercial-posture');
      await hiddenNotFound('**/api/license/entitlements');
      await hiddenNotFound('**/api/license/activate');
      await hiddenNotFound('**/api/license/clear');
      // Demo mode must fail closed on the trial-start route before either
      // `trial_signup_required` or `trial_rate_limited` plus `Retry-After`
      // can surface.
      await hiddenNotFound('**/api/license/trial/start');
      await hiddenNotFound('**/api/license/monitored-system-ledger**');
      await hiddenNotFound('**/api/admin/orgs/**/billing-state');
      await hiddenNotFound('**/api/upgrade-metrics/**');
      await hiddenNotFound('**/api/truenas/connections/preview');
      await hiddenNotFound('**/api/truenas/connections/*/preview');
      await hiddenNotFound('**/api/vmware/connections/preview');
      await hiddenNotFound('**/api/vmware/connections/*/preview');
      await hiddenNotFound('**/auth/license-purchase-start**');
      await hiddenNotFound('**/auth/trial-activate');

      await page.goto('/settings/infrastructure/install', { waitUntil: 'domcontentloaded' });

      const responses = await probeDemoCommercialBoundaryFromBrowser(page);
      expectPublicDemoRuntimeCapabilities(responses.runtimeCapabilities);
      expectHiddenDemoCommercialResponses(responses);
    },
  );
});

base.describe('Managed demo runtime commercial boundary', () => {
  base.setTimeout(180_000);
  base.skip(
    !integratedManagedDemoRuntime,
    'Run with PULSE_E2E_USE_LOCAL_BACKEND=1 DEMO_MODE=true PULSE_MOCK_MODE=true to exercise the real managed demo runtime.',
  );

  base(
    'hides commercial surfaces and APIs without browser route stubs',
    async ({ page }) => {
      await page.addInitScript(() => {
        localStorage.setItem('pulse_whats_new_v2_shown', 'true');
      });
      await ensureAuthenticated(page);

      const hiddenCommercialRequests = trackBrowserRequests(
        page,
        /\/api\/license\/(?!runtime-capabilities)|\/api\/admin\/orgs\/[^/]+\/billing-state|\/api\/upgrade-metrics\/|\/auth\/license-purchase-start|\/auth\/trial-activate/,
      );
      try {
        await page.goto('/settings/system/billing/usage?details=counting-rules', {
          waitUntil: 'domcontentloaded',
        });
        await page.waitForURL('**/settings/infrastructure/install', { timeout: 15_000 });

        await expect(
          page.getByText('Demo instance with mock data (read-only)', { exact: true }),
        ).toBeVisible();
        await expect(page.getByRole('heading', { level: 1, name: 'Infrastructure Operations' })).toBeVisible();
        const settingsNavigation = page.locator('[aria-label="Settings navigation"]');
        await expect(settingsNavigation).toBeVisible();
        await expect(page.getByText('Default Organization', { exact: true })).toHaveCount(0);
        await expect(settingsNavigation.getByText('Organization', { exact: true })).toHaveCount(0);
        await expect(settingsNavigation.getByText('Pulse Pro', { exact: true })).toHaveCount(0);
        await expect(page.getByText('Pro Trial:', { exact: false })).toHaveCount(0);
        await expect(page.getByText(/Monitored systems:\s*\d+\/\d+/)).toHaveCount(0);
        await expect(
          page
            .locator('[role="tab"]')
            .filter({ hasText: 'Settings' })
            .getByText('Pro', { exact: true }),
        ).toHaveCount(0);

        expect(
          hiddenCommercialRequests.urls(),
          'demo settings navigation should not request hidden commercial APIs',
        ).toEqual([]);
      } finally {
        hiddenCommercialRequests.stop();
      }

      const securityStatus = await page.evaluate(async () => {
        const res = await fetch('/api/security/status', { credentials: 'include' });
        return { status: res.status, body: await res.json() };
      });
      expect(securityStatus.status).toBe(200);
      expect(securityStatus.body).toMatchObject({
        sessionCapabilities: { demoMode: true },
        presentationPolicy: {
          demoMode: true,
          readOnly: true,
          hideCommercial: true,
          hideUpgrade: true,
        },
      });

      const responses = await probeDemoCommercialBoundaryFromBrowser(page);
      expectPublicDemoRuntimeCapabilities(responses.runtimeCapabilities);
      expectHiddenDemoCommercialResponses(responses);
    },
  );
});
