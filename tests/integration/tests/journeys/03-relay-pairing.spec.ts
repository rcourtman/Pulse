import { test, expect } from '@playwright/test';
import {
  ensureAuthenticated,
  apiRequest,
} from '../helpers';

/**
 * Journey: Relay Pairing → Mobile Connection → Live Data
 *
 * Covers the relay integration path:
 *   1. Check relay feature is available (requires Pro/relay license)
 *   2. Configure relay settings via server_url + enable
 *   3. Verify relay status connected (boolean)
 *   4. Generate onboarding QR code (schema, instance_url, relay, auth_token, deep_link)
 *   5. Verify deep link URL is available
 *   6. Verify relay settings page renders in UI
 *
 * This satisfies part of L12 score-3 criteria: "Relay pairing → mobile
 * connection → live data."
 *
 * NOTE: Full mobile-side validation (scan QR → connect → see live data)
 * requires the pulse-mobile app running against the relay server. This spec
 * validates the Pulse server-side relay setup and pairing readiness. The
 * mobile-side consumption is covered by the pulse-mobile E2E suite.
 *
 * Environment variables:
 *   PULSE_E2E_RELAY_HOST - Relay server hostname (required, skip if absent)
 *   PULSE_E2E_RELAY_PORT - Relay server port (optional, default: 443)
 *   PULSE_E2E_RELAY_HTTPS - Use HTTPS for relay (optional, default: "true")
 */

const RELAY_HOST = process.env.PULSE_E2E_RELAY_HOST || '';
const RELAY_PORT = parseInt(process.env.PULSE_E2E_RELAY_PORT || '443', 10);
const RELAY_HTTPS = process.env.PULSE_E2E_RELAY_HTTPS !== 'false';

/** Build the relay server_url from env vars (must be ws:// or wss://). */
function buildRelayServerURL(): string {
  const scheme = RELAY_HTTPS ? 'wss' : 'ws';
  const defaultPort = RELAY_HTTPS ? 443 : 80;
  const portSuffix = RELAY_PORT !== defaultPort ? `:${RELAY_PORT}` : '';
  return `${scheme}://${RELAY_HOST}${portSuffix}/ws/instance`;
}

type EntitlementPayload = {
  capabilities?: string[];
  valid?: boolean;
};

type CreateRelayMobileTokenResponse = {
  token?: string;
  record?: {
    id?: string;
  };
};

/** Saved relay config from before the journey, for afterAll restore. */
let savedRelayConfig: Record<string, unknown> | null = null;
const mintedRelayTokenIDs = new Set<string>();

async function createRelayMobileToken(page: import('@playwright/test').Page): Promise<{ id: string; token: string }> {
  const res = await apiRequest(page, '/api/security/tokens/relay-mobile', {
    method: 'POST',
    data: {},
    headers: { 'Content-Type': 'application/json' },
  });
  expect(
    res.ok(),
    `Relay mobile token creation failed: ${res.status()} ${await res.text()}`,
  ).toBeTruthy();

  const payload = (await res.json()) as CreateRelayMobileTokenResponse;
  const token = typeof payload.token === 'string' ? payload.token : '';
  const tokenID = typeof payload.record?.id === 'string' ? payload.record.id : '';

  expect(token, 'relay mobile token response must include a raw token').not.toBe('');
  expect(tokenID, 'relay mobile token response must include a token id').not.toBe('');

  mintedRelayTokenIDs.add(tokenID);
  return { id: tokenID, token };
}

test.describe.serial('Journey: Relay Pairing → Mobile Connection', () => {
  test.afterAll(async ({ browser }) => {
    if (!RELAY_HOST && !savedRelayConfig && mintedRelayTokenIDs.size === 0) return;

    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    try {
      await ensureAuthenticated(page);
      if (savedRelayConfig) {
        await apiRequest(page, '/api/settings/relay', {
          method: 'PUT',
          data: savedRelayConfig,
          headers: { 'Content-Type': 'application/json' },
        });
      }
      for (const tokenID of mintedRelayTokenIDs) {
        const res = await apiRequest(page, `/api/security/tokens/${encodeURIComponent(tokenID)}`, {
          method: 'DELETE',
        });
        if (res.ok() || res.status() === 404) {
          mintedRelayTokenIDs.delete(tokenID);
        } else {
          console.warn(`[journey cleanup] failed to delete relay mobile token ${tokenID}:`, res.status(), await res.text());
        }
      }
    } catch (err) {
      console.warn('[journey cleanup] failed to restore relay journey state:', err);
    } finally {
      await ctx.close();
    }
  });

  test('skip guard: relay credentials are configured', async ({}, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop relay journey');
    test.skip(!RELAY_HOST, 'PULSE_E2E_RELAY_HOST must be set');
  });

  test('relay feature is available via entitlements', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop relay journey');
    test.skip(!RELAY_HOST, 'Relay host not configured');

    await ensureAuthenticated(page);

    const res = await apiRequest(page, '/api/license/entitlements');
    expect(res.ok()).toBeTruthy();

    const entitlements = (await res.json()) as EntitlementPayload;
    const capabilities = new Set(entitlements.capabilities || []);

    expect(
      capabilities.has('relay'),
      'Relay feature must be available in license entitlements. ' +
        'Ensure the test instance has a Pro or higher license.',
    ).toBeTruthy();
  });

  test('configure relay settings', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop relay journey');
    test.skip(!RELAY_HOST, 'Relay host not configured');

    await ensureAuthenticated(page);

    // Save existing config for afterAll restore.
    const getRes = await apiRequest(page, '/api/settings/relay');
    if (getRes.status() === 402) {
      test.skip(true, 'Relay feature not licensed — 402 from relay config endpoint');
      return;
    }
    expect(getRes.ok(), `GET relay config failed: ${getRes.status()}`).toBeTruthy();
    savedRelayConfig = (await getRes.json()) as Record<string, unknown>;

    // PUT uses fields: enabled, server_url, instance_secret (all optional).
    const putRes = await apiRequest(page, '/api/settings/relay', {
      method: 'PUT',
      data: {
        enabled: true,
        server_url: buildRelayServerURL(),
      },
      headers: { 'Content-Type': 'application/json' },
    });

    expect(
      putRes.ok(),
      `PUT relay config failed: ${putRes.status()} ${await putRes.text()}`,
    ).toBeTruthy();
  });

  test('relay status shows connected', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop relay journey');
    test.skip(!RELAY_HOST, 'Relay host not configured');

    test.setTimeout(90_000);

    await ensureAuthenticated(page);

    // Poll relay status until connected (boolean field) — up to 60s.
    let connected = false;
    let lastJSON = '';
    for (let attempt = 0; attempt < 30; attempt++) {
      const res = await apiRequest(page, '/api/settings/relay/status');
      if (res.status() === 402) {
        test.skip(true, 'Relay feature not licensed');
        return;
      }
      if (res.ok()) {
        const status = await res.json();
        lastJSON = JSON.stringify(status);
        // ClientStatus.connected is a boolean.
        if ((status as any).connected === true) {
          connected = true;
          break;
        }
      }
      await page.waitForTimeout(2000);
    }

    expect(
      connected,
      `Relay did not connect within 60s (last status: ${lastJSON})`,
    ).toBeTruthy();
  });

  test('onboarding QR payload has required fields', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop relay journey');
    test.skip(!RELAY_HOST, 'Relay host not configured');

    await ensureAuthenticated(page);

    const relayToken = await createRelayMobileToken(page);

    const res = await apiRequest(page, '/api/onboarding/qr');
    const tokenBackedRes = await apiRequest(page, '/api/onboarding/qr', {
      headers: { 'X-API-Token': relayToken.token },
    });
    if (res.status() === 402) {
      test.skip(true, 'Relay feature not licensed');
      return;
    }
    expect(res.ok(), `QR endpoint failed: ${res.status()}`).toBeTruthy();
    expect(tokenBackedRes.ok(), `Token-backed QR endpoint failed: ${tokenBackedRes.status()} ${await tokenBackedRes.text()}`).toBeTruthy();

    const sessionQR = (await res.json()) as Record<string, unknown>;
    const qr = (await tokenBackedRes.json()) as Record<string, unknown>;

    // onboardingQRResponse: schema, instance_url, relay, auth_token, deep_link
    expect(qr).toHaveProperty('schema');
    expect(qr).toHaveProperty('instance_url');
    expect(qr).toHaveProperty('relay');
    expect(qr).toHaveProperty('auth_token');
    expect(qr).toHaveProperty('deep_link');
    expect(sessionQR).toHaveProperty('auth_token');

    expect(
      typeof sessionQR.auth_token === 'string' && sessionQR.auth_token.length === 0,
      'session-backed QR payload should not expose a bootstrap auth_token',
    ).toBeTruthy();
    expect(qr.auth_token).toBe(relayToken.token);

    // deep_link should be a pulse:// URL.
    expect(
      typeof qr.deep_link === 'string' && (qr.deep_link as string).length > 0,
      'deep_link must be a non-empty string',
    ).toBeTruthy();
    expect((qr.deep_link as string).includes(encodeURIComponent(relayToken.token))).toBeTruthy();

    // relay sub-object should indicate relay is enabled.
    const relay = qr.relay as Record<string, unknown> | undefined;
    expect(relay, 'relay field should be an object').toBeTruthy();
    expect(relay!.enabled, 'relay.enabled should be true').toBeTruthy();
  });

  test('onboarding deep link is available', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop relay journey');
    test.skip(!RELAY_HOST, 'Relay host not configured');

    await ensureAuthenticated(page);

    const relayToken = await createRelayMobileToken(page);

    const res = await apiRequest(page, '/api/onboarding/deep-link', {
      headers: { 'X-API-Token': relayToken.token },
    });
    if (res.status() === 402) {
      test.skip(true, 'Relay feature not licensed');
      return;
    }
    expect(res.ok(), `Deep link endpoint failed: ${res.status()}`).toBeTruthy();

    const body = (await res.json()) as Record<string, unknown>;
    // onboardingDeepLinkResponse: url, diagnostics
    const url = body.url as string | undefined;
    expect(
      typeof url === 'string' && url.length > 0,
      'Deep link response must contain a non-empty url field',
    ).toBeTruthy();
    expect(url).toContain('auth_token=');
    expect(url).toContain(encodeURIComponent(relayToken.token));
  });

  test('relay settings visible in UI', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop relay journey');
    test.skip(!RELAY_HOST, 'Relay host not configured');

    await ensureAuthenticated(page);

    await page.goto('/settings/system-relay', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings/, { timeout: 10_000 });
    await expect(page.locator('#root')).toBeVisible();

    const relayContent = page.locator(
      'text=/Relay|Connected|Enabled|Mobile|Pairing/i',
    ).first();

    await expect(
      relayContent,
      'Relay settings page should show relay-related content',
    ).toBeVisible({ timeout: 15_000 });
  });
});
