import { expect, test, type Page } from '@playwright/test';

const PRO_RUNTIME_IDENTITY = {
  build: 'pro',
  label: 'Pulse Pro runtime',
  download_url: 'https://pulserelay.pro/download.html',
};

const INACTIVE_ENTITLEMENTS = {
  capabilities: [],
  limits: [],
  subscription_state: 'expired',
  upgrade_reasons: [],
  tier: 'free',
  trial_eligible: false,
  hosted_mode: false,
  valid: false,
};

const ACTIVE_ENTITLEMENTS = {
  capabilities: [
    'relay',
    'mobile_app',
    'push_notifications',
    'ai_patrol',
    'ai_autofix',
    'ai_alerts',
    'kubernetes_ai',
    'rbac',
    'audit_logging',
  ],
  limits: [],
  subscription_state: 'active',
  upgrade_reasons: [],
  tier: 'pro',
  plan_version: 'pro_monthly',
  licensed_email: 'owner@example.com',
  trial_eligible: false,
  hosted_mode: false,
  valid: true,
  is_lifetime: false,
  days_remaining: 30,
  max_history_days: 90,
  runtime: PRO_RUNTIME_IDENTITY,
};

const INACTIVE_RUNTIME_CAPABILITIES = {
  capabilities: [],
  limits: [],
  hosted_mode: false,
  max_history_days: 7,
};

const ACTIVE_RUNTIME_CAPABILITIES = {
  capabilities: ACTIVE_ENTITLEMENTS.capabilities,
  limits: ACTIVE_ENTITLEMENTS.limits,
  hosted_mode: false,
  max_history_days: 90,
  runtime: PRO_RUNTIME_IDENTITY,
};

const INACTIVE_COMMERCIAL_POSTURE = {
  subscription_state: 'expired',
  upgrade_reasons: [],
  tier: 'free',
  trial_eligible: false,
  monitored_system_capacity: null,
  monitored_system_continuity: null,
  has_migration_gap: false,
};

const ACTIVE_COMMERCIAL_POSTURE = {
  subscription_state: 'active',
  upgrade_reasons: [],
  tier: 'pro',
  trial_eligible: false,
  monitored_system_capacity: null,
  monitored_system_continuity: null,
  has_migration_gap: false,
};

type ManualActivationRequestCounts = {
  inactive: {
    runtimeCapabilities: number;
    entitlements: number;
    commercialPosture: number;
  };
  active: {
    runtimeCapabilities: number;
    entitlements: number;
    commercialPosture: number;
  };
  activate: number;
};

async function stubAuthenticatedShell(page: Page) {
  await page.route('**/api/security/status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        hasAuthentication: true,
        hideLocalLogin: false,
        ssoProviders: [],
        sessionCapabilities: {},
        settingsCapabilities: {
          apiAccessRead: true,
          apiAccessWrite: true,
          authenticationRead: true,
          authenticationWrite: true,
          singleSignOnRead: true,
          singleSignOnWrite: true,
          roles: true,
          users: true,
          auditLog: true,
          auditWebhooksRead: true,
          auditWebhooksWrite: true,
          relayRead: true,
          relayWrite: true,
          billingAdmin: true,
        },
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
}

async function stubManualActivationEndpoints(page: Page) {
  let activated = false;
  const requestCounts: ManualActivationRequestCounts = {
    inactive: {
      runtimeCapabilities: 0,
      entitlements: 0,
      commercialPosture: 0,
    },
    active: {
      runtimeCapabilities: 0,
      entitlements: 0,
      commercialPosture: 0,
    },
    activate: 0,
  };

  await page.route('**/api/license/runtime-capabilities', async (route) => {
    const stateKey = activated ? 'active' : 'inactive';
    requestCounts[stateKey].runtimeCapabilities += 1;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(activated ? ACTIVE_RUNTIME_CAPABILITIES : INACTIVE_RUNTIME_CAPABILITIES),
    });
  });

  await page.route('**/api/license/entitlements', async (route) => {
    const stateKey = activated ? 'active' : 'inactive';
    requestCounts[stateKey].entitlements += 1;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(activated ? ACTIVE_ENTITLEMENTS : INACTIVE_ENTITLEMENTS),
    });
  });

  await page.route('**/api/license/commercial-posture', async (route) => {
    const stateKey = activated ? 'active' : 'inactive';
    requestCounts[stateKey].commercialPosture += 1;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(activated ? ACTIVE_COMMERCIAL_POSTURE : INACTIVE_COMMERCIAL_POSTURE),
    });
  });

  await page.route('**/api/license/activate', async (route) => {
    requestCounts.activate += 1;
    activated = true;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        success: true,
        message: 'License activated',
      }),
    });
  });

  return requestCounts;
}

test.describe('Self-hosted manual activation success', () => {
  test('shows the unlocked plan and capabilities immediately after a pasted key activates', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only plans coverage');

    await stubAuthenticatedShell(page);
    const requestCounts = await stubManualActivationEndpoints(page);
    await page.goto('/settings/system/billing/plan', { waitUntil: 'domcontentloaded' });

    const communityPlanCard = page
      .locator('div.rounded-md.border.border-border.bg-surface-alt.p-4')
      .filter({ has: page.getByText('Current plan: Community') })
      .first();

    await expect(communityPlanCard.getByText('Current plan: Community')).toBeVisible();
    await expect(communityPlanCard.getByText('Community', { exact: true })).toBeVisible();
    await expect(communityPlanCard.getByText('Expired', { exact: true })).toHaveCount(0);
    await expect(
      communityPlanCard.getByText(
        'Community is active on this instance. It includes self-hosted monitoring, 7-day metric history, Pulse Patrol (BYOK), update alerts, and SSO.',
      ),
    ).toBeVisible();
    await expect(page.getByText('Optional extras')).toHaveCount(0);
    await expect(page.getByText('What Relay adds')).toHaveCount(0);
    await expect(page.getByText('What Pulse Pro adds')).toHaveCount(0);
    await expect(page.getByRole('link', { name: 'Compare plans' })).toHaveCount(0);

    await page.locator('summary').filter({ hasText: 'Use existing key' }).first().click();
    const activationField = page.locator('#pulse-pro-license-key');
    await expect(activationField).toBeVisible();
    await activationField.fill('ppk_live_test_activation_key');
    await page.getByRole('button', { name: 'Activate Key' }).click();

    const activationSummary = page
      .locator('div.rounded-md.border.p-3.text-sm')
      .filter({ has: page.getByText('Pulse Pro is now active', { exact: true }) })
      .first();

    await expect(activationSummary.getByText('Pulse Pro is now active', { exact: true })).toBeVisible();
    await expect(
      activationSummary.getByText(
        'The activation key was accepted and this instance is now running Pulse Pro.',
      ),
    ).toBeVisible();
    await expect(activationSummary.getByText('Available now on this instance')).toBeVisible();
    await expect(activationSummary.getByText('Patrol Fixes Safe Issues')).toBeVisible();
    await expect(activationSummary.getByText('Patrol Investigates Alerts')).toBeVisible();
    await expect.poll(() => requestCounts.activate).toBe(1);
    await expect.poll(() => requestCounts.active.entitlements).toBeGreaterThan(0);
    await expect.poll(() => requestCounts.active.runtimeCapabilities).toBeGreaterThan(0);
    await expect.poll(() => requestCounts.active.commercialPosture).toBeGreaterThan(0);

    await expect(page.getByText('Current plan: Pulse Pro')).toBeVisible();
    const currentPlanCard = page
      .locator('div.rounded-md.border.border-border.bg-surface-alt.p-4')
      .filter({ has: page.getByText('Current plan: Pulse Pro') })
      .first();
    await expect(
      currentPlanCard.getByText(
        'Pulse Pro is active on this instance. Patrol control, alert investigation, verified fixes, 90-day history, and admin/reporting extras are available right now.',
      ),
    ).toBeVisible();
    await expect(currentPlanCard.getByText('Included extras')).toBeVisible();
    await expect(page.getByText('Optional extras')).toHaveCount(0);
  });
});
