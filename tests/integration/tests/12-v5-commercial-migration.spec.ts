import { test, expect } from '@playwright/test';
import { apiRequest, ensureAuthenticated } from './helpers';

type EntitlementPayload = {
  trial_eligible?: boolean;
  trial_eligibility_reason?: string;
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
    test.skip(
      !expectedState || !expectedReason || !expectedAction,
      'Set PULSE_E2E_EXPECT_COMMERCIAL_MIGRATION_STATE/REASON/ACTION to enable migration UI checks',
    );
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only migration UI coverage');
  });

  test('entitlements expose the expected unresolved commercial migration state', async ({
    page,
  }) => {
    const expectedCopy = expectedCopyFor(expectedState, expectedReason, expectedAction);

    await ensureAuthenticated(page);

    const res = await apiRequest(page, '/api/license/entitlements');
    expect(res.ok(), `GET /api/license/entitlements failed: ${res.status()}`).toBeTruthy();

    const entitlements = (await res.json()) as EntitlementPayload;
    expect(entitlements.commercial_migration?.state).toBe(expectedState);
    expect(entitlements.commercial_migration?.reason).toBe(expectedReason);
    expect(entitlements.commercial_migration?.recommended_action).toBe(expectedAction);
    expect(entitlements.trial_eligible).toBe(false);
    expect(entitlements.trial_eligibility_reason).toBe(expectedCopy.trialReason);
  });

  test('Pro settings renders the migration notice and hides the trial CTA', async ({ page }) => {
    const expectedCopy = expectedCopyFor(expectedState, expectedReason, expectedAction);

    await ensureAuthenticated(page);
    await page.goto('/settings/system-pro', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings/, { timeout: 10_000 });

    await expect(page.getByRole('heading', { name: /pro license/i })).toBeVisible();
    await expect(page.getByText(expectedCopy.title)).toBeVisible();

    for (const fragment of expectedCopy.bodyFragments) {
      await expect(page.getByText(fragment)).toBeVisible();
    }

    await expect(page.getByRole('button', { name: /start 14-day pro trial/i })).toHaveCount(0);
  });
});
