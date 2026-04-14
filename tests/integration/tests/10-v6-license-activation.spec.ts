import { test, expect } from '@playwright/test';
import { apiRequest, ensureAuthenticated } from './helpers';

/**
 * V6 License Activation E2E Test
 *
 * Exercises the full v6 activation flow: issue a test license via the license
 * server admin API, paste the activation key into the Pulse Settings UI,
 * verify activation succeeds, and confirm the UI shows the correct tier/features.
 *
 * Requires:
 *   PULSE_LICENSE_ADMIN_TOKEN — admin token for the license server
 *   PULSE_LICENSE_SERVER_URL — (optional) defaults to https://license.pulserelay.pro
 */

const LICENSE_SERVER_URL = (
  process.env.PULSE_LICENSE_SERVER_URL || 'https://license.pulserelay.pro'
).replace(/\/+$/, '');

const ADMIN_TOKEN = process.env.PULSE_LICENSE_ADMIN_TOKEN || '';

type IssuedLicense = {
  license: { license_id: string };
  activation_key: { activation_key: string };
};

type EntitlementPayload = {
  subscription_state?: string;
  tier?: string;
  valid?: boolean;
  is_lifetime?: boolean;
  licensed_email?: string;
  days_remaining?: number;
  limits?: Array<{ key: string; limit: number; current: number; state: string }>;
};

type AISettingsPayload = {
  enabled?: boolean;
  model?: string;
  patrol_model?: string;
  quickstart_credits_total?: number;
  quickstart_credits_remaining?: number;
  quickstart_credits_available?: boolean;
  quickstart_blocked_reason?: string;
};

type PatrolStatusPayload = {
  using_quickstart?: boolean;
  quickstart_credits_total?: number;
  quickstart_credits_remaining?: number;
  runtime_state?: string;
};

test.describe.serial('V6 license activation flow', () => {
  /** Issued license ID — used for cleanup/revocation. */
  let issuedLicenseId = '';
  /** The activation key string (ppk_live_*). */
  let activationKey = '';

  test.afterAll(async ({ browser }) => {
    // Best-effort cleanup: clear license in Pulse and revoke on the server.
    const context = await browser.newContext();
    const page = await context.newPage();

    try {
      await ensureAuthenticated(page);

      // Clear the license in Pulse (ignore errors — may already be cleared).
      await apiRequest(page, '/api/license/clear', { method: 'POST' }).catch(() => {});
    } catch {
      // If Pulse is unreachable, nothing to clean up locally.
    }

    // Revoke the test license on the license server.
    if (issuedLicenseId && ADMIN_TOKEN) {
      try {
        await page.request.fetch(
          `${LICENSE_SERVER_URL}/v1/licenses/${issuedLicenseId}/revoke`,
          {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              'X-API-Token': ADMIN_TOKEN,
            },
            data: { reason: 'E2E test cleanup', reason_code: 'test_cleanup' },
          },
        );
      } catch {
        // Best-effort — test license will expire in 1 day anyway.
      }
    }

    await context.close();
  });

  test('issues a v6 test license via admin API', async ({ page }, testInfo) => {
    test.skip(!ADMIN_TOKEN, 'PULSE_LICENSE_ADMIN_TOKEN not set — skipping v6 activation tests');
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only license workflow');

    const response = await page.request.fetch(`${LICENSE_SERVER_URL}/v1/licenses/issue`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-API-Token': ADMIN_TOKEN,
      },
      data: {
        generation: 'v6',
        email: 'e2e-playwright@test.local',
        tier: 'pro',
        plan_key: 'pro_monthly',
        billing_model: 'manual',
        duration_days: 1,
        max_monitored_systems: 15,
        max_guests: 5,
        max_installations: 3,
        send_email: false,
      },
    });

    expect(
      [200, 201].includes(response.status()),
      `License issue failed: HTTP ${response.status()}`,
    ).toBeTruthy();

    const body = (await response.json()) as IssuedLicense;
    expect(body.license?.license_id).toBeTruthy();
    expect(body.activation_key?.activation_key).toBeTruthy();
    expect(body.activation_key.activation_key).toMatch(/^ppk_live_/);

    issuedLicenseId = body.license.license_id;
    activationKey = body.activation_key.activation_key;
  });

  test('clears any existing license for a clean slate', async ({ page }, testInfo) => {
    test.skip(!ADMIN_TOKEN, 'PULSE_LICENSE_ADMIN_TOKEN not set');
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only');

    await ensureAuthenticated(page);

    const clearRes = await apiRequest(page, '/api/license/clear', { method: 'POST' });
    expect(clearRes.ok(), `Clear license failed: ${clearRes.status()}`).toBeTruthy();

    // Verify we're on the free tier now.
    const entRes = await apiRequest(page, '/api/license/entitlements');
    expect(entRes.ok()).toBeTruthy();
    const ent = (await entRes.json()) as EntitlementPayload;
    expect(ent.tier).toBe('free');
  });

  test('activates v6 license via the Settings UI', async ({ page }, testInfo) => {
    test.skip(!ADMIN_TOKEN, 'PULSE_LICENSE_ADMIN_TOKEN not set');
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only');
    test.skip(!activationKey, 'No activation key from previous step');

    await ensureAuthenticated(page);
    await page.goto('/settings/system/billing');
    await expect(page.getByRole('heading', { name: 'Pulse Pro' }).first()).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Activation' })).toBeVisible();

    // Fill in the activation key.
    const textarea = page.locator('#pulse-pro-license-key');
    await expect(textarea).toBeVisible();
    await textarea.fill(activationKey);

    // Click Activate.
    const activateButton = page.getByRole('button', { name: 'Activate License' });
    await expect(activateButton).toBeEnabled();
    await activateButton.click();

    // Wait for activation to complete: poll entitlements until active, which is
    // more reliable than matching a transient toast.
    await expect.poll(async () => {
      const res = await apiRequest(page, '/api/license/entitlements');
      if (!res.ok()) return '';
      const ent = (await res.json()) as EntitlementPayload;
      return ent.subscription_state;
    }, { timeout: 30_000, message: 'License did not activate within timeout' }).toBe('active');
  });

  test('verifies activated license in UI and API', async ({ page }, testInfo) => {
    test.skip(!ADMIN_TOKEN, 'PULSE_LICENSE_ADMIN_TOKEN not set');
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only');
    test.skip(!activationKey, 'No activation key from previous step');

    await ensureAuthenticated(page);
    await page.goto('/settings/system/billing');
    await expect(page.getByRole('heading', { name: 'Pulse Pro' }).first()).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Plan' })).toBeVisible();
    await expect(page.getByText(/^Active$/).first()).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText('No Pro license is active.')).toHaveCount(0);
    await expect(page.getByText('15').first()).toBeVisible();

    // Verify entitlements API agrees.
    const entRes = await apiRequest(page, '/api/license/entitlements');
    expect(entRes.ok(), `Entitlements request failed: ${entRes.status()}`).toBeTruthy();

    const ent = (await entRes.json()) as EntitlementPayload;
    expect(ent.tier).toBe('pro');
    expect(ent.subscription_state).toBe('active');
    expect(ent.valid).toBe(true);
    expect(ent.licensed_email).toBe('e2e-playwright@test.local');

    // Check max_monitored_systems limit.
    const agentLimit = ent.limits?.find((l) => l.key === 'max_monitored_systems');
    expect(agentLimit, 'max_monitored_systems limit not found in entitlements').toBeTruthy();
    expect(agentLimit!.limit).toBe(15);

    // Check max_guests limit.
    const guestLimit = ent.limits?.find((l) => l.key === 'max_guests');
    expect(guestLimit, 'max_guests limit not found in entitlements').toBeTruthy();
    expect(guestLimit!.limit).toBe(5);
  });

  test('surfaces activated-install quickstart readiness without requiring BYOK', async ({
    page,
  }, testInfo) => {
    test.skip(!ADMIN_TOKEN, 'PULSE_LICENSE_ADMIN_TOKEN not set');
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only');
    test.skip(!activationKey, 'No activation key from previous step');

    await ensureAuthenticated(page);

    const entRes = await apiRequest(page, '/api/license/entitlements');
    expect(entRes.ok(), `Entitlements request failed: ${entRes.status()}`).toBeTruthy();
    const ent = (await entRes.json()) as EntitlementPayload;
    expect(ent.subscription_state).toBe('active');

    await page.goto('/settings/system-ai');
    await expect(page.getByRole('heading', { name: 'Assistant & Patrol' }).first()).toBeVisible();

    const preSettingsRes = await apiRequest(page, '/api/settings/ai');
    expect(preSettingsRes.ok(), `AI settings request failed: ${preSettingsRes.status()}`).toBeTruthy();
    const preSettings = (await preSettingsRes.json()) as AISettingsPayload;
    expect(preSettings.quickstart_credits_available).toBe(true);
    expect(preSettings.quickstart_credits_total).toBe(25);
    expect(preSettings.quickstart_credits_remaining).toBe(25);
    expect(preSettings.quickstart_blocked_reason || '').toBe('');

    const enableAI = page.getByRole('button', { name: 'Enable Assistant and Patrol' });
    await expect(enableAI).toBeVisible();
    if (await page.getByText(/^Disabled$/).isVisible()) {
      await enableAI.click();
    }

    await expect
      .poll(async () => {
        const res = await apiRequest(page, '/api/settings/ai');
        if (!res.ok()) return null;
        const body = (await res.json()) as AISettingsPayload;
        return {
          enabled: body.enabled === true,
          model: body.model || '',
          quickstart_credits_available: body.quickstart_credits_available === true,
          quickstart_credits_remaining: body.quickstart_credits_remaining ?? -1,
        };
      }, { timeout: 30_000 })
      .toEqual({
        enabled: true,
        model: 'quickstart:pulse-hosted',
        quickstart_credits_available: true,
        quickstart_credits_remaining: 25,
      });

    await expect(
      page.getByText(/Patrol quickstart ready • 25\/25 runs left • no API key needed yet/i),
    ).toBeVisible();

    await page.goto('/ai');
    await expect(page.getByRole('heading', { name: 'Patrol' }).first()).toBeVisible();
    await expect(page.getByText('Patrol quickstart: 25/25 runs left')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Run Patrol' })).toBeEnabled();

    const patrolStatusRes = await apiRequest(page, '/api/ai/patrol/status');
    expect(
      patrolStatusRes.ok(),
      `Patrol status request failed: ${patrolStatusRes.status()}`,
    ).toBeTruthy();
    const patrolStatus = (await patrolStatusRes.json()) as PatrolStatusPayload;
    expect(patrolStatus.using_quickstart).toBe(true);
    expect(patrolStatus.quickstart_credits_total).toBe(25);
    expect(patrolStatus.quickstart_credits_remaining).toBe(25);
    expect(patrolStatus.runtime_state).toBe('active');
  });

  test('clears license via UI and verifies free tier', async ({ page }, testInfo) => {
    test.skip(!ADMIN_TOKEN, 'PULSE_LICENSE_ADMIN_TOKEN not set');
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only');
    test.skip(!activationKey, 'No activation key from previous step');

    await ensureAuthenticated(page);
    await page.goto('/settings/system/billing');
    await expect(page.getByRole('heading', { name: 'Pulse Pro' }).first()).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Activation' })).toBeVisible();

    // Set up one-shot dialog handler for the native confirm() prompt.
    page.once('dialog', (dialog) => dialog.accept());

    // Click Clear License.
    const clearButton = page.getByRole('button', { name: 'Clear License' });
    await expect(clearButton).toBeEnabled({ timeout: 5_000 });
    await clearButton.click();

    // Wait for success toast.
    await expect(
      page.locator('text=/License cleared/i').first(),
    ).toBeVisible({ timeout: 10_000 });

    // Verify entitlements API reverts to free tier.
    await expect.poll(async () => {
      const res = await apiRequest(page, '/api/license/entitlements');
      if (!res.ok()) return '';
      const ent = (await res.json()) as EntitlementPayload;
      return ent.tier;
    }, { timeout: 10_000 }).toBe('free');

    // Verify UI shows "No Pro license is active" or free-tier state.
    await page.reload();
    await expect(
      page.locator('text=/No Pro license is active/i').first(),
    ).toBeVisible({ timeout: 10_000 });
  });
});
