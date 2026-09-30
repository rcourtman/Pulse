// Direct production History renderer and query proof, not a complete PBS drawer or installed acceptance.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules/history-refresh-renderer-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-cache-')),
    server: { host: '127.0.0.1', port: 5219, strictPort: true },
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
    for (const width of process.argv.includes('--phone') ? [390] : [1365]) {
      for (const theme of ['light', 'dark']) {
        const page = await browser.newPage({
          viewport: { width, height: width === 390 ? 844 : 900 },
        });
        page.setDefaultTimeout(40_000);
        console.log(JSON.stringify({ width, theme, stage: 'opened context' }));
        const requests = [],
          errors = [],
          failures = [];
        let nextAction = 'success',
          held,
          historyValue = 42;
        page.on('pageerror', (error) => errors.push(error.message));
        page.on('requestfailed', (request) => {
          if (request.url().includes('/api/metrics-store/history'))
            failures.push({ url: request.url(), error: request.failure() });
        });
        const response = (id, range, value = historyValue) => ({
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
                value: value + i + step,
                min: value,
                max: value + i + step,
              })),
            ]),
          ),
        });
        await page.route('**/*', async (route) => {
          const request = route.request(),
            url = new URL(request.url());
          if (url.origin !== 'http://127.0.0.1:5219') return route.abort();
          if (!url.pathname.startsWith('/api/')) return route.continue();
          if (url.pathname === '/api/metrics-store/history') {
            const id = url.searchParams.get('resourceId'),
              range = url.searchParams.get('range');
            assert.ok(['agent-three', 'pbs-three'].includes(id), `unexpected target ${id}`);
            const action = nextAction;
            nextAction = 'success';
            requests.push({ id, range, action, method: request.method() });
            if (action === 'hold') {
              held = route;
              return;
            }
            if (action === 'failure')
              return route.fulfill({ status: 503, json: { error: 'Fixture unavailable' } });
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
        await page.goto('http://127.0.0.1:5219/browser-tests/pbs-history-refresh.html', {
          waitUntil: 'domcontentloaded',
          timeout: 30_000,
        });
        await page.evaluate(
          (dark) => document.documentElement.classList.toggle('dark', dark),
          theme === 'dark',
        );
        console.log(JSON.stringify({ width, theme, stage: 'page loaded' }));
        const detail = page.getByTestId('history-refresh-fixture');
        const plots = detail.locator('[data-testid="guest-history-plot"] path');
        console.log(JSON.stringify({ width, theme, stage: 'renderer available' }));
        const waitForPaths = async (count) =>
          page.waitForFunction(
            (expected) =>
              document.querySelectorAll('[data-testid="guest-history-plot"] path').length ===
              expected,
            count,
          );
        await waitForPaths(7);
        console.log(JSON.stringify({ width, theme, stage: 'history loaded' }));
        const status = detail.getByRole('status', { name: 'History refresh status' });
        assert.equal(await status.count(), 1);
        assert.equal(await status.innerText(), '');
        const paths = () =>
          plots.evaluateAll((items) => items.map((item) => item.getAttribute('d')));
        const original = await paths();
        const checkLayout = async () => {
          const dimensions = await page.evaluate(() => ({
            scroll: document.documentElement.scrollWidth,
            inner: innerWidth,
          }));
          assert.ok(dimensions.scroll <= dimensions.inner + 1, JSON.stringify(dimensions));
          const button = detail.getByRole('button', { name: /^(Retry|Refresh) history$/ });
          const bounds = await button.boundingBox();
          assert.ok(bounds && bounds.x >= 0 && bounds.x + bounds.width <= width + 1);
          if (width === 390) assert.ok(bounds.height >= 44, JSON.stringify(bounds));
          return dimensions;
        };
        const screenshot = (state) =>
          detail.screenshot({ path: path.join(artifacts, `${state}-${theme}-${width}.png`) });
        const waitForHeld = async () => {
          for (let i = 0; !held && i < 100; i++) await page.waitForTimeout(10);
          assert.ok(held, 'manual refresh was not intercepted');
        };

        // A failed background read must keep this source's observations, labelled honestly.
        nextAction = 'failure';
        await status
          .getByText('History refresh failed. Showing previously loaded history.', { exact: true })
          .waitFor();
        assert.deepEqual(await paths(), original);
        assert.equal(await status.getAttribute('aria-live'), 'polite');
        assert.equal(await status.getAttribute('aria-atomic'), 'true');
        await checkLayout();
        await screenshot('refresh-failed');
        console.log(JSON.stringify({ width, theme, stage: 'failure retained' }));

        // Enter starts one retry; focus and the old points survive until its successful response.
        const retry = detail.getByRole('button', { name: 'Retry history', exact: true });
        nextAction = 'hold';
        held = undefined;
        await retry.focus();
        await page.keyboard.press('Enter');
        await waitForHeld();
        assert.equal(await retry.getAttribute('aria-busy'), 'true');
        assert.equal(await retry.getAttribute('aria-disabled'), 'true');
        await page.keyboard.press('Enter');
        assert.equal(requests.filter((r) => r.action === 'hold').length, 1);
        assert.deepEqual(await paths(), original);
        historyValue = 12;
        await held.fulfill({ json: response('agent-three', '24h') });
        await status.getByText(/History refresh failed/).waitFor({ state: 'hidden' });
        const refresh = detail.getByRole('button', { name: 'Refresh history', exact: true });
        assert.equal(await refresh.evaluate((button) => document.activeElement === button), true);
        assert.notDeepEqual(await paths(), original);
        assert.equal(await refresh.getAttribute('aria-busy'), 'false');
        await screenshot('retry-recovered');
        console.log(JSON.stringify({ width, theme, stage: 'retry recovered' }));

        // A slow manual refresh replaced by polling must release its loading ownership.
        nextAction = 'hold';
        held = undefined;
        await refresh.click();
        await waitForHeld();
        const obsoleteRoute = held;
        historyValue = 25;
        await page.waitForFunction(
          () => document.querySelector('button[aria-busy="false"]') !== null,
        );
        for (let i = 0; (await refresh.getAttribute('aria-busy')) !== 'false' && i < 100; i++)
          await page.waitForTimeout(10);
        assert.equal(await refresh.getAttribute('aria-busy'), 'false');
        assert.equal(await refresh.getAttribute('aria-disabled'), 'false');
        const latest = await paths();
        let lateResponse;
        try {
          await obsoleteRoute.fulfill({ json: response('agent-three', '24h', 99) });
          lateResponse = 'fulfilled';
        } catch (error) {
          lateResponse = String(error);
        }
        await page.evaluate(() => new Promise(requestAnimationFrame));
        assert.deepEqual(await paths(), latest);

        // A different target has no right to the old host's history, including after a failure.
        nextAction = 'failure';
        await page.getByRole('button', { name: 'Switch to service target', exact: true }).click();
        await status.getByText('Failed to load history data', { exact: true }).waitFor();
        assert.equal(await plots.count(), 0);
        assert.equal(await detail.getByText('Collecting history', { exact: true }).count(), 0);
        assert.equal(await detail.getByText(/previously loaded/).count(), 0);
        await checkLayout();
        await screenshot('replacement-failed');
        await detail.getByRole('button', { name: 'Retry history', exact: true }).click();
        await waitForPaths(2);
        assert.equal(requests.at(-1).id, 'pbs-three');
        assert.equal(requests.at(-1).range, '24h');
        assert.equal(await detail.locator('[data-history-group="network"] path').count(), 0);
        assert.equal(await detail.locator('[data-history-group="disk-io"] path').count(), 0);
        const dimensions = await checkLayout();
        const beforeLocked = requests.length;
        await detail.getByTestId('guest-history-range-control').selectOption('14d');
        await detail.getByText(/14 days history requires a higher license plan/).waitFor();
        assert.equal(
          await detail.getByRole('button', { name: /^(Retry|Refresh) history$/ }).count(),
          0,
        );
        assert.equal(requests.length, beforeLocked);
        assert.ok(requests.every((r) => r.method === 'GET'));
        assert.deepEqual(errors, []);
        observations.push({ width, theme, requests, failures, dimensions, lateResponse, errors });
        await page.close();
      }
    }
    const files = [
      'src/components/Workloads/GuestDrawerHistory.tsx',
      'src/hooks/createNonSuspendingQuery.ts',
    ];
    const result = {
      result: 'passed',
      browser: browser.version(),
      playwright: require('playwright/package.json').version,
      content_sha256: Object.fromEntries(
        files.map((file) => [
          `frontend-modern/${file}`,
          createHash('sha256')
            .update(fs.readFileSync(path.join(root, file)))
            .digest('hex'),
        ]),
      ),
      observations,
    };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
    for (const name of fs.readdirSync(artifacts)) {
      const file = path.join(artifacts, name);
      if (fs.statSync(file).isFile()) fs.chmodSync(file, 0o644);
    }
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
