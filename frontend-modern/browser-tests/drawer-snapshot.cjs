const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const parent = false;
(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules/drawer-snapshot-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-')),
    server: { host: '127.0.0.1', port: 5242, strictPort: true },
  });
  const playwright = require('playwright/package.json').version;
  const expected = JSON.parse(
    fs.readFileSync('/workspace/tests/integration/package-lock.json', 'utf8'),
  ).packages['node_modules/@playwright/test'].version;
  assert.equal(playwright, expected);
  const observations = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, phone] of [
      ['chromium-desktop', chromium, false],
      ['webkit-phone', webkit, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: phone ? { width: 390, height: 844 } : { width: 1365, height: 900 },
        hasTouch: phone,
        isMobile: phone,
      });
      const requests = [],
        errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5242') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          const query = Object.fromEntries(url.searchParams);
          requests.push(query);
          const value =
            query.resourceId === 'history-a-current'
              ? 77
              : query.resourceId === 'history-b'
                ? 25
                : 12;
          return route.fulfill({
            json: {
              resourceType: 'agent',
              resourceId: query.resourceId,
              range: query.range,
              start: Date.now() - 60000,
              end: Date.now(),
              source: 'store',
              metrics: {
                cpu: [0, 1].map((i) => ({
                  timestamp: Date.now() - (1 - i) * 60000,
                  value,
                  min: value,
                  max: value,
                })),
              },
            },
          });
        }
        return route.fulfill({
          json:
            url.pathname === '/api/license/runtime-capabilities'
              ? {
                  capabilities: [],
                  limits: [],
                  max_history_days: 7,
                  hosted_mode: false,
                  runtime: { build: 'community' },
                  blocked_capabilities: [],
                }
              : {
                  capabilities: [],
                  relationships: [],
                  recentChanges: [],
                  audits: [],
                  count: 0,
                  available: false,
                  data: [],
                  enabled: false,
                },
        });
      });
      await page.goto('http://127.0.0.1:5242/browser-tests/drawer-snapshot.html');
      if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const detail = page.getByRole('region', { name: 'PBS detail verification' });
      const fleet = page.getByRole('region', { name: 'Availability fleet' });
      const capture = async (phase, scope) => {
        const dimensions = await page.evaluate(() => ({
          scroll: document.documentElement.scrollWidth,
          width: innerWidth,
        }));
        assert.ok(dimensions.scroll <= dimensions.width + 1, JSON.stringify(dimensions));
        const screenshot = `${parent ? 'parent-' : ''}${name}-${phase}.png`;
        await page.screenshot({ path: path.join(artifacts, screenshot), fullPage: true });
        observations.push({
          name,
          phase,
          browser: browser.version(),
          text: await scope.innerText(),
          dimensions,
          screenshot,
        });
      };
      await detail.getByRole('tab', { name: 'History', exact: true }).click();
      await detail.getByRole('combobox', { name: 'History range' }).selectOption('7d');
      await detail.getByText('12.0%', { exact: true }).waitFor();
      const history = await detail.getByTestId('resource-metrics-history-tab').elementHandle();
      await page.getByRole('button', { name: 'Update PBS A', exact: true }).click();
      await detail
        .getByRole('heading', { name: parent ? 'PBS A' : 'PBS A current', exact: true })
        .waitFor();
      await detail.getByText(parent ? '12.0%' : '77.0%', { exact: true }).waitFor();
      assert.equal(
        await detail.getByRole('combobox', { name: 'History range' }).inputValue(),
        '7d',
      );
      assert.ok(await history.evaluate((element) => element.isConnected));
      await capture('pbs-replacement', detail);
      await page.getByRole('button', { name: 'Withdraw target', exact: true }).click();
      if (parent) assert.equal(await detail.getByTestId('resource-metrics-history-tab').count(), 1);
      else await detail.getByText('Metrics history is unavailable.', { exact: true }).waitFor();
      await capture('pbs-withdrawn', detail);
      await page.getByRole('button', { name: 'Restore target', exact: true }).click();
      await detail.getByTestId('resource-metrics-history-tab').waitFor();
      assert.equal(
        await detail
          .getByRole('tab', { name: 'History', exact: true })
          .getAttribute('aria-selected'),
        'true',
      );
      await page.getByRole('button', { name: 'Select PBS B', exact: true }).click();
      await detail
        .getByRole('heading', { name: parent ? 'PBS A' : 'PBS B', exact: true })
        .waitFor();
      assert.equal(
        await detail
          .getByRole('tab', { name: parent ? 'History' : 'Overview', exact: true })
          .getAttribute('aria-selected'),
        'true',
      );
      if (!parent) await detail.getByRole('tab', { name: 'History', exact: true }).click();
      await detail.getByText(parent ? '12.0%' : '25.0%', { exact: true }).waitFor();
      await capture('pbs-selected-b', detail);
      const targets = [...new Set(requests.map((r) => r.resourceId))];
      assert.deepEqual(
        targets,
        parent ? ['history-a'] : ['history-a', 'history-a-current', 'history-b'],
      );
      assert.ok(
        requests.every(
          (r) => !('metric' in r) && r.resourceType === 'agent' && r.maxPoints === '240',
        ),
      );
      await fleet.getByRole('button', { name: 'Open details for Check A', exact: true }).click();
      const availability = fleet.getByTestId('availability-probe-status');
      await availability.getByText('Up', { exact: true }).waitFor();
      await page.getByRole('button', { name: 'Fail check A', exact: true }).click();
      await availability.getByText(parent ? 'Up' : 'Down', { exact: true }).waitFor();
      await capture('availability-replacement', fleet);
      await fleet.getByRole('button', { name: 'Open details for Check B', exact: true }).click();
      await fleet
        .getByRole('heading', { name: parent ? 'Check A' : 'Check B', exact: true })
        .waitFor();
      await capture('availability-selected-b', fleet);
      await page.getByRole('button', { name: 'Remove check B', exact: true }).click();
      assert.equal(
        await fleet
          .getByRole('heading', { name: parent ? 'Check A' : 'Check B', exact: true })
          .count(),
        parent ? 1 : 0,
      );
      await page.getByRole('button', { name: 'Restore check B', exact: true }).click();
      assert.equal(
        await fleet.locator('[id^="resource-detail-drawer-heading-"]').count(),
        parent ? 1 : 0,
      );
      await capture('availability-removed-restored', fleet);
      assert.deepEqual(errors, []);
      observations.push({ name, phase: 'requests', requests, errors });
      await browser.close();
      browser = null;
    }
    fs.writeFileSync(
      path.join(artifacts, parent ? 'parent.json' : 'result.json'),
      JSON.stringify({ playwright, expected, parent, observations }, null, 2),
    );
    console.log(JSON.stringify({ result: 'passed', parent, observations: observations.length }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
