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

test.describe.serial('Retired self-hosted trial acquisition routes', () => {
  test('does not expose in-app trial acquisition and preserves local entitlements', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only commercial route coverage');

    await ensureAuthenticated(page);

    const preRes = await apiRequest(page, '/api/license/entitlements');
    expect(preRes.ok(), `entitlements pre-check failed: HTTP ${preRes.status()}`).toBeTruthy();
    const pre = (await preRes.json()) as EntitlementPayload;

    const startRes = await apiRequest(page, '/api/license/trial/start', {
      method: 'POST',
    });
    expect(startRes.status(), `retired trial start route must not be exposed: HTTP ${startRes.status()}`).toBe(404);

    const postRes = await apiRequest(page, '/api/license/entitlements');
    expect(postRes.ok(), `entitlements post-check failed: HTTP ${postRes.status()}`).toBeTruthy();
    const post = (await postRes.json()) as EntitlementPayload;
    expect(post.subscription_state).toBe(pre.subscription_state);
    expect(post.tier).toBe(pre.tier);
    expect(post.valid ?? false).toBe(pre.valid ?? false);
    expect(post.is_lifetime ?? false).toBe(pre.is_lifetime ?? false);
    expect(post.trial_days_remaining ?? 0).toBe(pre.trial_days_remaining ?? 0);
    expect(post.trial_eligible).toBe(pre.trial_eligible);
  });
});
