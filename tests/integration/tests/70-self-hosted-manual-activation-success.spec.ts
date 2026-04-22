import { expect, test, type Page } from '@playwright/test';

import { ensureAuthenticated } from './helpers';

const INACTIVE_ENTITLEMENTS = {
  capabilities: [],
  limits: [],
  subscription_state: 'expired',
  upgrade_reasons: [],
  tier: 'free',
  trial_eligible: true,
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
};

const INACTIVE_COMMERCIAL_POSTURE = {
  subscription_state: 'expired',
  upgrade_reasons: [],
  tier: 'free',
  trial_eligible: true,
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

async function stubManualActivationEndpoints(page: Page) {
  let activated = false;

  await page.route('**/api/license/runtime-capabilities', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(activated ? ACTIVE_RUNTIME_CAPABILITIES : INACTIVE_RUNTIME_CAPABILITIES),
    });
  });

  await page.route('**/api/license/entitlements', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(activated ? ACTIVE_ENTITLEMENTS : INACTIVE_ENTITLEMENTS),
    });
  });

  await page.route('**/api/license/commercial-posture', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(activated ? ACTIVE_COMMERCIAL_POSTURE : INACTIVE_COMMERCIAL_POSTURE),
    });
  });

  await page.route('**/api/license/activate', async (route) => {
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
}

test.describe('Self-hosted manual activation success', () => {
  test('shows the unlocked plan and capabilities immediately after a pasted key activates', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only plans coverage');

    await stubManualActivationEndpoints(page);
    await ensureAuthenticated(page);
    await page.goto('/settings/system/billing/plan', { waitUntil: 'domcontentloaded' });

    await expect(page.getByText('Current plan: Community')).toBeVisible();

    await page.locator('summary').filter({ hasText: 'Redeem existing key' }).first().click();
    const activationField = page.locator('#pulse-pro-license-key');
    await expect(activationField).toBeVisible();
    await activationField.fill('ppk_live_test_activation_key');
    await page.getByRole('button', { name: 'Activate License' }).click();

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
    await expect(activationSummary.getByText('Patrol Auto-Fix')).toBeVisible();
    await expect(activationSummary.getByText('Pulse Alert Analysis')).toBeVisible();

    await expect(page.getByText('Current plan: Pulse Pro')).toBeVisible();
    await expect(
      page.getByText(
        'Pulse Pro is active on this instance. AI operations, advanced administration, and 90-day history are unlocked right now.',
      ),
    ).toBeVisible();
  });
});
