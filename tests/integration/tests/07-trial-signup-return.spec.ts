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

test.describe.serial('Trial signup return flow', () => {
  test('starts local trial without credit card and activates entitlements', async ({ page }, testInfo) => {
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

    // Start trial via API (no credit card required).
    const startRes = await apiRequest(page, '/api/license/trial/start', {
      method: 'POST',
    });
    expect(startRes.ok(), `trial start failed: HTTP ${startRes.status()}`).toBeTruthy();

    const startPayload = await startRes.json();
    expect(startPayload.subscription_state).toBe('trial');

    // Verify entitlements reflect active trial.
    await expect.poll(async () => {
      const res = await apiRequest(page, '/api/license/entitlements');
      if (!res.ok()) {
        return 0;
      }
      const payload = (await res.json()) as EntitlementPayload;
      return payload.trial_days_remaining ?? 0;
    }, { timeout: 30_000 }).toBeGreaterThan(0);

    const postRes = await apiRequest(page, '/api/license/entitlements');
    expect(postRes.ok(), `entitlements post-check failed: HTTP ${postRes.status()}`).toBeTruthy();
    const post = (await postRes.json()) as EntitlementPayload;
    expect(post.subscription_state).toBe('trial');
    expect(post.tier).toBe('pro');
    expect(post.valid).toBe(true);
    expect(post.is_lifetime ?? false).toBe(false);
    expect(post.trial_eligible).toBe(false);

    // Verify UI reflects trial state.
    await page.goto('/settings');
    await page.getByRole('button', { name: /pulse pro/i }).first().click();
    await expect(page.getByRole('heading', { name: 'Current License' })).toBeVisible();

    const expiresValue = page
      .locator('p')
      .filter({ hasText: /^Expires$/ })
      .first()
      .locator('xpath=following-sibling::p[1]');
    await expect(expiresValue).not.toHaveText(/Never/i);

    const daysRemainingValue = page
      .locator('p')
      .filter({ hasText: /^Days Remaining$/ })
      .first()
      .locator('xpath=following-sibling::p[1]');
    const daysRemainingText = (await daysRemainingValue.innerText()).trim();
    const daysRemaining = Number.parseInt(daysRemainingText, 10);
    expect(Number.isNaN(daysRemaining)).toBeFalsy();
    expect(daysRemaining).toBeGreaterThan(0);

    // Verify second trial start is rejected.
    const secondRes = await apiRequest(page, '/api/license/trial/start', {
      method: 'POST',
    });
    expect(secondRes.status()).toBe(409);
  });
});
