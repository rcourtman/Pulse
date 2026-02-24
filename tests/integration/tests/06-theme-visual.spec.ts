import { expect, Page, test } from '@playwright/test';
import { ensureAuthenticated, maybeCompleteSetupWizard, waitForPulseReady } from './helpers';

type ThemePreference = 'light' | 'dark';

const VISUAL_VIEWPORT = { width: 1366, height: 900 };
const STABLE_VERSION_INFO = {
  version: '99.1.0-rc.1',
  build: '',
  runtime: 'unknown',
  channel: 'stable',
  isDocker: false,
  isSourceBuild: true,
  isDevelopment: false,
  deploymentType: 'source',
};
const NO_UPDATE_INFO = {
  available: false,
  currentVersion: STABLE_VERSION_INFO.version,
  latestVersion: STABLE_VERSION_INFO.version,
  releaseNotes: '',
  releaseDate: '',
  downloadUrl: '',
  isPrerelease: false,
};

async function stabilizeUpdateChecks(page: Page): Promise<void> {
  await page.addInitScript(() => {
    localStorage.removeItem('pulse-updates');
  });
  await page.route('**/api/version', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(STABLE_VERSION_INFO),
    }),
  );
  await page.route('**/api/updates/check*', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(NO_UPDATE_INFO),
    }),
  );
}

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
      [data-testid="update-banner"] {
        display: none !important;
      }
      [role="tab"] span[class*="rounded-full"] {
        display: none !important;
      }
      .tabs [role="tab"]:nth-last-child(-n + 3) {
        visibility: hidden !important;
      }
    `,
  });

  // Allow any final layout flushes after navigation and style injection.
  await page.waitForTimeout(150);
}

async function forceLoggedOutAuthSurface(page: Page): Promise<void> {
  let liveSecurityStatus: Record<string, unknown> = {};
  const securityStatusResponse = await page.request.get('/api/security/status');
  if (securityStatusResponse.ok()) {
    const parsed = await securityStatusResponse.json().catch(() => null);
    if (parsed && typeof parsed === 'object') {
      liveSecurityStatus = parsed as Record<string, unknown>;
    }
  }

  const forcedSecurityStatus = {
    ...liveSecurityStatus,
    hasAuthentication: true,
    hideLocalLogin: false,
    hasProxyAuth: false,
    proxyAuthUsername: '',
    proxyAuthLogoutURL: '',
    oidcEnabled: false,
    oidcClientId: '',
    oidcIssuer: '',
    oidcLogoutURL: '',
    oidcUsername: '',
    requiresAuth: true,
  };

  await page.route('**/api/security/status', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(forcedSecurityStatus),
    }),
  );

  await page.route('**/api/state', (route) =>
    route.fulfill({
      status: 401,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'unauthorized' }),
    }),
  );
}

async function openLoggedOutLogin(page: Page, theme: ThemePreference): Promise<void> {
  await waitForPulseReady(page);
  await maybeCompleteSetupWizard(page);
  await page.context().clearCookies();
  await page.addInitScript(() => {
    try {
      sessionStorage.clear();
    } catch {
      // Ignore storage access errors in constrained browser contexts.
    }

    try {
      localStorage.removeItem('pulse_auth');
      localStorage.removeItem('pulse_api_token');
      localStorage.removeItem('pulse_auth_user');
      localStorage.removeItem('pulse_org_id');
      localStorage.setItem('just_logged_out', 'true');
    } catch {
      // Ignore storage access errors in constrained browser contexts.
    }
  });
  await forceLoggedOutAuthSurface(page);

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
    await stabilizeUpdateChecks(page);
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
