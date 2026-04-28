import { expect, test, type Page } from '@playwright/test';

import { ensureAuthenticated } from './helpers';

const PRO_PLAN_ENTITLEMENTS = {
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
  monitored_system_capacity: {
    mode: 'unlimited',
    urgency: 'ok',
    current: 23,
    limit: 0,
    current_available: true,
    available_slots: 0,
    overage: 0,
    blocks_new_systems: false,
    existing_monitoring_continues: true,
  },
  monitored_system_continuity: null,
  subscription_state: 'active',
  upgrade_reasons: [],
  tier: 'pro',
  plan_version: 'v5_pro_monthly_grandfathered',
  licensed_email: 'owner@example.com',
  trial_eligible: false,
  hosted_mode: false,
  valid: true,
  is_lifetime: false,
  days_remaining: 30,
  in_grace_period: false,
  grace_period_end: null,
  max_history_days: 90,
};

const PRO_PLAN_COMMERCIAL_POSTURE = {
  subscription_state: 'active',
  upgrade_reasons: [],
  tier: 'pro',
  trial_eligible: false,
  monitored_system_capacity: PRO_PLAN_ENTITLEMENTS.monitored_system_capacity,
  monitored_system_continuity: PRO_PLAN_ENTITLEMENTS.monitored_system_continuity,
  has_migration_gap: false,
};

const PRO_PLAN_RUNTIME_CAPABILITIES = {
  capabilities: PRO_PLAN_ENTITLEMENTS.capabilities,
  limits: PRO_PLAN_ENTITLEMENTS.limits,
  monitored_system_capacity: PRO_PLAN_ENTITLEMENTS.monitored_system_capacity,
  hosted_mode: false,
  max_history_days: 90,
};

async function stubSelfHostedPlanEndpoints(page: Page) {
  await page.route('**/api/license/runtime-capabilities', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(PRO_PLAN_RUNTIME_CAPABILITIES),
    });
  });

  await page.route('**/api/license/entitlements', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(PRO_PLAN_ENTITLEMENTS),
    });
  });

  await page.route('**/api/license/commercial-posture', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(PRO_PLAN_COMMERCIAL_POSTURE),
    });
  });
}

test.describe('Self-hosted plans entitlement summary', () => {
  test('shows paid Pulse Pro entitlements and continuity at the top of Plans', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only plans coverage');

    await stubSelfHostedPlanEndpoints(page);
    await ensureAuthenticated(page);
    await page.goto('/settings/system/billing/plan', { waitUntil: 'domcontentloaded' });

    await expect(
      page.locator('[aria-label="Settings navigation"]').getByText('Plans', { exact: true }),
    ).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Plans & Activation' }).first()).toBeVisible();
    const currentPlanCard = page
      .locator('div.rounded-md.border.border-border.bg-surface-alt.p-4')
      .filter({ has: page.getByText('Current plan: Pulse Pro') })
      .first();

    await expect(currentPlanCard.getByText('Current plan: Pulse Pro')).toBeVisible();
    await expect(
      currentPlanCard.getByText(
        'Pulse Pro is active on this instance. Root-cause analysis, safe remediation workflows, and 90-day history are unlocked right now.',
      ),
    ).toBeVisible();
    await expect(currentPlanCard.getByText('Grandfathered price')).toBeVisible();
    await expect(currentPlanCard.getByText('Grandfathered floor')).toHaveCount(0);
    await expect(
      currentPlanCard.getByText(
        /existing recurring price and uncapped monitored-system and guest capacity until cancellation/i,
      ),
    ).toBeVisible();
    await expect(
      page.getByText(
        /keeps its existing recurring price and uncapped monitored-system and guest capacity until you cancel/i,
      ),
    ).toBeVisible();
    await expect(currentPlanCard.getByText(/effective monitored-system limit/i)).toHaveCount(0);
    await expect(page.getByText('Core Monitoring', { exact: true })).toBeVisible();
    await expect(page.getByText('Guest Capacity', { exact: true })).toBeVisible();
    await expect(page.getByText('Unlimited').first()).toBeVisible();
    await expect(currentPlanCard.getByText('Primary capabilities')).toBeVisible();
    await expect(currentPlanCard.getByText('Included extras')).toBeVisible();
    await expect(
      currentPlanCard.getByText('Safe Remediation Workflows', { exact: true }),
    ).toBeVisible();
    await expect(
      currentPlanCard.getByText('Alert Root-Cause Analysis', { exact: true }),
    ).toBeVisible();
    await expect(
      currentPlanCard.getByText('Advanced SSO (SAML/Multi-Provider)', { exact: true }),
    ).toBeVisible();
  });
});
