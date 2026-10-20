// Static, exact-source real-consumer proof. No build or collector runs here.
const { chromium, webkit } = require('playwright');
const { expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/custom-sensor-proof/static');
const output = path.resolve('tmp/custom-sensor-proof/browser');
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
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      await page.clock.install({ time: new Date('2026-10-20T12:00:00Z') });
      const run = {
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
      page.on('pageerror', (err) => run.errors.push(err.message));
      page.on('console', (msg) => {
        if (msg.type() === 'error') run.consoleErrors.push(msg.text());
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
      await page.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin || req.method() !== 'GET') {
          run.unexpected.push({ url: url.href, method: req.method() });
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        run.reads.push(url.pathname);
        if (url.pathname === '/api/metrics-store/history') {
          assert.equal(url.searchParams.get('resourceType'), 'agent');
          assert.equal(url.searchParams.get('resourceId'), 'sensor-machine');
          return route.fulfill({
            json: {
              resourceType: 'agent',
              resourceId: 'sensor-machine',
              range: url.searchParams.get('range'),
              source: 'store',
              start: Date.parse('2026-10-19T12:00:00Z'),
              end: Date.parse('2026-10-20T12:00:00Z'),
              metrics: {},
            },
          });
        }
        const data = {
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
        if (!data) {
          run.unexpected.push({ url: url.href, method: req.method() });
          return route.fulfill({ status: 404, json: { error: 'Unexpected fixture read' } });
        }
        return route.fulfill({ json: data });
      });
      const check = (name) => run.checks.push(name);
      const capture = async (state) => {
        const card = page.getByText('Custom Metrics', { exact: true }).locator('..');
        await card.scrollIntoViewIfNeeded();
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), animations: 'disabled' });
        report.captures.push({
          file,
          sha256: hash(fs.readFileSync(path.join(output, file))),
          name,
          state,
        });
      };
      await page.goto(origin + '/browser-tests/custom-sensor-observation.html', {
        waitUntil: 'networkidle',
      });
      const details = page.getByTestId('resource-technical-details');
      const summary = details.locator(':scope > summary');
      if (width < 500) await summary.tap();
      else {
        await summary.focus();
        await page.keyboard.press('Enter');
      }
      await expect(details).toHaveAttribute('open', '');
      await expect(page.getByText('Custom Metrics', { exact: true })).toHaveCount(0);
      await page.evaluate(() => {
        window.__originalDetails = document.querySelector(
          '[data-testid="resource-technical-details"]',
        );
        window.__customSensors.report();
      });
      const card = page.getByText('Custom Metrics', { exact: true }).locator('..');
      await expect(card.getByText('Online', { exact: true })).toHaveCount(1);
      await expect(card.getByText('Online (stale)', { exact: true })).toHaveCount(1);
      await expect(card.getByText('2h ago', { exact: true })).toBeVisible();
      await expect(card.getByText('0 items', { exact: true })).toBeVisible();
      await expect(card.getByText('Unavailable', { exact: true })).toBeVisible();
      check(
        'First report appears in the same-open real drawer; typed current, stale, zero and unavailable values remain distinct.',
      );
      await capture('current');
      const before = await page.evaluate(() => window.__customSensors.evidence());
      await page.evaluate(() => window.__customSensors.stale(true));
      await expect(card.getByRole('status')).toHaveText(
        'Last known readings, not current: host agent stopped reporting.',
      );
      await expect(card.getByText('Online', { exact: true })).toHaveCount(0);
      await expect(card.getByText('Online (last known)', { exact: true })).toHaveCount(2);
      await expect(
        card.getByText('2h old when observed (last known)', { exact: true }),
      ).toBeVisible();
      await expect(card.getByText('0 items (last known)', { exact: true })).toBeVisible();
      await expect(card.getByText('Unavailable', { exact: true })).toBeVisible();
      const after = await page.evaluate(() => window.__customSensors.evidence());
      assert.deepEqual(after.agent.sensors, before.agent.sensors);
      assert.equal(after.lastSeen, before.lastSeen);
      assert.equal(after.status, 'online');
      const title = await card
        .getByText('Online (last known)', { exact: true })
        .last()
        .getAttribute('title');
      assert.match(title, /Last reported status: Error.*Synthetic read timed out/);
      assert.match(
        await card
          .getByText('2h old when observed (last known)', { exact: true })
          .getAttribute('title'),
        /Last reported status: Critical/,
      );
      check(
        'Parent lease expiry qualifies all custom values without mutating source values, times, errors, status or online platform state.',
      );
      await capture('retained');
      await page.evaluate(() => window.__customSensors.platform());
      await expect(card.getByText('Online (last known)', { exact: true })).toHaveCount(2);
      check('Independent platform updates do not renew a retained agent reading.');
      await page.evaluate(() => window.__customSensors.stale(false));
      await expect(card.getByText('Online', { exact: true })).toHaveCount(1);
      await expect(card.getByText('Online (stale)', { exact: true })).toHaveCount(1);
      await expect(card.getByRole('status')).toHaveCount(0);
      check('A current parent restores its own scope, never an independently failed sensor.');
      await page.evaluate(() => window.__customSensors.remove());
      await expect(page.getByText('Custom Metrics', { exact: true })).toHaveCount(0);
      await page.evaluate(() => window.__customSensors.report(true));
      await expect(card.getByText('Online (last known)', { exact: true })).toHaveCount(2);
      await expect(details).toHaveAttribute('open', '');
      assert.equal(await details.evaluate((el) => el === window.__originalDetails), true);
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
        false,
      );
      check('Removal and reappearance preserve the same disclosure and fit the viewport.');
      assert.deepEqual(run.errors, []);
      assert.deepEqual(run.consoleErrors, []);
      assert.deepEqual(run.unexpected, []);
      await context.close();
      context = undefined;
      run.contextClosed = true;
      await browser.close();
      browser = undefined;
      run.browserClosed = true;
    }
    report.completed = true;
  } finally {
    if (context) await context.close();
    if (browser) await browser.close();
    await new Promise((resolve) => server.close(resolve));
    report.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify(report));
  }
})().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
