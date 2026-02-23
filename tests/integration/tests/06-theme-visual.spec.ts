import { expect, Page, test } from '@playwright/test';
import { ensureAuthenticated, maybeCompleteSetupWizard, waitForPulseReady } from './helpers';

type ThemePreference = 'light' | 'dark';

const VISUAL_VIEWPORT = { width: 1366, height: 900 };

async function applyThemePreference(
  page: Page,
  theme: ThemePreference,
  options?: { forceLoggedOut?: boolean },
): Promise<void> {
  await page.addInitScript(({ pref, forceLoggedOut }: { pref: ThemePreference; forceLoggedOut?: boolean }) => {
    localStorage.setItem('pulseThemePreference', pref);
    localStorage.setItem('darkMode', String(pref === 'dark'));
    localStorage.removeItem('pulse_dark_mode');
    localStorage.setItem('pulse_whats_new_v2_shown', 'true');
    if (forceLoggedOut) {
      localStorage.setItem('just_logged_out', 'true');
    }
  }, { pref: theme, forceLoggedOut: options?.forceLoggedOut });
}

async function stabilizeVisualState(page: Page): Promise<void> {
  await page.addStyleTag({
    content: `
      *, *::before, *::after {
        animation: none !important;
        transition: none !important;
        caret-color: transparent !important;
      }
    `,
  });

  // Allow any final layout flushes after navigation and style injection.
  await page.waitForTimeout(150);
}

async function openLoggedOutLogin(page: Page, theme: ThemePreference): Promise<void> {
  await waitForPulseReady(page);
  await maybeCompleteSetupWizard(page);
  await page.context().clearCookies();

  await applyThemePreference(page, theme, { forceLoggedOut: true });
  await page.goto('/');

  await expect(page.getByRole('heading', { name: 'Welcome to Pulse' })).toBeVisible();
  await expect(page.locator('form').first()).toBeVisible();
  await stabilizeVisualState(page);
}

async function openAuthenticatedSecurityAuth(page: Page, theme: ThemePreference): Promise<void> {
  await applyThemePreference(page, theme);
  await page.goto('/settings/security-auth');
  const authHeading = page.getByRole('heading', { level: 1, name: 'Authentication' });
  const isAlreadyAuthenticated = await authHeading.isVisible({ timeout: 5000 }).catch(() => false);

  if (!isAlreadyAuthenticated) {
    await ensureAuthenticated(page);
    await page.goto('/settings/security-auth');
    await expect(authHeading).toBeVisible();
  }

  await stabilizeVisualState(page);
}

test.describe.serial('Theme visual regression (auth surfaces)', () => {
  test.skip(
    ({ browserName, isMobile }) => browserName !== 'chromium' || Boolean(isMobile),
    'Visual baselines are pinned to desktop Chromium for determinism.',
  );

  test.beforeEach(async ({ page }) => {
    await page.setViewportSize(VISUAL_VIEWPORT);
  });

  test('logged-out login surface (light)', async ({ page }) => {
    await openLoggedOutLogin(page, 'light');

    await expect(page).toHaveScreenshot('auth-login-light.png', {
      fullPage: true,
      animations: 'disabled',
      caret: 'hide',
    });

    await expect(page.locator('form').first()).toHaveScreenshot('auth-login-form-light.png', {
      animations: 'disabled',
      caret: 'hide',
    });
  });

  test('logged-out login surface (dark)', async ({ page }) => {
    await openLoggedOutLogin(page, 'dark');

    await expect(page).toHaveScreenshot('auth-login-dark.png', {
      fullPage: true,
      animations: 'disabled',
      caret: 'hide',
    });

    await expect(page.locator('form').first()).toHaveScreenshot('auth-login-form-dark.png', {
      animations: 'disabled',
      caret: 'hide',
    });
  });

  test('authenticated security-auth surface (light)', async ({ page }) => {
    await openAuthenticatedSecurityAuth(page, 'light');

    await expect(page).toHaveScreenshot('auth-settings-light.png', {
      fullPage: true,
      animations: 'disabled',
      caret: 'hide',
    });
  });

  test('authenticated security-auth surface (dark)', async ({ page }) => {
    await openAuthenticatedSecurityAuth(page, 'dark');

    await expect(page).toHaveScreenshot('auth-settings-dark.png', {
      fullPage: true,
      animations: 'disabled',
      caret: 'hide',
    });
  });
});
