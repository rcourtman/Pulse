import { expect, test, type Page } from '@playwright/test';

import { ensureAuthenticated } from './helpers';

const PRO_RUNTIME_IDENTITY = {
  build: 'pro',
  label: 'Pulse Pro runtime',
  download_url: 'https://pulserelay.pro/download.html',
};

const COMMUNITY_RUNTIME_IDENTITY = {
  build: 'community',
  label: 'Pulse Community runtime',
  download_url: 'https://pulserelay.pro/download.html',
};

const PRO_PLAN_ENTITLEMENTS = {
  capabilities: [
    'update_alerts',
    'sso',
    'ai_patrol',
    'relay',
    'mobile_app',
    'push_notifications',
    'long_term_metrics',
    'ai_alerts',
    'ai_autofix',
    'kubernetes_ai',
    'agent_profiles',
    'advanced_sso',
    'rbac',
    'audit_logging',
    'advanced_reporting',
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
  runtime: PRO_RUNTIME_IDENTITY,
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
  runtime: PRO_RUNTIME_IDENTITY,
};

const PRO_PLAN_COMMUNITY_RUNTIME_CAPABILITIES = {
  ...PRO_PLAN_RUNTIME_CAPABILITIES,
  capabilities: [
    'update_alerts',
    'sso',
    'ai_patrol',
    'relay',
    'mobile_app',
    'push_notifications',
    'long_term_metrics',
    'ai_alerts',
  ],
  runtime: COMMUNITY_RUNTIME_IDENTITY,
  blocked_capabilities: [
    {
      key: 'audit_logging',
      reason: 'paid_runtime_required',
      action_url: 'https://pulserelay.pro/download.html',
    },
    {
      key: 'rbac',
      reason: 'paid_runtime_required',
      action_url: 'https://pulserelay.pro/download.html',
    },
    {
      key: 'advanced_reporting',
      reason: 'paid_runtime_required',
      action_url: 'https://pulserelay.pro/download.html',
    },
  ],
};

async function stubSelfHostedPlanEndpoints(
  page: Page,
  payloads: {
    entitlements?: Record<string, unknown>;
    commercialPosture?: Record<string, unknown>;
    runtimeCapabilities?: Record<string, unknown>;
  } = {},
) {
  // The app shell makes auth-gated calls beyond the old stub set, so a
  // stubbed session renders the login page; sign in for real and stub only
  // the license surface under test.
  await ensureAuthenticated(page);

  const runtimeCapabilities = payloads.runtimeCapabilities ?? PRO_PLAN_RUNTIME_CAPABILITIES;
  const entitlements = payloads.entitlements ?? PRO_PLAN_ENTITLEMENTS;
  const commercialPosture = payloads.commercialPosture ?? PRO_PLAN_COMMERCIAL_POSTURE;

  await page.route('**/api/license/runtime-capabilities', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(runtimeCapabilities),
    });
  });

  await page.route('**/api/license/entitlements', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(entitlements),
    });
  });

  await page.route('**/api/license/commercial-posture', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(commercialPosture),
    });
  });
}

test.describe('Self-hosted plans entitlement summary', () => {
  test('shows paid Pulse Pro entitlements and continuity at the top of Plans', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only plans coverage');

    await stubSelfHostedPlanEndpoints(page);
    // The legacy system billing route must still land on the canonical
    // Pulse Intelligence billing route.
    await page.goto('/settings/system/billing/plan', { waitUntil: 'domcontentloaded' });
    await page.waitForURL('**/settings/pulse-intelligence/billing/plan', { timeout: 30_000 });

    await expect(
      page.locator('[aria-label="Settings navigation"]').getByText('Plans', { exact: true }),
    ).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Plans & Billing' }).first()).toBeVisible({
      timeout: 30_000,
    });
    await expect(page.getByRole('heading', { name: 'Current plan' })).toBeVisible();

    // The plan panel renders once per page, so the copy assertions anchor at
    // page scope; the old card container classes are private layout detail.
    await expect(page.getByText('Current plan: Pulse Pro')).toBeVisible();
    await expect(page.getByText(/^Pulse Pro is active on this instance\./)).toBeVisible();
    await expect(page.getByText('Grandfathered price')).toBeVisible();
    await expect(page.getByText('Grandfathered floor')).toHaveCount(0);
    await expect(
      page.getByText(/keeps its existing recurring price until cancellation/i),
    ).toBeVisible();
    await expect(
      page.getByText(/self-hosted monitoring and child-resource volume are not metered/i).first(),
    ).toBeVisible();
    const recurringContinuityNotice = page.getByText(
      /This migrated v5 Pro subscription keeps its existing recurring price until you cancel/i,
    );
    await expect(recurringContinuityNotice.first()).toBeVisible();
    await expect(recurringContinuityNotice.first()).toContainText(
      /Self-hosted monitoring and child-resource volume are not metered/i,
    );
    await expect(page.getByText(/effective monitored-system limit/i)).toHaveCount(0);
    await expect(page.getByText('Core Monitoring', { exact: true })).toBeVisible();
    await expect(page.getByText('Guest Capacity', { exact: true })).toHaveCount(0);
    await expect(page.getByText('Unlimited self-hosted monitoring')).toHaveCount(0);
    await expect(page.getByText('Included', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Primary capabilities')).toBeVisible();
    await expect(page.getByText('Included extras', { exact: true })).toBeVisible();
    await expect(
      page.getByText('Patrol Applies Safe Fixes and Verifies the Result', { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText('Patrol Investigates Issues and Explains the Root Cause', { exact: true }),
    ).toBeVisible();
    await expect(page.getByText('PDF/CSV Reporting', { exact: true })).toBeVisible();
    await expect(
      page.getByText('Centralized Agent Profiles', { exact: true }),
    ).toBeVisible();
  });

  test('warns when an active Pro plan does not report the private runtime', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only plans coverage');

    const unknownRuntimeEntitlements: Record<string, unknown> = { ...PRO_PLAN_ENTITLEMENTS };
    delete unknownRuntimeEntitlements.runtime;
    const unknownRuntimeCapabilities: Record<string, unknown> = {
      ...PRO_PLAN_RUNTIME_CAPABILITIES,
    };
    delete unknownRuntimeCapabilities.runtime;

    await stubSelfHostedPlanEndpoints(page, {
      entitlements: unknownRuntimeEntitlements,
      runtimeCapabilities: unknownRuntimeCapabilities,
    });
    await page.goto('/settings/system/billing/plan', { waitUntil: 'domcontentloaded' });

    await expect(page.getByText('Current plan: Pulse Pro')).toBeVisible({ timeout: 30_000 });
    await expect(page.getByText('Pro runtime missing').first()).toBeVisible();
    await expect(
      page.getByText(/not reporting (the private|a) Pulse Pro runtime/i).first(),
    ).toBeVisible();
    await expect(
      page.getByRole('link', { name: 'Open Pulse Pro downloads' }).first(),
    ).toHaveAttribute('href', 'https://pulserelay.pro/download.html');
  });

  test('warns when an active Pro plan reports the public community runtime', async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith('mobile-'),
      'Desktop-only plans coverage',
    );

    await stubSelfHostedPlanEndpoints(page, {
      entitlements: {
        ...PRO_PLAN_ENTITLEMENTS,
        runtime: COMMUNITY_RUNTIME_IDENTITY,
      },
      runtimeCapabilities: PRO_PLAN_COMMUNITY_RUNTIME_CAPABILITIES,
    });
    await page.goto('/settings/system/billing/plan', {
      waitUntil: 'domcontentloaded',
    });

    await expect(page.getByText('Current plan: Pulse Pro')).toBeVisible({ timeout: 30_000 });
    await expect(page.getByText('Pro runtime missing').first()).toBeVisible();
    await expect(
      page.getByText(/running the community runtime/i).first(),
    ).toBeVisible();
    await expect(
      page.getByText(/public GitHub releases and the public Docker image/i).first(),
    ).toBeVisible();
    await expect(
      page.getByRole('link', { name: 'Open Pulse Pro downloads' }).first(),
    ).toHaveAttribute('href', 'https://pulserelay.pro/download.html');
    await expect(page.getByText(/upgrade your plan/i)).toHaveCount(0);
  });

  test('surfaces advertised Pro settings sections when Pro capabilities are active', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only settings coverage');

    await stubSelfHostedPlanEndpoints(page);
    await page.goto('/settings/system/billing/plan', { waitUntil: 'domcontentloaded' });

    const settingsNav = page.locator('[aria-label="Settings navigation"]');
    await expect(settingsNav).toBeVisible({ timeout: 30_000 });
    for (const label of [
      'Data & Reports',
      'Roles',
      'Users',
      'Audit Log',
      'Audit Webhooks',
      'Remote Access',
    ]) {
      await expect(settingsNav.getByText(label, { exact: true })).toBeVisible();
    }

    await page.goto('/settings/security-sso', { waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('button', { name: 'Add SAML' })).toBeVisible();

    await page.goto('/settings/infrastructure?add=agent', { waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('button', { name: 'Manage agent profiles' })).toBeVisible();
  });
});
