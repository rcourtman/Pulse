// Mock-backed proof of the production PBS table/drawer, not installed acceptance.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const phaseFile = path.join(root, 'node_modules/history-proof-phase.txt');
  const phase = fs.existsSync(phaseFile) ? fs.readFileSync(phaseFile, 'utf8').trim() : 'candidate';
  const artifacts = path.join(root, `node_modules/history-source-isolation-${phase}`);
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5218, strictPort: true },
  });
  let browser;
  const observations = [];
  try {
    await server.listen();
    browser = await chromium.launch({
      headless: true,
      channel: 'chromium',
      args: ['--no-sandbox'],
    });
    for (const width of [1365, 390]) {
      const page = await browser.newPage({
        viewport: { width, height: width === 390 ? 844 : 900 },
      });
      page.setDefaultTimeout(20_000);
      const requests = [],
        failures = [],
        errors = [];
      const held = new Map();
      let holdOldRange = true,
        holdService = true;
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('requestfailed', (request) => {
        if (request.url().includes('/api/metrics-store/history'))
          failures.push({ url: request.url(), failure: request.failure() });
      });
      const response = (id, range) => ({
        resourceType: 'agent',
        resourceId: id,
        range,
        start: 1_700_000_000_000,
        end: 1_700_000_060_000,
        source: 'store',
        metrics: Object.fromEntries(
          (id === 'pbs-three'
            ? ['cpu', 'memory']
            : ['cpu', 'memory', 'disk', 'netin', 'netout', 'diskread', 'diskwrite']
          ).map((metric, i) => [
            metric,
            [0, 1].map((step) => ({
              timestamp: 1_700_000_000_000 + step * 60_000,
              value: (id === 'pbs-three' ? 12 : range === '1h' ? 31 : 88) + i + step,
              min: 10,
              max: 99,
            })),
          ]),
        ),
      });
      await page.route('**/*', async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        if (url.origin !== 'http://127.0.0.1:5218') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          const id = url.searchParams.get('resourceId'),
            range = url.searchParams.get('range');
          requests.push({ method: request.method(), id, range });
          assert.ok(['agent-three', 'pbs-three'].includes(id), `unexpected target ${id}`);
          if (id === 'agent-three' && range === '6h' && holdOldRange) {
            holdOldRange = false;
            held.set('range', route);
            return;
          }
          if (id === 'pbs-three' && range === '1h' && holdService) {
            holdService = false;
            held.set('service', route);
            return;
          }
          return route.fulfill({ json: response(id, range) });
        }
        if (url.pathname === '/api/license/runtime-capabilities')
          return route.fulfill({
            json: {
              capabilities: [],
              limits: [],
              max_history_days: 7,
              hosted_mode: false,
              runtime: { build: 'community', label: 'Pulse Community runtime' },
              blocked_capabilities: [],
            },
          });
        return route.fulfill({ json: { data: [], enabled: false } });
      });
      await page.goto('http://127.0.0.1:5218/browser-tests/pbs-identity-boundary.html', {
        waitUntil: 'domcontentloaded',
        timeout: 120_000,
      });
      await page.getByRole('button', { name: 'Corroborate links', exact: true }).click();
      const toggle = page.getByRole('button', {
        name: 'Expand details for backup-connection-three',
        exact: true,
      });
      if (width === 1365) {
        await toggle.focus();
        await page.keyboard.press('Enter');
      } else await page.locator('td[title="backup-connection-three · tank"]').click();
      const detail = page.locator('[data-inline-platform-resource-detail-for="pbs-three"]');
      const plots = detail.locator('[data-testid="guest-history-plot"] path');
      await detail.getByRole('tab', { name: 'History', exact: true }).click();
      await page.waitForFunction(
        () => document.querySelectorAll('[data-testid="guest-history-plot"] path').length === 7,
      );
      const originalPaths = await plots.evaluateAll((paths) =>
        paths.map((p) => p.getAttribute('d')),
      );
      const checkLayout = async () => {
        const dimensions = await page.evaluate(() => ({
          scroll: document.documentElement.scrollWidth,
          inner: innerWidth,
        }));
        assert.ok(dimensions.scroll <= dimensions.inner + 1, JSON.stringify(dimensions));
        assert.equal(
          await detail
            .getByRole('tab', { name: 'History', exact: true })
            .getAttribute('aria-selected'),
          'true',
        );
        return dimensions;
      };
      const screenshot = async (state) =>
        page.screenshot({ path: path.join(artifacts, `${state}-${width}.png`), fullPage: true });
      await screenshot('host-loaded');
      await detail.getByTestId('guest-history-range-control').selectOption('6h');
      await page.waitForFunction(
        () => document.querySelector('[data-testid="guest-history-range-control"]').value === '6h',
      );
      // Wait for the route, not an arbitrary sleep or the very assertion under test.
      for (let attempt = 0; !held.has('range') && attempt < 100; attempt++)
        await page.waitForTimeout(10);
      assert.ok(held.has('range'), 'range request was not intercepted');
      await screenshot('range-loading');
      assert.equal(await plots.count(), 0, 'old range plotted while 6h replacement is loading');
      assert.ok((await detail.getByText('Loading history', { exact: true }).count()) > 0);
      await checkLayout();
      await detail.getByTestId('guest-history-range-control').selectOption('1h');
      await page.waitForFunction(
        () => document.querySelectorAll('[data-testid="guest-history-plot"] path').length === 7,
      );
      const currentPaths = await plots.evaluateAll((paths) =>
        paths.map((p) => p.getAttribute('d')),
      );
      assert.notDeepEqual(currentPaths, originalPaths);
      let lateResponse;
      try {
        await held.get('range').fulfill({ json: response('agent-three', '6h') });
        lateResponse = 'fulfilled';
      } catch (error) {
        lateResponse = String(error);
      }
      // A marker evaluation after the obsolete response; the unit proof also
      // deliberately resolves a mock which ignores AbortSignal entirely.
      await page.evaluate(() => new Promise(requestAnimationFrame));
      assert.deepEqual(
        await plots.evaluateAll((paths) => paths.map((p) => p.getAttribute('d'))),
        currentPaths,
      );
      await page.getByRole('button', { name: 'Withdraw third link', exact: true }).click();
      for (let attempt = 0; !held.has('service') && attempt < 100; attempt++)
        await page.waitForTimeout(10);
      assert.ok(held.has('service'), 'withdrawal request was not intercepted');
      await screenshot('target-loading');
      assert.equal(await plots.count(), 0, 'former host plotted while revoked target is loading');
      assert.ok((await detail.getByText('Loading history', { exact: true }).count()) > 0);
      await held
        .get('service')
        .fulfill({ status: 503, json: { error: 'Fixture replacement unavailable' } });
      await detail.getByText('Failed to load history data', { exact: true }).waitFor();
      assert.equal(await plots.count(), 0);
      await screenshot('target-failed');
      await detail.getByTestId('guest-history-range-control').selectOption('6h');
      await page.waitForFunction(
        () => document.querySelectorAll('[data-testid="guest-history-plot"] path').length === 2,
      );
      assert.equal(await detail.locator('[data-history-group="network"] path').count(), 0);
      assert.equal(await detail.locator('[data-history-group="disk-io"] path').count(), 0);
      assert.equal(await detail.getByText('Loading history', { exact: true }).count(), 0);
      await screenshot('service-loaded');
      const dimensions = await checkLayout();
      const beforeLocked = requests.length;
      await detail.getByTestId('guest-history-range-control').selectOption('14d');
      await detail.getByText(/14 days history requires a higher license plan/).waitFor();
      assert.equal(await plots.count(), 0);
      assert.equal(requests.length, beforeLocked);
      assert.ok(requests.every((r) => r.method === 'GET'));
      assert.deepEqual(errors, []);
      observations.push({ width, requests, failures, lateResponse, dimensions, errors });
      await page.close();
    }
    const file = 'src/components/Workloads/GuestDrawerHistory.tsx';
    const result = {
      result: 'passed',
      phase,
      browser: browser.version(),
      playwright: require('playwright/package.json').version,
      content_sha256: {
        [`frontend-modern/${file}`]: createHash('sha256')
          .update(fs.readFileSync(path.join(root, file)))
          .digest('hex'),
      },
      observations,
    };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
    for (const name of fs.readdirSync(artifacts)) fs.chmodSync(path.join(artifacts, name), 0o644);
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
