import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { expect, test as base, type Page } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => use(authStorageStatePath),
  authStorageStatePath: [
    async ({ browser }, use, workerInfo) => {
      const storageStatePath = path.resolve(
        __dirname,
        '..',
        '..',
        'tmp',
        'playwright-auth',
        `alert-operator-qualification-${workerInfo.project.name}.json`,
      );
      fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
      await createAuthenticatedStorageState(browser, storageStatePath);
      try {
        await use(storageStatePath);
      } finally {
        fs.rmSync(storageStatePath, { force: true });
      }
    },
    { scope: 'worker' },
  ],
});

const now = Date.now();
const minutesAgo = (minutes: number) => new Date(now - minutes * 60_000).toISOString();

const qualificationHistory = [
  {
    id: 'qualification-database-critical',
    type: 'cpu',
    level: 'critical',
    resourceId: 'agent:database-primary',
    resourceName: 'Database Primary',
    node: 'database-node.internal.example',
    nodeDisplayName: 'Database Node With A Deliberately Long Operator-Facing Label',
    instance: 'qualification',
    message:
      'CPU saturation persisted while a deliberately long diagnostic message remained readable without widening the alert history page.',
    value: 97,
    threshold: 90,
    startTime: minutesAgo(25),
    lastSeen: minutesAgo(20),
    acknowledged: false,
    metadata: { resourceType: 'agent' },
  },
  {
    id: 'qualification-cache-warning',
    type: 'memory',
    level: 'warning',
    resourceId: 'agent:cache-primary',
    resourceName: 'Cache Primary',
    node: 'cache-node',
    instance: 'qualification',
    message: 'Memory pressure crossed the warning threshold.',
    value: 84,
    threshold: 80,
    startTime: minutesAgo(65),
    lastSeen: minutesAgo(50),
    acknowledged: false,
    metadata: { resourceType: 'agent' },
  },
  {
    id: 'qualification-router-critical',
    type: 'host-offline',
    level: 'critical',
    resourceId: 'agent:edge-router',
    resourceName: 'Edge Router',
    node: 'edge-router',
    instance: 'qualification',
    message: 'The edge router stopped reporting.',
    value: 0,
    threshold: 0,
    startTime: minutesAgo(125),
    lastSeen: minutesAgo(115),
    acknowledged: true,
    metadata: { resourceType: 'agent' },
  },
];

const activeDeliveryAlert = {
  id: 'qualification-delivery-alert',
  type: 'cpu',
  level: 'critical',
  resourceId: 'agent:delivery-node',
  resourceName: 'Delivery Node',
  node: 'delivery-node',
  instance: 'qualification',
  message: 'CPU saturation is waiting behind the hourly notification limit.',
  value: 96,
  threshold: 90,
  startTime: minutesAgo(10),
  lastSeen: minutesAgo(1),
  acknowledged: false,
  metadata: { resourceType: 'agent' },
};

async function routeStateWithActiveAlerts(page: Page, activeAlerts: unknown[]) {
  await page.routeWebSocket('**/ws*', (webSocket) => {
    webSocket.send(
      JSON.stringify({
        type: 'initialState',
        data: {
          resources: [],
          connectedInfrastructure: [],
          activeAlerts,
          recentlyResolved: [],
          capabilityCatalog: {},
          policyCatalog: {},
        },
      }),
    );
  });
}

async function routeAlertHistory(page: Page, history: unknown[]) {
  await page.route('**/api/alerts/config', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        enabled: true,
        activationState: 'active',
        overrides: {},
      }),
    }),
  );
  await page.route('**/api/alerts/active', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }),
  );
  await page.route('**/api/alerts/history**', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(history),
    }),
  );
}

async function expectNoHorizontalOverflow(page: Page) {
  await expect
    .poll(() =>
      page.evaluate(() => ({
        clientWidth: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      })),
    )
    .toEqual(
      expect.objectContaining({
        clientWidth: expect.any(Number),
        scrollWidth: expect.any(Number),
      }),
    );
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflow, 'Alert history must stay inside the viewport').toBeLessThanOrEqual(1);
}

test.describe('Alert operator qualification', () => {
  test('keeps History filters in the URL across refresh and browser navigation', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 1000 });
    await routeStateWithActiveAlerts(page, []);
    await routeAlertHistory(page, qualificationHistory);

    await page.goto('/alerts/history', { waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('heading', { name: 'Alert History' })).toBeVisible();
    const desktopHistory = page.locator('table.alert-history-responsive-table');
    await expect(desktopHistory.getByText('Database Primary')).toBeVisible();

    const search = page.getByPlaceholder('Search alerts...');
    await search.fill('database');
    await expect.poll(() => new URL(page.url()).searchParams.get('q')).toBe('database');

    await page
      .getByRole('group', { name: 'Severity' })
      .getByRole('button', { name: /Critical/ })
      .click();
    await page.getByRole('group', { name: 'Period' }).getByRole('button', { name: 'All time' }).click();
    await expect.poll(() => new URL(page.url()).searchParams.get('severity')).toBe('critical');
    await expect.poll(() => new URL(page.url()).searchParams.get('period')).toBe('all');
    await expect(desktopHistory.getByText('Database Primary')).toBeVisible();
    await expect(page.getByText('Cache Primary')).toHaveCount(0);

    await page.reload({ waitUntil: 'domcontentloaded' });
    await expect(search).toHaveValue('database');
    await expect(
      page.getByRole('group', { name: 'Severity' }).getByRole('button', { name: /Critical/ }),
    ).toHaveAttribute('aria-pressed', 'true');
    await expect(page.getByRole('group', { name: 'Period' }).getByRole('button', { name: 'All time' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );

    await page.getByRole('button', { name: 'Overview', exact: true }).click();
    await expect(page).toHaveURL(/\/alerts\/overview$/);
    await page.goBack({ waitUntil: 'domcontentloaded' });
    await expect.poll(() => new URL(page.url()).searchParams.get('q')).toBe('database');
    await page.goForward({ waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/alerts\/overview$/);
    await page.goBack({ waitUntil: 'domcontentloaded' });

    await page.getByRole('button', { name: 'Clear filters' }).click();
    await expect.poll(() => new URL(page.url()).search).toBe('');
    await expect(desktopHistory.getByText('Cache Primary')).toBeVisible();
  });

  test('keeps long History content readable, keyboard reachable, and contained at 390px', async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await routeStateWithActiveAlerts(page, []);
    await routeAlertHistory(page, qualificationHistory);

    await page.goto('/alerts/history', { waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('heading', { name: 'Alert History' })).toBeVisible();
    const mobileList = page.getByTestId('alert-history-mobile-list');
    await expect(mobileList).toBeVisible();
    const databaseCard = mobileList.locator('article').filter({ hasText: 'Database Primary' });
    await expect(databaseCard).toContainText('deliberately long diagnostic message');
    await expect(databaseCard).toContainText('Database Node With A Deliberately Long');

    const visibleAxisLabels = page.locator('[data-testid="alert-frequency-axis-label"]:visible');
    await expect(visibleAxisLabels).toHaveCount(3);
    const axisBoxes = (
      await visibleAxisLabels.evaluateAll((labels) =>
        labels.map((label) => {
          const box = label.getBoundingClientRect();
          return { left: box.left, right: box.right };
        }),
      )
    ).sort((a, b) => a.left - b.left);
    for (let index = 1; index < axisBoxes.length; index += 1) {
      expect(
        axisBoxes[index - 1]!.right,
        'Mobile alert-frequency axis labels must not overlap',
      ).toBeLessThanOrEqual(axisBoxes[index]!.left);
    }

    const search = page.getByPlaceholder('Search alerts...');
    await search.focus();
    await expect(search).toBeFocused();
    await search.fill('cache');
    await expect(page.getByText('Cache Primary').first()).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(search).toHaveValue('');

    const filtersButton = page.getByRole('button', { name: /Filters/ });
    await filtersButton.focus();
    await expect(filtersButton).toBeFocused();
    await page.keyboard.press('Enter');
    await expect(page.getByRole('group', { name: 'Severity' })).toBeVisible();
    await expectNoHorizontalOverflow(page);

    await testInfo.attach('alert-history-390px', {
      body: await page.screenshot({ fullPage: true }),
      contentType: 'image/png',
    });
  });

  test('shows per-alert delivery diagnosis and interleaves held notification evidence', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 1000 });
    await routeStateWithActiveAlerts(page, [activeDeliveryAlert]);
    await page.route('**/api/alerts/config', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          enabled: true,
          activationState: 'active',
          overrides: {},
        }),
      }),
    );
    await page.route('**/api/alerts/delivery-diagnosis', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          {
            alertIdentifier: activeDeliveryAlert.id,
            alertId: activeDeliveryAlert.id,
            status: 'suppressed',
            reason: 'rate_limited',
            message: 'Hourly notification limit reached.',
          },
        ]),
      }),
    );
    await page.route('**/api/notifications/health', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          overall_healthy: true,
          queue: {
            status: 'healthy',
            healthy: true,
            pending: 0,
            sending: 0,
            sent: 1,
            failed: 0,
            dlq: 0,
            terminal_failure_count: 0,
            attention_required: 0,
            reason_codes: [],
            completed_retention_days: 7,
            dead_letter_retention_days: 30,
            counts_are_retention_bounded: true,
            retry_attempts_affect_health: false,
            terminal_failures_affect_health: true,
            failure_classes_7d: {},
            failure_classes_available: true,
            failure_class_window_days: 7,
          },
        }),
      }),
    );
    await page.route('**/api/notifications/delivery-log**', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          window_days: 7,
          entries: [
            {
              notificationId: 'attempt-1',
              timestamp: minutesAgo(4),
              type: 'email',
              destinationId: 'email:qualification',
              outcome: 'sent',
              success: true,
              alertIds: ['qualification-delivered-alert'],
              alertCount: 1,
              attempts: 1,
            },
          ],
        }),
      }),
    );
    let heldEventsAvailable = true;
    await page.route('**/api/alerts/events**', (route) =>
      route.fulfill({
        status: heldEventsAvailable ? 200 : 503,
        contentType: 'application/json',
        body: heldEventsAvailable
          ? JSON.stringify([
              {
                id: 'held-1',
                type: 'notification_deferred',
                alertId: activeDeliveryAlert.id,
                alertType: activeDeliveryAlert.type,
                resourceName: activeDeliveryAlert.resourceName,
                reason: 'quiet_hours',
                message: 'Held until quiet hours end.',
                occurredAt: minutesAgo(2),
              },
            ])
          : JSON.stringify({ error: 'event log unavailable' }),
      }),
    );

    await page.goto('/alerts/overview', { waitUntil: 'domcontentloaded' });
    await expect(page.getByText('Delivery Node').first()).toBeVisible();
    await expect(page.getByText('Hourly notification limit reached')).toBeVisible();

    await page.goto('/alerts/notifications', { waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('heading', { name: 'Notifications', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Delivery activity' })).toBeVisible();
    await expect(page.getByText('Deferred')).toBeVisible();
    await expect(page.getByText('Quiet hours')).toBeVisible();
    await expect(page.getByText('Delivered', { exact: true })).toBeVisible();

    heldEventsAvailable = false;
    const refresh = page
      .getByRole('heading', { name: 'Recent delivery activity' })
      .locator('..')
      .locator('..')
      .getByRole('button', { name: 'Refresh delivery status' });
    await refresh.focus();
    await page.keyboard.press('Enter');
    await expect(page.getByText('Deferred')).toHaveCount(0);
    await expect(page.getByText('Delivered', { exact: true })).toBeVisible();
    await expect(page.getByText(/Delivery history is unavailable/i)).toHaveCount(0);
  });

  test('mounts a bounded runway and renders 10,000 History records within budget', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== 'chromium', 'One Chromium scale measurement owns this budget');
    test.setTimeout(120_000);
    await page.setViewportSize({ width: 1440, height: 1000 });
    await routeStateWithActiveAlerts(page, []);
    const scaleHistory = Array.from({ length: 10_000 }, (_, index) => ({
      id: `qualification-scale-${index}`,
      type: index % 3 === 0 ? 'cpu' : 'memory',
      level: index % 5 === 0 ? 'critical' : 'warning',
      resourceId: `agent:scale-${index}`,
      resourceName: `Scale Resource ${String(index).padStart(5, '0')}`,
      node: `scale-node-${index % 250}`,
      instance: 'qualification',
      message: `Qualification record ${index}`,
      value: 80 + (index % 20),
      threshold: 80,
      startTime: new Date(now - index * 30_000).toISOString(),
      lastSeen: new Date(now - index * 30_000 + 10_000).toISOString(),
      acknowledged: false,
      metadata: { resourceType: 'agent' },
    }));
    await routeAlertHistory(page, scaleHistory);

    const startedAt = Date.now();
    await page.goto('/alerts/history', { waitUntil: 'domcontentloaded' });
    await expect(page.locator('table.alert-history-responsive-table').getByText('Scale Resource 00000')).toBeVisible();
    const renderedAtMs = Date.now() - startedAt;

    const mountedRows = await page.locator('tbody tr').count();
    expect(mountedRows, '10,000 records must not become 10,000 mounted table rows').toBeLessThan(180);
    await expect(
      page.locator('table.alert-history-responsive-table').locator('[data-platform-window-spacer="bottom"]'),
    ).toHaveCount(1);
    expect(renderedAtMs, `10,000-row History first render took ${renderedAtMs}ms`).toBeLessThan(8_000);

    console.log(
      `[qualification] History 10,000-row first render: ${renderedAtMs}ms; mounted rows: ${mountedRows}`,
    );

    await testInfo.attach('alert-history-10000-performance.json', {
      body: Buffer.from(JSON.stringify({ recordCount: scaleHistory.length, mountedRows, renderedAtMs }, null, 2)),
      contentType: 'application/json',
    });
  });
});
