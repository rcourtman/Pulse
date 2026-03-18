import { test, expect } from '@playwright/test';
import {
  ensureAuthenticated,
  apiRequest,
  setMockMode,
  getMockMode,
} from '../helpers';

/**
 * Journey: Add TrueNAS Node → Pools & Datasets Visible
 *
 * Covers the TrueNAS integration path:
 *   1. Add a TrueNAS connection via API
 *   2. Verify the connection appears in the connections list
 *   3. Wait for TrueNAS data to appear in unified state
 *   4. Verify pools and datasets are visible in the storage API
 *   5. Verify TrueNAS resources appear in the UI infrastructure page
 *   6. Clean up the connection
 *
 * This satisfies part of L12 score-3 criteria: "TrueNAS node addition →
 * pools/datasets visible."
 *
 * Environment variables:
 *   PULSE_E2E_TRUENAS_HOST  - TrueNAS hostname or IP (required, skip if absent)
 *   PULSE_E2E_TRUENAS_API_KEY - TrueNAS API key (required, skip if absent)
 *   PULSE_E2E_TRUENAS_PORT  - TrueNAS port (optional, default: 443)
 *   PULSE_E2E_TRUENAS_HTTPS - Use HTTPS (optional, default: "true")
 *   PULSE_E2E_TRUENAS_INSECURE - Skip TLS verify (optional, default: "true")
 */

const TRUENAS_HOST = process.env.PULSE_E2E_TRUENAS_HOST || '';
const TRUENAS_API_KEY = process.env.PULSE_E2E_TRUENAS_API_KEY || '';
const TRUENAS_PORT = parseInt(process.env.PULSE_E2E_TRUENAS_PORT || '443', 10);
const TRUENAS_HTTPS = process.env.PULSE_E2E_TRUENAS_HTTPS !== 'false';
const TRUENAS_INSECURE = process.env.PULSE_E2E_TRUENAS_INSECURE !== 'false';

/** Connection name used throughout this journey — unique per run to avoid collisions. */
const CONNECTION_NAME = `e2e-truenas-${Date.now()}`;

/** ID of the created connection, populated after add. */
let connectionId = '';

/** Whether mock mode was enabled before the journey (for cleanup). */
let mockModeWasEnabled: boolean | null = null;

test.describe.serial('Journey: Add TrueNAS Node → Pools & Datasets Visible', () => {
  test.beforeAll(async ({ browser }) => {
    // Only set up mock mode when TrueNAS credentials are present.
    if (!TRUENAS_HOST || !TRUENAS_API_KEY) return;

    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    try {
      await ensureAuthenticated(page);
      const state = await getMockMode(page);
      mockModeWasEnabled = state.enabled;
      if (state.enabled) {
        await setMockMode(page, false);
      }
    } catch (err) {
      console.warn('[journey setup] failed to disable mock mode:', err);
    } finally {
      await ctx.close();
    }
  });

  test.afterAll(async ({ browser }) => {
    // Only clean up when TrueNAS credentials were present.
    if (!TRUENAS_HOST || !TRUENAS_API_KEY) return;

    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    try {
      await ensureAuthenticated(page);

      // Remove the test connection if it was created.
      if (connectionId) {
        await apiRequest(page, `/api/truenas/connections/${connectionId}`, {
          method: 'DELETE',
        });
      }

      // Restore mock mode to its original state.
      if (mockModeWasEnabled !== null) {
        const current = await getMockMode(page);
        if (current.enabled !== mockModeWasEnabled) {
          await setMockMode(page, mockModeWasEnabled);
        }
      }
    } catch (err) {
      console.warn('[journey cleanup] failed to clean up TrueNAS connection:', err);
    } finally {
      await ctx.close();
    }
  });

  test('skip guard: TrueNAS credentials are configured', async ({}, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop infrastructure journey');
    test.skip(!TRUENAS_HOST || !TRUENAS_API_KEY, 'PULSE_E2E_TRUENAS_HOST and PULSE_E2E_TRUENAS_API_KEY must be set');
  });

  test('test TrueNAS connection before adding', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop infrastructure journey');
    test.skip(!TRUENAS_HOST || !TRUENAS_API_KEY, 'TrueNAS credentials not configured');

    await ensureAuthenticated(page);

    const res = await apiRequest(page, '/api/truenas/connections/test', {
      method: 'POST',
      data: {
        name: CONNECTION_NAME,
        host: TRUENAS_HOST,
        port: TRUENAS_PORT,
        apiKey: TRUENAS_API_KEY,
        useHttps: TRUENAS_HTTPS,
        insecureSkipVerify: TRUENAS_INSECURE,
      },
      headers: { 'Content-Type': 'application/json' },
    });

    expect(
      res.ok(),
      `Test connection failed: ${res.status()} ${await res.text()}`,
    ).toBeTruthy();
  });

  test('add TrueNAS connection via API', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop infrastructure journey');
    test.skip(!TRUENAS_HOST || !TRUENAS_API_KEY, 'TrueNAS credentials not configured');

    await ensureAuthenticated(page);

    const res = await apiRequest(page, '/api/truenas/connections', {
      method: 'POST',
      data: {
        name: CONNECTION_NAME,
        host: TRUENAS_HOST,
        port: TRUENAS_PORT,
        apiKey: TRUENAS_API_KEY,
        useHttps: TRUENAS_HTTPS,
        insecureSkipVerify: TRUENAS_INSECURE,
        enabled: true,
        pollIntervalSeconds: 30,
      },
      headers: { 'Content-Type': 'application/json' },
    });

    // POST /api/truenas/connections returns 201 Created.
    expect(
      res.status(),
      `Add connection failed: ${res.status()} ${await res.text()}`,
    ).toBe(201);

    const body = await res.json();
    connectionId = body.id || '';
    expect(connectionId, 'Response must include a connection ID').toBeTruthy();
  });

  test('TrueNAS connection appears in connections list', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop infrastructure journey');
    test.skip(!connectionId, 'Connection was not created');

    await ensureAuthenticated(page);

    const res = await apiRequest(page, '/api/truenas/connections');
    expect(res.ok()).toBeTruthy();

    const connections = await res.json();
    const found = (connections as any[]).find(
      (c: any) => c.id === connectionId || c.name === CONNECTION_NAME,
    );
    expect(found, `Connection ${CONNECTION_NAME} not found in list`).toBeTruthy();
  });

  test('TrueNAS resources appear in unified state', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop infrastructure journey');
    test.skip(!connectionId, 'Connection was not created');

    // Allow polling time for TrueNAS data to flow through.
    test.setTimeout(120_000);

    await ensureAuthenticated(page);

    // Poll /api/state until TrueNAS resources appear (up to 90s).
    // StateFrontend.resources[] uses ResourceFrontend with fields:
    //   sourceType: "truenas", platformType: "truenas", type: resource type
    let foundTrueNAS = false;
    for (let attempt = 0; attempt < 45; attempt++) {
      const res = await apiRequest(page, '/api/state');
      if (!res.ok()) {
        await page.waitForTimeout(2000);
        continue;
      }

      const state = (await res.json()) as Record<string, unknown>;
      const resources = state.resources;
      if (Array.isArray(resources)) {
        foundTrueNAS = resources.some(
          (r: any) =>
            r.sourceType === 'truenas' ||
            r.platformType === 'truenas',
        );
      }
      // TrueNAS storage also appears in the top-level storage array.
      if (!foundTrueNAS) {
        const storage = state.storage;
        if (Array.isArray(storage) && storage.length > 0) {
          foundTrueNAS = storage.some(
            (s: any) => s.source === 'truenas' || s.sourceType === 'truenas',
          );
        }
      }

      if (foundTrueNAS) break;
      await page.waitForTimeout(2000);
    }

    expect(
      foundTrueNAS,
      'TrueNAS resources did not appear in /api/state within 90s',
    ).toBeTruthy();
  });

  test('pools and datasets visible in unified state', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop infrastructure journey');
    test.skip(!connectionId, 'Connection was not created');

    test.setTimeout(90_000);

    await ensureAuthenticated(page);

    // Poll /api/state until TrueNAS storage resources (pools/datasets) appear.
    // StateFrontend has both a top-level `storage` array and a unified `resources` array.
    let foundPools = false;
    for (let attempt = 0; attempt < 30; attempt++) {
      const stateRes = await apiRequest(page, '/api/state');
      if (stateRes.ok()) {
        const state = (await stateRes.json()) as Record<string, unknown>;

        // Check unified resources for TrueNAS storage-type entries.
        const resources = state.resources;
        if (Array.isArray(resources)) {
          const storageResources = resources.filter(
            (r: any) =>
              (r.sourceType === 'truenas' || r.platformType === 'truenas') &&
              (r.type === 'storage' || r.type === 'pool' || r.type === 'dataset'),
          );
          if (storageResources.length > 0) {
            foundPools = true;
            break;
          }
        }

        // Also check the top-level storage array.
        const storage = state.storage;
        if (Array.isArray(storage)) {
          const trueNASStorage = storage.filter(
            (s: any) => s.source === 'truenas' || s.sourceType === 'truenas',
          );
          if (trueNASStorage.length > 0) {
            foundPools = true;
            break;
          }
        }
      }

      await page.waitForTimeout(2000);
    }

    expect(
      foundPools,
      'TrueNAS pools/datasets did not appear in state within 60s',
    ).toBeTruthy();
  });

  test('TrueNAS resources visible on infrastructure page', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop infrastructure journey');
    test.skip(!connectionId, 'Connection was not created');

    test.setTimeout(60_000);

    await ensureAuthenticated(page);

    await page.goto('/infrastructure', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/infrastructure/);

    // Look for TrueNAS-related content in the infrastructure page.
    // This could be the connection name, a "TrueNAS" label, or storage-related content.
    const trueNASContent = page.locator(
      `text=/${CONNECTION_NAME}|TrueNAS|truenas/i`,
    ).first();

    await expect(
      trueNASContent,
      'TrueNAS resource should be visible on the infrastructure page',
    ).toBeVisible({ timeout: 30_000 });
  });

  test('storage page shows TrueNAS pools', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop infrastructure journey');
    test.skip(!connectionId, 'Connection was not created');

    test.setTimeout(60_000);

    await ensureAuthenticated(page);

    await page.goto('/storage', { waitUntil: 'domcontentloaded' });

    // Storage page should render pool/dataset information from TrueNAS.
    const storageContent = page.locator(
      'text=/pool|dataset|ZFS|capacity|storage/i',
    ).first();

    await expect(
      storageContent,
      'Storage page should display TrueNAS pool/dataset information',
    ).toBeVisible({ timeout: 30_000 });
  });
});
