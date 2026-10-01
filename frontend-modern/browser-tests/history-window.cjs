// Production PBS table/drawers/History with synthetic APIs, not installed collection.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, firefox, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const engine =
    process.argv.find((arg) => arg.startsWith('--engine='))?.split('=')[1] || 'chromium';
  assert.ok(['chromium', 'firefox', 'webkit'].includes(engine));
  const width = process.argv.includes('--phone') ? 390 : 1365;
  const baseline = process.argv.includes('--baseline');
  const requestedTheme = process.argv.find((arg) => arg.startsWith('--theme='))?.split('=')[1];
  assert.ok(!requestedTheme || ['light', 'dark'].includes(requestedTheme));
  const artifacts = path.join(
    root,
    'node_modules',
    `history-window-${engine}-${width}${baseline ? '-base' : ''}${requestedTheme ? `-${requestedTheme}` : ''}`,
  );
  fs.mkdirSync(artifacts, { recursive: true });
  fs.chmodSync(artifacts, 0o755);
  const progress = (stage, detail = {}) => {
    const record = { stage, ...detail, at: new Date().toISOString() };
    fs.writeFileSync(path.join(artifacts, 'progress.json'), JSON.stringify(record) + '\n', {
      mode: 0o644,
    });
    console.log(JSON.stringify(record));
  };
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-cache-')),
    server: { host: '127.0.0.1', port: 5225, strictPort: true },
  });
  const sourceHashes = Object.fromEntries(
    [
      'src/components/Workloads/GuestDrawerHistory.tsx',
      'src/components/Workloads/guestDrawerModel.ts',
    ].map((file) => [
      file,
      createHash('sha256')
        .update(fs.readFileSync(path.join(root, file)))
        .digest('hex'),
    ]),
  );
  const playwrightVersion = require('playwright/package.json').version;
  assert.equal(
    playwrightVersion,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  let browser;
  const observations = [];
  try {
    await server.listen();
    browser = await { chromium, firefox, webkit }[engine].launch(
      engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    for (const theme of requestedTheme ? [requestedTheme] : ['light', 'dark']) {
      const page = await browser.newPage({
        viewport: { width, height: width === 390 ? 844 : 900 },
        locale: 'en-GB',
        timezoneId: 'UTC',
        ...(width === 390 ? { isMobile: true, hasTouch: true } : {}),
      });
      page.setDefaultTimeout(25_000);
      const requests = [],
        errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      let state = 'success',
        advance = 0,
        held;
      const hour = 3_600_000;
      const end = Date.UTC(2026, 9, 1, 12);
      const durations = { '1h': hour, '6h': 6 * hour, '24h': 24 * hour, '7d': 7 * 24 * hour };
      const point = (minutes, value) => ({
        timestamp: end - minutes * 60_000,
        value,
        min: value,
        max: value,
      });
      const response = (type, id, range, empty = false) => ({
        resourceType: type,
        resourceId: id,
        range,
        start: end + advance - durations[range],
        end: end + advance,
        source: 'store',
        metrics: empty
          ? {}
          : Object.fromEntries(
              Object.entries({
                cpu: [
                  point(10, id.endsWith('one') ? 21 : id.endsWith('two') ? 31 : 41),
                  point(5, 45),
                ],
                memory: [point(5, 55)],
                disk: [point(8, 20), point(4, 30)],
                netin: [point(60, 0), point(5, 2048)],
                netout: [point(5, 4096)],
                diskread: [point(2, 1024), point(0, 2048)],
                diskwrite: [point(5, 0), point(0, 0)],
                temperature: [point(5, 50)],
              }).map(([metric, points]) => [
                metric,
                points.filter(
                  (point) =>
                    point.timestamp >= end + advance - durations[range] &&
                    point.timestamp <= end + advance,
                ),
              ]),
            ),
      });
      await page.route('**/*', async (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5225') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          const type = url.searchParams.get('resourceType'),
            id = url.searchParams.get('resourceId'),
            range = url.searchParams.get('range');
          assert.ok(
            ['vm-one', 'vm-two', 'agent-three'].includes(id),
            `uncorroborated target ${id}`,
          );
          assert.ok(durations[range], `unexpected or locked read ${range}`);
          requests.push({ type, id, range, state, advance, method: route.request().method() });
          if (state === 'hold') {
            held = { route, type, id, range };
            return;
          }
          if (state === 'failure')
            return route.fulfill({ status: 503, json: { error: 'Private fixture detail' } });
          const result = response(type, id, range, state === 'empty');
          if (state === 'sparse')
            result.metrics = {
              cpu: [point(5, 0)],
              netin: [point(5, 0)],
              diskwrite: [point(5, 0)],
              temperature: [point(5, 50)],
            };
          return route.fulfill({ json: result });
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
      progress('loading-production-table', { theme });
      await page.goto('http://127.0.0.1:5225/browser-tests/pbs-identity-boundary.html', {
        waitUntil: 'domcontentloaded',
        timeout: 120_000,
      });
      await page.evaluate(
        (dark) => document.documentElement.classList.toggle('dark', dark),
        theme === 'dark',
      );
      await page.getByRole('button', { name: 'Corroborate links', exact: true }).click();
      progress('linked-table-ready', { theme });
      const xs = async (plot) =>
        [
          ...(await plot.locator('path').first().getAttribute('d')).matchAll(/[ML](-?[\d.]+),/g),
        ].map((match) => Number(match[1]));
      const expectedX = (minutes, range) =>
        34 + ((durations[range] - advance - minutes * 60_000) / durations[range]) * 318;
      const verifyWindow = async (detail, range) => {
        const axes = detail.getByTestId('guest-history-time-window');
        const groups = await detail.getByTestId('guest-history-group-chart').count();
        await axes.first().waitFor();
        assert.equal(await axes.count(), groups);
        for (const axis of await axes.all()) {
          const times = axis.locator('time');
          assert.equal(
            await times.nth(0).getAttribute('datetime'),
            new Date(end + advance - durations[range]).toISOString(),
          );
          assert.equal(
            await times.nth(1).getAttribute('datetime'),
            new Date(end + advance).toISOString(),
          );
          assert.match(await times.nth(0).getAttribute('aria-label'), /^Window start: /);
          assert.match(await times.nth(1).getAttribute('aria-label'), /^Window end: /);
          assert.notEqual(await times.nth(0).innerText(), await times.nth(1).innerText());
        }
      };
      let detail;
      for (const suffix of baseline ? ['three'] : ['one', 'two', 'three']) {
        const toggle = page.getByRole('button', {
          name: `Expand details for backup-connection-${suffix}`,
          exact: true,
        });
        // WebKit phone uses the native keyboard disclosure; its full-row touch
        // opening was not established. Touch inspection is exercised below.
        if (width === 1365 || engine === 'webkit') {
          await toggle.focus();
          await page.keyboard.press('Enter');
        } else await page.locator(`td[title="backup-connection-${suffix} · tank"]`).tap();
        assert.equal(
          await page
            .getByRole('button', {
              name: `Collapse details for backup-connection-${suffix}`,
              exact: true,
            })
            .getAttribute('aria-expanded'),
          'true',
        );
        detail = page.locator(`[data-inline-platform-resource-detail-for="pbs-${suffix}"]`);
        await detail.getByRole('tab', { name: 'History', exact: true }).click();
        const utilization = detail.locator('[data-history-group="utilization"]');
        const plot = utilization.getByTestId('guest-history-plot');
        await plot.locator('path').first().waitFor();
        const observedXs = await xs(plot);
        const observed = {
          suffix,
          theme,
          observedXs,
          expectedXs: [expectedX(10, '24h'), expectedX(5, '24h')],
          axes: await detail.getByTestId('guest-history-time-window').count(),
        };
        console.log(JSON.stringify({ timeWindowObservation: observed, sourceHashes }));
        await detail.screenshot({ path: path.join(artifacts, `${suffix}-24h-${theme}.png`) });
        assert.ok(Math.abs(observedXs[0] - expectedX(10, '24h')) < 0.01, JSON.stringify(observed));
        assert.ok(Math.abs(observedXs[1] - expectedX(5, '24h')) < 0.01, JSON.stringify(observed));
        await verifyWindow(detail, '24h');
        assert.ok(
          requests.some(
            (request) => request.id === (suffix === 'three' ? 'agent-three' : `vm-${suffix}`),
          ),
        );
        const dims = await page.evaluate(() => ({
          scroll: document.documentElement.scrollWidth,
          width: innerWidth,
        }));
        assert.ok(dims.scroll <= dims.width + 1, JSON.stringify(dims));
        observations.push({ ...observed, dims });
        progress('drawer-verified', { suffix, theme });
      }
      const utilization = detail.locator('[data-history-group="utilization"]');
      const slider = utilization.getByRole('slider', { name: 'Inspect Utilization history' });
      const network = detail.locator('[data-history-group="network"]');
      const networkSlider = network.getByRole('slider', { name: 'Inspect Network I/O history' });
      const readsBefore = requests.length;
      await slider.focus();
      await slider.press('Home');
      await slider.press('ArrowRight');
      await slider.press('ArrowRight');
      assert.match(
        await slider.getAttribute('aria-valuetext'),
        /01\/10\/2026, 11:55:00\. CPU 45\.0%\. Memory 55\.0%\. Disk no observation/,
      );
      const utilizationX = await utilization
        .getByTestId('guest-history-plot')
        .locator('circle[r="3"]')
        .first()
        .getAttribute('cx');
      await networkSlider.focus();
      await networkSlider.press('End');
      const networkX = await network
        .getByTestId('guest-history-plot')
        .locator('circle[r="3"]')
        .first()
        .getAttribute('cx');
      assert.equal(networkX, utilizationX);
      assert.match(
        (await network.innerText()).replace(/\s+/g, ' '),
        /In\s*2\.00 KB\/s Out\s*4\.00 KB\/s/,
      );
      if (width === 390) {
        await networkSlider.scrollIntoViewIfNeeded();
        const box = await networkSlider.boundingBox();
        assert.ok(box.height >= 44);
        await page.touchscreen.tap(box.x + 3, box.y + box.height / 2);
        assert.match(
          await networkSlider.getAttribute('aria-valuetext'),
          /In 0 B\/s\. Out no observation/,
        );
      }
      assert.equal(requests.length, readsBefore, 'inspection must not request metrics');
      progress('inspection-verified', { theme });
      await networkSlider.press('Tab');
      state = 'failure';
      await detail.getByRole('button', { name: 'Refresh history' }).click();
      await detail
        .getByText('History refresh failed. Showing previously loaded history.')
        .waitFor();
      await verifyWindow(detail, '24h');
      assert.ok(
        Math.abs(
          (await xs(utilization.getByTestId('guest-history-plot')))[0] - expectedX(10, '24h'),
        ) < 0.01,
      );
      assert.ok(!(await detail.innerText()).includes('Private fixture detail'));
      await detail.screenshot({ path: path.join(artifacts, `retained-window-${theme}.png`) });
      progress('retained-window-verified', { theme });
      advance = 10 * 60_000;
      state = 'success';
      await detail.getByRole('button', { name: 'Retry history' }).click();
      await detail.getByRole('button', { name: 'Refresh history' }).waitFor();
      await page.waitForFunction(
        (expected) =>
          document
            .querySelector('[data-testid="guest-history-time-window"] time:last-child')
            ?.getAttribute('datetime') === expected,
        new Date(end + advance).toISOString(),
      );
      await verifyWindow(detail, '24h');
      assert.ok(
        Math.abs(
          (await xs(utilization.getByTestId('guest-history-plot')))[0] - expectedX(10, '24h'),
        ) < 0.01,
      );
      // Source/range replacement must clear the old window as well as its samples.
      state = 'hold';
      progress('refreshed-window-verified', { theme });
      await detail.getByRole('combobox', { name: 'History range' }).selectOption('1h');
      await detail.getByText('Loading history', { exact: true }).first().waitFor();
      assert.equal(await detail.getByTestId('guest-history-time-window').count(), 0);
      assert.equal(await detail.getByTestId('guest-history-plot').locator('path').count(), 0);
      assert.equal(await detail.getByRole('slider').count(), 0);
      assert.ok(held);
      state = 'success';
      await held.route.fulfill({ json: response(held.type, held.id, held.range) });
      await utilization.getByTestId('guest-history-plot').locator('path').first().waitFor();
      await verifyWindow(detail, '1h');
      assert.ok(
        Math.abs(
          (await xs(utilization.getByTestId('guest-history-plot')))[0] - expectedX(10, '1h'),
        ) < 0.01,
      );
      await detail.screenshot({ path: path.join(artifacts, `one-hour-${theme}.png`) });
      progress('replacement-window-verified', { theme });
      state = 'sparse';
      await detail.getByRole('button', { name: 'Refresh history' }).click();
      await page.waitForFunction(
        () => document.querySelectorAll('[data-testid="guest-history-plot"] path').length === 0,
      );
      await detail.screenshot({ path: path.join(artifacts, `sparse-${theme}.png`) });
      const sparseObservation = {
        theme,
        pointCount: await detail.locator('[data-history-observation]').count(),
        collectingClaims: await detail.getByText('Collecting history', { exact: true }).count(),
        descriptions: await detail
          .getByTestId('guest-history-plot')
          .evaluateAll((plots) =>
            plots.map(
              (plot) => document.getElementById(plot.getAttribute('aria-describedby'))?.textContent,
            ),
          ),
      };
      progress('sparse-observation', sparseObservation);
      assert.equal(sparseObservation.pointCount, 4, 'each lone series must be visible');
      assert.equal(sparseObservation.collectingClaims, 0);
      assert.equal(await detail.getByRole('slider').count(), 0);
      assert.equal(await detail.getByText('Single observation. No trend yet.').count(), 4);
      for (const dot of await detail.locator('[data-history-observation]').all()) {
        assert.ok(Math.abs(Number(await dot.getAttribute('cx')) - expectedX(5, '1h')) < 0.01);
        assert.ok(Number.isFinite(Number(await dot.getAttribute('cy'))));
      }
      await verifyWindow(detail, '1h');
      state = 'failure';
      await detail.getByRole('button', { name: 'Refresh history' }).click();
      await detail
        .getByText('History refresh failed. Showing previously loaded history.')
        .waitFor();
      assert.equal(await detail.locator('[data-history-observation]').count(), 4);
      assert.equal(await detail.getByText('Single observation. No trend yet.').count(), 4);
      state = 'empty';
      await detail.getByRole('button', { name: 'Retry history' }).click();
      await page.waitForFunction(
        () => document.querySelectorAll('[data-history-observation]').length === 0,
      );
      await verifyWindow(detail, '1h');
      assert.equal(await detail.getByRole('slider').count(), 0);
      assert.equal(
        await detail.getByText('No stored history in this range', { exact: true }).count(),
        4,
      );
      assert.equal(await detail.getByText('Single observation. No trend yet.').count(), 0);
      await detail.screenshot({ path: path.join(artifacts, `empty-${theme}.png`) });
      state = 'success';
      await detail.getByRole('combobox', { name: 'History range' }).selectOption('7d');
      await utilization.getByTestId('guest-history-plot').locator('path').first().waitFor();
      await verifyWindow(detail, '7d');
      const readsBeforeLock = requests.length;
      await detail.getByRole('combobox', { name: 'History range' }).selectOption('14d');
      await detail.getByText(/14 days history requires a higher license plan/).waitFor();
      assert.equal(requests.length, readsBeforeLock);
      assert.equal(await detail.getByTestId('guest-history-time-window').count(), 0);
      assert.equal(errors.length, 0, errors.join('\n'));
      // The direct production renderer deliberately supplies current readings.
      // A live fallback must be labelled, never plotted or attributed to a stored time.
      state = 'empty';
      await page.goto('http://127.0.0.1:5225/browser-tests/pbs-history-refresh.html');
      await page.evaluate(
        (dark) => document.documentElement.classList.toggle('dark', dark),
        theme === 'dark',
      );
      detail = page.getByTestId('history-refresh-fixture');
      await detail.getByText('No stored history in this range', { exact: true }).first().waitFor();
      const liveCPU = detail.locator(
        '[data-history-group="utilization"] [data-history-current="cpu"]',
      );
      assert.match((await liveCPU.innerText()).replace(/\s+/g, ' '), /CPU\s*42\.0%\s*current/);
      assert.equal(await detail.locator('[data-history-observation]').count(), 0);
      assert.equal(await detail.getByTestId('guest-history-plot').locator('path').count(), 0);
      await detail.screenshot({ path: path.join(artifacts, `current-empty-${theme}.png`) });
      observations.push({ theme, requests, errors, utilizationX, networkX });
      progress('completed-theme', { theme });
      await page.close();
    }
    fs.writeFileSync(
      path.join(artifacts, 'result.json'),
      JSON.stringify(
        {
          engine,
          width,
          browserVersion: browser.version(),
          playwrightVersion,
          sourceHashes,
          observations,
        },
        null,
        2,
      ) + '\n',
    );
    console.log(
      JSON.stringify({
        result: 'passed',
        engine,
        width,
        browserVersion: browser.version(),
        sourceHashes,
        cases: observations.length,
      }),
    );
  } finally {
    await browser?.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
