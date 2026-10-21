// Root-disk canonical deltas through the real merger, full machine drawer and
// shipped CSS. Reuses the static fixture; no guest, disk or service operation.
const { chromium, webkit, expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/resource-agent-proof/static');
const output = path.resolve('tmp/root-disk-proof/browser');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const report = {
  result: 'incomplete',
  source: null,
  harness_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  profiles: [],
  captures: [],
  cleanup: {
    browsersCreated: 0,
    browsersClosed: 0,
    contextsCreated: 0,
    contextsClosed: 0,
    serverClosed: false,
  },
  limits:
    'Synthetic host canonical deltas and loopback GETs, real merger/Infrastructure drawer/root card. Not a disk/guest operation, collection, alert policy, installed recovery or release acceptance.',
};
let server, browser, context;
module.exports = (async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
    assert.equal(
      report.playwright,
      JSON.parse(fs.readFileSync('tests/integration/package-lock.json')).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
    report.source = binding.source_sha;
    report.binding_sha256 = hash(fs.readFileSync(path.join(compiled, 'binding.json')));
    for (const [file, digest] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Changed source: ${file}`);
    for (const file of [
      'src/components/Infrastructure/useResourceDetailDrawerDerivedState.ts',
      'src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx',
      'src/components/Infrastructure/resourceDetailMappers.ts',
      'src/components/shared/cards/RootDiskCard.tsx',
      'src/utils/resourceStateAdapters.ts',
    ])
      assert(binding.sources[file], `Missing source ${file}`);
    const assets = new Map(
      Object.entries(binding.artifacts).map(([file, digest]) => {
        assert(!path.isAbsolute(file) && !file.split('/').includes('..'));
        const bytes = fs.readFileSync(path.join(compiled, file));
        assert.equal(hash(bytes), digest, file);
        return ['/' + file, bytes];
      }),
    );
    server = http.createServer((req, res) => {
      const file = new URL(req.url, 'http://127.0.0.1').pathname,
        bytes = assets.get(file);
      res.setHeader(
        'Content-Type',
        file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html',
      );
      res.setHeader('Cache-Control', 'no-store');
      res.writeHead(bytes ? 200 : 404);
      res.end(bytes);
    });
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
      report.cleanup.browsersCreated++;
      context = await browser.newContext({
        viewport: { width, height: 900 },
        hasTouch: width < 500,
        isMobile: width < 500,
        locale: 'en-GB',
        timezoneId: 'UTC',
        reducedMotion: 'reduce',
      });
      report.cleanup.contextsCreated++;
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      await page.clock.install({ time: new Date('2026-10-20T12:00:00Z') });
      const record = {
        name,
        width,
        height: 900,
        dark,
        browser: browser.version(),
        checks: [],
        requests: [],
        unexpected: [],
        errors: [],
        failed: [],
      };
      report.profiles.push(record);
      page.on('pageerror', (err) => record.errors.push(err.message));
      page.on('console', (msg) => {
        if (msg.type() === 'error') record.errors.push(msg.text());
      });
      page.on('requestfailed', (req) =>
        record.failed.push({ path: new URL(req.url()).pathname, error: req.failure() }),
      );
      await page.addInitScript(
        (dark) =>
          document.addEventListener(
            'DOMContentLoaded',
            () => document.documentElement.classList.toggle('dark', dark),
            { once: true },
          ),
        dark,
      );
      await page.routeWebSocket('**/*', (socket) => socket.close());
      await page.route('**/*', (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin || req.method() !== 'GET') {
          record.unexpected.push({ path: url.pathname, method: req.method() });
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        record.requests.push({ path: url.pathname, method: req.method() });
        if (url.pathname === '/api/metrics-store/history')
          return route.fulfill({
            json: {
              resourceType: url.searchParams.get('resourceType'),
              resourceId: url.searchParams.get('resourceId'),
              range: url.searchParams.get('range'),
              source: 'store',
              metrics: {},
            },
          });
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
          record.unexpected.push({ path: url.pathname, method: req.method() });
          return route.fulfill({ status: 404, json: { error: 'Unexpected fixture read' } });
        }
        return route.fulfill({ json: data });
      });
      const act = async (method, ...args) =>
        page.evaluate(([method, args]) => window.__resourceAgent[method](...args), [method, args]);
      const check = (value) => record.checks.push(value);
      const capture = async (label) => {
        const host = page.getByTestId('resource-host-details-section');
        await host.scrollIntoViewIfNeeded();
        await page.evaluate(() => document.fonts.ready);
        const file = `${name}-${label}.png`;
        await host.screenshot({ path: path.join(output, file) });
        report.captures.push({
          file,
          sha256: hash(fs.readFileSync(path.join(output, file))),
          name,
          label,
        });
      };
      await page.goto(origin + '/browser-tests/resource-agent-observation.html?kind=root-disk', {
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
      await page.evaluate(() => {
        window.__rootDetails = document.querySelector('[data-testid="resource-technical-details"]');
      });
      const card = page.getByTestId('root-disk-card');
      const unknown = async (capacity = false) => {
        await expect(card.getByRole('status')).toHaveText(
          capacity ? 'Usage unavailable · 100 GB total' : 'Usage unavailable',
        );
        await expect(card.locator('[data-stacked-disk-trigger]')).toHaveCount(0);
        await expect(card).not.toContainText('0 B / 0 B');
        const status = card.getByRole('status');
        await expect(status).toBeVisible();
        assert(await status.evaluate((el) => el.classList.contains('text-muted')));
      };
      await unknown();
      check('Missing aggregate is visibly unavailable, not a zero/healthy reading');
      await capture('missing');
      await act('rootDisk', { current: 0, total: 100 * 1024 ** 3 });
      await unknown(true);
      check('Capacity-only canonical delta keeps actual total, no invented used bytes/bar');
      await capture('capacity');
      const GB = 1024 ** 3;
      await act('rootDisk', { current: 93, total: 100 * GB, used: 93 * GB });
      await expect(card.getByText('93.0 GB / 100 GB', { exact: true })).toBeVisible();
      await expect(card.locator('[data-stacked-disk-trigger]')).toContainText('93%');
      await expect(card.getByRole('status')).toHaveCount(0);
      check('First measured report reaches the real bar and byte summary in the open drawer');
      await capture('measured');
      for (const disk of [
        { current: 0 },
        { current: 0, total: 0, used: 0 },
        { current: -1, total: 100 * GB, used: 93 * GB },
        { current: 25, total: 100 * GB },
        { current: 25, total: 100 * GB, used: 101 * GB },
      ]) {
        await act('rootDisk', disk);
        await unknown(disk.total > 0);
      }
      check(
        'Percent-only, zero-capacity, unavailable sentinel, missing used and contradictory byte deltas remove prior live utilization',
      );
      await act('rootDisk', { current: 0, total: 100 * GB, used: 0 });
      await expect(card.getByText('0 B / 100 GB', { exact: true })).toBeVisible();
      await expect(card.locator('[data-stacked-disk-trigger]')).toContainText('0%');
      await expect(card.getByRole('status')).toHaveCount(0);
      check('Explicit zero used bytes remains a real measured zero');
      await capture('zero');
      await act('rootDisk');
      await unknown();
      await expect(details).toHaveAttribute('open', '');
      assert(
        await page.evaluate(
          () =>
            window.__rootDetails ===
            document.querySelector('[data-testid="resource-technical-details"]'),
        ),
      );
      const fit = await card.evaluate((el) => ({
        viewport: innerWidth,
        page: document.documentElement.scrollWidth,
        width: el.clientWidth,
        scroll: el.scrollWidth,
      }));
      assert(fit.page <= fit.viewport + 1 && fit.scroll <= fit.width + 1, JSON.stringify(fit));
      check(
        'Removal keeps the same disclosure open, clears numbers and fits viewport: ' +
          JSON.stringify(fit),
      );
      if (width < 500) await summary.tap();
      else {
        await summary.focus();
        await page.keyboard.press('Enter');
      }
      await expect(details).not.toHaveAttribute('open', '');
      if (width < 500) await summary.tap();
      else await page.keyboard.press('Enter');
      await unknown();
      check('Keyboard/touch disclosure closes and reopens with current unavailable state');
      assert.equal(record.errors.length, 0, 'page/console errors');
      assert.equal(record.failed.length, 0, 'failed requests');
      assert.equal(record.unexpected.length, 0, 'unexpected or non-GET requests');
      check('Only synthetic allowlisted loopback GETs, no page/console/request errors');
      await context.close();
      context = undefined;
      report.cleanup.contextsClosed++;
      await browser.close();
      browser = undefined;
      report.cleanup.browsersClosed++;
    }
    report.result = 'passed';
  } catch (err) {
    report.error = String(err.stack || err);
    throw err;
  } finally {
    if (context) {
      await context.close();
      report.cleanup.contextsClosed++;
    }
    if (browser) {
      await browser.close();
      report.cleanup.browsersClosed++;
    }
    if (server) {
      await new Promise((resolve) => server.close(resolve));
      report.cleanup.serverClosed = true;
    }
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify(report));
  }
})().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
