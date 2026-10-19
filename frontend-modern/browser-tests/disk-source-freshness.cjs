// Real HTTP -> canonical resource owner -> Storage and guest SMART consumers.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/disk-source-freshness');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json')));
for (const [file, digest] of Object.entries(binding.files))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Source changed: ${file}`);
const report = {
  binding,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  errors: [],
  cleanup: {},
};
let server, browser, current;
function reply(res, value) {
  res.writeHead(200, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(value));
}
function serve(req, res, next) {
  if (!req.url.startsWith('/api/')) return next();
  try {
    assert.equal(req.method, 'GET', 'No product mutations or guest probes');
    const url = new URL(req.url, 'http://fixture.invalid');
    current.requests.push({ method: req.method, path: req.url, mode: current.mode });
    if (url.pathname === '/api/resources') {
      assert.equal(url.searchParams.get('type'), 'physical_disk');
      assert.equal(url.searchParams.get('parent'), 'guest-101');
      assert.equal(url.searchParams.get('limit'), '100');
      assert.equal(url.searchParams.get('page'), '1');
      const expired = ['stale', 'risk'].includes(current.mode);
      const risk = current.mode === 'risk';
      return reply(res, {
        resources: [
          {
            id: 'disk-101',
            parentId: 'guest-101',
            name: 'Guest SATA',
            displayName: 'Guest SATA',
            type: 'physical_disk',
            platformId: 'fixture-a',
            platformType: 'proxmox-pve',
            sourceType: 'agent',
            status: 'online',
            lastSeen: '2026-10-19T05:00:00Z',
            identity: { hostname: 'guest-101' },
            metricsTarget: { resourceType: 'disk', resourceId: 'agent-101:sda' },
            health: {
              verdict: risk ? 'critical' : expired ? 'stale' : 'ok',
              reasons: expired
                ? [
                    ...(risk ? [{ code: 'critical_alert' }] : []),
                    { code: 'telemetry_stale', detail: '9m' },
                  ]
                : [],
            },
            physicalDisk: {
              model: 'Guest SATA',
              devPath: '/dev/sda',
              diskType: 'ssd',
              sizeBytes: 1_000_000_000,
              health: risk ? 'FAILED' : 'PASSED',
              temperature: current.mode === 'restored' ? 44 : 41,
              wearout: 99,
              collection: {
                temperature: { state: 'available', source: 'smartctl' },
                io: { state: 'available' },
              },
              smart: {
                powerOnHours: 100,
                reallocatedSectors: 0,
                pendingSectors: risk ? 7 : 0,
                udmaCrcErrors: 2,
              },
            },
          },
        ],
        meta: { totalPages: 1 },
      });
    }
    if (url.pathname === '/api/metrics-store/history') {
      const end = Date.now();
      return reply(res, {
        resourceType: url.searchParams.get('resourceType'),
        resourceId: url.searchParams.get('resourceId'),
        metric: url.searchParams.get('metric'),
        range: url.searchParams.get('range'),
        start: end - 600_000,
        end,
        source: 'store',
        points: [0, 1, 2].map((i) => ({
          timestamp: end - (2 - i) * 300_000,
          value: 40 + i,
          min: 40 + i,
          max: 40 + i,
        })),
      });
    }
    assert(
      [
        '/api/license/runtime-capabilities',
        '/api/alerts/config',
        '/api/alerts/active',
        '/api/security/status',
      ].includes(url.pathname),
      `Unexpected API: ${url.pathname}`,
    );
    reply(
      res,
      url.pathname.startsWith('/api/license/')
        ? { capabilities: [], limits: [], max_history_days: 7, runtime: { build: 'community' } }
        : url.pathname === '/api/alerts/active'
          ? []
          : { enabled: true, activationState: 'active' },
    );
  } catch (error) {
    report.errors.push(String(error.stack || error));
    res.writeHead(500);
    res.end('{}');
  }
}
(async () => {
  try {
    assert.equal(
      report.playwright,
      JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    const { createServer } = await import(
      pathToFileURL(path.join(frontend, 'node_modules/vite/dist/node/index.js')).href
    );
    server = await createServer({
      root: frontend,
      configFile: path.join(frontend, 'vite.config.ts'),
      cacheDir: path.join(output, 'vite-cache'),
      logLevel: 'error',
      plugins: [
        {
          name: 'offline-disk-source-freshness',
          configureServer(s) {
            s.middlewares.use(serve);
          },
        },
      ],
      server: { host: '127.0.0.1', port: 0, strictPort: false },
      optimizeDeps: { noDiscovery: true, include: ['solid-js', 'solid-js/web', '@solidjs/router'] },
    });
    await server.listen();
    const origin = `http://127.0.0.1:${server.httpServer.address().port}`;
    for (const [name, engine, narrow] of [
      ['chromium', chromium, false],
      ['webkit', webkit, true],
    ]) {
      current = { name, mode: 'fresh', requests: [] };
      browser = await engine.launch(
        narrow
          ? { headless: true }
          : { headless: true, channel: 'chromium', args: ['--no-sandbox'] },
      );
      const context = await browser.newContext({
        viewport: { width: narrow ? 320 : 1365, height: 1000 },
        hasTouch: narrow,
        colorScheme: narrow ? 'dark' : 'light',
      });
      const page = await context.newPage();
      page.on('pageerror', (error) => report.errors.push(`${name}: ${error.stack || error}`));
      await page.routeWebSocket(/\/ws(?:\?|$)/, () => {});
      await page.route('**/*', (route) => {
        if (
          route
            .request()
            .url()
            .startsWith(origin + '/')
        )
          return route.continue();
        report.errors.push('Unexpected off-origin request');
        return route.abort();
      });
      const now = new Date('2026-10-19T12:00:00Z');
      await page.clock.install({ time: now });
      await page.clock.pauseAt(now);
      const activate = async (locator) => {
        if (narrow) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      const capture = async (state) => {
        assert(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          'No horizontal page overflow',
        );
        const file = `${name}-${state}.png`;
        await page.screenshot({
          path: path.join(output, file),
          fullPage: true,
          animations: 'disabled',
        });
        report.captures.push({
          file,
          state,
          sha256: hash(fs.readFileSync(path.join(output, file))),
        });
      };
      const row = () => page.locator('[data-row-id="disk-101"]');
      const detail = () => page.getByTestId('disk-detail-health');
      const revealPhoneCharts = async () => {
        if (!narrow) return;
        const toggle = page.getByTestId('disk-detail-phone-toggle');
        if ((await toggle.getAttribute('aria-expanded')) === 'false') await activate(toggle);
      };
      const refresh = async (mode) => {
        current.mode = mode;
        const before = current.requests.filter((r) => r.path.startsWith('/api/resources?')).length;
        await activate(page.getByRole('button', { name: 'Refresh collected inventory' }));
        await expect
          .poll(() => current.requests.filter((r) => r.path.startsWith('/api/resources?')).length)
          .toBe(before + 1);
      };
      const verifyRetained = async (risk) => {
        await expect(detail()).toContainText('current disk health is unconfirmed');
        await expect(page.getByRole('heading', { name: /Live I\/O \(30m\)/ })).toHaveCount(0);
        await expect(page.getByText('Last known temperature', { exact: true })).toBeVisible();
        assert(
          await page
            .getByText('Last known temperature', { exact: true })
            .evaluate(
              (label) =>
                label.scrollWidth <= label.clientWidth && label.scrollHeight <= label.clientHeight,
            ),
          'The last-known qualifier fits inside its SMART card, including inside a nowrap table',
        );
        const value = page
          .getByText('CRC Errors', { exact: true })
          .locator('..')
          .locator('div')
          .last();
        await expect(value).toHaveText('2');
        await expect(value).toHaveClass(/text-muted/);
        if (risk) {
          await expect(detail()).toContainText('Last reported: Replace Now.');
          await expect(
            page.getByText('Pending Sectors', { exact: true }).locator('..'),
          ).toContainText('7');
        }
      };
      await page.goto(`${origin}/browser-tests/disk-source-freshness.html`, {
        waitUntil: 'domcontentloaded',
      });
      if (narrow) await page.evaluate(() => document.documentElement.classList.add('dark'));
      await expect(row().getByText('Healthy', { exact: true })).toBeVisible();
      if (narrow) await row().getByText('Guest SATA', { exact: true }).tap();
      else await activate(row().getByRole('button', { name: 'Expand Guest SATA', exact: true }));
      await expect(detail()).toContainText('Healthy');
      await revealPhoneCharts();
      await expect(page.getByRole('heading', { name: /Live I\/O \(30m\)/ })).toHaveCount(1);
      await capture('storage-current');

      await refresh('stale');
      await expect(
        row().getByText(narrow ? 'Old data' : 'Last known', { exact: true }),
      ).toBeVisible();
      await verifyRetained(false);
      await expect(row().locator('[data-temperature-reading]')).toHaveAttribute(
        'data-temperature-reading',
        'last-known',
      );
      await capture('storage-expired');
      await activate(page.getByRole('button', { name: 'Toggle healthy filter' }));
      await expect(row()).toHaveCount(0);
      await activate(page.getByRole('button', { name: 'Toggle healthy filter' }));
      await expect(detail()).toContainText('current disk health is unconfirmed');

      await refresh('risk');
      await expect(
        row().getByText(narrow ? 'Past risk' : 'Last known risk', { exact: true }),
      ).toBeVisible();
      await verifyRetained(true);
      await capture('storage-retained-risk');
      await activate(page.getByRole('button', { name: 'Guest overview' }));
      const guestRow = page.getByTestId('guest-physical-disk');
      await expect(guestRow.getByText('Last known risk', { exact: true })).toBeVisible();
      await activate(guestRow.locator('summary'));
      await verifyRetained(true);
      await capture('guest-retained-risk');

      await refresh('stale');
      await expect(guestRow.getByText('Last known', { exact: true }).first()).toBeVisible();
      await verifyRetained(false);
      assert(await guestRow.evaluate((element) => element.open), 'Guest detail remains open');
      await capture('guest-expired');
      await refresh('restored');
      await expect(detail()).toContainText('Healthy');
      await expect(guestRow.locator('[data-temperature-reading]')).toHaveAttribute(
        'data-temperature-reading',
        'current',
      );
      await revealPhoneCharts();
      await expect(page.getByRole('heading', { name: /Live I\/O \(30m\)/ })).toHaveCount(1);
      await expect(page.getByText('Last known temperature', { exact: true })).toHaveCount(0);
      assert(
        await guestRow.evaluate((element) => element.open),
        'Recovered guest detail remains open',
      );
      await capture('guest-recovered');
      await activate(page.getByRole('button', { name: 'Storage disks' }));
      await expect(row().getByText('Healthy', { exact: true })).toBeVisible();
      await expect(detail()).toContainText('Healthy');
      await expect(row().locator('[data-temperature-reading]')).toHaveText('44°C');
      await revealPhoneCharts();
      await capture('storage-recovered');
      report.cases.push({
        browser: name,
        version: browser.version(),
        viewport: page.viewportSize(),
        requests: current.requests,
        checked: [
          'successful inventory with stale source',
          'staleness behind critical alert',
          'retained adverse evidence',
          'neutral SMART cards',
          'no live I/O on expired source',
          'history retained',
          'healthy filter excludes stale',
          'open-detail recovery',
        ],
      });
      await context.close();
      await browser.close();
      browser = null;
    }
    report.result = 'passed';
  } catch (error) {
    report.result = 'failed';
    report.errors.push(String(error.stack || error));
    process.exitCode = 1;
    if (current)
      report.failedCase = { name: current.name, mode: current.mode, requests: current.requests };
  } finally {
    if (browser) await browser.close();
    report.cleanup.browser = true;
    if (server) await server.close();
    report.cleanup.server = true;
    if (report.errors.length) {
      report.result = 'failed';
      process.exitCode = 1;
    }
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2));
    console.log(
      JSON.stringify({
        result: report.result,
        cases: report.cases.length,
        captures: report.captures.length,
        errors: report.errors,
        cleanup: report.cleanup,
      }),
    );
  }
})();
