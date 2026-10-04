const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const output = '/workspace/tmp/notification-evidence-proof';
const root = '/workspace/frontend-modern';
const runtime = [
  'frontend-modern/src/features/alerts/AlertDeliveryLogCard.tsx',
  'frontend-modern/src/features/alerts/useNotificationDeliveryLog.ts',
  'frontend-modern/src/features/alerts/useAlertDestinationsTabState.ts',
  'frontend-modern/src/features/alerts/tabs/DestinationsTab.tsx',
  'frontend-modern/src/utils/alertDestinationsPresentation.ts',
];
const log = (entries = []) => ({
  entries,
  window_days: 30,
  completed_retention_days: 7,
  dead_letter_retention_days: 30,
});
const sent = {
  notificationId: 'synthetic-attempt',
  type: 'email',
  outcome: 'sent',
  alertIds: ['synthetic-disk-alert'],
  alertCount: 1,
  attempts: 1,
  success: true,
  timestamp: '2026-10-03T10:00:00Z',
};
const held = [
  {
    id: 42,
    type: 'notification_deferred',
    alertId: 'synthetic-held-alert',
    resourceName: 'synthetic-nas',
    alertType: 'usage',
    reason: 'quiet_hours:performance',
    occurredAt: '2026-10-03T10:01:00Z',
  },
];
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const playwrightVersion = require('playwright/package.json').version;
  assert.equal(
    playwrightVersion,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const hashes = Object.fromEntries(
    runtime.map((p) => [
      p,
      crypto
        .createHash('sha256')
        .update(fs.readFileSync(path.join('/workspace', p)))
        .digest('hex'),
    ]),
  );
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5291, strictPort: true },
  });
  const results = [],
    screens = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1365, false],
      ['phone', webkit, 390, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: width === 390 ? 844 : 900 },
        isMobile: width === 390,
        hasTouch: width === 390,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(20000);
      const errors = [],
        offOrigin = [],
        writes = [],
        requests = [],
        checks = [],
        releases = [];
      let attemptMode = 'hold-empty',
        heldMode = 'hold-empty';
      page.on('pageerror', (error) => errors.push(error.message));
      await page.route('**/*', async (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== 'http://127.0.0.1:5291') {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (request.method() !== 'GET') {
          writes.push({ method: request.method(), path: url.pathname });
          return route.abort();
        }
        if (
          url.pathname === '/api/notifications/delivery-log' ||
          url.pathname === '/api/alerts/events'
        ) {
          const isHeld = url.pathname === '/api/alerts/events';
          const mode = isHeld ? heldMode : attemptMode;
          requests.push({ path: url.pathname, query: url.search, mode });
          if (mode.startsWith('hold-'))
            await new Promise((resolve) => releases.push({ isHeld, resolve }));
          if (mode.endsWith('denied') || mode.endsWith('unavailable'))
            return route.fulfill({
              status: isHeld && mode.endsWith('denied') ? 403 : 503,
              json: { error: 'Synthetic unavailable evidence' },
            });
          return route.fulfill({
            json: isHeld
              ? mode.endsWith('known')
                ? held
                : []
              : log(mode.endsWith('known') ? [sent] : []),
          });
        }
        if (url.pathname === '/api/notifications/health')
          return route.fulfill({
            json: {
              overall_healthy: true,
              queue: {
                pending: 0,
                sending: 0,
                sent: 1,
                failed: 0,
                dlq: 0,
                healthy: true,
                status: 'healthy',
                attention_required: 0,
                reason_codes: [],
                completed_retention_days: 7,
                dead_letter_retention_days: 30,
                counts_are_retention_bounded: true,
              },
            },
          });
        if (url.pathname === '/api/license/runtime-capabilities')
          return route.fulfill({
            json: {
              capabilities: [],
              limits: [],
              max_history_days: 7,
              hosted_mode: false,
              runtime: { build: 'community' },
              blocked_capabilities: [],
            },
          });
        if (url.pathname === '/api/license/entitlements')
          return route.fulfill({ json: { entitlements: [] } });
        if (url.pathname === '/api/security/status')
          return route.fulfill({ json: { hasAuthentication: true, requiresAuth: false } });
        return route.fulfill({ json: [] });
      });
      const check = async (label, action) => {
        await action();
        checks.push({ label, passed: true });
      };
      const activity = page.locator('#notification-delivery-activity');
      const empty = activity.getByText(/No alert deliveries were attempted/);
      const heldWarning = activity.getByText(/could not read held or deferred notifications/);
      const attemptWarning = activity.getByText(/could not read the delivery log/);
      const refresh = activity.getByRole('button', { name: 'Refresh delivery status' });
      const release = (isHeld) => {
        for (const pending of releases.splice(0).filter((item) => {
          if (item.isHeld === isHeld) {
            item.resolve();
            return false;
          }
          return true;
        }))
          releases.push(pending);
      };
      const capture = async (state) => {
        const filename = `${name}-${state}.png`;
        await activity.screenshot({ path: path.join(output, filename) });
        screens.push({
          file: filename,
          state,
          viewport: { width, height: width === 390 ? 844 : 900 },
          sha256: crypto
            .createHash('sha256')
            .update(fs.readFileSync(path.join(output, filename)))
            .digest('hex'),
        });
      };
      try {
        await page.goto('http://127.0.0.1:5291/browser-tests/notification-evidence.html');
        await page.evaluate(
          (dark) => document.documentElement.classList.toggle('dark', dark),
          dark,
        );
        await check('Initial reads are loading, not empty', async () => {
          await activity.getByText('Loading delivery attempts...').waitFor();
          await activity.getByText('Loading held and deferred notifications...').waitFor();
          assert.equal(await empty.count(), 0);
        });
        await check(
          'A slow held read does not block the successful attempts read or its refresh',
          async () => {
            release(false);
            await activity.getByText('Loading delivery attempts...').waitFor({ state: 'hidden' });
            assert.equal(await refresh.isEnabled(), true);
            assert.equal(await empty.count(), 0);
            await activity.getByText('Loading held and deferred notifications...').waitFor();
          },
        );
        await capture('held-pending');
        await check('Only two completed empty reads establish the empty window', async () => {
          release(true);
          await empty.waitFor();
          assert.equal(await activity.getByRole('status').count(), 0);
        });
        await check(
          'Keyboard or touch refresh shows attempts and held events from their real API adapters',
          async () => {
            attemptMode = heldMode = 'known';
            if (width === 390) await refresh.tap();
            else {
              await refresh.focus();
              await page.keyboard.press('Enter');
            }
            await activity.getByText('Delivered', { exact: true }).waitFor();
            await activity.getByText('synthetic-nas (usage)', { exact: true }).waitFor();
            assert.equal(await empty.count(), 0);
          },
        );
        await check(
          'The CI locator matches the 503 warning after the held row is withdrawn',
          async () => {
            heldMode = 'unavailable';
            await refresh.focus();
            await page.keyboard.press('Enter');
            await heldWarning.waitFor();
            const broadTexts = await page.getByText('Deferred').allTextContents();
            const exactBadgeCount = await activity.getByText('Deferred', { exact: true }).count();
            assert.deepEqual(broadTexts, [
              'Pulse could not read held or deferred notifications. Refresh to try again.',
            ]);
            assert.equal(exactBadgeCount, 0);
            assert.equal(await activity.getByText('Delivered', { exact: true }).count(), 1);
            checks.push({ label: 'Observed broad versus exact status match', broadTexts, exactBadgeCount });
          },
        );
        await capture('held-503-ci-locator');
        await check('A healthy refresh restores the held row before permission withdrawal', async () => {
          heldMode = 'known';
          await refresh.click();
          await activity.getByText('Deferred', { exact: true }).waitFor();
          await heldWarning.waitFor({ state: 'hidden' });
        });
        await check(
          'Held-event 403 withdraws held rows and warns without hiding delivered evidence',
          async () => {
            heldMode = 'denied';
            await refresh.click();
            await heldWarning.waitFor();
            assert.equal(
              await activity.getByText('synthetic-nas (usage)', { exact: true }).count(),
              0,
            );
            assert.equal(await activity.getByText('Delivered', { exact: true }).count(), 1);
            assert.equal(await empty.count(), 0);
          },
        );
        await capture('held-unavailable');
        await check('The held warning persists during a pending retry', async () => {
          heldMode = 'hold-empty';
          await refresh.click();
          await activity.getByText('Loading held and deferred notifications...').waitFor();
          assert.equal(await heldWarning.count(), 1);
          await refresh.waitFor({ state: 'visible' });
          await page.waitForFunction(
            () => !document.querySelector('#notification-delivery-activity button').disabled,
          );
          assert.equal(await empty.count(), 0);
          release(true);
          await heldWarning.waitFor({ state: 'hidden' });
        });
        await check(
          'An attempt-read failure keeps independently readable held evidence',
          async () => {
            attemptMode = 'denied';
            heldMode = 'known';
            await refresh.click();
            await attemptWarning.waitFor();
            await activity.getByText('synthetic-nas (usage)', { exact: true }).waitFor();
            assert.equal(await activity.getByText('Delivered', { exact: true }).count(), 0);
            assert.equal(await empty.count(), 0);
          },
        );
        await capture('attempts-unavailable');
        await check('Both failed sources are reported, not called empty', async () => {
          heldMode = 'denied';
          await refresh.click();
          await heldWarning.waitFor();
          assert.equal(await attemptWarning.count(), 1);
          assert.equal(await activity.getByRole('listitem').count(), 0);
          assert.equal(await empty.count(), 0);
        });
        await check(
          'Both successful reads clear the warnings and restore a genuine empty state',
          async () => {
            attemptMode = heldMode = 'empty';
            await refresh.click();
            await empty.waitFor();
            assert.equal(await activity.getByRole('alert').count(), 0);
            assert.equal(await activity.getByRole('status').count(), 0);
          },
        );
        await check(
          'No horizontal clipping, JavaScript page error, off-origin request or mutation',
          async () => {
            assert.equal(
              await activity.evaluate((element) => element.scrollWidth <= element.clientWidth + 1),
              true,
            );
            assert.deepEqual(errors, []);
            assert.deepEqual(offOrigin, []);
            assert.deepEqual(writes, []);
            assert(
              requests
                .filter((r) => r.path === '/api/alerts/events')
                .every(
                  (r) => r.query.includes('limit=100') && r.query.includes('notification_deferred'),
                ),
            );
            assert(
              requests
                .filter((r) => r.path === '/api/notifications/delivery-log')
                .every((r) => r.query === '?limit=200'),
            );
          },
        );
        results.push({
          name,
          browser_version: browser.version(),
          dark,
          checks,
          requests,
          errors,
          offOrigin,
          writes,
        });
      } finally {
        for (const pending of releases) pending.resolve();
        await browser.close();
        browser = null;
      }
    }
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          passed: true,
          playwright_version: playwrightVersion,
          content_sha256: hashes,
          changed_paths: runtime,
          results,
          screenshots: screens,
        },
        null,
        2,
      ) + '\n',
    );
    console.log(
      JSON.stringify({
        passed: true,
        checks: results.reduce((n, r) => n + r.checks.length, 0),
        screenshots: screens.length,
        playwright_version: playwrightVersion,
        results: results.map((r) => ({ name: r.name, browser_version: r.browser_version })),
      }),
    );
  } catch (error) {
    fs.writeFileSync(
      path.join(output, 'failed.json'),
      JSON.stringify(
        { passed: false, error: error.message, results, screenshots: screens },
        null,
        2,
      ) + '\n',
    );
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
