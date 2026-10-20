// Actual Notifications tab, real API client and bounded read owners. Only the
// local HTTP producer is synthetic. No destination, queue or external service.
const { chromium, webkit } = require('playwright');
const { expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/notification-recovery-proof/static');
const output = path.resolve('tmp/notification-deadline-proof/browser');
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
const LIMIT = 150_000;
assert.match(
  fs.readFileSync(path.join(root, 'src/features/alerts/notificationEvidenceRead.ts'), 'utf8'),
  /NOTIFICATION_EVIDENCE_READ_TIMEOUT_MS = 150_000/,
);
const endpoints = {
  health: '/api/notifications/health',
  attempts: '/api/notifications/delivery-log',
  held: '/api/alerts/events',
};
const empty = 'No alert deliveries were attempted in this window.';
const logGap = 'Pulse could not read the delivery log. Refresh to check delivery attempts.';
const heldGap = 'Pulse could not read held or deferred notifications. Refresh to try again.';
const failure = {
  notificationId: 'fixture-failure',
  type: 'webhook',
  destinationId: 'webhook:ops',
  outcome: 'dead_letter',
  alertIds: ['host-offline-101'],
  alertCount: 1,
  attempts: 1,
  success: false,
  errorMessage: 'Synthetic authentication failure',
  failureClass: 'authentication',
  timestamp: '2026-10-20T12:00:00Z',
};
const health = {
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
    retry_attempts_affect_health: false,
    terminal_failures_affect_health: true,
    failure_classes_7d: {
      authentication: 0,
      rate_limited: 0,
      connectivity: 0,
      tls: 0,
      configuration: 0,
      rejected: 0,
      server_error: 0,
      unknown: 0,
    },
    failure_classes_available: true,
    failure_class_window_days: 7,
  },
};
const report = {
  source: binding.source_sha,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  cleanup: {},
  completed: false,
};
const blocked = new Set();
let run, modes, entries;
const server = http.createServer((req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1');
  if (url.pathname === '/favicon.ico') {
    res.writeHead(204);
    return res.end();
  }
  if (!url.pathname.startsWith('/api/')) {
    const bytes = assets.get(url.pathname);
    res.setHeader(
      'Content-Type',
      url.pathname.endsWith('.js')
        ? 'text/javascript'
        : url.pathname.endsWith('.css')
          ? 'text/css'
          : 'text/html',
    );
    res.writeHead(bytes ? 200 : 404);
    return res.end(bytes);
  }
  const stream = Object.keys(endpoints).find((key) => endpoints[key] === url.pathname);
  const observation = {
    path: url.pathname,
    method: req.method,
    stream,
    mode: stream ? modes[stream] : 'normal',
  };
  run.reads.push(observation);
  if (req.method !== 'GET') {
    run.unexpected.push(req.method + ' ' + url.pathname);
    res.writeHead(405);
    return res.end();
  }
  if (stream === 'attempts') assert.equal(url.searchParams.get('limit'), '200');
  if (stream === 'held') {
    assert.equal(url.searchParams.get('limit'), '100');
    assert.equal(url.searchParams.get('type'), 'notification_suppressed,notification_deferred');
  }
  res.setHeader('Content-Type', 'application/json');
  if (stream && ['headers', 'body'].includes(modes[stream])) {
    blocked.add(res);
    res.once('close', () => {
      observation.closedBeforeEnd = !res.writableEnded;
      blocked.delete(res);
    });
    if (modes[stream] === 'body') {
      res.writeHead(200);
      res.write('{');
    }
    return;
  }
  if (stream && modes[stream] === 'rate-limit') {
    modes[stream] = 'normal';
    res.setHeader('Retry-After', '120');
    res.writeHead(429);
    return res.end(JSON.stringify({ error: 'Synthetic rate limit' }));
  }
  const payload = {
    [endpoints.health]: health,
    [endpoints.attempts]: {
      entries,
      window_days: 30,
      completed_retention_days: 7,
      dead_letter_retention_days: 30,
    },
    [endpoints.held]: [],
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
  if (payload === undefined) {
    run.unexpected.push(url.pathname);
    res.writeHead(404);
    return res.end('{}');
  }
  res.writeHead(200);
  res.end(JSON.stringify(payload));
});
let browser, context;
(async () => {
  try {
    assert.equal(
      report.playwright,
      JSON.parse(fs.readFileSync('tests/integration/package-lock.json')).packages[
        'node_modules/@playwright/test'
      ].version,
    );
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
        isMobile: dark,
        hasTouch: dark,
        locale: 'en-GB',
        timezoneId: 'UTC',
        colorScheme: dark ? 'dark' : 'light',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      run = {
        name,
        width,
        dark,
        browser: browser.version(),
        checks: [],
        reads: [],
        errors: [],
        consoleErrors: [],
        unexpected: [],
      };
      report.cases.push(run);
      page.on('pageerror', (error) => run.errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') run.consoleErrors.push(message.text());
      });
      await page.route('**/*', (route) => {
        if (new URL(route.request().url()).origin === origin) return route.continue();
        run.unexpected.push(route.request().url());
        return route.abort();
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
      const now = new Date('2026-10-20T12:00:00Z');
      await page.clock.install({ time: now });
      await page.clock.pauseAt(now);
      const card = page.locator('#notification-delivery-activity');
      const warning = page.getByRole('alert').filter({
        has: page.getByRole('heading', { name: 'Notification delivery status is unavailable' }),
      });
      const count = (stream) => run.reads.filter((read) => read.stream === stream).length;
      const activate = async (locator) => {
        await locator.scrollIntoViewIfNeeded();
        if (dark) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      const capture = async (state, locator) => {
        await locator.scrollIntoViewIfNeeded();
        assert(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          'No horizontal overflow',
        );
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), animations: 'disabled' });
        report.captures.push({
          file,
          state,
          viewport: { width, height: 900 },
          sha256: hash(fs.readFileSync(path.join(output, file))),
        });
      };
      modes = { health: 'headers', attempts: 'body', held: 'headers' };
      entries = [];
      await page.goto(`${origin}/browser-tests/notification-recovery-boundary.html`, {
        waitUntil: 'domcontentloaded',
      });
      for (const stream of Object.keys(endpoints)) await expect.poll(() => count(stream)).toBe(1);
      await page.clock.fastForward(LIMIT - 1);
      await expect(warning).toHaveCount(0);
      await expect(card.getByRole('button', { name: 'Refresh delivery status' })).toBeDisabled();
      await expect(card.getByText(empty, { exact: true })).toHaveCount(0);
      await page.clock.fastForward(1);
      await expect(card.getByText(logGap, { exact: true })).toBeVisible();
      await expect(card.getByText(heldGap, { exact: true })).toBeVisible();
      await expect(card.getByText(empty, { exact: true })).toHaveCount(0);
      await expect(card.getByRole('button', { name: 'Refresh delivery status' })).toBeEnabled();
      await expect(warning).toBeVisible();
      await expect.poll(() => run.reads.filter((read) => read.closedBeforeEnd).length).toBe(3);
      for (const stream of Object.keys(endpoints))
        assert.equal(count(stream), 1, 'Expiry never retries automatically');
      await capture('unavailable', warning);
      await capture('activity-unavailable', card);
      run.checks.push(
        'Exact 150-second header/body deadline, three transport aborts, explicit unavailable-not-empty evidence and enabled refresh; no automatic retry.',
      );
      modes.attempts = 'normal';
      modes.held = 'normal';
      entries = [failure];
      await activate(card.getByRole('button', { name: 'Refresh delivery status' }));
      await expect(card.getByText('Failed, no further retries', { exact: true })).toBeVisible();
      await expect(card.getByText(logGap, { exact: true })).toHaveCount(0);
      await expect(card.getByText(heldGap, { exact: true })).toHaveCount(0);
      await expect(warning).toBeVisible();
      assert.equal(count('health'), 1);
      modes.health = 'rate-limit';
      const waiting = page.waitForEvent('console', {
        predicate: (message) => message.text().includes('retrying GET after 120000ms'),
      });
      await activate(warning.getByRole('button', { name: 'Refresh delivery status' }));
      await waiting;
      await page.evaluate(() => Promise.resolve());
      await page.clock.fastForward(119_999);
      assert.equal(count('health'), 2);
      await page.clock.fastForward(1);
      await expect.poll(() => count('health')).toBe(3);
      await expect(warning).toHaveCount(0);
      await expect(card.getByText('Failed, no further retries', { exact: true })).toBeVisible();
      run.checks.push(
        'Activity recovers independently; actual client honours two-minute Retry-After and accepts healthy evidence before the deadline, preserving failed historical attempts.',
      );
      await capture('recovered', card);
      modes.held = 'headers';
      await activate(card.getByRole('button', { name: 'Refresh delivery status' }));
      await expect.poll(() => count('held')).toBe(3);
      // Seeing an HTTP request is not evidence its independent response has
      // reached the owner. Let attempts finish before aging only the held read.
      await expect(card.getByRole('button', { name: 'Refresh delivery status' })).toBeEnabled();
      await page.clock.fastForward(LIMIT);
      await expect(card.getByText(heldGap, { exact: true })).toBeVisible();
      await expect(card.getByText(logGap, { exact: true })).toHaveCount(0);
      await expect(card.getByText('Failed, no further retries', { exact: true })).toBeVisible();
      await expect(card.getByText(empty, { exact: true })).toHaveCount(0);
      await capture('held-gap', card);
      modes.held = 'normal';
      entries = [];
      await activate(card.getByRole('button', { name: 'Refresh delivery status' }));
      await expect(card.getByText(empty, { exact: true })).toBeVisible();
      await expect(card.getByText(heldGap, { exact: true })).toHaveCount(0);
      run.checks.push(
        'A stalled held-event stream leaves readable failed attempts intact, never earns the empty claim, then clears on an explicit complete refresh.',
      );
      assert.deepEqual(run.errors, []);
      assert.deepEqual(run.unexpected, []);
      assert(run.reads.every((read) => read.method === 'GET'));
      run.expectedUnavailableDiagnostics = run.consoleErrors.filter((message) =>
        /Failed to load (notification delivery health|notification delivery log|held alert notification events)/.test(
          message,
        ),
      ).length;
      assert.equal(run.expectedUnavailableDiagnostics, 4);
      assert(
        run.consoleErrors.every((message) => /Failed to load |429/.test(message)),
        'Expected timeout/429 diagnostics only',
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
    for (const response of blocked) response.destroy();
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
    report.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify(report));
  }
})();
