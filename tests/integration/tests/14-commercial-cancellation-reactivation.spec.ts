import { writeFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';

import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

import { apiRequest, ensureAuthenticated, navigateToSettings } from './helpers';
import { completeStripeSandboxCheckout } from './stripe-sandbox';
import { sendStripeWebhook } from './stripe-webhooks';

type StripeSubscription = {
  id: string;
  customer?: string;
  status?: string;
  metadata?: Record<string, string>;
  cancel_at_period_end?: boolean;
  canceled_at?: number | null;
  current_period_end?: number | null;
  test_clock?: string | { id?: string } | null;
  items?: {
    data?: Array<{ price?: { id?: string; metadata?: Record<string, string> } }>;
  };
};

type StripePortalSession = {
  id: string;
  url: string;
};

type StripeTestClock = {
  id?: string;
  status?: string;
  frozen_time?: number;
};

type CheckoutCreateResponse = {
  url?: string;
  plan_key?: string;
  tier?: string;
  billing_cycle?: string;
};

type CheckoutFulfillmentResponse = {
  status?: string;
  plan_key?: string;
  owner_email?: string;
  checkout_status?: string;
  payment_status?: string;
};

type EntitlementPayload = {
  subscription_state?: string;
  plan_version?: string;
};

type BillingStatePayload = {
  capabilities?: string[];
  limits?: Record<string, number>;
  meters_enabled?: string[];
  plan_version?: string;
  subscription_state?: string;
  stripe_customer_id?: string;
  stripe_subscription_id?: string;
  stripe_price_id?: string;
};

const DEFAULT_PUBLIC_V6_MONTHLY_PLAN_KEY = 'price_1T47OVBrHBocJIGHg4sMHMV7';
const DEFAULT_PUBLIC_V6_ANNUAL_PLAN_KEY = 'price_1T47OVBrHBocJIGHQv65Mrkb';

function requiredEnv(name: string): string {
  const value = (process.env[name] || '').trim();
  if (value === '') {
    throw new Error(`Missing required env var: ${name}`);
  }
  return value;
}

function commercialBaseURL(): string {
  const base =
    process.env.PULSE_COMMERCIAL_BASE_URL ||
    process.env.PULSE_BASE_URL ||
    process.env.PLAYWRIGHT_BASE_URL ||
    'http://localhost:7655';
  return base.replace(/\/+$/, '');
}

function commercialWebhookBaseURL(): string {
  const raw =
    process.env.PULSE_CCR_WEBHOOK_BASE_URL ||
    process.env.PULSE_COMMERCIAL_WEBHOOK_BASE_URL ||
    commercialBaseURL();
  return raw.replace(/\/+$/, '');
}

function commercialWebhookPath(): string {
  return (process.env.PULSE_CCR_WEBHOOK_PATH || '/api/stripe/webhook').trim() || '/api/stripe/webhook';
}

function commercialOrgID(): string {
  return (process.env.PULSE_CCR_ORG_ID || 'default').trim() || 'default';
}

function allowBillingStateSeed(): boolean {
  return /^(1|true|yes)$/i.test((process.env.PULSE_CCR_ALLOW_BILLING_STATE_SEED || '').trim());
}

function entitlementWriteCommand(): string {
  return (process.env.PULSE_E2E_ENTITLEMENT_WRITE_COMMAND || '').trim();
}

function entitlementBillingStatePath(): string {
  return (process.env.PULSE_E2E_BILLING_STATE_PATH || '').trim();
}

function checkoutBaseURL(): string {
  return (
    process.env.PULSE_CCR_CHECKOUT_BASE_URL ||
    process.env.PULSE_LICENSE_SERVER_URL ||
    'https://license.pulserelay.pro'
  ).replace(/\/+$/, '');
}

function checkoutResultBaseURL(): string {
  return (process.env.PULSE_CCR_CHECKOUT_RESULT_BASE_URL || checkoutBaseURL()).replace(/\/+$/, '');
}

function commercialLandingBaseURL(): string {
  return (process.env.PULSE_CCR_LANDING_BASE_URL || 'https://pulserelay.pro').replace(/\/+$/, '');
}

function successURL(baseURL: string): string {
  return (
    process.env.PULSE_CCR_SUCCESS_URL ||
    `${baseURL}/thanks.html?session_id={CHECKOUT_SESSION_ID}`
  ).trim();
}

function cancelURL(baseURL: string): string {
  return (
    process.env.PULSE_CCR_CANCEL_URL ||
    `${baseURL}/index.html?checkout=cancelled#pricing`
  ).trim();
}

function captureSessionID(url: string): string | null {
  return url.match(/\/(cs_[^/?#]+)/)?.[1] ?? null;
}

async function stripeRequest<T>(
  request: APIRequestContext,
  secretKey: string,
  path: string,
  init: {
    method?: 'GET' | 'POST' | 'DELETE';
    form?: Record<string, string>;
  } = {},
): Promise<T> {
  const url = `https://api.stripe.com${path}`;
  const response = await request.fetch(url, {
    method: init.method || 'GET',
    headers: {
      Authorization: `Bearer ${secretKey}`,
      ...(init.form ? { 'Content-Type': 'application/x-www-form-urlencoded' } : {}),
      Accept: 'application/json',
    },
    data: init.form ? new URLSearchParams(init.form).toString() : undefined,
  });
  const payload = (await response.json().catch(() => ({}))) as Record<string, unknown>;
  if (!response.ok()) {
    const message =
      ((payload.error as Record<string, unknown> | undefined)?.message as string | undefined) ||
      `Stripe API error (${response.status()})`;
    throw new Error(message);
  }
  return payload as T;
}

async function fetchSubscription(
  request: APIRequestContext,
  stripeSecretKey: string,
  subscriptionID: string,
): Promise<StripeSubscription> {
  return stripeRequest<StripeSubscription>(
    request,
    stripeSecretKey,
    `/v1/subscriptions/${encodeURIComponent(subscriptionID)}`,
  );
}

function subscriptionObjectForWebhook(subscription: StripeSubscription): Record<string, unknown> {
  return {
    id: subscription.id,
    customer:
      typeof subscription.customer === 'string'
        ? subscription.customer.trim()
        : subscription.customer,
    status: subscription.status || '',
    cancel_at_period_end: Boolean(subscription.cancel_at_period_end),
    canceled_at:
      typeof subscription.canceled_at === 'number' ? subscription.canceled_at : null,
    current_period_end:
      typeof subscription.current_period_end === 'number'
        ? subscription.current_period_end
        : null,
    test_clock:
      typeof subscription.test_clock === 'string'
        ? subscription.test_clock
        : subscription.test_clock?.id || null,
    items: subscription.items || { data: [] },
    metadata: subscription.metadata || {},
  };
}

async function replaySubscriptionStateIntoPulse(
  page: Page,
  stripeSecretKey: string,
  webhookSecret: string,
  subscriptionID: string,
  eventType: 'customer.subscription.updated' | 'customer.subscription.deleted',
) {
  const subscription = await fetchSubscription(page.request, stripeSecretKey, subscriptionID);
  await sendStripeWebhook(
    page.request,
    commercialWebhookBaseURL(),
    webhookSecret,
    eventType,
    subscriptionObjectForWebhook(subscription),
    { path: commercialWebhookPath() },
  );
}

async function fetchBillingState(page: Page, orgID: string): Promise<BillingStatePayload> {
  const response = await apiRequest(page, `/api/admin/orgs/${encodeURIComponent(orgID)}/billing-state`);
  expect(response.ok(), `GET billing state failed: HTTP ${response.status()}`).toBeTruthy();
  return (await response.json()) as BillingStatePayload;
}

async function putBillingState(page: Page, orgID: string, payload: BillingStatePayload) {
  const response = await apiRequest(page, `/api/admin/orgs/${encodeURIComponent(orgID)}/billing-state`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    data: payload,
  });
  expect(response.ok(), `PUT billing state failed: HTTP ${response.status()}`).toBeTruthy();
}

async function seedStripeCustomerMapping(
  page: Page,
  orgID: string,
  customerID: string,
  subscriptionID: string,
  priceID: string,
) {
  if (!allowBillingStateSeed()) {
    return;
  }
  const seededStateFromCurrent = async (current: BillingStatePayload) => ({
    capabilities: current.capabilities || [],
    limits: current.limits || {},
    meters_enabled: current.meters_enabled || [],
    plan_version: current.plan_version || 'expired',
    subscription_state: current.subscription_state || 'expired',
    stripe_customer_id: customerID,
    stripe_subscription_id: subscriptionID,
    stripe_price_id: priceID,
  });

  const writeCommand = entitlementWriteCommand();
  const billingPath = entitlementBillingStatePath();
  let current: BillingStatePayload | null = null;
  try {
    current = await fetchBillingState(page, orgID);
  } catch (error) {
    if (writeCommand === '' && billingPath === '') {
      throw error;
    }
  }

  const payload = JSON.stringify(
    await seededStateFromCurrent(current || {
      capabilities: [],
      limits: {},
      meters_enabled: [],
      plan_version: 'expired',
      subscription_state: 'expired',
    }),
    null,
    2,
  ) + '\n';

  if (current) {
    await putBillingState(page, orgID, JSON.parse(payload) as BillingStatePayload);
    return;
  }

  if (billingPath !== '') {
    writeFileSync(billingPath, payload, 'utf8');
    return;
  }

  if (writeCommand !== '') {
    const result = spawnSync('sh', ['-lc', writeCommand], {
      input: payload,
      encoding: 'utf8',
    });
    if (result.status !== 0) {
      throw new Error(
        `billing state write command failed: ${result.stderr || result.stdout || `exit ${result.status}`}`,
      );
    }
    return;
  }
}

async function advanceTestClock(
  request: APIRequestContext,
  stripeSecretKey: string,
  testClockID: string,
  frozenTime: number,
) {
  await stripeRequest<Record<string, unknown>>(
    request,
    stripeSecretKey,
    `/v1/test_helpers/test_clocks/${encodeURIComponent(testClockID)}/advance`,
    {
      method: 'POST',
      form: { frozen_time: String(frozenTime) },
    },
  );

  await expect
    .poll(async () => {
      const clock = await stripeRequest<StripeTestClock>(
        request,
        stripeSecretKey,
        `/v1/test_helpers/test_clocks/${encodeURIComponent(testClockID)}`,
      );
      return clock.status || '';
    }, { timeout: 120_000 })
    .toBe('ready');
}

async function createBillingPortalSession(
  request: APIRequestContext,
  stripeSecretKey: string,
  customerID: string,
): Promise<StripePortalSession> {
  return stripeRequest<StripePortalSession>(request, stripeSecretKey, '/v1/billing_portal/sessions', {
    method: 'POST',
    form: {
      customer: customerID,
      return_url: `${commercialLandingBaseURL()}/manage.html`,
    },
  });
}

async function createCheckoutSession(
  request: APIRequestContext,
  planKey: string,
): Promise<CheckoutCreateResponse> {
  const response = await request.post(`${checkoutBaseURL()}/v1/checkout/session`, {
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
      Origin: commercialLandingBaseURL(),
    },
    data: {
      plan_key: planKey,
      success_url: successURL(commercialLandingBaseURL()),
      cancel_url: cancelURL(commercialLandingBaseURL()),
    },
  });
  expect(response.ok(), `checkout session creation failed: HTTP ${response.status()}`).toBeTruthy();
  return (await response.json()) as CheckoutCreateResponse;
}

async function fetchCheckoutFulfillment(
  request: APIRequestContext,
  sessionID: string,
): Promise<CheckoutFulfillmentResponse> {
  const response = await request.get(
    `${checkoutResultBaseURL()}/v1/checkout/session?session_id=${encodeURIComponent(sessionID)}`,
    {
      headers: {
        Accept: 'application/json',
        Origin: commercialLandingBaseURL(),
      },
    },
  );
  expect(response.ok(), `checkout result fetch failed: HTTP ${response.status()}`).toBeTruthy();
  return (await response.json()) as CheckoutFulfillmentResponse;
}

async function openPulseProPanel(page: Page) {
  await navigateToSettings(page);
  await page.getByRole('button', { name: /pulse pro/i }).first().click();
  await expect(page.getByRole('heading', { name: /pro license/i })).toBeVisible();
}

async function fetchEntitlements(page: Page): Promise<EntitlementPayload> {
  const response = await apiRequest(page, '/api/license/entitlements');
  expect(response.ok(), `GET /api/license/entitlements failed: ${response.status()}`).toBeTruthy();
  return (await response.json()) as EntitlementPayload;
}

async function expectGrandfatheredState(
  page: Page,
  expectedPlanVersion: string,
  expectedPriceID: string,
) {
  const entitlements = await fetchEntitlements(page);
  expect(entitlements.subscription_state).toBe('active');
  expect(entitlements.plan_version).toBe(expectedPlanVersion);
  await expect(page.getByText(/grandfathered v5 pricing/i)).toBeVisible();
  await expect(page.getByText(new RegExp(expectedPriceID.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))).toHaveCount(0);
}

async function clickFirstVisible(page: Page, patterns: RegExp[], timeoutMs = 20_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    for (const pattern of patterns) {
      const button = page.getByRole('button', { name: pattern }).first();
      if (await button.isVisible().catch(() => false)) {
        await button.click();
        return true;
      }
      const link = page.getByRole('link', { name: pattern }).first();
      if (await link.isVisible().catch(() => false)) {
        await link.click();
        return true;
      }
    }
    await page.waitForTimeout(500);
  }
  return false;
}

async function portalScheduleCancellation(
  page: Page,
  request: APIRequestContext,
  stripeSecretKey: string,
  customerID: string,
  subscriptionID: string,
) {
  const portal = await createBillingPortalSession(request, stripeSecretKey, customerID);
  await page.goto(portal.url);
  const clickedStart = await clickFirstVisible(page, [
    /cancel plan/i,
    /cancel subscription/i,
    /cancel$/i,
  ]);
  expect(clickedStart, 'expected to find a cancel action in Stripe billing portal').toBeTruthy();

  await clickFirstVisible(page, [
    /cancel at period end/i,
    /end of billing period/i,
    /keep until period end/i,
    /continue/i,
  ]);
  await clickFirstVisible(page, [
    /confirm/i,
    /cancel subscription/i,
    /cancel plan/i,
    /confirm cancellation/i,
  ]);

  await expect
    .poll(async () => {
      const subscription = await fetchSubscription(request, stripeSecretKey, subscriptionID);
      return subscription.cancel_at_period_end === true;
    }, { timeout: 60_000 })
    .toBe(true);
}

async function portalResumeCancellation(
  page: Page,
  request: APIRequestContext,
  stripeSecretKey: string,
  customerID: string,
  subscriptionID: string,
) {
  const portal = await createBillingPortalSession(request, stripeSecretKey, customerID);
  await page.goto(portal.url);
  const clickedResume = await clickFirstVisible(page, [
    /don't cancel subscription/i,
    /resume subscription/i,
    /resume plan/i,
    /reactivate/i,
    /keep subscription/i,
  ]);
  expect(clickedResume, 'expected to find a resume action in Stripe billing portal').toBeTruthy();
  await clickFirstVisible(page, [/confirm/i, /resume/i, /renew subscription/i, /reactivate/i]);

  await expect
    .poll(async () => {
      const subscription = await fetchSubscription(request, stripeSecretKey, subscriptionID);
      return subscription.cancel_at_period_end === false;
    }, { timeout: 60_000 })
    .toBe(true);
}

test.describe.serial('Commercial cancellation/reactivation', () => {
  test.describe.configure({ timeout: 180_000 });

  test('monthly grandfathered continuity, cancel/resume, cancellation, and v6 re-entry', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only commercial coverage');

    const stripeSecretKey = requiredEnv('PULSE_E2E_STRIPE_API_KEY');
    const webhookSecret = requiredEnv('PULSE_E2E_STRIPE_WEBHOOK_SECRET');
    const customerID = requiredEnv('PULSE_CCR_MONTHLY_CUSTOMER_ID');
    const subscriptionID = requiredEnv('PULSE_CCR_MONTHLY_SUBSCRIPTION_ID');
    const legacyPriceID = requiredEnv('PULSE_CCR_MONTHLY_LEGACY_PRICE_ID');
    const testClockID = requiredEnv('PULSE_CCR_MONTHLY_TEST_CLOCK_ID');
    const v6PlanKey =
      process.env.PULSE_CCR_V6_MONTHLY_PLAN_KEY || DEFAULT_PUBLIC_V6_MONTHLY_PLAN_KEY;
    const returnerEmail = requiredEnv('PULSE_CCR_RETURNER_EMAIL');
    const baseURL = commercialBaseURL();
    const orgID = commercialOrgID();

    await ensureAuthenticated(page);
    await seedStripeCustomerMapping(page, orgID, customerID, subscriptionID, legacyPriceID);
    await replaySubscriptionStateIntoPulse(
      page,
      stripeSecretKey,
      webhookSecret,
      subscriptionID,
      'customer.subscription.updated',
    );

    await openPulseProPanel(page);

    const activeSubscription = await fetchSubscription(page.request, stripeSecretKey, subscriptionID);
    expect(activeSubscription.status).toMatch(/active|trialing|past_due/i);
    expect(activeSubscription.items?.data?.[0]?.price?.id).toBe(legacyPriceID);

    await expectGrandfatheredState(page, 'v5_pro_monthly_grandfathered', legacyPriceID);

    await portalScheduleCancellation(page, page.request, stripeSecretKey, customerID, subscriptionID);
    await replaySubscriptionStateIntoPulse(
      page,
      stripeSecretKey,
      webhookSecret,
      subscriptionID,
      'customer.subscription.updated',
    );

    let scheduledSubscription = await fetchSubscription(page.request, stripeSecretKey, subscriptionID);
    expect(scheduledSubscription.cancel_at_period_end).toBe(true);
    expect(scheduledSubscription.items?.data?.[0]?.price?.id).toBe(legacyPriceID);

    await openPulseProPanel(page);
    let entitlements = await fetchEntitlements(page);
    expect(entitlements.subscription_state).toBe('active');
    expect(entitlements.plan_version).toBe('v5_pro_monthly_grandfathered');
    await expect(page.getByText(/grandfathered v5 pricing/i)).toBeVisible();

    await portalResumeCancellation(page, page.request, stripeSecretKey, customerID, subscriptionID);
    await replaySubscriptionStateIntoPulse(
      page,
      stripeSecretKey,
      webhookSecret,
      subscriptionID,
      'customer.subscription.updated',
    );

    scheduledSubscription = await fetchSubscription(page.request, stripeSecretKey, subscriptionID);
    expect(scheduledSubscription.cancel_at_period_end).toBe(false);
    expect(scheduledSubscription.items?.data?.[0]?.price?.id).toBe(legacyPriceID);

    const currentPeriodEnd = scheduledSubscription.current_period_end;
    expect(typeof currentPeriodEnd).toBe('number');

    await portalScheduleCancellation(page, page.request, stripeSecretKey, customerID, subscriptionID);
    await replaySubscriptionStateIntoPulse(
      page,
      stripeSecretKey,
      webhookSecret,
      subscriptionID,
      'customer.subscription.updated',
    );
    await advanceTestClock(page.request, stripeSecretKey, testClockID, Number(currentPeriodEnd) + 3600);

    await expect
      .poll(async () => {
        const canceled = await fetchSubscription(page.request, stripeSecretKey, subscriptionID);
        return canceled.status;
      }, { timeout: 120_000 })
      .toMatch(/canceled|incomplete_expired|unpaid/);
    await replaySubscriptionStateIntoPulse(
      page,
      stripeSecretKey,
      webhookSecret,
      subscriptionID,
      'customer.subscription.deleted',
    );

    await openPulseProPanel(page);
    await expect
      .poll(async () => {
        const payload = await fetchEntitlements(page);
        return payload.subscription_state || '';
      }, { timeout: 60_000 })
      .toMatch(/canceled|expired|free/);
    await expect(page.getByText(/grandfathered v5 pricing/i)).toHaveCount(0);

    const checkout = await createCheckoutSession(page.request, v6PlanKey);
    expect(checkout.plan_key).toBe(v6PlanKey);
    expect(checkout.url, 'checkout session response missing url').toBeTruthy();

    await page.goto(String(checkout.url));
    const checkoutSessionID = captureSessionID(page.url());
    expect(checkoutSessionID).toBeTruthy();
    await completeStripeSandboxCheckout(page, {
      email: returnerEmail,
      cardholderName: 'Pulse Commercial Reentry',
    });
    await page.waitForURL(/session_id=/, { timeout: 120_000 });

    const fulfilledSessionID =
      new URL(page.url()).searchParams.get('session_id') || checkoutSessionID || '';
    const fulfillment = await fetchCheckoutFulfillment(page.request, fulfilledSessionID);
    expect(fulfillment.status).toBe('fulfilled');
    expect(fulfillment.plan_key).toBe(v6PlanKey);
    expect(fulfillment.plan_key).not.toMatch(/^v5_/);

    const legacyRejection = await page.request.post(`${checkoutBaseURL()}/v1/checkout/session`, {
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
        Origin: commercialLandingBaseURL(),
      },
      data: { plan_key: 'price_v5_pro_monthly' },
    });
    expect(legacyRejection.status()).toBe(400);
    await expect((await legacyRejection.text()).toLowerCase()).toContain('not a v6 checkout plan');

    await page.goto(baseURL);
  });

  test('annual grandfathered path matches monthly continuity boundary', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only commercial coverage');

    const stripeSecretKey = requiredEnv('PULSE_E2E_STRIPE_API_KEY');
    const webhookSecret = requiredEnv('PULSE_E2E_STRIPE_WEBHOOK_SECRET');
    const customerID = requiredEnv('PULSE_CCR_ANNUAL_CUSTOMER_ID');
    const subscriptionID = requiredEnv('PULSE_CCR_ANNUAL_SUBSCRIPTION_ID');
    const legacyPriceID = requiredEnv('PULSE_CCR_ANNUAL_LEGACY_PRICE_ID');
    const testClockID = requiredEnv('PULSE_CCR_ANNUAL_TEST_CLOCK_ID');
    const v6PlanKey =
      process.env.PULSE_CCR_V6_ANNUAL_PLAN_KEY || DEFAULT_PUBLIC_V6_ANNUAL_PLAN_KEY;
    const returnerEmail = requiredEnv('PULSE_CCR_RETURNER_EMAIL');
    const orgID = commercialOrgID();

    await ensureAuthenticated(page);
    await seedStripeCustomerMapping(page, orgID, customerID, subscriptionID, legacyPriceID);
    await replaySubscriptionStateIntoPulse(
      page,
      stripeSecretKey,
      webhookSecret,
      subscriptionID,
      'customer.subscription.updated',
    );
    await openPulseProPanel(page);

    const activeSubscription = await fetchSubscription(page.request, stripeSecretKey, subscriptionID);
    expect(activeSubscription.items?.data?.[0]?.price?.id).toBe(legacyPriceID);

    await portalScheduleCancellation(page, page.request, stripeSecretKey, customerID, subscriptionID);
    await replaySubscriptionStateIntoPulse(
      page,
      stripeSecretKey,
      webhookSecret,
      subscriptionID,
      'customer.subscription.updated',
    );
    const currentPeriodEnd = (await fetchSubscription(page.request, stripeSecretKey, subscriptionID)).current_period_end;
    expect(typeof currentPeriodEnd).toBe('number');
    await advanceTestClock(page.request, stripeSecretKey, testClockID, Number(currentPeriodEnd) + 3600);

    await expect
      .poll(async () => {
        const canceled = await fetchSubscription(page.request, stripeSecretKey, subscriptionID);
        return canceled.status;
      }, { timeout: 120_000 })
      .toMatch(/canceled|incomplete_expired|unpaid/);
    await replaySubscriptionStateIntoPulse(
      page,
      stripeSecretKey,
      webhookSecret,
      subscriptionID,
      'customer.subscription.deleted',
    );

    const checkout = await createCheckoutSession(page.request, v6PlanKey);
    expect(checkout.plan_key).toBe(v6PlanKey);
    await page.goto(String(checkout.url));
    const checkoutSessionID = captureSessionID(page.url());
    expect(checkoutSessionID).toBeTruthy();
    await completeStripeSandboxCheckout(page, {
      email: returnerEmail,
      cardholderName: 'Pulse Commercial Annual Reentry',
    });
    await page.waitForURL(/session_id=/, { timeout: 120_000 });

    const fulfilledSessionID =
      new URL(page.url()).searchParams.get('session_id') || checkoutSessionID || '';
    const fulfillment = await fetchCheckoutFulfillment(page.request, fulfilledSessionID);
    expect(fulfillment.status).toBe('fulfilled');
    expect(fulfillment.plan_key).toBe(v6PlanKey);
    expect(fulfillment.plan_key).not.toMatch(/^v5_/);
  });
});
