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
  },
  presentationPolicy: {
    demoMode: false,
    readOnly: false,
    hideCommercial: false,
    hideUpgrade: true,
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
  test('keeps paid-only navigation and trial CTAs out of the default self-hosted UI', async ({
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

    await expect(
      page.getByRole('heading', { name: 'Infrastructure', exact: true }),
    ).toBeVisible();
    await expect(page.getByText('Custom Roles (Pro)')).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Remote Access' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Roles' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Users' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Audit Log' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Audit Webhooks' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Plans & Activation' })).toHaveCount(0);
    await expect(page.getByRole('link', { name: /upgrade to pro/i })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /start free trial/i })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /start trial/i })).toHaveCount(0);
    await expect(page.getByText(/free 14-day trial/i)).toHaveCount(0);
    await expect(page.getByText(/open hosted handoff/i)).toHaveCount(0);
  });
});
