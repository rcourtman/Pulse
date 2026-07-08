import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect, type Locator, type Page } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => {
    await use(authStorageStatePath);
  },
  authStorageStatePath: [async ({ browser }, use, workerInfo) => {
    const storageStatePath = path.resolve(
      __dirname,
      '..',
      '..',
      'tmp',
      'playwright-auth',
      `local-doc-links-${workerInfo.project.name}.json`,
    );
    fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
    await createAuthenticatedStorageState(browser, storageStatePath);
    try {
      await use(storageStatePath);
    } finally {
      fs.rmSync(storageStatePath, { force: true });
    }
  }, { scope: 'worker' }],
});

async function expectPopupDoc(
  page: Page,
  link: Locator,
  pathname: string,
  expectedText: string,
) {
  const [popup] = await Promise.all([
    page.waitForEvent('popup'),
    link.click(),
  ]);

  await popup.waitForLoadState('domcontentloaded');
  expect(new URL(popup.url()).pathname).toBe(pathname);
  await expect(popup.locator('body')).toContainText(expectedText);
  await popup.close();
}

test.describe('Local docs links', () => {
  test.setTimeout(180_000);

  test('security overview opens the shipped proxy auth guide', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only local docs coverage');

    await page.route('**/api/security/status', async (route) => {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          apiTokenConfigured: true,
          clientIP: '127.0.0.1',
          exportProtected: true,
          hasAPIToken: true,
          hasAuditLogging: true,
          hasAuthentication: true,
          hasHTTPS: false,
          hasProxyAuth: true,
          isPrivateNetwork: true,
          proxyAuthIsAdmin: true,
          proxyAuthLogoutURL: 'https://idp.example.test/logout',
          proxyAuthUsername: 'admin@example.test',
          publicAccess: false,
        }),
      });
    });

    await page.goto('/settings/security-overview', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings/, { timeout: 15_000 });

    const guideLink = page.getByRole('link', { name: /Read proxy auth guide/i });
    await expect(guideLink).toHaveAttribute('href', '/docs/PROXY_AUTH.md');
    await expectPopupDoc(page, guideLink, '/docs/PROXY_AUTH.md', 'Proxy Authentication');
  });

  test('api access opens the shipped token scope reference', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only local docs coverage');

    await page.goto('/settings/security/api', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings/, { timeout: 15_000 });

    const scopeReferenceLink = page.getByRole('link', { name: 'View scope reference' });
    await expect(scopeReferenceLink).toHaveAttribute('href', '/docs/CONFIGURATION.md');
    await expectPopupDoc(
      page,
      scopeReferenceLink,
      '/docs/CONFIGURATION.md',
      'API tokens provide scoped, revocable access to Pulse.',
    );
  });

  test('security warning opens the shipped security guide', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only local docs coverage');

    await page.addInitScript(() => {
      localStorage.removeItem('securityWarningDismissed');
    });

    await page.route('**/api/security/status', async (route) => {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          apiTokenConfigured: false,
          clientIP: '127.0.0.1',
          credentialsEncrypted: true,
          exportProtected: false,
          hasAPIToken: false,
          hasAuditLogging: false,
          hasAuthentication: true,
          hasHTTPS: false,
          isPrivateNetwork: true,
          publicAccess: false,
        }),
      });
    });

    await page.goto('/infrastructure', { waitUntil: 'domcontentloaded' });

    const securityGuideLink = page.getByRole('link', { name: 'Learn More' }).first();
    await expect(securityGuideLink).toHaveAttribute('href', '/docs/SECURITY.md');
    await expectPopupDoc(page, securityGuideLink, '/docs/SECURITY.md', 'Pulse Security');
  });

  test('ai runtime controls open the shipped terms doc', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only local docs coverage');

    // The autonomous chat-action option is gated on runtime autonomy, which
    // the community e2e runtime blocks; unblock it at the capability layer so
    // the terms disclosure it carries can be exercised.
    await page.route('**/api/license/runtime-capabilities', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          capabilities: ['ai_patrol', 'ai_alerts', 'ai_autofix'],
          limits: [],
          hosted_mode: false,
          max_history_days: 7,
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
          blocked_capabilities: [],
        }),
      });
    });

    // Chat action controls live on the Assistant panel; /settings/system-ai
    // canonicalizes to the Provider & Models panel.
    await page.goto('/settings/pulse-intelligence/assistant', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings/, { timeout: 15_000 });

    const permissionSelect = page
      .locator('select')
      .filter({ has: page.locator('option[value="autonomous"]') })
      .first();
    await permissionSelect.selectOption('autonomous');

    const termsLink = page.getByRole('link', { name: 'Terms of Service' }).first();
    await expect(termsLink).toHaveAttribute('href', '/docs/TERMS.md');
    await expectPopupDoc(
      page,
      termsLink,
      '/docs/TERMS.md',
      'Pulse Pro - Terms of Service & Software License Agreement',
    );
  });

  test('self-hosted commercial activation opens the shipped terms doc', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only local docs coverage');

    await page.goto('/settings/pulse-intelligence/billing/plan', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings/, { timeout: 15_000 });

    // The terms disclosure sits inside the collapsed manual key recovery
    // section unless the panel is already in a recovery posture.
    await page.getByText('Manual key recovery', { exact: true }).click();

    const termsLink = page.getByRole('link', { name: /terms of service/i }).first();
    await expect(termsLink).toHaveAttribute('href', '/docs/TERMS.md');
    await expectPopupDoc(
      page,
      termsLink,
      '/docs/TERMS.md',
      'Pulse Pro - Terms of Service & Software License Agreement',
    );
  });
});
