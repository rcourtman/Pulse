import { createHmac, randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';
import { completeStripeSandboxCheckout } from './stripe-sandbox';

type Tenant = {
  id: string;
  email?: string;
  state?: string;
  stripe_customer_id?: string;
  stripe_subscription_id?: string;
};

type StripeSessionResponse = {
  id: string;
  customer?: string;
  customer_email?: string;
  metadata?: Record<string, string>;
  subscription?: string | { id?: string };
};

type StripeSubscriptionResponse = {
  id: string;
  customer?: string;
  status?: string;
  items?: {
    data?: Array<{ price?: { id?: string; metadata?: Record<string, string> } }>;
  };
  metadata?: Record<string, string>;
};

function cloudBaseURL(): string {
  const base =
    process.env.PULSE_CLOUD_BASE_URL ||
    process.env.PULSE_BASE_URL ||
    process.env.PLAYWRIGHT_BASE_URL ||
    'http://localhost:7655';
  return base.replace(/\/+$/, '');
}

function requiredEnv(name: string): string {
  const value = (process.env[name] || '').trim();
  if (value === '') {
    throw new Error(`Missing required env var: ${name}`);
  }
  return value;
}

function signupIdentity() {
  const suffix = `${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`;
  const domain = (process.env.PULSE_E2E_CLOUD_EMAIL_DOMAIN || 'example.com').trim() || 'example.com';
  return {
    email: `cloud-lifecycle-${suffix}@${domain}`,
    orgName: `Cloud Lifecycle ${suffix}`,
    cardholderName: `Cloud Lifecycle ${suffix}`,
  };
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

function webhookSignature(secret: string, payload: string) {
  const ts = Math.floor(Date.now() / 1000);
  const signedPayload = `${ts}.${payload}`;
  const digest = createHmac('sha256', secret).update(signedPayload, 'utf8').digest('hex');
  return `t=${ts},v1=${digest}`;
}

async function sendStripeWebhook(
  request: import('@playwright/test').APIRequestContext,
  baseURL: string,
  webhookSecret: string,
  eventType: string,
  objectPayload: Record<string, unknown>,
) {
  const eventPayload = {
    id: `evt_e2e_${Date.now()}_${randomBytes(4).toString('hex')}`,
    object: 'event',
    type: eventType,
    data: { object: objectPayload },
    created: Math.floor(Date.now() / 1000),
    livemode: false,
    pending_webhooks: 1,
  };
  const body = JSON.stringify(eventPayload);
  const response = await request.post(`${baseURL}/api/stripe/webhook`, {
    headers: {
      'Content-Type': 'application/json',
      'Stripe-Signature': webhookSignature(webhookSecret, body),
    },
    data: body,
  });
  expect(response.ok(), `Webhook ${eventType} failed: HTTP ${response.status()}`).toBeTruthy();
}

async function listTenants(
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
  expect(response.ok(), `List tenants failed: HTTP ${response.status()}`).toBeTruthy();
  const payload = (await response.json()) as { tenants?: Tenant[] };
  return Array.isArray(payload.tenants) ? payload.tenants : [];
}

test.describe.serial('Cloud billing lifecycle (post-checkout)', () => {
  test('provisions tenant and transitions to canceled after subscription deletion', async ({ page }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith('mobile-'),
      'Desktop-only cloud billing lifecycle coverage',
    );

    const baseURL = cloudBaseURL();
    const adminKey = requiredEnv('PULSE_CP_ADMIN_KEY');
    const stripeSecretKey = requiredEnv('PULSE_E2E_STRIPE_API_KEY');
    const webhookSecret = requiredEnv('PULSE_E2E_STRIPE_WEBHOOK_SECRET');

    const identity = signupIdentity();
    await page.goto(`${baseURL}/cloud/signup`);
    await expect(page.getByRole('heading', { name: /start pulse cloud|pulse cloud signup/i })).toBeVisible();

    await page.locator('#email').fill(identity.email);
    await page.locator('#org_name').fill(identity.orgName);
    await page.getByRole('button', { name: /continue to secure checkout/i }).click();

    const checkout = await completeStripeSandboxCheckout(page, {
      email: identity.email,
      cardholderName: identity.cardholderName,
    });
    await page.waitForURL(/\/(cloud\/)?signup\/complete/, { timeout: 120_000 });
    expect(checkout.checkoutSessionID).toBeTruthy();

    const session = await stripeRequest<StripeSessionResponse>(
      stripeSecretKey,
      `/v1/checkout/sessions/${encodeURIComponent(checkout.checkoutSessionID || '')}?expand[]=subscription`,
    );
    const customerID = (session.customer || '').trim();
    const subscriptionID =
      typeof session.subscription === 'string'
        ? session.subscription.trim()
        : (session.subscription?.id || '').trim();
    expect(customerID).toBeTruthy();
    expect(subscriptionID).toBeTruthy();

    const subscription = await stripeRequest<StripeSubscriptionResponse>(
      stripeSecretKey,
      `/v1/subscriptions/${encodeURIComponent(subscriptionID)}`,
    );

    await sendStripeWebhook(page.request, baseURL, webhookSecret, 'checkout.session.completed', {
      id: session.id,
      mode: 'subscription',
      customer: customerID,
      subscription: subscriptionID,
      customer_email: identity.email,
      customer_details: { email: identity.email },
      metadata: session.metadata || { account_display_name: identity.orgName },
    });

    let tenantID = '';
    await expect.poll(async () => {
      const tenants = await listTenants(page.request, baseURL, adminKey);
      const tenant = tenants.find((entry) => (entry.stripe_customer_id || '') === customerID);
      if (!tenant) {
        return '';
      }
      tenantID = tenant.id;
      return tenant.state || '';
    }, { timeout: 60_000 }).toBe('active');
    expect(tenantID).toBeTruthy();

    await stripeRequest<StripeSubscriptionResponse>(
      stripeSecretKey,
      `/v1/subscriptions/${encodeURIComponent(subscriptionID)}`,
      'DELETE',
    );

    await sendStripeWebhook(page.request, baseURL, webhookSecret, 'customer.subscription.deleted', {
      id: subscription.id,
      customer: customerID,
      status: 'canceled',
      items: subscription.items || { data: [] },
      metadata: subscription.metadata || {},
    });

    await expect.poll(async () => {
      const tenants = await listTenants(page.request, baseURL, adminKey);
      const tenant = tenants.find((entry) => entry.id === tenantID);
      return tenant?.state || '';
    }, { timeout: 60_000 }).toBe('canceled');
  });
});
