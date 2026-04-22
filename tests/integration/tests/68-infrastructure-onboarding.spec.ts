import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base, type Page } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

type UpgradeMetricEventPayload = {
  type: string;
  surface: string;
  capability?: string;
  idempotency_key?: string;
};

type OverflowAudit = {
  viewportWidth: number;
  pageWidth: number;
  overflowPx: number;
  offenders: Array<{ tag: string; className: string; overflow: number }>;
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
      `infrastructure-onboarding-${workerInfo.project.name}.json`,
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

async function stubConnectionsList(page: Page): Promise<void> {
  await page.route('**/api/connections', async (route) => {
    const requestUrl = new URL(route.request().url());
    if (route.request().method() !== 'GET' || requestUrl.pathname !== '/api/connections') {
      await route.continue();
      return;
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ connections: [] }),
    });
  });
}

async function recordUpgradeMetricEvents(page: Page): Promise<UpgradeMetricEventPayload[]> {
  const events: UpgradeMetricEventPayload[] = [];

  await page.route('**/api/upgrade-metrics/events', async (route) => {
    const payload = route.request().postDataJSON() as UpgradeMetricEventPayload;
    events.push(payload);
    await route.fulfill({
      status: 202,
      contentType: 'application/json',
      body: '{}',
    });
  });

  return events;
}

async function prepareOnboardingPage(page: Page): Promise<void> {
  await page.addInitScript(() => {
    localStorage.setItem('pulse_whats_new_v2_shown', 'true');
  });

  await stubConnectionsList(page);
}

function countUpgradeEvents(
  events: readonly UpgradeMetricEventPayload[],
  type: string,
  capability?: string,
): number {
  const matchingKeys = new Set<string>();

  for (const event of events) {
    if (event.surface !== 'settings_infrastructure_add') continue;
    if (event.type !== type) continue;
    if (capability !== undefined && event.capability !== capability) continue;

    matchingKeys.add(
      event.idempotency_key ??
        `${event.type}:${event.surface}:${event.capability ?? ''}:${matchingKeys.size}`,
    );
  }

  return matchingKeys.size;
}

async function scrollToBottom(page: Page): Promise<void> {
  const viewportHeight = await page.evaluate(() => window.innerHeight || 800);
  const step = Math.max(240, Math.floor(viewportHeight * 0.75));
  let wheelSupported = true;

  for (let i = 0; i < 20; i += 1) {
    if (wheelSupported) {
      try {
        await page.mouse.wheel(0, step);
      } catch {
        wheelSupported = false;
        await page.evaluate((deltaY) => window.scrollBy(0, deltaY), step);
      }
    } else {
      await page.evaluate((deltaY) => window.scrollBy(0, deltaY), step);
    }
    await page.waitForTimeout(60);
  }
}

async function auditHorizontalOverflow(page: Page): Promise<OverflowAudit> {
  return page.evaluate(() => {
    const viewportWidth = Math.max(document.documentElement.clientWidth, window.innerWidth || 0);
    const pageWidth = Math.max(
      document.body.scrollWidth,
      document.documentElement.scrollWidth,
      document.body.offsetWidth,
      document.documentElement.offsetWidth,
    );

    const offenders = Array.from(document.querySelectorAll('body *'))
      .map((el) => {
        const rect = el.getBoundingClientRect();
        if (rect.width <= 0 || rect.height <= 0) return null;
        const style = window.getComputedStyle(el);
        if (style.position === 'fixed' || style.position === 'absolute') return null;
        const overflow = rect.right - viewportWidth;
        if (overflow <= 1) return null;
        return {
          tag: el.tagName.toLowerCase(),
          className: (el.getAttribute('class') || '').trim().slice(0, 120),
          overflow: Number(overflow.toFixed(1)),
        };
      })
      .filter((entry): entry is { tag: string; className: string; overflow: number } => Boolean(entry))
      .slice(0, 8);

    return {
      viewportWidth,
      pageWidth,
      overflowPx: Number((pageWidth - viewportWidth).toFixed(1)),
      offenders,
    };
  });
}

test.describe('Infrastructure onboarding', () => {
  test.setTimeout(180_000);

  test('desktop landing shows platform sections before any onboarding flow opens', async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith('mobile-'),
      'Desktop-only infrastructure manager coverage',
    );

    const metricEvents = await recordUpgradeMetricEvents(page);
    await prepareOnboardingPage(page);

    await page.goto('/settings/infrastructure', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings\/infrastructure(?:\?.*)?$/, { timeout: 15_000 });

    await expect(page.getByText('Infrastructure sources', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: /Run discovery/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /Discovery settings/i })).toBeVisible();
    await expect(page.getByText('VMware vCenter', { exact: true })).toBeVisible();
    await expect(page.getByText('TrueNAS SCALE', { exact: true })).toBeVisible();
    await expect(page.getByText('Proxmox VE', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: /Add TrueNAS SCALE/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /Detect from address/i })).toBeVisible();
    await expect(page.getByText('Monitored systems', { exact: true })).toHaveCount(0);
    await expect(page.getByText('Connection types', { exact: true })).toHaveCount(0);

    await expect
      .poll(() => countUpgradeEvents(metricEvents, 'infrastructure_onboarding_opened'))
      .toBe(0);
  });

  test('desktop per-platform add opens the matching modal and records direct type onboarding', async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith('mobile-'),
      'Desktop-only manager add-path instrumentation coverage',
    );

    const metricEvents = await recordUpgradeMetricEvents(page);
    await prepareOnboardingPage(page);

    await page.goto('/settings/infrastructure', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings\/infrastructure(?:\?.*)?$/, { timeout: 15_000 });

    await page.getByRole('button', { name: /Add TrueNAS SCALE/i }).click();
    await page.waitForURL(/\/settings\/infrastructure\?add=truenas$/, { timeout: 15_000 });

    await expect(page.getByText('Infrastructure sources', { exact: true })).toBeVisible();
    await expect(page.getByRole('dialog')).toBeVisible();
    await expect(
      page.getByRole('dialog').getByRole('heading', { name: 'Add TrueNAS SCALE', exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole('button', { name: 'Close add infrastructure dialog', exact: true }),
    ).toBeVisible();

    await expect
      .poll(() => countUpgradeEvents(metricEvents, 'infrastructure_onboarding_opened'))
      .toBe(1);
    await expect
      .poll(() =>
        countUpgradeEvents(metricEvents, 'infrastructure_onboarding_path_selected', 'api'),
      )
      .toBe(1);
    await expect
      .poll(() =>
        countUpgradeEvents(metricEvents, 'infrastructure_onboarding_catalog_selected', 'truenas'),
      )
      .toBe(1);
    await expect
      .poll(() =>
        countUpgradeEvents(metricEvents, 'infrastructure_onboarding_credentials_opened', 'truenas'),
      )
      .toBe(1);
  });

  test('desktop explicit discovery scan surfaces a candidate row and opens a prefilled review dialog', async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith('mobile-'),
      'Desktop-only discovery-candidate review coverage',
    );

    const metricEvents = await recordUpgradeMetricEvents(page);
    await prepareOnboardingPage(page);

    await page.route('**/api/discover', async (route) => {
      const requestUrl = new URL(route.request().url());
      if (requestUrl.pathname !== '/api/discover') {
        await route.continue();
        return;
      }

      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            servers: [],
            errors: [],
            cached: true,
            updated: 0,
            age: 0,
          }),
        });
        return;
      }

      if (route.request().method() === 'POST') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            servers: [
              {
                ip: '10.0.0.55',
                port: 8006,
                type: 'pve',
                version: '8.2.2',
                hostname: 'discovered-pve.lab',
              },
            ],
            errors: [],
            cached: false,
            scanning: false,
            timestamp: 1_700_000_000_000,
          }),
        });
        return;
      }

      await route.continue();
    });

    await page.goto('/settings/infrastructure', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings\/infrastructure(?:\?.*)?$/, { timeout: 15_000 });

    await page.getByRole('button', { name: /Run discovery/i }).click();

    await expect(page.getByText('discovered-pve.lab', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: /^Review$/i })).toBeVisible();

    await page.getByRole('button', { name: /^Review$/i }).click();
    await page.waitForURL(/\/settings\/infrastructure\?add=pve$/, { timeout: 15_000 });

    await expect(page.getByRole('dialog')).toBeVisible();
    await expect(
      page.getByRole('dialog').getByRole('heading', { name: 'Add Proxmox VE', exact: true }),
    ).toBeVisible();
    await expect(
      page.getByPlaceholder('https://proxmox.example.com:8006'),
    ).toHaveValue('https://discovered-pve.lab:8006');

    await expect
      .poll(() => countUpgradeEvents(metricEvents, 'infrastructure_onboarding_opened'))
      .toBe(1);
    await expect
      .poll(() =>
        countUpgradeEvents(metricEvents, 'infrastructure_onboarding_path_selected', 'api'),
      )
      .toBe(1);
    await expect
      .poll(() =>
        countUpgradeEvents(metricEvents, 'infrastructure_onboarding_credentials_opened', 'pve'),
      )
      .toBe(1);
    await expect
      .poll(() =>
        countUpgradeEvents(metricEvents, 'infrastructure_onboarding_catalog_selected', 'pve'),
      )
      .toBe(0);
  });

  test('desktop detect utility records no-match agent fallback from the landing header action', async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith('mobile-'),
      'Desktop-only onboarding probe instrumentation coverage',
    );

    const metricEvents = await recordUpgradeMetricEvents(page);
    await prepareOnboardingPage(page);

    await page.route('**/api/connections/probe', async (route) => {
      const requestUrl = new URL(route.request().url());
      if (route.request().method() !== 'POST' || requestUrl.pathname !== '/api/connections/probe') {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          candidates: [],
          probedMs: 184,
        }),
      });
    });

    await page.goto('/settings/infrastructure', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings\/infrastructure(?:\?.*)?$/, { timeout: 15_000 });
    await page.getByRole('button', { name: /Detect from address/i }).click();
    await page.waitForURL(/\/settings\/infrastructure\?add=detect$/, { timeout: 15_000 });

    await page.getByLabel('Address').fill('baremetal.lab');
    await page.getByRole('button', { name: 'Probe address', exact: true }).click();

    await expect(
      page.getByText('No supported API-backed platform detected at that address.', { exact: true }),
    ).toBeVisible();

    await expect
      .poll(() => countUpgradeEvents(metricEvents, 'infrastructure_onboarding_path_selected', 'api'))
      .toBe(1);
    await expect
      .poll(() =>
        countUpgradeEvents(metricEvents, 'infrastructure_onboarding_probe_result', 'no-match'),
      )
      .toBe(1);

    await page.getByRole('button', { name: 'install Pulse Agent instead', exact: true }).click();
    await expect(page.getByRole('button', { name: /Back to detect/i })).toBeVisible();

    await expect
      .poll(() =>
        countUpgradeEvents(metricEvents, 'infrastructure_onboarding_path_selected', 'agent'),
      )
      .toBe(1);
    await expect
      .poll(() =>
        countUpgradeEvents(metricEvents, 'infrastructure_onboarding_credentials_opened', 'agent'),
      )
      .toBe(1);
  });

  test('mobile landing and grouped platform tables stay inside the viewport', async ({
    page,
  }, testInfo) => {
    test.skip(
      !testInfo.project.name.startsWith('mobile-'),
      'Mobile-only infrastructure manager overflow coverage',
    );

    await prepareOnboardingPage(page);

    await page.goto('/settings/infrastructure', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings\/infrastructure(?:\?.*)?$/, { timeout: 15_000 });
    await expect(page.getByText('Infrastructure sources', { exact: true })).toBeVisible();
    await expect(page.getByText('Pulse Agent', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: /Add Pulse Agent/i })).toBeVisible();

    await scrollToBottom(page);
    const audit = await auditHorizontalOverflow(page);

    expect(
      audit.pageWidth,
      `Mobile onboarding overflow (viewport=${audit.viewportWidth}, page=${audit.pageWidth}, offenders=${JSON.stringify(audit.offenders)})`,
    ).toBeLessThanOrEqual(audit.viewportWidth + 1);
  });
});
