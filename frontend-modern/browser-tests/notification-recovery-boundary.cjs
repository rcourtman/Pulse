// Exact-source compiled full Notifications tab, local synthetic HTTP responses.
const { chromium, webkit } = require('playwright');
const { expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/notification-recovery-proof/static');
const output = path.resolve('tmp/notification-recovery-proof/browser');
fs.mkdirSync(output, { recursive: true });
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
for (const [file, digest] of Object.entries(binding.sources))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Changed source: ${file}`);
const assets = new Map(
  Object.entries(binding.artifacts).map(([file, digest]) => {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, `Changed artifact: ${file}`);
    return ['/' + file, bytes];
  }),
);
const failure = {
  notificationId: 'fixture-failure',
  type: 'webhook',
  destinationId: 'webhook:ops',
  outcome: 'dead_letter',
  alertIds: ['host-offline-101', 'host-offline-102'],
  alertCount: 2,
  attempts: 1,
  success: false,
  errorMessage: 'Synthetic authentication failure',
  failureClass: 'authentication',
  timestamp: '2026-10-20T12:00:00Z',
};
const sent = {
  ...failure,
  notificationId: 'fixture-sent',
  outcome: 'sent',
  success: true,
  errorMessage: '',
  failureClass: undefined,
  alertIds: ['disk-201'],
  alertCount: 1,
};
const invalid = { ...sent, notificationId: 'fixture-contradiction', success: false };
const report = {
  source: binding.source_sha,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  cleanup: {},
  completed: false,
};
assert.equal(
  report.playwright,
  JSON.parse(fs.readFileSync('tests/integration/package-lock.json')).packages[
    'node_modules/@playwright/test'
  ].version,
);
const server = http.createServer((req, res) => {
  const file = new URL(req.url, 'http://127.0.0.1').pathname;
  const bytes = assets.get(file);
  res.setHeader(
    'Content-Type',
    file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html',
  );
  res.writeHead(bytes ? 200 : 404);
  res.end(bytes);
});
let browser, context;
(async () => {
  try {
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1365, false],
      ['narrow', webkit, 320, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      context = await browser.newContext({
        viewport: { width, height: 900 },
        hasTouch: width < 500,
        isMobile: width < 500,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      await context.addCookies([
        { name: 'pulse_csrf', value: 'synthetic-fixture-only', url: origin },
      ]);
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      const run = {
        name,
        width,
        dark,
        browser: browser.version(),
        checks: [],
        reads: [],
        writes: [],
        dialogs: [],
        errors: [],
        consoleErrors: [],
        unexpected: [],
      };
      report.cases.push(run);
      page.on('pageerror', (error) => run.errors.push(error.message));
      page.on('console', (msg) => {
        if (msg.type() === 'error') run.consoleErrors.push(msg.text());
      });
      page.on('dialog', async (dialog) => {
        run.dialogs.push(dialog.message());
        await dialog.accept();
      });
      await page.addInitScript(
        (dark) =>
          document.addEventListener(
            'DOMContentLoaded',
            () => document.documentElement.classList.toggle('dark', dark),
            { once: true },
          ),
        dark,
      );
      let rawEntries = {},
        actionResult = { success: false, affected: 2 },
        healthy = false;
      await page.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin) {
          run.unexpected.push(url.href);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        const queue = {
          pending: 0,
          sending: 0,
          sent: 1,
          failed: 0,
          dlq: healthy ? 0 : 2,
          healthy,
          status: healthy ? 'healthy' : 'degraded',
          attention_required: healthy ? 0 : 2,
          reason_codes: healthy ? [] : ['retained_dead_letter_deliveries'],
          completed_retention_days: 7,
          dead_letter_retention_days: 30,
          counts_are_retention_bounded: true,
          retry_attempts_affect_health: false,
          terminal_failures_affect_health: true,
          failure_classes_available: true,
          failure_class_window_days: 7,
          failure_classes_7d: {
            authentication: 2,
            rate_limited: 0,
            connectivity: 0,
            tls: 0,
            configuration: 0,
            rejected: 0,
            server_error: 0,
            unknown: 0,
          },
        };
        if (
          req.method() === 'POST' &&
          /^\/api\/notifications\/terminal-failures\/(retry|dismiss)$/.test(url.pathname)
        ) {
          assert.equal(req.headers()['x-csrf-token'], 'synthetic-fixture-only');
          run.writes.push(url.pathname);
          return route.fulfill({ json: actionResult });
        }
        if (req.method() !== 'GET') {
          run.unexpected.push(req.method() + url.pathname);
          return route.abort();
        }
        run.reads.push(url.pathname);
        if (url.pathname === '/api/notifications/delivery-log') {
          assert.equal(url.searchParams.get('limit'), '200');
          return route.fulfill({
            json: {
              entries: rawEntries,
              window_days: 30,
              completed_retention_days: 7,
              dead_letter_retention_days: 30,
            },
          });
        }
        if (url.pathname === '/api/alerts/events') {
          assert.equal(url.searchParams.get('limit'), '100');
          return route.fulfill({ json: [] });
        }
        const data = {
          '/api/notifications/health': { overall_healthy: healthy, queue },
          '/api/notifications/webhook-templates': [],
          '/api/alerts/deadman/status': {
            configured: false,
            state: 'disabled',
            heartbeatIntervalSeconds: 60,
            recommendedGraceSeconds: 180,
            consecutiveFailures: 0,
          },
          '/api/security/status': {
            hasAuthentication: true,
            requiresAuth: false,
            sessionCapabilities: {},
          },
          '/api/health': { status: 'ok' },
          '/api/license/runtime-capabilities': {
            capabilities: [],
            limits: [],
            hosted_mode: false,
            blocked_capabilities: [],
            runtime: { build: 'community' },
          },
        }[url.pathname];
        if (data === undefined) {
          run.unexpected.push(url.pathname);
          return route.fulfill({ status: 404, json: { error: 'Unexpected fixture read' } });
        }
        return route.fulfill({ json: data });
      });
      const activate = async (locator) => {
        await locator.scrollIntoViewIfNeeded();
        if (width < 500) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      const card = page.locator('#notification-delivery-activity');
      const feedback = page.getByRole('region', { name: 'Notification recovery feedback' });
      const notices = page.getByRole('region', { name: 'Notifications', exact: true });
      const empty = 'No alert deliveries were attempted in this window.';
      const incomplete =
        'Some delivery attempts could not be read. This activity list is incomplete. Refresh to check again.';
      const capture = async (state) => {
        // Partial/recovered captures show activity. The unconfirmed capture
        // must show the recovery message itself, especially at 320px.
        await (state === 'unconfirmed' ? feedback : card).scrollIntoViewIfNeeded();
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), animations: 'disabled' });
        report.captures.push({
          file,
          sha256: hash(fs.readFileSync(path.join(output, file))),
          name,
          state,
        });
      };
      const check = (label) => run.checks.push(label);
      const refresh = async () =>
        activate(card.getByRole('button', { name: 'Refresh delivery status' }));
      await page.goto(origin + '/browser-tests/notification-recovery-boundary.html', {
        waitUntil: 'networkidle',
      });
      await expect(
        card.getByText(
          'Pulse could not read the delivery log. Refresh to check delivery attempts.',
        ),
      ).toBeVisible();
      await expect(card.getByText(empty, { exact: true })).toHaveCount(0);
      check('Malformed collection is unavailable, not empty, in the full Notifications tab.');
      rawEntries = [invalid];
      await refresh();
      await expect(card.getByText(incomplete)).toBeVisible();
      await expect(card.getByText(empty, { exact: true })).toHaveCount(0);
      await expect(card.getByText('Delivered', { exact: true })).toHaveCount(0);
      check('An entirely unreadable row batch cannot claim no attempts or delivered.');
      rawEntries = [failure, invalid, sent];
      await refresh();
      await expect(card.getByText('Failed, no further retries', { exact: true })).toBeVisible();
      await expect(card.getByText('Delivered', { exact: true })).toBeVisible();
      const disclosure = card.locator('summary').filter({ hasText: 'host-offline-101' });
      await activate(disclosure);
      await expect(disclosure.locator('..')).toHaveAttribute('open', '');
      await expect(card.getByText(incomplete)).toBeVisible();
      await capture('partial');
      check(
        'Mixed data keeps readable failure, success and grouped alert identity with an incomplete warning.',
      );
      await page.evaluate(() => {
        window.__recoveryNotices = [];
        const original = window.showToast;
        window.showToast = (...args) => {
          window.__recoveryNotices.push(args.slice(0, 2));
          return original(...args);
        };
      });
      const healthReads = run.reads.filter((p) => p === '/api/notifications/health').length;
      for (const [action, payload, copy] of [
        [
          'retry',
          { success: false, affected: 2 },
          'Retry could not be confirmed. Refresh notification activity before trying again.',
        ],
        [
          'dismiss',
          { success: true, affected: '2' },
          'Dismissal could not be confirmed. Refresh notification activity before trying again.',
        ],
      ]) {
        actionResult = payload;
        await activate(
          page.getByRole('button', {
            name: action === 'retry' ? 'Retry retained deliveries' : 'Dismiss retained failures',
          }),
        );
        await expect(feedback.getByText(copy)).toBeVisible();
        await expect(notices.getByText(copy)).toBeVisible();
        await expect(card.getByText('Failed, no further retries', { exact: true })).toBeVisible();
        await activate(notices.getByRole('button', { name: 'Dismiss notification' }));
        await expect(notices.getByRole('button')).toHaveCount(0);
      }
      assert.equal(run.writes.length, 2);
      assert.equal(run.dialogs.length, 2);
      assert.equal(run.reads.filter((p) => p === '/api/notifications/health').length, healthReads);
      assert.equal(
        await page.evaluate(() => window.__recoveryNotices.some(([type]) => type === 'success')),
        false,
      );
      await capture('unconfirmed');
      check(
        'Rejected Retry and malformed Dismiss acknowledgements keep persistent feedback, no success or automatic follow-up, one confirmed local POST each.',
      );
      await activate(feedback.getByRole('button', { name: 'Clear recovery message' }));
      if (width >= 500) await expect(feedback).toBeFocused();
      await expect(feedback.getByRole('button')).toHaveCount(0);
      rawEntries = [];
      await refresh();
      await expect(card.getByText(empty, { exact: true })).toBeVisible();
      await expect(card.getByText(incomplete)).toHaveCount(0);
      actionResult = { success: true, affected: 0 };
      await activate(page.getByRole('button', { name: 'Retry retained deliveries' }));
      await expect(notices.getByText('0 retained deliveries queued for retry.')).toBeVisible();
      await activate(notices.getByRole('button', { name: 'Dismiss notification' }));
      await expect(notices.getByRole('button')).toHaveCount(0);
      actionResult = { success: true, affected: 2 };
      healthy = true;
      rawEntries = [failure];
      await activate(page.getByRole('button', { name: 'Dismiss retained failures' }));
      await expect(notices.getByText('2 retained failures dismissed.')).toBeVisible();
      await expect(page.getByRole('button', { name: 'Dismiss retained failures' })).toHaveCount(0);
      await expect(card.getByText('Failed, no further retries', { exact: true })).toBeVisible();
      await expect(card.getByText(incomplete)).toHaveCount(0);
      await activate(notices.getByRole('button', { name: 'Dismiss notification' }));
      await expect(notices.getByRole('button')).toHaveCount(0);
      await capture('recovered');
      check(
        'Valid empty refresh clears incompleteness; acknowledged zero retry and two dismiss preserve audit evidence and refresh health without claiming delivery.',
      );
      assert.equal(run.writes.length, 4);
      assert.equal(run.dialogs.length, 4);
      assert.deepEqual(run.errors, []);
      assert.deepEqual(run.unexpected, []);
      assert.equal(
        run.consoleErrors.length,
        3,
        'Only one unavailable log and two unconfirmed-action errors are expected',
      );
      assert(
        run.consoleErrors.every((line) =>
          /Failed to (load notification delivery log|retry retained notification deliveries|dismiss retained notification failures)/.test(
            line,
          ),
        ),
      );
      assert(
        await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
        'Horizontal overflow',
      );
      check(
        'Expected failure diagnostics only; no page errors, unexpected/off-origin requests or horizontal overflow.',
      );
      await context.close();
      context = null;
      run.contextClosed = true;
      await browser.close();
      browser = null;
      run.browserClosed = true;
    }
    report.completed = true;
  } catch (error) {
    report.failure = { message: error.message, stack: error.stack };
    process.exitCode = 1;
  } finally {
    if (context) {
      await context.close();
      report.cleanup.exceptionalContextClosed = true;
    }
    if (browser) {
      await browser.close();
      report.cleanup.exceptionalBrowserClosed = true;
    }
    await new Promise((resolve) => server.close(resolve));
    report.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify(report));
  }
})();
