import { test, expect } from '@playwright/test';
import {
  ensureAuthenticated,
  login,
  logout,
  waitForPulseReady,
  E2E_CREDENTIALS,
  apiRequest,
  navigateToSettings,
  setMockMode,
  getMockMode,
} from '../helpers';

/**
 * Journey: Bootstrap → First Login → Dashboard Renders
 *
 * Covers the core smoke path every new Pulse user hits:
 *   1. Pulse boots and responds to health checks
 *   2. Setup wizard completes (handled by ensureAuthenticated)
 *   3. Login succeeds and redirects to infrastructure dashboard
 *   4. Dashboard renders with key navigation tabs
 *   5. Mock mode can be enabled for deterministic data
 *   6. Main navigation tabs are clickable and render content
 *   7. Logout and re-login cycle works
 *   8. API returns valid state after full cycle
 *
 * This satisfies L12 score-2 criteria: "Stable smoke journeys green:
 * bootstrap → first login → dashboard renders."
 */

/** Regex matching valid authenticated landing pages (superset of helpers.ts login()). */
const AUTHENTICATED_URL = /\/(infrastructure|proxmox|dashboard|nodes|hosts|docker)/;

let mockModeWasEnabled: boolean | null = null;

/** Read mock mode and, if this is the first read, capture the baseline for afterAll restore. */
async function captureMockBaseline(page: import('@playwright/test').Page): Promise<{ enabled: boolean }> {
  const state = await getMockMode(page);
  if (mockModeWasEnabled === null) {
    mockModeWasEnabled = state.enabled;
  }
  return state;
}

test.describe.serial('Journey: Bootstrap → Login → Dashboard', () => {
  test.afterAll(async ({ browser }) => {
    // Restore mock mode to its original state to avoid leaking into other suites.
    if (mockModeWasEnabled !== null) {
      const ctx = await browser.newContext();
      const page = await ctx.newPage();
      try {
        await login(page);
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

  test('health check responds with retry', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    // Use waitForPulseReady which retries with backoff — avoids flakes on cold starts.
    await waitForPulseReady(page);

    const res = await page.request.get('/api/health');
    expect(res.ok(), `Health check failed with status ${res.status()}`).toBeTruthy();
  });

  test('bootstrap and login land on infrastructure dashboard', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureAuthenticated(page);

    // v6 lands on /infrastructure after login (other routes are valid legacy landings)
    await expect(page).toHaveURL(AUTHENTICATED_URL);
    await expect(page.locator('#root')).toBeVisible();

    // The page must have meaningful content, not a blank shell
    const mainContent = page.locator('main, [role="main"], #root > div').first();
    await expect(mainContent).toBeVisible();
  });

  test('navigation tabs are visible after login', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureAuthenticated(page);

    // Wait for the primary navigation tablist to render before checking individual
    // tabs. After login redirect, the SPA may still be hydrating the layout.
    // Target the specific aria-label to avoid matching other tablists (e.g. mobile nav).
    await expect(
      page.locator('[role="tablist"][aria-label="Primary navigation"]'),
      'Primary navigation tablist should render',
    ).toBeVisible({ timeout: 15_000 });

    // Core navigation tabs that must always be present
    const expectedTabs = [
      'Dashboard',
      'Infrastructure',
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

    await ensureAuthenticated(page);

    // Capture initial state so afterAll can restore it.
    const initialState = await captureMockBaseline(page);

    if (!initialState.enabled) {
      const result = await setMockMode(page, true);
      expect(result.enabled).toBe(true);
    }

    const currentState = await getMockMode(page);
    expect(currentState.enabled).toBe(true);
  });

  test('dashboard tab renders KPI content', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureAuthenticated(page);

    await page.goto('/dashboard', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/dashboard/);
    await expect(page.locator('#root')).toBeVisible();

    // Wait for at least one heading or KPI widget to appear.
    const content = page.locator('h1, h2, h3, [role="heading"], [class*="card"], [class*="kpi"], [class*="summary"]').first();
    await expect(
      content,
      'Dashboard should render content (headings, cards, or KPI widgets)',
    ).toBeVisible({ timeout: 15_000 });
  });

  test('infrastructure tab renders resource content', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureAuthenticated(page);

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

    await ensureAuthenticated(page);

    await page.goto('/alerts/overview', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/alerts/);

    const heading = page.getByRole('heading', { name: /alerts/i }).first();
    await expect(heading).toBeVisible({ timeout: 10_000 });
  });

  test('settings tab renders sidebar categories', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureAuthenticated(page);
    await navigateToSettings(page);

    const sidebar = page.locator('[aria-label="Settings navigation"]');
    await expect(sidebar).toBeVisible({ timeout: 10_000 });

    const categories = ['Platforms', 'Operations', 'System', 'Security'];
    for (const category of categories) {
      const heading = sidebar.getByText(category, { exact: false }).first();
      await expect(
        heading,
        `Settings category "${category}" should appear in sidebar`,
      ).toBeVisible({ timeout: 10_000 });
    }
  });

  test('logout and re-login cycle completes successfully', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureAuthenticated(page);

    await logout(page);
    await expect(page.locator('input[name="username"]')).toBeVisible();

    await login(page, E2E_CREDENTIALS);
    await expect(page).toHaveURL(AUTHENTICATED_URL);
  });

  test('API returns valid state after full journey', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop smoke journey');

    await ensureAuthenticated(page);

    const stateRes = await apiRequest(page, '/api/state');
    expect(stateRes.ok(), `GET /api/state failed: ${stateRes.status()}`).toBeTruthy();

    const healthRes = await apiRequest(page, '/api/health');
    expect(healthRes.ok(), `GET /api/health failed: ${healthRes.status()}`).toBeTruthy();

    const entRes = await apiRequest(page, '/api/license/entitlements');
    expect(entRes.ok(), `GET /api/license/entitlements failed: ${entRes.status()}`).toBeTruthy();

    const entitlements = await entRes.json();
    expect(entitlements).toHaveProperty('valid');
  });
});
