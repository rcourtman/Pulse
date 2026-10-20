process.env.RAYON_NUM_THREADS = '2';
process.env.UV_THREADPOOL_SIZE = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit, expect } = require('playwright/test');
const {
  default: AxeBuilder,
} = require('/workspace/tests/integration/node_modules/@axe-core/playwright');
const root = '/workspace';
const output = path.join(root, 'tmp/alert-ack-proof/browser');
const binding = JSON.parse(fs.readFileSync(path.join(root, 'tmp/alert-ack-proof/binding.json')));
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const report = {
  binding,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  unexpected: [],
  cleanup: {},
};
let server, browser, current;
let pending = [];
const health = {
  overall_healthy: true,
  queue: {
    status: 'healthy',
    healthy: true,
    pending: 0,
    sending: 0,
    sent: 0,
    failed: 0,
    dlq: 0,
    attention_required: 0,
    completed_retention_days: 7,
    dead_letter_retention_days: 30,
    counts_are_retention_bounded: true,
    retry_attempts_affect_health: false,
    terminal_failures_affect_health: true,
    reason_codes: [],
    failure_class_window_days: 7,
    failure_classes_available: true,
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
  },
};
function api(req, res, next) {
  const url = new URL(req.url, 'http://fixture.invalid');
  if (!url.pathname.startsWith('/api/')) return next();
  const request = { method: req.method, path: url.pathname };
  current.requests.push(request);
  if (req.method === 'POST' && url.pathname === '/api/alerts/bulk/acknowledge') {
    let body = '';
    req.on('data', (chunk) => {
      body += chunk;
    });
    req.on('end', () => {
      const value = JSON.parse(body);
      assert.ok(Array.isArray(value.alertIdentifiers));
      assert.ok(
        value.alertIdentifiers.every((id) =>
          ['primary', 'critical', 'other-primary', 'other-related'].includes(id),
        ),
      );
      request.identifiers = value.alertIdentifiers;
      pending.push({ res, identifiers: value.alertIdentifiers });
    });
    return;
  }
  const value =
    req.method === 'GET'
      ? url.pathname === '/api/alerts/delivery-diagnosis'
        ? []
        : url.pathname === '/api/notifications/health'
          ? health
          : undefined
      : undefined;
  if (value === undefined) {
    report.unexpected.push(`${req.method} ${url.pathname}`);
    res.writeHead(405);
    res.end('{}');
    return;
  }
  res.writeHead(200, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(value));
}
function release(results, status = 200) {
  assert.ok(pending.length > 0);
  for (const { res } of pending.splice(0)) {
    res.writeHead(status, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(status === 200 ? { results } : { error: 'Synthetic rejected request' }));
  }
}
async function capture(page, state) {
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
  const file = `${current.phase}-${current.name}-${state}.png`;
  await page.screenshot({ path: path.join(output, file), animations: 'disabled', fullPage: true });
  report.captures.push({ file, state, sha256: hash(fs.readFileSync(path.join(output, file))) });
}
async function scan(page, state) {
  const result = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .analyze();
  const violations = result.violations.map(({ id, impact, nodes }) => ({
    id,
    impact,
    nodes: nodes.map(({ target, failureSummary }) => ({ target, failureSummary })),
  }));
  current.scans.push({ state, violations });
  assert.equal(violations.length, 0, `${current.name}/${state} accessibility`);
}
async function dismissToasts(page) {
  const buttons = page
    .getByRole('region', { name: 'Notifications' })
    .getByRole('button', { name: 'Dismiss notification' });
  while (await buttons.count()) {
    await buttons.first().click();
    await page.clock.runFor(350);
  }
}
async function activate(page, button, touch) {
  if (touch) await button.tap();
  else {
    await button.focus();
    await page.keyboard.press('Enter');
  }
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
    for (const [file, digest] of Object.entries(binding.files))
      assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, file);
    assert.equal(
      report.playwright,
      JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    for (const phase of ['baseline', 'fixed']) {
      const frontend = path.join(
        root,
        phase === 'baseline'
          ? 'tmp/alert-ack-proof/baseline-source/frontend-modern'
          : 'frontend-modern',
      );
      const { createServer } = await import(
        path.join(root, 'frontend-modern/node_modules/vite/dist/node/index.js')
      );
      server = await createServer({
        root: frontend,
        configFile: path.join(frontend, 'vite.config.ts'),
        cacheDir: path.join(output, `${phase}-vite-cache`),
        optimizeDeps: { noDiscovery: true, include: [] },
        plugins: [
          {
            name: 'synthetic-acknowledgement-producer',
            configureServer(vite) {
              vite.middlewares.use(api);
            },
          },
        ],
        server: { host: '127.0.0.1', port: 5267, strictPort: true, fs: { allow: [root] } },
      });
      await server.listen();
      const origin = 'http://127.0.0.1:5267';
      for (const spec of [
        { name: 'desktop', engine: chromium, width: 1440, height: 900, touch: false },
        { name: 'phone', engine: webkit, width: 320, height: 844, touch: true },
      ]) {
        current = {
          phase,
          name: spec.name,
          viewport: { width: spec.width, height: spec.height },
          requests: [],
          scans: [],
          checks: [],
          errors: [],
        };
        report.cases.push(current);
        browser = await spec.engine.launch(
          spec.touch
            ? { headless: true }
            : { headless: true, channel: 'chromium', args: ['--no-sandbox'] },
        );
        current.browser_version = browser.version();
        const context = await browser.newContext({
          viewport: current.viewport,
          hasTouch: spec.touch,
          isMobile: spec.touch,
          colorScheme: spec.touch ? 'dark' : 'light',
          reducedMotion: 'reduce',
          timezoneId: 'UTC',
          locale: 'en-GB',
        });
        await context.addCookies([
          { name: 'pulse_csrf', value: 'synthetic-fixture-only', url: origin },
        ]);
        const page = await context.newPage();
        page.setDefaultTimeout(10000);
        page.on('pageerror', (error) => current.errors.push(error.message));
        await page.route('**/*', (route) => {
          const req = route.request(),
            url = new URL(req.url());
          if (
            url.origin !== origin ||
            (!['GET', 'HEAD'].includes(req.method()) &&
              !(req.method() === 'POST' && url.pathname === '/api/alerts/bulk/acknowledge'))
          ) {
            report.unexpected.push(`${req.method()} ${url.origin}${url.pathname}`);
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
        await page.clock.install({ time: new Date('2026-10-20T12:00:00Z') });
        await page.goto(`${origin}/browser-tests/alert-group-acknowledgement.html`, {
          waitUntil: 'domcontentloaded',
          timeout: 60000,
        });
        await expect(page.locator('#alert-primary')).toBeVisible();
        const group = page.locator('#alert-primary').locator('..');
        const otherGroup = page.locator('#alert-other-primary').locator('..');
        await group.getByRole('button', { name: /^\+2 linked signals/ }).click();
        await expect(group.locator('#alert-critical')).toBeVisible();
        const groupButton = group.getByRole('button', { name: /^Acknowledge all/ });
        await expect(groupButton).toHaveText(`Acknowledge all (${phase === 'baseline' ? 3 : 2})`);
        const allButton = page.getByRole('button', { name: 'Acknowledge all (4)', exact: true });
        const otherButton = otherGroup.getByRole('button', { name: /^Acknowledge all/ });
        await activate(page, groupButton, spec.touch);
        await expect.poll(() => pending.length).toBe(1);
        // Native activation of a disabled button cannot submit another write.
        await groupButton.evaluate((button) => button.click());
        if (phase === 'baseline') {
          await expect.poll(() => pending.length).toBe(2);
          await expect(groupButton).toBeEnabled();
          await expect(allButton).toBeEnabled();
          current.checks.push(
            'Baseline: group count includes an acknowledged member, repeated activation submits twice, and Acknowledge all remains enabled over the same pending members.',
          );
        } else {
          await expect(groupButton).toBeDisabled();
          await expect(groupButton).toHaveAttribute('aria-busy', 'true');
          await expect(allButton).toBeDisabled();
          await allButton.evaluate((button) => button.click());
          await expect(otherButton).toBeEnabled();
          await expect(
            group
              .locator('#alert-primary')
              .getByRole('button', { name: 'Acknowledge', exact: true }),
          ).toBeDisabled();
          assert.equal(pending.length, 1);
          current.checks.push(
            'Group count excludes the acknowledged member; pending group, card and all controls cannot duplicate the request; the independent same-name group remains enabled.',
          );
          await capture(page, 'pending');
          await scan(page, 'pending');
        }
        release([
          { alertIdentifier: 'primary', success: true },
          { alertIdentifier: 'critical', success: false, error: 'Synthetic rejection' },
        ]);
        await expect(
          page.getByText('Acknowledged 1 alert.', { exact: true }).first(),
        ).toBeVisible();
        if (phase === 'baseline') {
          await expect(
            page.getByText('Failed to acknowledge 1 alert.', { exact: true }),
          ).toHaveCount(0);
          await capture(page, 'silent-partial-failure');
          current.checks.push(
            'Baseline: the endpoint reports a failed member, but only success notifications render; the critical card remains active without acknowledgement-failure feedback.',
          );
        } else {
          await expect(
            page.getByRole('alert').filter({ hasText: 'Failed to acknowledge 1 alert.' }),
          ).toBeVisible();
          await page.clock.runFor(1600);
          const retry = group.getByRole('button', { name: 'Acknowledge all (1)', exact: true });
          await expect(retry).toBeEnabled();
          const snapshot = await page.evaluate(() => window.__alertAckProof.snapshot());
          assert.equal(snapshot.primary.acknowledged, true);
          assert.equal(snapshot.critical.acknowledged, false);
          assert.equal(snapshot.otherPrimary.acknowledged, false);
          await capture(page, 'partial-failure');
          await scan(page, 'partial-failure');
          await dismissToasts(page);
          await activate(page, retry, spec.touch);
          await expect.poll(() => pending.length).toBe(1);
          assert.deepEqual(pending[0].identifiers, ['critical']);
          release([{ alertIdentifier: 'critical', success: false }]);
          await expect(
            page.getByText('Failed to acknowledge 1 alert.', { exact: true }),
          ).toBeVisible();
          await expect(page.getByText('Acknowledged 1 alert.', { exact: true })).toHaveCount(0);
          await page.clock.runFor(1600);
          await expect(retry).toBeEnabled();
          await dismissToasts(page);
          await activate(page, retry, spec.touch);
          await expect.poll(() => pending.length).toBe(1);
          release([], 400);
          await expect(
            page.getByText('Failed to acknowledge alerts', { exact: true }),
          ).toBeVisible();
          await page.clock.runFor(1600);
          await expect(retry).toBeEnabled();
          await capture(page, 'rejected-request');
          await scan(page, 'rejected-request');
          await dismissToasts(page);
          await activate(page, retry, spec.touch);
          await expect.poll(() => pending.length).toBe(1);
          release([{ alertIdentifier: 'critical', success: true }]);
          await expect(page.getByText('Acknowledged 1 alert.', { exact: true })).toBeVisible();
          await page.clock.runFor(1600);
          await expect(group.getByRole('button', { name: /^Acknowledge all/ })).toHaveCount(0);
          assert.equal(
            (await page.evaluate(() => window.__alertAckProof.snapshot())).critical.acknowledged,
            true,
          );
          await capture(page, 'acknowledged-not-resolved');
          await dismissToasts(page);
          const acknowledgeAll = page
            .getByRole('button', { name: 'Acknowledge all (2)', exact: true })
            .first();
          await activate(page, acknowledgeAll, spec.touch);
          await expect.poll(() => pending.length).toBe(1);
          assert.deepEqual(pending[0].identifiers.sort(), ['other-primary', 'other-related']);
          await expect(otherButton).toBeDisabled();
          await otherButton.evaluate((button) => button.click());
          assert.equal(pending.length, 1);
          release([
            { alertIdentifier: 'other-primary', success: false },
            { alertIdentifier: 'other-related', success: true },
          ]);
          await expect(
            page.getByText('Failed to acknowledge 1 alert.', { exact: true }),
          ).toBeVisible();
          await expect(page.getByText('Acknowledged 1 alert.', { exact: true })).toBeVisible();
          await page.clock.runFor(1600);
          const final = await page.evaluate(() => window.__alertAckProof.snapshot());
          assert.equal(final.otherPrimary.acknowledged, false);
          assert.equal(final.otherRelated.acknowledged, true);
          assert.equal(Object.keys(final).length, 5);
          await capture(page, 'all-partial-failure');
          await scan(page, 'all-partial-failure');
          current.checks.push(
            'Partial and all-failed group replies retain rejected alerts and render failure counts; whole-request rejection retains retry; deliberate success only acknowledges, without deleting observations. Acknowledge all uses the same result semantics and reserves every member.',
          );
        }
        assert.equal(pending.length, 0);
        assert.deepEqual(current.errors, []);
        await browser.close();
        browser = null;
      }
      await server.close();
      server = null;
    }
    assert.deepEqual(report.unexpected, []);
    report.result = 'passed';
  } catch (error) {
    report.result = 'failed';
    report.error = error.stack || String(error);
    process.exitCode = 1;
  } finally {
    for (const { res } of pending.splice(0)) {
      res.writeHead(503);
      res.end('{}');
    }
    if (browser) await browser.close();
    report.cleanup.browser_closed = true;
    if (server) await server.close();
    report.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: report.result,
        cases: report.cases.length,
        captures: report.captures.length,
        output,
        error: report.error || null,
      }),
    );
  }
})();
