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
    first_failed_at?: number;
  };
};

type ExpectedCopy = {
  title: RegExp;
  bodyFragments: RegExp[];
};

type MigrationFixture = {
  state: string;
  reason: string;
  recommended_action: string;
  first_failed_at?: number;
};

const expectedState = process.env.PULSE_E2E_EXPECT_COMMERCIAL_MIGRATION_STATE || '';
const expectedReason = process.env.PULSE_E2E_EXPECT_COMMERCIAL_MIGRATION_REASON || '';
const expectedAction = process.env.PULSE_E2E_EXPECT_COMMERCIAL_MIGRATION_ACTION || '';
const expectedTier = process.env.PULSE_E2E_EXPECT_TIER || '';
const expectedPlanVersion = process.env.PULSE_E2E_EXPECT_PLAN_VERSION || '';
const expectedLicensedEmail = process.env.PULSE_E2E_EXPECT_LICENSED_EMAIL || '';
const expectedMaxMonitoredSystems = Number(process.env.PULSE_E2E_EXPECT_MAX_AGENTS || '0');
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
      ? /retry from this instance/i
      : action === 'use_v6_activation_key'
        ? /use the current v6 key for this purchase/i
        : action === 'enter_supported_v5_key'
          ? /retry with the original v5 pro\/lifetime key from this instance/i
          : action === 'free_installation_slot'
            ? /support@pulserelay\.pro/i
            : action === 'retrieve_current_key'
              ? /pulserelay\.pro\/retrieve-license/i
              : action === 'allow_license_egress'
                ? /allow outbound HTTPS to license\.pulserelay\.pro/i
                : /review the plan state from this instance/i;

  if (state === 'pending') {
    const reasonFragment =
      reason === 'exchange_rate_limited'
        ? /rate-limited right now/i
        : reason === 'exchange_conflict'
          ? /another v6 license handoff is still settling/i
          : reason === 'exchange_connectivity_required'
            ? /paid v6 features require periodic outbound HTTPS/i
            : /automatic v6 exchange did not complete yet/i;

    return {
      title: /v5 license migration pending/i,
      bodyFragments: [reasonFragment, actionFragment],
    };
  }

  const reasonFragment =
    reason === 'exchange_installation_limit'
      ? /maximum number of v6 installations/i
      : reason === 'exchange_invalid'
        ? /key was rejected during v6 migration/i
        : reason === 'exchange_stale_key'
          ? /superseded by a renewal/i
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
    bodyFragments: [reasonFragment, actionFragment],
  };
}

function freeEntitlementsWithMigration(migration: MigrationFixture): EntitlementPayload {
  return {
    valid: false,
    tier: 'free',
    plan_version: 'community',
    subscription_state: 'expired',
    trial_eligible: false,
    trial_eligibility_reason: '',
    limits: [],
    commercial_migration: migration,
  };
}

async function stubCommercialMigrationFixture(page: Page, migration: MigrationFixture) {
  const entitlements = freeEntitlementsWithMigration(migration);
  const commercialPosture = {
    ...entitlements,
    upgrade_reasons: [],
    has_migration_gap: true,
  };

  await page.route('**/api/license/runtime-capabilities', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        capabilities: [],
        limits: [],
        hosted_mode: false,
        max_history_days: 7,
        runtime: {
          build: 'community',
          label: 'Pulse Community runtime',
        },
      }),
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

async function expectMigrationNotice(page: Page, migration: MigrationFixture) {
  await page.goto('/settings/pulse-intelligence/billing/plan', { waitUntil: 'domcontentloaded' });
  await page.waitForURL(/\/settings/, { timeout: 10_000 });
  await expect(page.getByRole('heading', { name: /plans & billing/i }).first()).toBeVisible();

  const expectedCopy = expectedCopyFor(
    migration.state,
    migration.reason,
    migration.recommended_action,
  );
  // Scope to the settings content: the global CommercialMigrationBanner
  // renders the same title outside <main>, which trips strict mode.
  const settingsContent = page.getByRole('main');
  await expect(settingsContent.getByText(expectedCopy.title)).toBeVisible();
  for (const fragment of expectedCopy.bodyFragments) {
    await expect(settingsContent.getByText(fragment)).toBeVisible();
  }
  await expect(page.getByRole('button', { name: /start 14-day pro trial/i })).toHaveCount(0);
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
      const monitoredSystemLimit = entitlements.limits?.find(
        (limit) => limit.key === 'max_monitored_systems',
      );
      if (expectedIsLifetime === 'true') {
        expect(monitoredSystemLimit).toBeUndefined();
      } else if (expectedMaxMonitoredSystems > 0) {
        expect(monitoredSystemLimit?.limit).toBe(expectedMaxMonitoredSystems);
      }
      return;
    }

    const expectedCopy = expectedCopyFor(expectedState, expectedReason, expectedAction);
    expect(entitlements.commercial_migration?.state).toBe(expectedState);
    expect(entitlements.commercial_migration?.reason).toBe(expectedReason);
    expect(entitlements.commercial_migration?.recommended_action).toBe(expectedAction);
    expect(entitlements.trial_eligible).toBe(false);
    expect(entitlements.trial_eligibility_reason || '').toBe('');
  });

  test('Pro settings renders the expected migration state', async ({ page }) => {
    await ensureAuthenticated(page);
    await page.goto('/settings/pulse-intelligence/billing/plan', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings/, { timeout: 10_000 });

    await expect(page.getByRole('heading', { name: /plans & billing/i }).first()).toBeVisible();

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
        await expectFieldValue(expectFieldLocator(page, 'Core Monitoring'), 'Unlimited');
      } else if (expectedMaxMonitoredSystems > 0) {
        await expectFieldValue(
          expectFieldLocator(page, 'Included Monitored Systems'),
          String(expectedMaxMonitoredSystems),
        );
      } else {
        await expectFieldValue(expectFieldLocator(page, 'Core Monitoring'), 'Unlimited');
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

test.describe('v5 commercial migration notice fixtures', () => {
  test('renders a stale renewed-key failure with retrieve-license guidance', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only migration UI coverage');

    const migration = {
      state: 'failed',
      reason: 'exchange_stale_key',
      recommended_action: 'retrieve_current_key',
    };
    await stubCommercialMigrationFixture(page, migration);
    await ensureAuthenticated(page);

    await expectMigrationNotice(page, migration);
  });

  test('renders a sustained license-server egress requirement without offering a trial', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only migration UI coverage');

    const migration = {
      state: 'pending',
      reason: 'exchange_connectivity_required',
      recommended_action: 'allow_license_egress',
      first_failed_at: Math.floor(Date.now() / 1000) - 90_000,
    };
    await stubCommercialMigrationFixture(page, migration);
    await ensureAuthenticated(page);

    await expectMigrationNotice(page, migration);
  });
});
