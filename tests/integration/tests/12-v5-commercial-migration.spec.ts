import { test, expect, type Locator, type Page } from '@playwright/test';
import { apiRequest, ensureAuthenticated } from './helpers';

type EntitlementPayload = {
  valid?: boolean;
  tier?: string;
  plan_version?: string;
  licensed_email?: string;
  is_lifetime?: boolean;
  subscription_state?: string;
  trial_eligible?: boolean;
  trial_eligibility_reason?: string;
  limits?: Array<{ key: string; limit: number }>;
  commercial_migration?: {
    state?: string;
    reason?: string;
    recommended_action?: string;
  };
};

type ExpectedCopy = {
  title: RegExp;
  bodyFragments: RegExp[];
  trialReason: string;
};

const expectedState = process.env.PULSE_E2E_EXPECT_COMMERCIAL_MIGRATION_STATE || '';
const expectedReason = process.env.PULSE_E2E_EXPECT_COMMERCIAL_MIGRATION_REASON || '';
const expectedAction = process.env.PULSE_E2E_EXPECT_COMMERCIAL_MIGRATION_ACTION || '';
const expectedTier = process.env.PULSE_E2E_EXPECT_TIER || '';
const expectedPlanVersion = process.env.PULSE_E2E_EXPECT_PLAN_VERSION || '';
const expectedLicensedEmail = process.env.PULSE_E2E_EXPECT_LICENSED_EMAIL || '';
const expectedMaxAgents = Number(process.env.PULSE_E2E_EXPECT_MAX_AGENTS || '0');
const expectedIsLifetime = (process.env.PULSE_E2E_EXPECT_IS_LIFETIME || '').trim().toLowerCase();
const successMode = expectedState === 'success';

function formatTitleCase(value: string): string {
  return value.replace(/[_-]/g, ' ').replace(/\b\w/g, (match) => match.toUpperCase());
}

function expectedTierLabelFor(tier: string): string {
  switch (tier) {
    case 'free':
      return 'Community';
    case 'relay':
      return 'Relay';
    case 'pro':
      return 'Pro';
    case 'pro_plus':
      return 'Pro+';
    case 'pro_annual':
      return 'Pro Annual';
    case 'lifetime':
      return 'Lifetime';
    case 'cloud':
      return 'Cloud';
    case 'msp':
      return 'MSP';
    case 'enterprise':
      return 'Enterprise';
    default:
      return formatTitleCase(tier);
  }
}

function expectFieldValue(pageTextLocator: Locator, value: RegExp | string) {
  if (typeof value === 'string') {
    return expect(pageTextLocator).toHaveText(value);
  }
  return expect(pageTextLocator).toContainText(value);
}

function expectFieldLocator(page: Page, label: string) {
  return page
    .locator('p')
    .filter({ hasText: new RegExp(`^${label}$`) })
    .first()
    .locator('xpath=following-sibling::p[1]');
}

function expectedCopyFor(state: string, reason: string, action: string): ExpectedCopy {
  const actionFragment =
    action === 'retry_activation'
      ? /retry activation from this instance/i
      : action === 'use_v6_activation_key'
        ? /use the current v6 activation key for this purchase/i
        : action === 'enter_supported_v5_key'
          ? /retry with the original v5 pro\/lifetime key from this instance/i
          : /review the activation state from this instance/i;

  if (state === 'pending') {
    const reasonFragment =
      reason === 'exchange_rate_limited'
        ? /rate-limited right now/i
        : reason === 'exchange_conflict'
          ? /another v6 activation handoff is still settling/i
          : /automatic v6 exchange did not complete yet/i;

    return {
      title: /v5 license migration pending/i,
      bodyFragments: [reasonFragment, actionFragment, /new pro trial stays blocked/i],
      trialReason: 'commercial_migration_pending',
    };
  }

  const reasonFragment =
    reason === 'exchange_invalid'
      ? /key was rejected during v6 migration/i
      : reason === 'exchange_malformed'
        ? /malformed and cannot be migrated/i
        : reason === 'exchange_revoked'
          ? /no longer eligible for automatic migration/i
          : reason === 'exchange_non_migratable'
            ? /not eligible for automatic v6 migration/i
            : reason === 'exchange_unsupported'
              ? /not a supported v5 pro\/lifetime migration input/i
              : /could not be migrated automatically/i;

  return {
    title: /v5 license migration needs attention/i,
    bodyFragments: [reasonFragment, actionFragment, /new pro trial stays blocked/i],
    trialReason: 'commercial_migration_failed',
  };
}

test.describe.serial('v5 commercial migration notice', () => {
  test.beforeEach(async ({}, testInfo) => {
    test.skip(!expectedState, 'Set PULSE_E2E_EXPECT_COMMERCIAL_MIGRATION_STATE to enable migration UI checks');
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only migration UI coverage');
    if (successMode) {
      test.skip(
        !expectedTier || !expectedPlanVersion || !expectedLicensedEmail || !expectedIsLifetime,
        'Set success-mode expectation env vars to enable migrated-license assertions',
      );
      return;
    }
    test.skip(
      !expectedReason || !expectedAction,
      'Set PULSE_E2E_EXPECT_COMMERCIAL_MIGRATION_REASON/ACTION for unresolved migration checks',
    );
  });

  test('entitlements expose the expected v5 migration result', async ({ page }) => {
    await ensureAuthenticated(page);

    const res = await apiRequest(page, '/api/license/entitlements');
    expect(res.ok(), `GET /api/license/entitlements failed: ${res.status()}`).toBeTruthy();

    const entitlements = (await res.json()) as EntitlementPayload;

    if (successMode) {
      expect(entitlements.commercial_migration).toBeFalsy();
      expect(entitlements.subscription_state).toBe('active');
      expect(entitlements.valid).toBe(true);
      expect(entitlements.tier).toBe(expectedTier);
      expect(entitlements.plan_version).toBe(expectedPlanVersion);
      expect(entitlements.licensed_email).toBe(expectedLicensedEmail);
      expect(entitlements.is_lifetime).toBe(expectedIsLifetime === 'true');
      expect(entitlements.trial_eligible).toBe(false);
      if (expectedMaxAgents > 0) {
        const maxAgents = entitlements.limits?.find((limit) => limit.key === 'max_agents');
        expect(maxAgents?.limit).toBe(expectedMaxAgents);
      }
      return;
    }

    const expectedCopy = expectedCopyFor(expectedState, expectedReason, expectedAction);
    expect(entitlements.commercial_migration?.state).toBe(expectedState);
    expect(entitlements.commercial_migration?.reason).toBe(expectedReason);
    expect(entitlements.commercial_migration?.recommended_action).toBe(expectedAction);
    expect(entitlements.trial_eligible).toBe(false);
    expect(entitlements.trial_eligibility_reason).toBe(expectedCopy.trialReason);
  });

  test('Pro settings renders the expected migration state', async ({ page }) => {
    await ensureAuthenticated(page);
    await page.goto('/settings/system-pro', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings/, { timeout: 10_000 });

    await expect(page.getByRole('heading', { name: /pro license/i })).toBeVisible();
    await expect(page.getByRole('heading', { name: /current license/i })).toBeVisible();

    if (successMode) {
      await expect(page.getByText(/v5 license migration pending/i)).toHaveCount(0);
      await expect(page.getByText(/v5 license migration needs attention/i)).toHaveCount(0);
      await expect(page.getByRole('button', { name: /start 14-day pro trial/i })).toHaveCount(0);
      await expect(page.locator('span').filter({ hasText: /^Active$/ }).first()).toBeVisible();
      await expectFieldValue(expectFieldLocator(page, 'Tier'), expectedTierLabelFor(expectedTier));
      await expectFieldValue(expectFieldLocator(page, 'Licensed Email'), expectedLicensedEmail);
      await expectFieldValue(expectFieldLocator(page, 'Plan Terms'), formatTitleCase(expectedPlanVersion));
      if (expectedIsLifetime === 'true') {
        await expectFieldValue(expectFieldLocator(page, 'Expires'), 'Never (Lifetime)');
      }
      if (expectedMaxAgents > 0) {
        await expectFieldValue(expectFieldLocator(page, 'Max Agents'), String(expectedMaxAgents));
      }
      return;
    }

    const expectedCopy = expectedCopyFor(expectedState, expectedReason, expectedAction);
    await expect(page.getByText(expectedCopy.title)).toBeVisible();
    for (const fragment of expectedCopy.bodyFragments) {
      await expect(page.getByText(fragment)).toBeVisible();
    }
    await expect(page.getByRole('button', { name: /start 14-day pro trial/i })).toHaveCount(0);
  });
});
