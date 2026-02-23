import { expect, test } from '@playwright/test';
import { completeStripeSandboxCheckout } from './stripe-sandbox';

function cloudBaseURL(): string {
  const base =
    process.env.PULSE_CLOUD_BASE_URL ||
    process.env.PULSE_BASE_URL ||
    process.env.PLAYWRIGHT_BASE_URL ||
    'http://localhost:7655';
  return base.replace(/\/+$/, '');
}

function signupIdentity() {
  const suffix = `${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`;
  const domain = (process.env.PULSE_E2E_CLOUD_EMAIL_DOMAIN || 'example.com').trim() || 'example.com';
  return {
    email: `cloud-e2e-${suffix}@${domain}`,
    orgName: `Cloud E2E ${suffix}`,
    cardholderName: `Cloud E2E ${suffix}`,
  };
}

test.describe.serial('Cloud hosting public signup flows', () => {
  let createdEmail = '';

  test('completes hosted cloud signup checkout through Stripe sandbox', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only cloud hosting workflow coverage');

    const baseURL = cloudBaseURL();
    const identity = signupIdentity();
    createdEmail = identity.email;

    await page.goto(`${baseURL}/cloud/signup`);
    await expect(page.getByRole('heading', { name: /start pulse cloud|pulse cloud signup/i })).toBeVisible();

    const cpEmailInput = page.locator('#email');
    if (await cpEmailInput.isVisible({ timeout: 2_000 }).catch(() => false)) {
      await cpEmailInput.fill(identity.email);
      await page.locator('#org_name').fill(identity.orgName);
      await page.getByRole('button', { name: /continue to secure checkout/i }).click();
    } else {
      await page.locator('#hosted-email').fill(identity.email);
      await page.locator('#hosted-org-name').fill(identity.orgName);
      await page.getByRole('button', { name: /create hosted workspace/i }).click();
    }

    await completeStripeSandboxCheckout(page, {
      email: identity.email,
      cardholderName: identity.cardholderName,
    });

    await page.waitForURL(/\/(cloud\/)?signup\/complete/, { timeout: 120_000 });
    await expect(page.getByRole('heading', { name: /signup received/i })).toBeVisible();
    await expect(page.getByText(/checkout completed|provisioning your workspace/i)).toBeVisible();
  });

  test('accepts magic-link request via real public endpoint', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only cloud hosting workflow coverage');

    const baseURL = cloudBaseURL();
    const email = createdEmail || signupIdentity().email;
    const response = await page.request.post(`${baseURL}/api/public/magic-link/request`, {
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      data: { email },
    });

    expect(response.ok(), `magic-link request failed: HTTP ${response.status()}`).toBeTruthy();
    const payload = (await response.json()) as { success?: boolean; message?: string };
    expect(payload.success).toBe(true);
    expect(payload.message || '').toMatch(/magic link/i);
  });
});
