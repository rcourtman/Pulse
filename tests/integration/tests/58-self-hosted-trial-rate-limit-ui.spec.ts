import { expect, test } from '@playwright/test';
import { ensureAuthenticated } from './helpers';

const FREE_SELF_HOSTED_ENTITLEMENTS = {
  capabilities: [],
  limits: [],
  subscription_state: 'active',
  upgrade_reasons: [],
  tier: 'free',
  trial_eligible: false,
};

const SELF_HOSTED_SECURITY_STATUS = {
  hasAuthentication: true,
  hideLocalLogin: false,
  ssoProviders: [],
  sessionCapabilities: {
    demoMode: false,
    businessEstate: false,
  },
  // 2026-08-07 commercial-surfaces revision: the canonical free self-hosted
  // policy shows upgrade CTAs; suppression is reserved for demo mode and
  // white-label runtimes.
  presentationPolicy: {
    demoMode: false,
    readOnly: false,
    hideCommercial: false,
    hideUpgrade: false,
  },
  settingsCapabilities: {
    apiAccessRead: true,
    authenticationRead: true,
    singleSignOnRead: true,
    roles: true,
    users: true,
    auditLog: true,
    auditWebhooksRead: true,
    relayRead: true,
    relayWrite: true,
  },
};

test.describe.serial('Self-hosted paid prompt visibility', () => {
  test('surfaces paid-only navigation with inline gates and no trial ceremony', async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith('mobile-'),
      'Desktop-only settings navigation coverage',
    );

    await ensureAuthenticated(page);

    await page.route('**/api/security/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(SELF_HOSTED_SECURITY_STATUS),
      });
    });

    await page.route('**/api/license/runtime-capabilities', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          capabilities: [],
          limits: [],
          hosted_mode: false,
          max_history_days: 7,
        }),
      });
    });

    await page.route('**/api/license/commercial-posture', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(FREE_SELF_HOSTED_ENTITLEMENTS),
      });
    });

    await page.route('**/api/license/entitlements', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(FREE_SELF_HOSTED_ENTITLEMENTS),
      });
    });

    await page.goto('/settings/security-roles');

    await expect(page.getByRole('heading', { level: 1, name: 'Roles' })).toBeVisible();
    // The panel gates inline with the canonical upgrade CTA (2026-08-07
    // commercial-surfaces revision: paid-only tabs follow the Relay
    // precedent instead of hiding).
    await expect(page.getByRole('heading', { name: 'Custom Roles' }).first()).toBeVisible();
    await expect(page.getByRole('link', { name: 'View plans' }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Remote Access' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Roles' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Users' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Audit Log' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Audit Webhooks' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Plans & Billing' })).toBeVisible();
    // Trial ceremony and hosted handoff remain absent: the revision opens
    // discoverability, not trial or hosted flows.
    await expect(page.getByRole('link', { name: /upgrade to pro/i })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /start free trial/i })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /start trial/i })).toHaveCount(0);
    await expect(page.getByText(/free 14-day trial/i)).toHaveCount(0);
    await expect(page.getByText(/open hosted handoff/i)).toHaveCount(0);
  });
});
