import { test, expect, type Page } from '@playwright/test';

const DEV_SERVER_URL = 'http://127.0.0.1:5173';

const MONITORED_SYSTEM_ENTITLEMENTS = {
  capabilities: [],
  limits: [{ key: 'max_monitored_systems', limit: 5, current: 16, state: 'enforced' }],
  subscription_state: 'expired',
  upgrade_reasons: [],
  tier: 'free',
  trial_eligible: false,
  hosted_mode: false,
  legacy_connections: {
    proxmox_nodes: 0,
    docker_hosts: 0,
    kubernetes_clusters: 0,
  },
  has_migration_gap: false,
  overflow_days_remaining: 14,
};

const MONITORED_SYSTEM_RUNTIME_CAPABILITIES = {
  capabilities: [],
  limits: [{ key: 'max_monitored_systems', limit: 5, current: 16, state: 'enforced' }],
  hosted_mode: false,
  max_history_days: 90,
};

const MONITORED_SYSTEM_COMMERCIAL_POSTURE = {
  subscription_state: 'expired',
  upgrade_reasons: [],
  tier: 'free',
  trial_eligible: false,
  legacy_connections: {
    proxmox_nodes: 0,
    docker_hosts: 0,
    kubernetes_clusters: 0,
  },
  has_migration_gap: false,
  overflow_days_remaining: 14,
};

const MONITORED_SYSTEM_LEDGER = {
  systems: [
    {
      name: 'edge-cluster',
      type: 'cluster',
      status: 'warning',
      status_explanation: {
        summary:
          'At least one included source has a warning state, so Pulse keeps this monitored system under review.',
        reasons: [],
      },
      latest_included_signal: {
        name: 'edge-cluster',
        type: 'cluster',
        source: 'kubernetes',
        at: '2026-04-07T09:00:00Z',
      },
      source: 'kubernetes',
      explanation: {
        summary:
          'Counts as one monitored system because Pulse merged multiple top-level views into one canonical cluster.',
        reasons: [],
        surfaces: [{ name: 'edge-cluster', type: 'cluster', source: 'kubernetes' }],
      },
    },
  ],
  total: 16,
  limit: 5,
};

async function configureMonitoredSystemBillingFixtures(page: Page) {
  await page.addInitScript(() => {
    localStorage.setItem('pulse_whats_new_v2_shown', 'true');
  });

  await page.route('**/api/security/status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        hasAuthentication: true,
        hideLocalLogin: false,
        ssoProviders: [],
        sessionCapabilities: {},
      }),
    });
  });

  await page.route('**/api/state', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ ok: true }),
    });
  });

  await page.route('**/api/system/settings', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        theme: 'system',
        fullWidthMode: false,
      }),
    });
  });

  await page.route('**/api/license/runtime-capabilities', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        capabilities: [],
        limits: [{ key: 'max_monitored_systems', limit: 5, current: 16, state: 'enforced' }],
        hosted_mode: false,
        max_history_days: 90,
      }),
    });
  });

  await page.route('**/api/license/entitlements', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(MONITORED_SYSTEM_ENTITLEMENTS),
    });
  });

  await page.route('**/api/license/runtime-capabilities', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(MONITORED_SYSTEM_RUNTIME_CAPABILITIES),
    });
  });

  await page.route('**/api/license/commercial-posture', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(MONITORED_SYSTEM_COMMERCIAL_POSTURE),
    });
  });

  await page.route('**/api/license/monitored-system-ledger', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(MONITORED_SYSTEM_LEDGER),
    });
  });
}

async function openPageWithUrgentMonitoredSystemBanner(page: Page) {
  await page.goto(`${DEV_SERVER_URL}/settings/system-general`, {
    waitUntil: 'domcontentloaded',
  });
  await expect(
    page.getByRole('status').filter({ hasText: 'Monitored systems: 16/5' }),
  ).toBeVisible();
}

test.describe('Monitored-system billing focus', () => {
  test('keeps Learn more and Upgrade arrivals distinct on the billing surface', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only billing navigation');

    await configureMonitoredSystemBillingFixtures(page);
    await openPageWithUrgentMonitoredSystemBanner(page);

    const banner = page.getByRole('status').filter({ hasText: 'Monitored systems: 16/5' });

    await banner.getByRole('link', { name: 'Learn more' }).click();
    await page.waitForURL('**/settings/system/billing/usage?details=counting-rules');
    await expect(page.getByRole('tab', { name: 'Usage' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await expect(page.getByRole('button', { name: 'Hide counting rules' })).toBeVisible();

    await openPageWithUrgentMonitoredSystemBanner(page);

    await banner.getByRole('link', { name: 'Upgrade to add more' }).click();
    await page.waitForURL('**/settings/system/billing/plan?intent=max_monitored_systems');
    await expect(page.getByRole('tab', { name: 'Plan' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await expect(page.getByText('Need a higher monitored-system cap?')).toBeVisible();
    await expect(page.getByRole('link', { name: 'Compare plans' })).toHaveAttribute(
      'href',
      '/auth/license-purchase-start?feature=max_monitored_systems',
    );
  });
});
