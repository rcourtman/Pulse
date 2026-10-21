// Synthetic HTTP producer -> real AlertsAPI -> actual overview controls/toasts.
const { chromium, webkit } = require('playwright');
const { expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/alert-action-confirmation/static');
const output = path.resolve('tmp/alert-action-confirmation/browser');
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
const actions = ['acknowledge', 'unacknowledge', 'snooze', 'unsnooze'];
const queue = {
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
  failure_classes_7d: Object.fromEntries(
    [
      'authentication',
      'rate_limited',
      'connectivity',
      'tls',
      'configuration',
      'rejected',
      'server_error',
      'unknown',
    ].map((name) => [name, 0]),
  ),
  failure_classes_available: true,
  failure_class_window_days: 7,
};
const report = {
  source: binding.source_sha,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  unexpected: [],
  completed: false,
  cleanup: {},
};
assert.equal(
  report.playwright,
  JSON.parse(fs.readFileSync('tests/integration/package-lock.json')).packages[
    'node_modules/@playwright/test'
  ].version,
);
let run, pending, browser, context;
const sockets = new Set();
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
  const observation = { method: req.method, path: url.pathname };
  run.requests.push(observation);
  if (req.method === 'POST' && actions.some((action) => url.pathname === `/api/alerts/${action}`)) {
    assert.equal(req.headers['x-csrf-token'], 'synthetic-fixture-only');
    let body = '';
    req.on('data', (chunk) => {
      body += chunk;
    });
    req.on('end', () => {
      const payload = JSON.parse(body);
      assert.equal(payload.alertIdentifier, 'proof-target');
      assert.equal(url.pathname, `/api/alerts/${run.action}`);
      if (run.action === 'snooze') assert.equal(payload.until, '2026-10-21T13:00:00.000Z');
      observation.payload = payload;
      assert.equal(pending, undefined, 'Only one write can be pending');
      pending = { res, payload };
    });
    return;
  }
  const value =
    req.method === 'GET'
      ? url.pathname === '/api/alerts/delivery-diagnosis'
        ? []
        : url.pathname === '/api/notifications/health'
          ? { overall_healthy: true, queue }
          : url.pathname === '/api/alerts/config'
            ? { enabled: true, activationState: 'active' }
            : undefined
      : undefined;
  if (value === undefined) {
    report.unexpected.push(observation);
    res.writeHead(405);
    return res.end('{}');
  }
  res.writeHead(200, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(value));
});
server.on('connection', (socket) => {
  sockets.add(socket);
  socket.on('close', () => sockets.delete(socket));
});
function release(value, status = 200) {
  assert.ok(pending);
  pending.res.writeHead(status, { 'Content-Type': 'application/json' });
  pending.res.end(JSON.stringify(value));
  pending = undefined;
}
async function activate(page, control, touch) {
  if (touch) await control.tap();
  else {
    await control.focus();
    await page.keyboard.press('Enter');
  }
}
async function capture(page, state) {
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
  const file = `${run.viewportName}-${run.locale}-${run.action}-${state}.png`;
  await page.screenshot({ path: path.join(output, file), animations: 'disabled', fullPage: true });
  report.captures.push({ file, state, sha256: hash(fs.readFileSync(path.join(output, file))) });
}
async function dismissToasts(page) {
  const region = page.getByRole('region', { name: 'Notifications' });
  const dismiss = region.getByRole('button', { name: 'Dismiss notification' });
  while (await dismiss.count()) {
    await dismiss.first().click();
    await page.clock.runFor(350);
  }
}
(async () => {
  try {
    await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const spec of [
      { name: 'desktop', engine: chromium, width: 1365, height: 900, touch: false },
      { name: 'phone', engine: webkit, width: 320, height: 900, touch: true },
    ]) {
      browser = await spec.engine.launch(
        spec.touch
          ? { headless: true }
          : { headless: true, channel: 'chromium', args: ['--no-sandbox'] },
      );
      for (const locale of ['en', 'de', 'es'])
        for (const action of actions) {
          run = {
            viewportName: spec.name,
            viewport: { width: spec.width, height: spec.height },
            browser: browser.version(),
            locale,
            action,
            requests: [],
            errors: [],
            console: [],
            checks: [],
          };
          report.cases.push(run);
          context = await browser.newContext({
            viewport: run.viewport,
            hasTouch: spec.touch,
            isMobile: spec.touch,
            colorScheme: spec.touch ? 'dark' : 'light',
            reducedMotion: 'reduce',
            locale: locale === 'en' ? 'en-GB' : locale,
            timezoneId: 'UTC',
          });
          await context.addCookies([
            { name: 'pulse_csrf', value: 'synthetic-fixture-only', url: origin },
          ]);
          const page = await context.newPage();
          page.setDefaultTimeout(8000);
          page.on('pageerror', (error) => run.errors.push(error.message));
          page.on('console', (message) => {
            if (message.type() === 'error') run.console.push(message.text());
          });
          await page.route('**/*', (route) => {
            const request = route.request(),
              url = new URL(request.url());
            if (
              url.origin !== origin ||
              !(
                ['GET', 'HEAD'].includes(request.method()) ||
                (request.method() === 'POST' &&
                  actions.some((item) => url.pathname === `/api/alerts/${item}`))
              )
            ) {
              report.unexpected.push(`${request.method()} ${url.origin}${url.pathname}`);
              return route.abort();
            }
            return route.continue();
          });
          if (spec.touch)
            await page.addInitScript(() =>
              document.addEventListener('DOMContentLoaded', () =>
                document.documentElement.classList.add('dark'),
              ),
            );
          await page.clock.install({ time: new Date('2026-10-21T12:00:00Z') });
          await page.clock.pauseAt(new Date('2026-10-21T12:00:00Z'));
          await page.goto(
            `${origin}/browser-tests/alert-action-confirmation.html?locale=${locale}&action=${action}`,
            { waitUntil: 'domcontentloaded' },
          );
          await expect.poll(() => page.evaluate(() => window.__actionProof?.ready)).toBe(true);
          const labels = await page.evaluate(() => window.__actionProof.labels);
          assert.equal(typeof labels.error, 'string');
          assert.ok(labels.error.length > 45);
          const snapshot = () => page.evaluate(() => window.__actionProof.snapshot());
          const before = await snapshot();
          const card = page.locator('#alert-proof-target');
          await expect(card).toBeVisible();
          const controlName =
            labels[
              action === 'unacknowledge' ? 'restore' : action === 'unsnooze' ? 'resume' : action
            ];
          const submit = async () => {
            if (action === 'snooze' && !(await page.getByRole('dialog').count()))
              await activate(
                page,
                card.getByRole('button', { name: controlName, exact: true }),
                spec.touch,
              );
            const control =
              action === 'snooze'
                ? page
                    .getByRole('dialog')
                    .getByRole('button', { name: labels.oneHour, exact: true })
                : card.getByRole('button', { name: controlName, exact: true });
            const element = await control.elementHandle();
            assert.ok(element);
            await activate(page, control, spec.touch);
            await expect.poll(() => Boolean(pending)).toBe(true);
            await expect.poll(() => element.evaluate((button) => button.disabled)).toBe(true);
            await element.evaluate((button) => button.click());
            await element.dispose();
          };
          // Unconfirmed, conflicting and negative bodies use the same real error path.
          await submit();
          const writesBefore = run.requests.filter((request) => request.method === 'POST').length;
          const response =
            action === 'acknowledge'
              ? { success: false }
              : action === 'unacknowledge'
                ? {}
                : action === 'snooze'
                  ? { success: true, snoozedUntil: '2026-10-21T15:00:00Z' }
                  : { success: true, error: 'Synthetic diagnostic must not be echoed' };
          release(response);
          const notifications = page.getByRole('region', { name: 'Notifications' });
          await expect(notifications.getByRole('alert')).toContainText(labels.error);
          await expect(notifications.getByRole('status')).toHaveCount(0);
          await expect.poll(snapshot).toEqual(before);
          assert.equal(
            run.requests.filter((request) => request.method === 'POST').length,
            writesBefore,
          );
          await expect(
            page.getByText('Synthetic diagnostic must not be echoed', { exact: false }),
          ).toHaveCount(0);
          run.checks.push(
            'Unconfirmed response leaves original state and same-name neighbour; no success, diagnostic echo, second click or automatic write.',
          );
          if (action === 'snooze') {
            await expect(page.getByRole('dialog')).toBeVisible();
            await expect(
              page.getByRole('dialog').getByRole('button', { name: labels.oneHour, exact: true }),
            ).toBeEnabled();
          }
          if (action === 'snooze' || (locale === 'en' && action === 'acknowledge'))
            await capture(page, 'unconfirmed');
          await dismissToasts(page);
          await page.clock.runFor(1600);
          // Explicit operator choice after the readback; no automatic retry.
          assert.deepEqual(await snapshot(), before);
          await submit();
          const accepted =
            action === 'snooze'
              ? { success: true, snoozedUntil: pending.payload.until.replace('.000Z', 'Z') }
              : { success: true };
          release(accepted);
          await expect(notifications.getByRole('status')).toHaveCount(1);
          await expect(notifications.getByRole('alert')).toHaveCount(0);
          if (action === 'snooze') await expect(page.getByRole('dialog')).toHaveCount(0);
          const after = await snapshot();
          assert.deepEqual(after['proof-neighbour'], before['proof-neighbour']);
          if (action === 'acknowledge') assert.equal(after['proof-target'].acknowledged, true);
          if (action === 'unacknowledge') assert.equal(after['proof-target'].acknowledged, false);
          if (action === 'snooze')
            assert.equal(
              after['proof-target'].operationalRecord.suppression.expiresAt,
              '2026-10-21T13:00:00.000Z',
            );
          if (action === 'unsnooze')
            assert.equal(after['proof-target'].operationalRecord.suppression, undefined);
          assert.equal(run.requests.filter((request) => request.method === 'POST').length, 2);
          run.checks.push(
            'Explicit confirmed action advances only the target; canonical request body and snooze UTC instant preserved.',
          );
          if (locale === 'en' && action === 'unsnooze') await capture(page, 'confirmed');
          assert.deepEqual(run.errors, []);
          await context.close();
          context = undefined;
        }
      await browser.close();
      browser = undefined;
    }
    assert.deepEqual(report.unexpected, []);
    report.completed = true;
  } finally {
    if (pending) {
      pending.res.destroy();
      pending = undefined;
    }
    if (context) await context.close();
    if (browser) await browser.close();
    report.cleanup.browserClosed = true;
    for (const socket of sockets) socket.destroy();
    await new Promise((resolve) => server.close(resolve));
    report.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
  }
  console.log(JSON.stringify(report));
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
