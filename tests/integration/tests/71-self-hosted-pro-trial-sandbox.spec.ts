import { expect, test } from '@playwright/test';
import { apiRequest, ensureAuthenticated } from './helpers';
import { completeStripeSandboxCheckout } from './stripe-sandbox';
import { preferredPlaywrightRouteBaseURL } from './runtime-defaults';

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

type Tenant = {
  id?: string;
  email?: string;
  state?: string;
};

type StripeSessionResponse = {
  id: string;
  subscription?: string | { id?: string };
};

function explicitCloudBaseURL(): string {
  return String(process.env.PULSE_CLOUD_BASE_URL || '').trim();
}

function cloudBaseURL(): string {
  return preferredPlaywrightRouteBaseURL(process.env, [
    explicitCloudBaseURL(),
  ]);
}

function requiredEnv(name: string): string {
  const value = String(process.env[name] || '').trim();
  if (value === '') {
    throw new Error(`Missing required env var: ${name}`);
  }
  return value;
}

function trialIdentity() {
  const suffix = `${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`;
  return {
    name: `Pulse Trial E2E ${suffix}`,
    email: `trial-${suffix}@customer-pulse-e2e.test`,
    company: `Pulse Trial E2E ${suffix}`,
  };
}

async function listControlPlaneTenants(
  request: import('@playwright/test').APIRequestContext,
  baseURL: string,
  adminKey: string,
): Promise<Tenant[]> {
  const response = await request.get(`${baseURL}/admin/tenants`, {
    headers: {
      'X-Admin-Key': adminKey,
      Accept: 'application/json',
    },
  });
  expect(response.ok(), `control-plane tenant list failed: HTTP ${response.status()}`).toBeTruthy();
  const payload = (await response.json()) as { tenants?: Tenant[] };
  return Array.isArray(payload.tenants) ? payload.tenants : [];
}

async function stripeRequest<T>(secretKey: string, path: string, method = 'GET') {
  const response = await fetch(`https://api.stripe.com${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${secretKey}`,
      Accept: 'application/json',
    },
  });
  const payload = (await response.json().catch(() => ({}))) as Record<string, unknown>;
  if (!response.ok) {
    const message =
      ((payload.error as Record<string, unknown> | undefined)?.message as string | undefined) ||
      `Stripe API error (${response.status})`;
    throw new Error(message);
  }
  return payload as T;
}

test.describe.serial('Self-hosted Pro trial Stripe sandbox activation', () => {
  test('completes checkout, returns to Pulse, and activates the local trial entitlement', async ({ page }, testInfo) => {
    test.setTimeout(180_000);
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only Stripe checkout workflow coverage');
    test.skip(
      explicitCloudBaseURL() === '',
      'Set PULSE_CLOUD_BASE_URL to run self-hosted Pro trial sandbox activation coverage.',
    );

    const requiredVars = ['PULSE_CP_ADMIN_KEY'] as const;
    const missingVars = requiredVars.filter((name) => String(process.env[name] || '').trim() === '');
    test.skip(missingVars.length > 0, `Missing required env var(s): ${missingVars.join(', ')}`);

    const cpBaseURL = cloudBaseURL();
    const cpAdminKey = requiredEnv('PULSE_CP_ADMIN_KEY');
    const identity = trialIdentity();
    const stripeSecretKey = String(process.env.PULSE_E2E_STRIPE_API_KEY || '').trim();
    let subscriptionID = '';

    try {
      await ensureAuthenticated(page);

      const preRes = await apiRequest(page, '/api/license/entitlements');
      expect(preRes.ok(), `entitlements pre-check failed: HTTP ${preRes.status()}`).toBeTruthy();
      const pre = (await preRes.json()) as EntitlementPayload;
      test.skip(
        pre.trial_eligible !== true,
        `Skipping full trial activation because trial_eligible is ${String(pre.trial_eligible)} in this environment.`,
      );
      expect(pre.tier).toBe('free');
      expect(pre.valid ?? false).toBe(false);
      expect(pre.is_lifetime ?? false).toBe(false);

      const startRes = await apiRequest(page, '/api/license/trial/start', {
        method: 'POST',
      });
      expect(startRes.status(), `trial start failed: HTTP ${startRes.status()}`).toBe(409);
      const startPayload = (await startRes.json()) as TrialStartPayload;
      expect(startPayload.code).toBe('trial_signup_required');

      const actionURL = startPayload.details?.action_url ?? '';
      expect(actionURL).toContain('/start-pro-trial');
      const parsedActionURL = new URL(actionURL);
      expect(parsedActionURL.origin).toBe(new URL(cpBaseURL).origin);
      expect(parsedActionURL.searchParams.get('return_url')).toContain('/auth/trial-activate');
      expect(parsedActionURL.searchParams.get('instance_token') || '').toMatch(/^tsi1_/);

      await page.goto(actionURL);
      await expect(page.getByRole('heading', { name: /start your 14-day pro trial/i })).toBeVisible();
      await page.locator('#name').fill(identity.name);
      await page.locator('#email').fill(identity.email);
      await page.locator('#company').fill(identity.company);
      await page.getByRole('button', { name: /continue to secure trial setup/i }).click();

      const checkout = await completeStripeSandboxCheckout(page, {
        email: identity.email,
        cardholderName: identity.name,
      });
      if (stripeSecretKey !== '' && checkout.checkoutSessionID) {
        const session = await stripeRequest<StripeSessionResponse>(
          stripeSecretKey,
          `/v1/checkout/sessions/${encodeURIComponent(checkout.checkoutSessionID)}?expand[]=subscription`,
        );
        subscriptionID = typeof session.subscription === 'string'
          ? session.subscription.trim()
          : (session.subscription?.id || '').trim();
      }

      await page.waitForURL(/\/trial-signup\/complete\?session_id=/, { timeout: 120_000 });
      await expect(page.getByRole('heading', { name: /trial entitlement ready/i })).toBeVisible();
      await page.getByRole('link', { name: /return to pulse/i }).click();
      await page.waitForURL(/\/settings\/system-pro\?trial=activated/, { timeout: 120_000 });

      const postRes = await apiRequest(page, '/api/license/entitlements');
      expect(postRes.ok(), `entitlements post-activation failed: HTTP ${postRes.status()}`).toBeTruthy();
      const post = (await postRes.json()) as EntitlementPayload;
      expect(post.subscription_state).toBe('trial');
      expect(post.trial_days_remaining ?? 0).toBeGreaterThan(0);
      expect(post.trial_eligible).toBe(false);

      const tenants = await listControlPlaneTenants(page.request, cpBaseURL, cpAdminKey);
      const matchingTenant = tenants.find((tenant) => tenant.email === identity.email);
      expect(
        matchingTenant,
        'self-hosted Pro trial checkout must not create a hosted Cloud tenant',
      ).toBeUndefined();
    } finally {
      if (stripeSecretKey !== '' && subscriptionID !== '') {
        await stripeRequest(stripeSecretKey, `/v1/subscriptions/${encodeURIComponent(subscriptionID)}`, 'DELETE')
          .catch((error) => {
            console.warn(`Stripe sandbox subscription cleanup failed: ${error instanceof Error ? error.message : String(error)}`);
          });
      }
    }
  });
});
