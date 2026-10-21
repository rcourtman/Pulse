// Offline HTTP producer -> real API adapters -> real settings/save consumer.
const { chromium, webkit } = require('playwright');
const { expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/notification-recovery-proof/static');
const output = path.resolve('tmp/alert-configuration-deadline/browser');
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
const configPaths = [
  '/api/alerts/config',
  '/api/notifications/email',
  '/api/notifications/apprise',
  '/api/alerts/deadman/config',
  '/api/settings/relay',
  '/api/notifications/webhooks',
];
const config = {
  enabled: true,
  activationState: 'active',
  overrides: {},
  guestDefaults: { cpu: { trigger: 80, clear: 75 } },
};
const email = {
  enabled: true,
  provider: 'smtp',
  server: 'smtp.saved.example.test',
  port: 587,
  username: '',
  password: '',
  from: 'pulse@example.test',
  to: ['operator@example.test'],
  tls: false,
  startTLS: true,
  rateLimit: 17,
  minimumSeverity: 'critical',
};
const payloads = {
  [configPaths[0]]: config,
  [configPaths[1]]: email,
  [configPaths[2]]: { enabled: false, mode: 'cli', targets: [], apiKey: '' },
  [configPaths[3]]: { pingUrl: '', configured: false },
  [configPaths[4]]: {
    enabled: true,
    server_url: 'wss://relay.example.test',
    alert_minimum_severity: 'critical',
  },
  [configPaths[5]]: [],
  '/api/license/runtime-capabilities': {
    capabilities: ['relay'],
    limits: [],
    hosted_mode: false,
    blocked_capabilities: [],
    runtime: { build: 'pro' },
  },
  '/api/notifications/webhook-templates': [],
  '/api/notifications/email-providers': [],
  '/api/notifications/health': {
    overall_healthy: true,
    queue: {
      pending: 0,
      sending: 0,
      sent: 0,
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
  },
  '/api/notifications/delivery-log': {
    entries: [],
    window_days: 30,
    completed_retention_days: 7,
    dead_letter_retention_days: 30,
  },
  '/api/alerts/events': [],
  '/api/alerts/deadman/status': {
    configured: false,
    state: 'disabled',
    heartbeatIntervalSeconds: 60,
    recommendedGraceSeconds: 180,
    consecutiveFailures: 0,
  },
  '/api/security/status': { hasAuthentication: true, requiresAuth: false, sessionCapabilities: {} },
  '/api/health': { status: 'ok' },
};
const report = {
  source: binding.source_sha,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  completed: false,
  cleanup: {},
};
const blocked = new Set();
let run, blockedPath, mode;
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
  const observation = {
    method: req.method,
    path: url.pathname,
    csrf: req.headers['x-csrf-token'] === 'synthetic-fixture-only',
  };
  run.requests.push(observation);
  res.setHeader('Content-Type', 'application/json');
  if (req.method === 'GET' && url.pathname === blockedPath) {
    if (mode === 'rate-limit') {
      mode = 'normal';
      res.setHeader('Retry-After', '120');
      res.writeHead(429);
      return res.end(JSON.stringify({ error: 'Synthetic rate limit' }));
    }
    if (mode === 'headers' || mode === 'body') {
      blocked.add(res);
      res.once('close', () => {
        observation.closedBeforeEnd = !res.writableEnded;
        blocked.delete(res);
      });
      if (mode === 'body') {
        res.writeHead(200);
        res.write('{');
      }
      return;
    }
  }
  if (req.method === 'PUT' && configPaths.slice(0, 5).includes(url.pathname)) {
    let body = '';
    req.on('data', (bytes) => (body += bytes));
    req.on('end', () => {
      observation.body = JSON.parse(body);
      res.writeHead(200);
      res.end(
        JSON.stringify(
          url.pathname === configPaths[2]
            ? { ...observation.body, apiKey: '', hasApiKey: false }
            : { success: true, configured: true },
        ),
      );
    });
    return;
  }
  if (req.method !== 'GET' || !Object.hasOwn(payloads, url.pathname)) {
    run.unexpected.push(`${req.method} ${url.pathname}`);
    res.writeHead(404);
    return res.end('{}');
  }
  res.writeHead(200);
  res.end(JSON.stringify(payloads[url.pathname]));
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
      const activate = async (locator) => {
        await locator.scrollIntoViewIfNeeded();
        if (dark) await locator.tap();
        else {
          await locator.focus();
          await locator.press('Enter');
        }
      };
      for (const [index, endpoint] of configPaths.entries()) {
        run = {
          name,
          endpoint,
          mode: index % 2 ? 'body' : 'headers',
          browser: browser.version(),
          requests: [],
          checks: [],
          errors: [],
          consoleErrors: [],
          unexpected: [],
        };
        report.cases.push(run);
        blockedPath = endpoint;
        mode = run.mode;
        context = await browser.newContext({
          viewport: { width, height: 900 },
          colorScheme: dark ? 'dark' : 'light',
          isMobile: dark,
          hasTouch: dark,
          locale: 'en-GB',
          timezoneId: 'UTC',
        });
        await context.addCookies([
          { name: 'pulse_csrf', value: 'synthetic-fixture-only', url: origin },
        ]);
        const page = await context.newPage();
        page.setDefaultTimeout(8000);
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
        const now = new Date('2026-10-21T01:00:00Z');
        await page.clock.install({ time: now });
        await page.clock.pauseAt(now);
        await page.goto(`${origin}/browser-tests/alert-configuration-deadline.html`, {
          waitUntil: 'domcontentloaded',
        });
        const count = () => run.requests.filter((request) => request.path === endpoint).length;
        await expect.poll(count).toBe(1);
        await page.clock.fastForward(LIMIT - 1);
        await expect(
          page.getByText('Loading saved alert settings…', { exact: true }),
        ).toBeVisible();
        await expect(
          page.getByRole('button', { name: 'Reload settings', exact: true }),
        ).toHaveCount(0);
        await page.clock.fastForward(1);
        const feedback =
          index === 0
            ? page
                .getByRole('alert')
                .filter({ hasText: 'Saved alert settings could not be loaded.' })
            : page.getByText(/Unable to load notification settings.*Saving is blocked/);
        await expect(feedback).toBeVisible();
        await expect(page.getByText('Loading saved alert settings…', { exact: true })).toHaveCount(
          0,
        );
        await expect
          .poll(() => run.requests.filter((request) => request.closedBeforeEnd).length)
          .toBe(1);
        assert.equal(count(), 1, 'Timeout never retries automatically');
        assert(run.requests.every((request) => request.method === 'GET'));
        if (index !== 0) {
          await page
            .getByLabel('Healthchecks-compatible success ping URL')
            .fill('https://watchdog.example.test/unsaved');
          await expect(
            page.getByRole('button', { name: 'Save Changes', exact: true }),
          ).toBeDisabled();
        }
        const capture = async (state, target) => {
          await target.scrollIntoViewIfNeeded();
          assert(
            await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
            'No horizontal overflow',
          );
          const file = `${name}-${state}.png`;
          await page.screenshot({ path: path.join(output, file), animations: 'disabled' });
          report.captures.push({
            file,
            width,
            height: 900,
            sha256: hash(fs.readFileSync(path.join(output, file))),
          });
        };
        if (index < 2)
          await capture(index === 0 ? 'policy-unavailable' : 'destinations-unavailable', feedback);
        run.checks.push(
          'Exact 150-second settings header/body deadline releases loading, aborts one read, retains error/save lock and issues no automatic retry or write.',
        );
        mode = index === 0 ? 'rate-limit' : 'normal';
        const waiting =
          index === 0
            ? page.waitForEvent('console', {
                predicate: (message) => message.text().includes('retrying GET after 120000ms'),
              })
            : null;
        await activate(
          page.getByRole('button', {
            name: index === 0 ? 'Reload settings' : 'Retry',
            exact: true,
          }),
        );
        if (waiting) {
          await waiting;
          await page.clock.fastForward(119_999);
          assert.equal(count(), 2);
          await page.clock.fastForward(1);
          await expect.poll(count).toBe(3);
          run.checks.push(
            'An explicit reload still honours the actual client two-minute Retry-After.',
          );
        }
        await expect(page.getByLabel('SMTP server', { exact: true })).toHaveValue(email.server);
        await expect(page.getByText(/Unable to load notification settings/)).toHaveCount(0);
        await expect(
          page.getByRole('alert').filter({ hasText: 'Saved alert settings could not be loaded.' }),
        ).toHaveCount(0);
        await expect(page.getByText('Loading saved alert settings…', { exact: true })).toHaveCount(
          0,
        );
        await page
          .getByLabel('Healthchecks-compatible success ping URL')
          .fill('https://watchdog.example.test/corrected');
        const save = page.getByRole('button', { name: 'Save Changes', exact: true });
        await expect(save).toBeEnabled();
        if (index === 1) {
          await activate(save);
          await expect
            .poll(() => run.requests.filter((request) => request.method === 'PUT').length)
            .toBe(5);
          await expect(
            page.getByText('Configuration saved successfully!', { exact: true }),
          ).toBeVisible();
          const writes = run.requests.filter((request) => request.method === 'PUT');
          assert(writes.every((request) => request.csrf));
          assert.equal(
            writes.find((request) => request.path === configPaths[1]).body.server,
            email.server,
          );
          assert.equal(
            writes.find((request) => request.path === configPaths[1]).body.rateLimit,
            17,
          );
          assert.equal(
            writes.find((request) => request.path === configPaths[3]).body.pingUrl,
            'https://watchdog.example.test/corrected',
          );
          assert.deepEqual(writes.find((request) => request.path === configPaths[4]).body, {
            alert_minimum_severity: 'critical',
          });
          await capture(
            'recovered-saved',
            page.getByRole('heading', { name: 'Email notifications' }),
          );
          run.checks.push(
            'One explicit Save writes only the five expected policies with CSRF, exact recovered SMTP settings, corrected ping and unchanged legacy push severity.',
          );
        }
        assert.deepEqual(run.unexpected, []);
        assert.deepEqual(run.errors, []);
        await context.close();
        context = null;
      }
      // Early retirement also aborts a still-unanswered policy read, without
      // waiting for its deadline or reopening the retired settings view.
      run = {
        name,
        endpoint: 'retirement',
        browser: browser.version(),
        requests: [],
        checks: [],
        errors: [],
        consoleErrors: [],
        unexpected: [],
      };
      report.cases.push(run);
      blockedPath = configPaths[0];
      mode = 'headers';
      context = await browser.newContext({
        viewport: { width, height: 900 },
        isMobile: dark,
        hasTouch: dark,
      });
      const page = await context.newPage();
      page.on('pageerror', (error) => run.errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') run.consoleErrors.push(message.text());
      });
      await page.route('**/*', (route) => {
        if (new URL(route.request().url()).origin === origin) return route.continue();
        run.unexpected.push(route.request().url());
        return route.abort();
      });
      await page.goto(`${origin}/browser-tests/alert-configuration-deadline.html`, {
        waitUntil: 'domcontentloaded',
      });
      await expect
        .poll(() => run.requests.filter((request) => request.path === blockedPath).length)
        .toBe(1);
      await activate(page.getByRole('button', { name: 'Retire synthetic settings view' }));
      await expect
        .poll(() => run.requests.filter((request) => request.closedBeforeEnd).length)
        .toBe(1);
      await expect(page.getByText('Loading saved alert settings…', { exact: true })).toHaveCount(0);
      assert(run.requests.every((request) => request.method === 'GET'));
      run.checks.push(
        'Unmount aborts a live policy read before expiry, without a mutation or retry.',
      );
      await context.close();
      context = null;
      await browser.close();
      browser = null;
    }
    report.completed = true;
  } catch (error) {
    report.failure = { message: error.message, stack: error.stack };
    process.exitCode = 1;
  } finally {
    if (context) await context.close();
    if (browser) await browser.close();
    report.cleanup.contextAndBrowserClosed = true;
    for (const response of blocked) response.destroy();
    await new Promise((resolve) => server.close(resolve));
    report.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify(report, null, 2));
  }
})();
