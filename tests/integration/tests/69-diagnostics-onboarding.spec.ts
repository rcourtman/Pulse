import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base, type Page } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

type OverflowAudit = {
  viewportWidth: number;
  pageWidth: number;
  overflowPx: number;
  offenders: Array<{ tag: string; className: string; overflow: number }>;
};

const DIAGNOSTICS_PAYLOAD = {
  version: '6.0.0',
  runtime: 'go',
  uptime: 3600,
  nodes: [],
  pbs: [],
  system: {
    os: 'linux',
    arch: 'amd64',
    goVersion: 'go1.25',
    numCPU: 8,
    numGoroutine: 32,
    memoryMB: 128,
  },
  metricsStore: {
    enabled: true,
    status: 'healthy',
    dbSize: 4 * 1024 * 1024,
    rawCount: 12,
    minuteCount: 24,
    hourlyCount: 36,
    dailyCount: 48,
    totalPoints: 120,
    bufferSize: 0,
    notes: [],
  },
  commercialFunnel: {
    enabled: true,
    status: 'active',
    windowDays: 30,
    summary: {
      pricing_viewed: 3,
      paywall_viewed: 0,
      trial_started: 1,
      upgrade_clicked: 0,
      checkout_clicked: 2,
      checkout_started: 2,
      checkout_completed: 1,
      license_activated: 1,
      license_activation_failed: 0,
      period: {
        from: '2026-03-19T00:00:00Z',
        to: '2026-04-18T00:00:00Z',
      },
    },
    daily: [
      {
        day: '2026-04-17',
        pricing_viewed: 1,
        paywall_viewed: 0,
        trial_started: 0,
        upgrade_clicked: 0,
        checkout_clicked: 1,
        checkout_started: 1,
        checkout_completed: 0,
        license_activated: 0,
        license_activation_failed: 0,
      },
    ],
    surfaces: [
      {
        key: 'settings_self_hosted_billing_compare_prompt',
        pricing_viewed: 0,
        paywall_viewed: 0,
        trial_started: 0,
        upgrade_clicked: 0,
        checkout_clicked: 2,
        checkout_started: 0,
        checkout_completed: 0,
        license_activated: 0,
        license_activation_failed: 0,
      },
    ],
    capabilities: [
      {
        key: 'self_hosted_plan',
        pricing_viewed: 3,
        paywall_viewed: 0,
        trial_started: 0,
        upgrade_clicked: 0,
        checkout_clicked: 2,
        checkout_started: 2,
        checkout_completed: 1,
        license_activated: 1,
        license_activation_failed: 0,
      },
    ],
    notes: ['Local pricing and activation events show at least one completed conversion.'],
  },
  infrastructureOnboarding: {
    enabled: true,
    status: 'warning',
    windowDays: 30,
    summary: {
      opened: 4,
      api_path_selected: 2,
      agent_path_selected: 1,
      probe_detected: 1,
      probe_no_match: 2,
      probe_error: 0,
      catalog_selected: 2,
      credentials_opened: 1,
      period: {
        from: '2026-03-19T00:00:00Z',
        to: '2026-04-18T00:00:00Z',
      },
    },
    daily: [
      {
        day: '2026-04-17',
        opened: 2,
        api_path_selected: 1,
        agent_path_selected: 1,
        probe_detected: 0,
        probe_no_match: 1,
        probe_error: 0,
        catalog_selected: 1,
        credentials_opened: 0,
      },
      {
        day: '2026-04-18',
        opened: 2,
        api_path_selected: 1,
        agent_path_selected: 0,
        probe_detected: 1,
        probe_no_match: 1,
        probe_error: 0,
        catalog_selected: 1,
        credentials_opened: 1,
      },
    ],
    paths: [
      { key: 'api', count: 2 },
      { key: 'agent', count: 1 },
    ],
    platforms: [
      { key: 'truenas', catalog_selected: 2, credentials_opened: 1 },
    ],
    notes: ['More probed addresses miss than detect a supported API-backed platform.'],
  },
  discovery: {
    enabled: true,
    configuredSubnet: 'auto',
    scanInterval: '5m',
    lastResultServers: 3,
  },
  apiTokens: {
    enabled: true,
    tokenCount: 2,
    recommendTokenSetup: false,
    unusedTokenCount: 0,
    notes: [],
  },
  dockerAgents: {
    agentsTotal: 1,
    agentsOnline: 1,
    agentsReportingVersion: 1,
    agentsWithTokenBinding: 1,
    agentsWithoutTokenBinding: 0,
    agentsNeedingAttention: 0,
    notes: [],
  },
  alerts: {
    missingCooldown: false,
    missingGroupingWindow: false,
    notes: [],
  },
  aiChat: {
    enabled: true,
    running: true,
    healthy: true,
    mcpConnected: true,
    notes: [],
  },
  errors: [],
  nodeSnapshots: [],
  guestSnapshots: [],
  memorySources: [],
  memorySourceBreakdown: [],
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
      `diagnostics-onboarding-${workerInfo.project.name}.json`,
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

async function prepareDiagnosticsRoute(page: Page): Promise<void> {
  await page.route('**/api/diagnostics', async (route) => {
    const requestUrl = new URL(route.request().url());
    if (route.request().method() !== 'GET' || requestUrl.pathname !== '/api/diagnostics') {
      await route.continue();
      return;
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(DIAGNOSTICS_PAYLOAD),
    });
  });
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

test.describe('Diagnostics onboarding analytics', () => {
  test.setTimeout(180_000);

  test('desktop diagnostics renders the onboarding analytics card in the shared settings shell', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only diagnostics shell coverage');

    await prepareDiagnosticsRoute(page);

    await page.goto('/settings/support/diagnostics', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings\/support\/diagnostics$/, { timeout: 15_000 });

    await expect(page.getByRole('heading', { name: 'Diagnostics & Health' })).toBeVisible();
    await page.getByRole('button', { name: 'Run Diagnostics', exact: true }).first().click();

    await expect(page.getByText('Commercial Funnel', { exact: true })).toBeVisible();
    await expect(page.getByText('Infrastructure Onboarding', { exact: true })).toBeVisible();
    await expect(page.getByText('Credentials Opened', { exact: true })).toBeVisible();
    await expect(page.getByText('TrueNAS SCALE', { exact: true })).toBeVisible();
    await expect(page.getByText('API', { exact: true })).toBeVisible();
    await expect(
      page.getByText('More probed addresses miss than detect a supported API-backed platform.', {
        exact: true,
      }),
    ).toBeVisible();
  });

  test('mobile diagnostics keeps the populated onboarding analytics page inside the viewport', async ({
    page,
  }, testInfo) => {
    test.skip(!testInfo.project.name.startsWith('mobile-'), 'Mobile-only diagnostics overflow coverage');

    await prepareDiagnosticsRoute(page);

    await page.goto('/settings/support/diagnostics', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings\/support\/diagnostics$/, { timeout: 15_000 });

    await page.getByRole('button', { name: 'Run', exact: true }).click();
    await expect(page.getByText('Infrastructure Onboarding', { exact: true })).toBeVisible();

    await scrollToBottom(page);
    const audit = await auditHorizontalOverflow(page);

    expect(
      audit.pageWidth,
      `Mobile diagnostics overflow (viewport=${audit.viewportWidth}, page=${audit.pageWidth}, offenders=${JSON.stringify(audit.offenders)})`,
    ).toBeLessThanOrEqual(audit.viewportWidth + 1);
  });
});
