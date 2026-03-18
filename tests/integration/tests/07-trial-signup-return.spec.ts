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
    expect(startRes.status(), `trial start failed: HTTP ${startRes.status()}`).toBe(409);

    const startPayload = (await startRes.json()) as TrialStartPayload;
    expect(startPayload.code).toBe('trial_signup_required');

    const actionUrl = startPayload.details?.action_url ?? '';
    expect(actionUrl).toContain('/start-pro-trial');
    const parsedActionUrl = new URL(actionUrl);
    expect(parsedActionUrl.searchParams.get('org_id')).toBe('default');
    expect(parsedActionUrl.searchParams.get('return_url')).toContain('/auth/trial-activate');

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
    await expect(page.getByRole('heading', { name: 'Current License' })).toBeVisible();
    await expect(page.getByText(/No Pro license is active/i)).toBeVisible();

    // Verify duplicate initiation is rate limited.
    const secondRes = await apiRequest(page, '/api/license/trial/start', {
      method: 'POST',
    });
    expect(secondRes.status()).toBe(429);
    const secondPayload = (await secondRes.json()) as TrialStartPayload;
    expect(secondPayload.code).toBe('trial_rate_limited');
  });
});
