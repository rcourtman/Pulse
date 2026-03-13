import { createHmac, randomBytes } from 'node:crypto';
import { expect, request as playwrightRequest, type APIRequestContext } from '@playwright/test';

export function webhookSignature(secret: string, payload: string) {
  const ts = Math.floor(Date.now() / 1000);
  const signedPayload = `${ts}.${payload}`;
  const digest = createHmac('sha256', secret).update(signedPayload, 'utf8').digest('hex');
  return `t=${ts},v1=${digest}`;
}

export async function sendStripeWebhook(
  request: APIRequestContext,
  baseURL: string,
  webhookSecret: string,
  eventType: string,
  objectPayload: Record<string, unknown>,
  options: { path?: string; isolated?: boolean } = {},
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
  const webhookPath = options.path?.trim() || '/api/stripe/webhook';
  const useIsolatedContext = options.isolated !== false;
  const webhookRequest = useIsolatedContext
    ? await playwrightRequest.newContext({ baseURL })
    : request;
  try {
    const response = await webhookRequest.post(
      useIsolatedContext ? webhookPath : `${baseURL}${webhookPath}`,
      {
        headers: {
          'Content-Type': 'application/json',
          'Stripe-Signature': webhookSignature(webhookSecret, body),
        },
        data: body,
      },
    );
    expect(response.ok(), `Webhook ${eventType} failed: HTTP ${response.status()}`).toBeTruthy();
  } finally {
    if (useIsolatedContext) {
      await webhookRequest.dispose();
    }
  }
}
