const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/workload-disk-metrics-static/final');
const output = path.join(root, 'tmp/disk-reading-proof/browser');
const origin = 'http://127.0.0.1:5321';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  result: 'incomplete',
  playwright: require('playwright/package.json').version,
  scope:
    'Synthetic Resources REST metrics through production useWorkloads, selection, GuestRow/GuestDrawer/History/CSS. Fixture selection controls, local empty History and supporting APIs, not full-app authentication, native measurements or recovery.',
  binding,
  driver_sha256: hash(fs.readFileSync(__filename)),
  cases: [],
  requests: [],
  captures: [],
  errors: [],
  cleanup: {},
};
const GB = 1024 ** 3;
const baseMetrics = [
  ['empty', { percent: 0, used: 0, total: 100 }],
  ['half-percent', { percent: 0.5, used: 0.5, total: 100 }],
  ['one-percent', { percent: 1, used: 1, total: 100 }],
  ['forty-percent', { percent: 40, used: 40, total: 100 }],
  ['critical', { percent: 95, used: 95, total: 100 }],
  ['capacity-only', { total: 100 }],
];
const resource = (name, metric, extras = {}) => ({
  id: `fixture-${name}`,
  type: 'vm',
  name,
  status: 'online',
  sources: ['proxmox'],
  platformType: 'proxmox-pve',
  lastSeen: '2026-10-14T19:00:00Z',
  metricsTarget: { resourceType: 'vm', resourceId: `fixture-${name}` },
  metrics: {
    cpu: { percent: 10 },
    memory: { percent: 25, used: GB, total: 4 * GB },
    disk: {
      value: 0,
      unit: 'bytes',
      ...Object.fromEntries(
        Object.entries(metric).map(([key, value]) => [
          key,
          key === 'used' || key === 'total' ? value * GB : value,
        ]),
      ),
    },
  },
  proxmox: {
    vmid: 101,
    instance: 'lab',
    nodeName: 'fixture-node',
    runtimeStatus: 'running',
    guestAgentStatus: 'available',
    ...extras,
  },
});
let server;
let browser;
(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(root + '/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  for (const [file, sha] of Object.entries(binding.importedSources))
    assert.equal(hash(fs.readFileSync(root + '/frontend-modern/' + file)), sha, file);
  for (const [file, sha] of Object.entries(binding.artifacts))
    assert.equal(hash(fs.readFileSync(compiled + '/' + file)), sha, file);
  server = http.createServer((req, res) => {
    const pathname = decodeURIComponent(new URL(req.url, origin).pathname);
    const file = path.join(compiled, pathname);
    if (!file.startsWith(compiled + '/') || !fs.existsSync(file) || !fs.statSync(file).isFile()) {
      res.writeHead(404);
      res.end();
      return;
    }
    res.setHeader(
      'Content-Type',
      file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html',
    );
    res.end(fs.readFileSync(file));
  });
  await new Promise((resolve) => server.listen(5321, '127.0.0.1', resolve));
  try {
    for (const [label, engine, width, dark] of [
      ['desktop', chromium, 1280, false],
      ['phone', webkit, 390, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: 1000 },
        hasTouch: width < 640,
        isMobile: width < 640,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(12000);
      let resources = baseMetrics.map(([name, metric]) => resource(name, metric));
      const record = { label, width, height: 1000, dark, browser: browser.version(), states: [] };
      result.cases.push(record);
      page.on('pageerror', (error) => result.errors.push({ label, message: error.message }));
      await page.route('**/*', async (route) => {
        const url = new URL(route.request().url());
        assert.equal(url.origin, origin);
        if (!url.pathname.startsWith('/api/')) return route.continue();
        const method = route.request().method();
        result.requests.push({ label, path: url.pathname, method });
        assert.ok(
          method === 'GET' || (method === 'POST' && url.pathname === '/api/usage/workload-history'),
        );
        if (url.pathname === '/api/resources')
          return route.fulfill({
            contentType: 'application/json',
            body: JSON.stringify({ data: resources, meta: { totalPages: 1 } }).replaceAll(
              '\"total\":null',
              '\"total\":1e309',
            ),
          });
        if (url.pathname === '/api/metrics-store/history')
          return route.fulfill({
            json: {
              resourceType: 'vm',
              resourceId: url.searchParams.get('resourceId'),
              range: url.searchParams.get('range'),
              start: Date.UTC(2026, 9, 14, 18),
              end: Date.UTC(2026, 9, 14, 19),
              source: 'store',
              metrics: {},
            },
          });
        if (url.pathname === '/api/license/runtime-capabilities')
          return route.fulfill({
            json: {
              capabilities: [],
              limits: [],
              max_history_days: 7,
              hosted_mode: false,
              runtime: { build: 'community' },
              blocked_capabilities: [],
            },
          });
        return route.fulfill({
          json: { success: true, data: [], alerts: [], anomalies: [], config: null },
        });
      });
      await page.addInitScript((isDark) => {
        if (isDark)
          document.addEventListener('DOMContentLoaded', () =>
            document.documentElement.classList.add('dark'),
          );
      }, dark);
      await page.goto(origin + '/browser-tests/workload-disk-metrics.html');
      const table = page.getByRole('region', { name: 'Workload readings' });
      const order = () =>
        table
          .locator('tbody tr[data-guest-id]')
          .evaluateAll((rows) =>
            rows.map((row) => row.getAttribute('data-guest-id').replace('fixture-', '')),
          );
      const settle = () =>
        page.evaluate(async () => {
          await document.fonts.ready;
          await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
        });
      const capture = async (name) => {
        await settle();
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          true,
        );
        const file = path.join(output, `${label}-${name}.png`);
        await page.screenshot({ path: file, fullPage: true });
        result.captures.push({ label, name, file, sha256: hash(fs.readFileSync(file)) });
      };
      await table.locator('tr[data-guest-id="fixture-capacity-only"]').waitFor();
      assert.deepEqual(await order(), [
        'critical',
        'forty-percent',
        'one-percent',
        'half-percent',
        'empty',
        'capacity-only',
      ]);
      const capacityCell = table.locator(
        '[data-guest-id="fixture-capacity-only"] [data-workload-col="disk"]',
      );
      assert.equal(await capacityCell.locator('[data-animated-number]').count(), 0);
      await capture('descending');
      await page.getByRole('combobox', { name: 'Disk order' }).selectOption('asc');
      assert.deepEqual(await order(), [
        'empty',
        'half-percent',
        'one-percent',
        'forty-percent',
        'critical',
        'capacity-only',
      ]);
      await page.getByRole('textbox', { name: 'Search fixtures' }).fill('disk>90');
      assert.deepEqual(await order(), ['critical']);
      await page.getByRole('textbox', { name: 'Search fixtures' }).fill('disk<=1');
      assert.deepEqual(await order(), ['empty', 'half-percent', 'one-percent']);
      await page.getByRole('textbox', { name: 'Search fixtures' }).fill('');
      await page.getByRole('combobox', { name: 'Metrics', exact: true }).selectOption('sparklines');
      const one = table.locator('[data-guest-id="fixture-one-percent"]');
      assert.equal(
        await one.getByRole('img', { name: 'one-percent disk usage history, current 1%' }).count(),
        1,
      );
      if (width < 640) await table.getByText('capacity-only', { exact: true }).tap();
      else {
        await table.getByRole('button', { name: /capacity-only/ }).focus();
        await page.keyboard.press('Enter');
      }
      const drawer = page.getByRole('region', { name: 'Selected guest' });
      await drawer.locator('[data-history-group="utilization"]').waitFor();
      const current = () => drawer.locator('[data-history-current="disk"]');
      assert.equal(await current().count(), 0);
      assert.equal(await drawer.locator('path[data-history-series]').count(), 0);
      await capture('capacity-only');
      const observe = async (name, metric, extras, expected) => {
        resources = resources.map((row) =>
          row.id === 'fixture-capacity-only'
            ? { ...resource('capacity-only', metric, extras) }
            : row,
        );
        await page.evaluate(() => window.__diskMetrics.refresh());
        await settle();
        const evidence = await page.evaluate(() =>
          window.__diskMetrics.evidence().find((row) => row.id === 'fixture-capacity-only'),
        );
        assert.equal(evidence.current ?? null, expected);
        record.states.push({
          name,
          metric,
          extras,
          evidence,
          observedTotalFinite: Number.isFinite(evidence.disk.total),
        });
      };
      await observe('percentage-only-low', { percent: 0.5 }, {}, 0.5);
      assert.match(await current().innerText(), /0.5%/);
      assert.equal((await drawer.innerText()).includes('0 B of'), false);
      await capture('percentage-only');
      // Deliberately over-range JSON numeral: a valid independent percent
      // cannot validate malformed capacity or print an Infinity byte fact.
      await observe('nonfinite-capacity', { percent: 50, used: 50, total: Infinity }, {}, 50);
      assert.equal(await drawer.getByText(/of .*disk$/).count(), 0);
      assert.equal(/Infinity|undefined/.test(await drawer.innerText()), false);
      await capture('invalid-capacity');
      await observe('explicit-zero', { percent: 0 }, {}, 0);
      assert.match(await current().innerText(), /0.0%/);
      await observe('byte-only', { used: 50, total: 100 }, {}, 50);
      assert.match(await current().innerText(), /50.0%/);
      await page.getByRole('combobox', { name: 'Metrics', exact: true }).selectOption('bars');
      await observe('capacity-only-restored', { total: 100 }, {}, null);
      assert.equal(await current().count(), 0);
      await observe(
        'retained',
        { percent: 95, used: 95, total: 100 },
        { diskStatusReason: 'prev-agent-busy', guestAgentStatus: 'deferred' },
        null,
      );
      assert.match(await drawer.locator('[data-history-last-known="disk"]').innerText(), /95.0%/);
      await page.getByRole('textbox', { name: 'Search fixtures' }).fill('disk>90');
      assert.deepEqual(await order(), ['critical']);
      await page.getByRole('textbox', { name: 'Search fixtures' }).fill('');
      await capture('retained');
      await observe(
        'unavailable',
        { percent: 95, used: 95, total: 100 },
        { diskStatusReason: 'agent-timeout' },
        null,
      );
      assert.equal(await drawer.locator('[data-history-last-known="disk"]').count(), 0);
      await observe('recovered-current', { percent: 1, used: 1, total: 100 }, {}, 1);
      assert.match(await current().innerText(), /1.0%/);
      assert.equal(await drawer.locator('path[data-history-series]').count(), 0);
      await capture('recovered');
      await observe(
        'filesystem-omitted-zero',
        { percent: 0, used: 0, total: 100 },
        { disks: [{ mountpoint: '/empty', total: 100 * GB, free: 100 * GB }] },
        0,
      );
      assert.match(
        await drawer.locator('tr').filter({ hasText: '/empty' }).innerText(),
        /0 B.*100 GB/,
      );
      await capture('filesystem-zero');
      await drawer.getByRole('button', { name: 'Collapse capacity-only details' }).click();
      assert.equal(await drawer.count(), 0);
      assert.deepEqual(result.errors, []);
      await page.close();
      await browser.close();
      browser = undefined;
    }
    result.result = 'passed';
  } finally {
    if (browser) await browser.close();
    result.cleanup.browser_closed = true;
    await new Promise((resolve) => server.close(resolve));
    result.cleanup.server_closed = true;
  }
})()
  .catch((error) => {
    result.errors.push({ driver: error.stack });
    process.exitCode = 1;
  })
  .finally(() => {
    fs.mkdirSync(output, { recursive: true });
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: result.result,
        output,
        captures: result.captures.length,
        errors: result.errors,
        cleanup: result.cleanup,
      }),
    );
  });
