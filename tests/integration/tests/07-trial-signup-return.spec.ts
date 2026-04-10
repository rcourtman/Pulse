import { test, expect } from '@playwright/test';
import { apiRequest, ensureAuthenticated } from './helpers';

type EntitlementPayload = {
  subscription_state?: string;
  tier?: string;
  trial_eligible?: boolean;
  trial_days_remaining?: number;
  valid?: boolean;
  is_lifetime?: boolean;
};

type TrialStartPayload = {
  code?: string;
  details?: Record<string, string>;
};

function expectTrialRateLimited(payload: TrialStartPayload, retryAfterHeader?: string | null) {
  expect(payload.code).toBe('trial_rate_limited');
  const retryAfterSeconds = Number.parseInt(payload.details?.retry_after_seconds || '', 10);
  expect(retryAfterSeconds).toBeGreaterThan(0);
  if (retryAfterHeader) {
    expect(String(retryAfterSeconds)).toBe(retryAfterHeader);
  }
}

function expectHostedTrialRedirect(payload: TrialStartPayload) {
  expect(payload.code).toBe('trial_signup_required');

  const actionUrl = payload.details?.action_url ?? '';
  expect(actionUrl).toContain('/start-pro-trial');
  const parsedActionUrl = new URL(actionUrl);
  expect(parsedActionUrl.searchParams.get('org_id')).toBe('default');
  expect(parsedActionUrl.searchParams.get('return_url')).toContain('/auth/trial-activate');
}

test.describe.serial('Trial signup return flow', () => {
  test('initiates hosted trial signup and preserves local entitlements until activation', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only trial workflow coverage');

    await ensureAuthenticated(page);

    const preRes = await apiRequest(page, '/api/license/entitlements');
    expect(preRes.ok(), `entitlements pre-check failed: HTTP ${preRes.status()}`).toBeTruthy();
    const pre = (await preRes.json()) as EntitlementPayload;
    test.skip(
      pre.trial_eligible !== true,
      `Skipping trial flow because trial_eligible is ${String(pre.trial_eligible)} in this environment.`,
    );

    expect(
      pre.trial_eligible,
      'Expected trial_eligible=true before test.',
    ).toBe(true);

    // Start hosted trial via API.
    const startRes = await apiRequest(page, '/api/license/trial/start', {
      method: 'POST',
    });
    expect(
      [409, 429],
      `trial start failed: expected 409 or 429, got HTTP ${startRes.status()}`,
    ).toContain(startRes.status());

    const startPayload = (await startRes.json()) as TrialStartPayload;
    if (startRes.status() === 409) {
      expectHostedTrialRedirect(startPayload);
    } else {
      expectTrialRateLimited(startPayload, startRes.headers()['retry-after'] ?? null);
    }

    // Verify entitlements remain unchanged until the hosted flow returns.
    const postRes = await apiRequest(page, '/api/license/entitlements');
    expect(postRes.ok(), `entitlements post-check failed: HTTP ${postRes.status()}`).toBeTruthy();
    const post = (await postRes.json()) as EntitlementPayload;
    expect(post.subscription_state).toBe('expired');
    expect(post.tier).toBe('free');
    expect(post.valid).toBe(false);
    expect(post.is_lifetime ?? false).toBe(false);
    expect(post.trial_eligible).toBe(true);

    // Verify UI still reflects the unactivated local state.
    await page.goto('/settings');
    await page.getByRole('button', { name: /pulse pro/i }).first().click();
    await expect(page.getByRole('heading', { name: 'Pulse Pro' }).first()).toBeVisible();
    await expect(page.getByText(/No Pro license is active/i)).toBeVisible();

    // Verify duplicate initiation stays on the owned retry-burst contract.
    const secondRes = await apiRequest(page, '/api/license/trial/start', {
      method: 'POST',
    });
    const secondPayload = (await secondRes.json()) as TrialStartPayload;
    if (startRes.status() === 429) {
      expect(secondRes.status()).toBe(429);
      expectTrialRateLimited(secondPayload, secondRes.headers()['retry-after'] ?? null);
    } else if (secondRes.status() === 409) {
      expectHostedTrialRedirect(secondPayload);
    } else {
      expect(secondRes.status()).toBe(429);
      expectTrialRateLimited(secondPayload, secondRes.headers()['retry-after'] ?? null);
    }
  });
});
