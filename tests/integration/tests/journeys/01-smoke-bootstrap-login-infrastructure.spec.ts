import { test as loginFlowTest, expect } from '@playwright/test';
import { test, ensureJourneyReady } from './journeyAuth';
import {
  ensureSessionAuthenticated,
  login,
  logout,
  waitForPulseReady,
  E2E_CREDENTIALS,
  apiRequest,
  navigateToSettings,
  setMockMode,
  getMockMode,
  trackBrowserRequests,
} from '../helpers';

/**
 * Journey: Bootstrap → First Login → Infrastructure Renders
 *
 * Covers the core smoke path every new Pulse user hits:
 *   1. Pulse boots and responds to health checks
 *   2. Setup wizard completes (handled by ensureAuthenticated)
 *   3. Login succeeds and redirects to Infrastructure
 *   4. Infrastructure renders with key navigation tabs
 *   5. Mock mode can be enabled for deterministic data
 *   6. Main navigation tabs are clickable and render content
 *   7. Logout and re-login cycle works
 *   8. API returns valid state after full cycle
 *
 * This satisfies L12 score-2 criteria: "Stable smoke journeys green:
 * bootstrap → first login → infrastructure renders."
 */

/** Regex matching valid authenticated landing pages (superset of helpers.ts login()). */
const AUTHENTICATED_URL = /\/(infrastructure|proxmox|nodes|hosts|docker)/;

let mockModeWasEnabled: boolean | null = null;

/** Read mock mode and, if this is the first read, capture the baseline for afterAll restore. */
async function captureMockBaseline(page: import('@playwright/test').Page): Promise<{ enabled: boolean }> {
  const state = await getMockMode(page);
  if (mockModeWasEnabled === null) {
    mockModeWasEnabled = state.enabled;
  }
  return state;
}

// These two tests exercise the login flow itself, so they must start from an
// unauthenticated context. They use the plain @playwright/test `test`
// (loginFlowTest) rather than the cookie-session fixture, and issue their own
// real login. Every other journey test inherits the worker cookie session.
loginFlowTest.describe.serial('Journey: Bootstrap → Login (login flow)', () => {
  loginFlowTest('health check responds with retry', async ({ page }, testInfo) => {
    loginFlowTest.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    // Use waitForPulseReady which retries with backoff — avoids flakes on cold starts.
    await waitForPulseReady(page);

    const res = await page.request.get('/api/health');
    expect(res.ok(), `Health check failed with status ${res.status()}`).toBeTruthy();
  });

  loginFlowTest('bootstrap and login land on Infrastructure', async ({ page }, testInfo) => {
    loginFlowTest.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    // This test exercises the real login flow from an unauthenticated context.
    await ensureSessionAuthenticated(page);

    // v6 lands on a platform page after login (Infrastructure superset of landings)
    await expect(page).toHaveURL(AUTHENTICATED_URL);
    await expect(page.locator('#root')).toBeVisible();

    // The page must have meaningful content, not a blank shell
    const mainContent = page.locator('main, [role="main"], #root > div').first();
    await expect(mainContent).toBeVisible();
  });

  // Exercises the logout → re-login cycle. It runs on its OWN session (no
  // shared cookie fixture): logout() destroys the current session server-side,
  // so it must never run against the worker's shared cookie session or every
  // subsequent fixture-backed journey test would find its cookie dead and fall
  // back to a per-test login, tripping the login rate limit.
  loginFlowTest('logout and re-login cycle completes successfully', async ({ page }, testInfo) => {
    loginFlowTest.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureSessionAuthenticated(page);

    await logout(page);
    await expect(page.locator('input[name="username"]')).toBeVisible();

    await login(page, E2E_CREDENTIALS);
    await expect(page).toHaveURL(AUTHENTICATED_URL);
  });
});

test.describe.serial('Journey: Bootstrap → Login → Infrastructure', () => {
  test.beforeAll(async ({ browser, authStorageStatePath }) => {
    // Platform nav tabs (e.g. Proxmox) are resource-admitted — they only
    // render when the resource snapshot proves the surface is present. Enable
    // mock mode up front so the platform-first tab list and resource-content
    // assertions below are deterministic even on a fresh (non-mock) backend
    // such as CI. Capture the true baseline first so afterAll restores it.
    const ctx = await browser.newContext({ storageState: authStorageStatePath });
    const page = await ctx.newPage();
    try {
      await ensureJourneyReady(page);
      const state = await getMockMode(page);
      if (mockModeWasEnabled === null) {
        mockModeWasEnabled = state.enabled;
      }
      if (!state.enabled) {
        await setMockMode(page, true);
      }
    } catch (err) {
      console.warn('[journey setup] failed to enable mock mode:', err);
    } finally {
      await ctx.close();
    }
  });

  test.afterAll(async ({ browser, authStorageStatePath }) => {
    // Restore mock mode to its original state to avoid leaking into other suites.
    if (mockModeWasEnabled !== null) {
      const ctx = await browser.newContext({ storageState: authStorageStatePath });
      const page = await ctx.newPage();
      try {
        await ensureJourneyReady(page);
        const current = await getMockMode(page);
        if (current.enabled !== mockModeWasEnabled) {
          await setMockMode(page, mockModeWasEnabled);
        }
      } catch (err) {
        // Best-effort restore; don't fail the suite on cleanup but warn for diagnosis.
        console.warn('[journey cleanup] failed to restore mock mode:', err);
      } finally {
        await ctx.close();
      }
    }
  });

  test('navigation tabs are visible after login', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureJourneyReady(page);

    // Wait for the primary navigation tablist to render before checking individual
    // tabs. After login redirect, the SPA may still be hydrating the layout.
    // Target the specific aria-label to avoid matching other tablists (e.g. mobile nav).
    await expect(
      page.locator('[role="tablist"][aria-label="Primary navigation"]'),
      'Primary navigation tablist should render',
    ).toBeVisible({ timeout: 15_000 });

    // Core navigation tabs. The IA is platform-first: there is no single
    // 'Infrastructure' tab — the first platform lens (Proxmox, admitted by the
    // mock data enabled in beforeAll) plus the always-present Alerts and
    // Settings tabs.
    const expectedTabs = [
      'Proxmox',
      'Alerts',
      'Settings',
    ];

    for (const tabName of expectedTabs) {
      const tab = page.locator('[role="tab"]').filter({ hasText: tabName }).first();
      await expect(
        tab,
        `Navigation tab "${tabName}" should be visible`,
      ).toBeVisible({ timeout: 10_000 });
    }
  });

  test('mock mode can be enabled for deterministic data', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureJourneyReady(page);

    // Capture initial state so afterAll can restore it.
    const initialState = await captureMockBaseline(page);

    if (!initialState.enabled) {
      const result = await setMockMode(page, true);
      expect(result.enabled).toBe(true);
    }

    const currentState = await getMockMode(page);
    expect(currentState.enabled).toBe(true);
  });

  test('infrastructure landing renders resource content', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureJourneyReady(page);

    await page.goto('/infrastructure', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/infrastructure/);
    await expect(page.locator('#root')).toBeVisible();

    const content = page.locator(
      'table, [role="table"], [role="grid"], [class*="resource"], [class*="table"], h1, h2',
    ).first();
    await expect(
      content,
      'Infrastructure should render resource content',
    ).toBeVisible({ timeout: 15_000 });
  });

  test('infrastructure tab renders resource content', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureJourneyReady(page);

    // Ensure mock mode is on so we always have resources to display.
    const mockState = await captureMockBaseline(page);
    if (!mockState.enabled) {
      await setMockMode(page, true);
    }

    await page.goto('/infrastructure', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/infrastructure/);

    // Infrastructure page should have a resource table, or at minimum a heading/content area.
    const resourceContent = page.locator(
      'table, [role="table"], [role="grid"], [class*="resource"], [class*="table"], h1, h2',
    ).first();
    await expect(
      resourceContent,
      'Infrastructure should render resource content',
    ).toBeVisible({ timeout: 15_000 });
  });

  test('alerts tab renders overview', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureJourneyReady(page);

    await page.goto('/alerts/overview', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/alerts/);

    const heading = page.getByRole('heading', { name: /alerts/i }).first();
    await expect(heading).toBeVisible({ timeout: 10_000 });
  });

  test('settings tab renders sidebar categories', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureJourneyReady(page);
    await navigateToSettings(page);

    const sidebar = page.locator('[aria-label="Settings navigation"]');
    await expect(sidebar).toBeVisible({ timeout: 10_000 });

    const categories = ['Infrastructure', 'System', 'Security'];
    for (const category of categories) {
      const heading = sidebar.getByText(category, { exact: false }).first();
      await expect(
        heading,
        `Settings category "${category}" should appear in sidebar`,
      ).toBeVisible({ timeout: 10_000 });
    }
  });

  test('core non-billing navigation does not read billing entitlements', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureJourneyReady(page);

    const entitlementsRequests = trackBrowserRequests(page, '/api/license/entitlements');
    try {
      await page.goto('/infrastructure', { waitUntil: 'domcontentloaded' });
      await expect(page.locator('#root')).toBeVisible();

      await page.goto('/infrastructure', { waitUntil: 'domcontentloaded' });
      await expect(page.locator('#root')).toBeVisible();

      await page.goto('/alerts/overview', { waitUntil: 'domcontentloaded' });
      await expect(page.locator('#root')).toBeVisible();

      await navigateToSettings(page);
      await expect(page.locator('[aria-label="Settings navigation"]')).toBeVisible({
        timeout: 10_000,
      });
    } finally {
      const requestedURLs = entitlementsRequests.urls();
      entitlementsRequests.stop();
      expect(
        requestedURLs,
        'Core non-billing navigation must not trigger billing entitlements reads in the browser shell',
      ).toEqual([]);
    }
  });

  test('API returns runtime capabilities after full journey', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureJourneyReady(page);

    const stateRes = await apiRequest(page, '/api/state');
    expect(stateRes.ok(), `GET /api/state failed: ${stateRes.status()}`).toBeTruthy();

    const healthRes = await apiRequest(page, '/api/health');
    expect(healthRes.ok(), `GET /api/health failed: ${healthRes.status()}`).toBeTruthy();

    const runtimeRes = await apiRequest(page, '/api/license/runtime-capabilities');
    expect(
      runtimeRes.ok(),
      `GET /api/license/runtime-capabilities failed: ${runtimeRes.status()}`,
    ).toBeTruthy();

    const runtimeCapabilities = await runtimeRes.json();
    expect(Array.isArray(runtimeCapabilities.capabilities)).toBe(true);
  });
});
