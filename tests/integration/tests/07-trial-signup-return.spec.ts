import { test, expect } from '@playwright/test';
import { apiRequest, ensureAuthenticated } from './helpers';
import { completeStripeSandboxCheckout } from './stripe-sandbox';

type EntitlementPayload = {
  subscription_state?: string;
  tier?: string;
  trial_eligible?: boolean;
  trial_days_remaining?: number;
  valid?: boolean;
  is_lifetime?: boolean;
};

function trialIdentity() {
  const suffix = `${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`;
  const domain = (process.env.PULSE_E2E_TRIAL_EMAIL_DOMAIN || 'example.com').trim() || 'example.com';
  return {
    name: `Trial E2E ${suffix}`,
    email: `trial-e2e-${suffix}@${domain}`,
    company: 'Pulse E2E',
  };
}

test.describe.serial('Trial signup return flow', () => {
  test('completes hosted signup via Stripe sandbox and activates real trial', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only trial workflow coverage');

    await ensureAuthenticated(page);

    const preRes = await apiRequest(page, '/api/license/entitlements');
    expect(preRes.ok(), `entitlements pre-check failed: HTTP ${preRes.status()}`).toBeTruthy();
    const pre = (await preRes.json()) as EntitlementPayload;
    expect(
      pre.trial_eligible,
      'Expected trial_eligible=true before test. Reset snapshot baseline before rerun.',
    ).toBe(true);

    await page.goto('/settings');
    await page.getByRole('button', { name: /pulse pro/i }).first().click();
    await expect(page.getByRole('heading', { name: 'Current License' })).toBeVisible();

    const refreshButton = page.getByRole('button', { name: /refresh/i }).first();
    if (await refreshButton.isVisible({ timeout: 2_000 }).catch(() => false)) {
      await refreshButton.click();
    }

    const startTrialButton = page.getByRole('button', { name: /start.*trial/i }).first();
    await expect(startTrialButton).toBeVisible();
    await startTrialButton.click();

    await page.waitForURL(/\/start-pro-trial\?/, { timeout: 30_000 });

    const identity = trialIdentity();
    await page.locator('#name').fill(identity.name);
    await page.locator('#email').fill(identity.email);
    await page.locator('#company').fill(identity.company);
    await page.getByRole('button', { name: /continue to secure checkout/i }).click();

    await completeStripeSandboxCheckout(page, {
      email: identity.email,
      cardholderName: identity.name,
    });

    await page.waitForURL(/\/settings\?trial=activated/, { timeout: 120_000 });

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
  });
});
