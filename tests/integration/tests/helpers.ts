/**
 * Test helpers for Pulse update integration tests
 */

import { Page, expect } from '@playwright/test';

/**
 * Default admin credentials for testing
 */
export const ADMIN_CREDENTIALS = {
  username: 'admin',
  // Pulse enforces a minimum password length of 12 characters.
  password: 'adminadminadmin',
};

const DEFAULT_E2E_BOOTSTRAP_TOKEN = '0123456789abcdef0123456789abcdef0123456789abcdef';

export const E2E_CREDENTIALS = {
  bootstrapToken: process.env.PULSE_E2E_BOOTSTRAP_TOKEN || DEFAULT_E2E_BOOTSTRAP_TOKEN,
  username: process.env.PULSE_E2E_USERNAME || ADMIN_CREDENTIALS.username,
  password: process.env.PULSE_E2E_PASSWORD || ADMIN_CREDENTIALS.password,
};

export async function waitForPulseReady(page: Page, timeoutMs = 120_000) {
  const startedAt = Date.now();
  let lastError: unknown = null;
  while (Date.now() - startedAt < timeoutMs) {
    try {
      const res = await page.request.get('/api/health');
      if (res.ok()) {
        return;
      }
      lastError = new Error(`Health check returned ${res.status()}`);
    } catch (err) {
      lastError = err;
    }
    await page.waitForTimeout(1000);
  }
  throw lastError ?? new Error('Timed out waiting for Pulse to become ready');
}

type SecurityStatus = {
  hasAuthentication?: boolean;
};

export async function getSecurityStatus(page: Page): Promise<SecurityStatus> {
  const res = await page.request.get('/api/security/status');
  if (!res.ok()) {
    throw new Error(`Failed to fetch security status: ${res.status()}`);
  }
  return (await res.json()) as SecurityStatus;
}

export async function maybeCompleteSetupWizard(page: Page) {
  const security = await getSecurityStatus(page);
  if (security.hasAuthentication !== false) {
    return;
  }

  if (!E2E_CREDENTIALS.bootstrapToken) {
    throw new Error(
      'Pulse requires first-run setup but PULSE_E2E_BOOTSTRAP_TOKEN is not set (or is empty)',
    );
  }

  await page.goto('/');

  const wizard = page.getByRole('main', { name: 'Pulse Setup Wizard' });
  await expect(wizard).toBeVisible();

  await page.getByPlaceholder('Paste your bootstrap token').fill(E2E_CREDENTIALS.bootstrapToken);
  await page.getByRole('button', { name: /continue/i }).click();

  await expect(wizard.getByText('Secure Your Dashboard')).toBeVisible();
  await wizard.getByRole('button', { name: /custom password/i }).click();

  await wizard.locator('input[type="text"]').first().fill(E2E_CREDENTIALS.username);
  await wizard.locator('input[type="password"]').nth(0).fill(E2E_CREDENTIALS.password);
  await wizard.locator('input[type="password"]').nth(1).fill(E2E_CREDENTIALS.password);

  await wizard.getByRole('button', { name: /create account/i }).click();
  await expect(wizard.getByText(/security configured/i)).toBeVisible();

  await wizard.getByRole('button', { name: /go to dashboard|skip for now/i }).click();

  await page.waitForLoadState('domcontentloaded');
}

/**
 * Login as admin user
 */
export async function loginAsAdmin(page: Page) {
  await page.goto('/');
  await page.waitForSelector('input[name="username"]', { state: 'visible' });
  await page.fill('input[name="username"]', E2E_CREDENTIALS.username);
  await page.fill('input[name="password"]', E2E_CREDENTIALS.password);
  await page.click('button[type="submit"]');

  // Wait for redirect to dashboard
  await page.waitForURL(/\/(dashboard|nodes|proxmox)/);
}

export async function login(page: Page, credentials = E2E_CREDENTIALS) {
  await page.goto('/');
  await page.waitForLoadState('domcontentloaded');

  const authenticatedURL = /\/(proxmox|dashboard|nodes|hosts|docker)/;
  const usernameInput = page.locator('input[name="username"]');

  const state = await Promise.race([
    usernameInput
      .waitFor({ state: 'visible', timeout: 10_000 })
      .then(() => 'login')
      .catch(() => undefined),
    page
      .waitForURL(authenticatedURL, { timeout: 10_000 })
      .then(() => 'authenticated')
      .catch(() => undefined),
  ]);

  if (state === 'authenticated') {
    return;
  }

  if (state !== 'login') {
    const url = page.url();
    const preview = ((await page.locator('body').textContent()) || '').replace(/\s+/g, ' ').slice(0, 200);
    throw new Error(`Login did not render and did not redirect (url=${url}, body="${preview}")`);
  }

  await page.fill('input[name="username"]', credentials.username);
  await page.fill('input[name="password"]', credentials.password);
  await page.click('button[type="submit"]');
  await page.waitForURL(authenticatedURL);
}

export async function ensureAuthenticated(page: Page) {
  await waitForPulseReady(page);
  await maybeCompleteSetupWizard(page);
  await login(page);
  await expect(page).toHaveURL(/\/(proxmox|dashboard|nodes|hosts|docker)/);
}

export async function logout(page: Page) {
  const logoutButton = page.locator('button[aria-label="Logout"]').first();
  await expect(logoutButton).toBeVisible();
  await logoutButton.click();
  await page.waitForURL(/\/$/, { timeout: 15000 });
  await expect(page.locator('input[name="username"]')).toBeVisible();
}

export async function setMockMode(page: Page, enabled: boolean) {
  const res = await page.request.post('/api/system/mock-mode', {
    data: { enabled },
    headers: { 'Content-Type': 'application/json' },
  });
  if (!res.ok()) {
    throw new Error(`Failed to update mock mode: ${res.status()} ${await res.text()}`);
  }
  return (await res.json()) as { enabled: boolean };
}

export async function getMockMode(page: Page) {
  const res = await page.request.get('/api/system/mock-mode');
  if (!res.ok()) {
    throw new Error(`Failed to read mock mode: ${res.status()} ${await res.text()}`);
  }
  return (await res.json()) as { enabled: boolean };
}

/**
 * Navigate to settings page
 */
export async function navigateToSettings(page: Page) {
  await page.goto('/settings');

  // Wait for settings UI scaffolding (nav rail) to render
  await expect(
    page.locator('[aria-label="Settings navigation"], [data-testid="settings-nav"]')
  ).toBeVisible();
}

/**
 * Wait for update banner to appear
 */
export async function waitForUpdateBanner(page: Page, timeout = 30000) {
  const banner = page.locator('[data-testid="update-banner"], .update-banner').first();
  await expect(banner).toBeVisible({ timeout });
  return banner;
}

/**
 * Click "Apply Update" button in update banner
 */
export async function clickApplyUpdate(page: Page) {
  const applyButton = page.locator('button').filter({ hasText: /apply update/i }).first();
  await expect(applyButton).toBeVisible();
  await applyButton.click();
}

/**
 * Wait for update confirmation modal
 */
export async function waitForConfirmationModal(page: Page) {
  const modal = page.locator('[role="dialog"], .modal').filter({ hasText: /confirm/i }).first();
  await expect(modal).toBeVisible({ timeout: 10000 });
  return modal;
}

/**
 * Confirm update in modal (check acknowledgement and click confirm)
 */
export async function confirmUpdate(page: Page) {
  // Check acknowledgement checkbox if present
  const checkbox = page.locator('input[type="checkbox"]').first();
  if (await checkbox.isVisible({ timeout: 2000 }).catch(() => false)) {
    await checkbox.check();
  }

  // Click confirm button
  const confirmButton = page.locator('button').filter({ hasText: /confirm|proceed|continue/i }).first();
  await confirmButton.click();
}

/**
 * Wait for update progress modal
 */
export async function waitForProgressModal(page: Page) {
  const modal = page.locator('[data-testid="update-progress-modal"], [role="dialog"]')
    .filter({ hasText: /updating|progress|downloading/i })
    .first();
  await expect(modal).toBeVisible({ timeout: 10000 });
  return modal;
}

/**
 * Count visible modals on page
 */
export async function countVisibleModals(page: Page): Promise<number> {
  const modals = page.locator('[role="dialog"], .modal').filter({ hasText: /update|progress/i });
  return await modals.count();
}

/**
 * Wait for error message in modal
 */
export async function waitForErrorInModal(page: Page, modal: any) {
  const errorText = modal.locator('text=/error|failed|invalid/i').first();
  await expect(errorText).toBeVisible({ timeout: 30000 });
  return errorText;
}

/**
 * Check that error message is user-friendly (not a raw stack trace or API error)
 */
export async function assertUserFriendlyError(errorText: string) {
  // User-friendly errors should NOT contain:
  expect(errorText).not.toMatch(/stack trace|at Object\.|Error:/i);
  expect(errorText).not.toMatch(/500 Internal Server Error/i);
  expect(errorText).not.toMatch(/\/api\//i); // No API paths

  // User-friendly errors SHOULD be concise
  expect(errorText.length).toBeLessThan(200);
}

/**
 * Dismiss modal (click close button or backdrop)
 */
export async function dismissModal(page: Page) {
  // Try close button first
  const closeButton = page.locator('button[aria-label="Close"], button.close, button').filter({ hasText: /close|dismiss/i }).first();
  if (await closeButton.isVisible({ timeout: 2000 }).catch(() => false)) {
    await closeButton.click();
    return;
  }

  // Try ESC key
  await page.keyboard.press('Escape');
}

/**
 * Wait for progress to reach a certain percentage
 */
export async function waitForProgress(page: Page, modal: any, minPercent: number) {
  await page.waitForFunction(
    ({ modalSelector, min }) => {
      const modal = document.querySelector(modalSelector);
      if (!modal) return false;

      // Check for progress bar or percentage text
      const progressBar = modal.querySelector('[role="progressbar"]');
      if (progressBar) {
        const value = progressBar.getAttribute('aria-valuenow');
        return value && parseInt(value) >= min;
      }

      // Check for percentage text
      const text = modal.textContent || '';
      const match = text.match(/(\d+)%/);
      return match && parseInt(match[1]) >= min;
    },
    { modalSelector: '[role="dialog"]', min: minPercent },
    { timeout: 30000 }
  );
}

/**
 * Restart test environment with specific mock configuration
 */
export async function restartWithMockConfig(config: {
  checksumError?: boolean;
  networkError?: boolean;
  rateLimit?: boolean;
  staleRelease?: boolean;
}) {
  // This would be implemented by CI/test runner to restart containers
  // with new environment variables
  console.log('Mock config:', config);
}

/**
 * Reset test environment to clean state
 */
export async function resetTestEnvironment() {
  // Clear any cached update checks
  // Reset database state
  // Restart services
}

/**
 * Make API request to Pulse backend
 */
export async function apiRequest(page: Page, endpoint: string, options: any = {}) {
  const baseURL = 'http://localhost:7655';
  const response = await page.request.fetch(`${baseURL}${endpoint}`, options);
  return response;
}

export async function isMultiTenantEnabled(page: Page): Promise<boolean> {
  const res = await apiRequest(page, '/api/license/status');
  if (!res.ok()) return false;

  const data = (await res.json()) as { features?: string[] };
  if (!(data.features?.includes('multi_tenant') ?? false)) {
    return false;
  }

  const orgsRes = await apiRequest(page, '/api/orgs');
  return orgsRes.ok();
}

const toOrgID = (displayName: string) => {
  const base = displayName
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 36) || 'org';
  const suffix = `${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`;
  return `${base}-${suffix}`.slice(0, 64);
};

export async function createOrg(page: Page, displayName: string): Promise<{ id: string }> {
  const res = await apiRequest(page, '/api/orgs', {
    method: 'POST',
    data: { id: toOrgID(displayName), displayName },
    headers: { 'Content-Type': 'application/json' },
  });
  if (!res.ok()) throw new Error(`Failed to create org: ${res.status()} ${await res.text()}`);

  const payload = (await res.json()) as { id?: string };
  if (!payload.id) {
    throw new Error('Failed to create org: response missing org id');
  }

  return { id: payload.id };
}

export async function deleteOrg(page: Page, orgId: string): Promise<void> {
  const res = await apiRequest(page, `/api/orgs/${encodeURIComponent(orgId)}`, {
    method: 'DELETE',
  });
  if (!res.ok() && res.status() !== 404) {
    throw new Error(`Failed to delete org: ${res.status()} ${await res.text()}`);
  }
}

export async function switchOrg(page: Page, orgId: string): Promise<void> {
  await page.evaluate((id) => {
    window.sessionStorage.setItem('pulse_org_id', id);
    window.localStorage.setItem('pulse_org_id', id);
    document.cookie = `pulse_org_id=${encodeURIComponent(id)}; Path=/; SameSite=Lax`;
  }, orgId);
  await page.reload();
  await page.waitForLoadState('networkidle');
}

/**
 * Check for updates via API
 */
export async function checkForUpdatesAPI(page: Page, channel: 'stable' | 'rc' = 'stable') {
  const response = await apiRequest(page, `/api/updates/check?channel=${channel}`);
  return response.json();
}

/**
 * Apply update via API
 */
export async function applyUpdateAPI(page: Page, downloadUrl: string) {
  const response = await apiRequest(page, '/api/updates/apply', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    data: { url: downloadUrl },
  });
  return response;
}

/**
 * Get update status via API
 */
export async function getUpdateStatusAPI(page: Page) {
  const response = await apiRequest(page, '/api/updates/status');
  return response.json();
}

/**
 * Measure time between events
 */
export class Timer {
  private start: number;

  constructor() {
    this.start = Date.now();
  }

  elapsed(): number {
    return Date.now() - this.start;
  }

  reset() {
    this.start = Date.now();
  }
}

/**
 * Poll until condition is met
 */
export async function pollUntil<T>(
  fn: () => Promise<T>,
  condition: (result: T) => boolean,
  options: { timeout?: number; interval?: number } = {}
): Promise<T> {
  const timeout = options.timeout || 30000;
  const interval = options.interval || 1000;
  const start = Date.now();

  while (Date.now() - start < timeout) {
    const result = await fn();
    if (condition(result)) {
      return result;
    }
    await new Promise(resolve => setTimeout(resolve, interval));
  }

  throw new Error(`Polling timed out after ${timeout}ms`);
}
