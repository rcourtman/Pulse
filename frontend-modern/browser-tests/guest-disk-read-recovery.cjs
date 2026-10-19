// Real HTTP -> API client/resource hook -> guest Overview/SMART/CSS.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/guest-disk-read-recovery');
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
const held = new Set();
function reply(res, status, value) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(value));
}
function inventory(temperature = 45, model = 'Guest SATA') {
  const disk = {
    id: 'disk-guest-101',
    parentId: 'vm-resource-101',
    type: 'physical_disk',
    name: 'guest-sata',
    displayName: model,
    platformId: 'fixture-a',
    platformType: 'proxmox-pve',
    sourceType: 'agent',
    status: 'online',
    lastSeen: '2026-10-19T05:00:00Z',
    physicalDisk: {
      devPath: '/dev/sda',
      model,
      diskType: 'sata',
      sizeBytes: 1_000_000_000,
      health: 'PASSED',
      temperature,
      smart: { reallocatedSectors: 0, pendingSectors: 0, udmaCrcErrors: 2 },
    },
  };
  return {
    resources: [
      disk,
      { ...disk, id: 'other-host-disk', parentId: 'unrelated-host', name: 'Other host' },
    ],
    meta: { totalPages: 1 },
  };
}
function serve(req, res, next) {
  if (!req.url.startsWith('/api/')) return next();
  try {
    assert.equal(req.method, 'GET', 'No mutations, guest actions or probes');
    const url = new URL(req.url, 'http://fixture.invalid');
    const org = req.headers['x-pulse-org-id'] || 'default';
    current.requests.push({ path: req.url, method: req.method, org });
    if (url.pathname === '/api/resources') {
      assert.equal(url.searchParams.get('type'), 'physical_disk');
      assert.equal(url.searchParams.get('parent'), 'vm-resource-101');
      assert.equal(url.searchParams.get('page'), '1');
      assert.equal(url.searchParams.get('limit'), '100');
      if (org === 'other-org') return reply(res, 200, inventory(37, 'Other organisation disk'));
      if (current.mode === 'hold') {
        held.add(res);
        current.held.push(res);
        res.on('close', () => held.delete(res));
        return;
      }
      if (current.mode === '503' || current.mode === '403')
        return reply(res, Number(current.mode), { error: 'PRIVATE_PROVIDER_TEXT' });
      return reply(
        res,
        200,
        current.mode === 'empty'
          ? { resources: [], meta: { totalPages: 1 } }
          : inventory(current.temperature),
      );
    }
    if (url.pathname === '/api/metrics-store/history') {
      const end = Date.now();
      return reply(res, 200, {
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
      200,
      url.pathname.startsWith('/api/license/')
        ? { capabilities: [], limits: [], max_history_days: 7, runtime: { build: 'community' } }
        : url.pathname === '/api/alerts/active'
          ? []
          : { enabled: true, activationState: 'active' },
    );
  } catch (error) {
    report.errors.push(String(error.stack || error));
    reply(res, 500, {});
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
          name: 'offline-guest-disk-read-recovery',
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
      current = { name, mode: 'normal', temperature: 45, requests: [], held: [] };
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
      await page.clock.install();
      const capture = async (state) => {
        const file = `${name}-${state}.png`;
        const local = path.join(output, file);
        await page.screenshot({ path: local, fullPage: true });
        report.captures.push({ file, sha256: hash(fs.readFileSync(local)), state });
      };
      const activate = async (locator) => {
        if (narrow) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      const refresh = () =>
        activate(page.getByRole('button', { name: 'Refresh collected inventory' }));
      const retry = () => page.getByRole('button', { name: 'Retry physical disk details' });
      const card = () => page.getByTestId('guest-physical-disks');
      const resourceReads = () =>
        current.requests.filter((r) => r.path.startsWith('/api/resources?')).length;
      const historyReads = () =>
        current.requests.filter((r) => r.path.startsWith('/api/metrics-store/history?')).length;
      await page.goto(`${origin}/browser-tests/guest-disk-read-recovery.html`, {
        waitUntil: 'domcontentloaded',
      });
      if (narrow) await page.evaluate(() => document.documentElement.classList.add('dark'));
      await expect(card()).toBeVisible();
      await expect(card().getByText('45°C', { exact: true })).toBeVisible();
      await expect(card().getByText('Guest SATA', { exact: true })).toHaveCount(1);
      assert.equal(
        await page.getByTestId('guest-physical-disk').count(),
        1,
        'Unrelated host disk must not leak',
      );
      await activate(page.locator('[data-testid="guest-physical-disk"] summary'));
      await expect(page.getByTestId('disk-detail-history')).toBeVisible();
      await expect.poll(historyReads).toBeGreaterThan(0);
      await capture('expanded-current');

      current.mode = '503';
      await refresh();
      await expect(retry()).toBeVisible();
      await expect(card()).toHaveCount(0);
      await expect(page.getByTestId('disk-detail-history')).toHaveCount(0);
      await expect(page.locator('[data-temperature-reading]')).toHaveCount(0);
      await expect(page.getByRole('status')).toContainText('not a guest probe');
      const afterFailure = historyReads();
      await page.clock.runFor(35_000);
      assert.equal(historyReads(), afterFailure, 'Withdrawn SMART charts must not keep polling');
      await capture('read-failed');

      const beforeRetry = resourceReads();
      current.mode = 'hold';
      await activate(retry());
      await expect(retry()).toBeDisabled();
      await expect(retry()).toHaveText('Retrying…');
      if (!narrow) await page.keyboard.press('Enter');
      await expect.poll(() => current.held.length).toBe(1);
      assert.equal(resourceReads(), beforeRetry + 1, 'Exactly one scoped explicit retry');
      await expect(card()).toHaveCount(0);
      const box = await retry().boundingBox();
      if (narrow) assert(box.height >= 44 && box.width >= 44, 'Narrow retry target');
      await capture('retry-pending');
      reply(current.held.shift(), 403, { error: 'PRIVATE_DENIAL_DETAIL' });
      await expect(retry()).toBeEnabled();
      await expect(card()).toHaveCount(0);
      assert(!/PRIVATE_(PROVIDER_TEXT|DENIAL_DETAIL)/.test(await page.locator('body').innerText()));
      await capture('access-denied');

      current.mode = 'hold';
      const beforeRemount = resourceReads();
      await activate(page.getByRole('button', { name: 'Close guest details' }));
      await expect(retry()).toHaveCount(0);
      await activate(page.getByRole('button', { name: 'Reopen guest details' }));
      await expect.poll(() => current.held.length).toBe(1);
      await expect(retry()).toBeVisible();
      await expect(card()).toHaveCount(0);
      assert.equal(resourceReads(), beforeRemount + 1, 'Failed warm cache must be revalidated');
      await activate(retry());
      await expect(retry()).toBeDisabled();
      assert.equal(resourceReads(), beforeRemount + 1, 'Explicit retry coalesces a remount read');
      await capture('remount-unavailable');
      reply(current.held.shift(), 403, { error: 'PRIVATE_REMOUNT_DETAIL' });
      await expect(retry()).toBeEnabled();
      await expect(card()).toHaveCount(0);

      current.mode = 'normal';
      current.temperature = 48;
      await activate(retry());
      await expect(card()).toBeVisible();
      await expect(card().getByText('48°C', { exact: true })).toBeVisible();
      await expect(page.getByRole('status')).toHaveCount(0);
      assert.equal(
        await page.getByTestId('guest-physical-disk').evaluate((row) => row.open),
        false,
        'Recovered SMART detail must be collapsed/inert',
      );
      await expect(page.getByTestId('disk-detail-history')).toHaveCount(0);
      await capture('read-recovered');

      current.mode = 'empty';
      const beforeEmpty = resourceReads();
      await refresh();
      await expect.poll(resourceReads).toBe(beforeEmpty + 1);
      await expect(card()).toHaveCount(0);
      await expect(page.getByRole('status')).toHaveCount(0);
      current.mode = '503';
      await refresh();
      await expect(retry()).toBeVisible();
      current.mode = 'hold';
      await activate(retry());
      await expect.poll(() => current.held.length).toBe(1);
      await activate(page.getByRole('button', { name: 'Switch organisation' }));
      await expect(card().getByText('Other organisation disk', { exact: true })).toBeVisible();
      reply(current.held.shift(), 200, inventory(99, 'OLD_ORG_DISK'));
      await page.clock.runFor(1_000);
      await expect(card().getByText('37°C', { exact: true })).toBeVisible();
      await expect(page.getByText('OLD_ORG_DISK', { exact: true })).toHaveCount(0);
      await capture('organisation-fenced');
      report.cases.push({
        browser: name,
        version: browser.version(),
        viewport: page.viewportSize(),
        requests: current.requests,
        historyReadsStopped: true,
        scopeFenced: true,
      });
      await context.close();
      await browser.close();
      browser = null;
    }
    report.result = 'passed';
  } catch (error) {
    report.result = 'failed';
    report.errors.push(String(error.stack || error));
    if (current)
      report.failedCase = { name: current.name, mode: current.mode, requests: current.requests };
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    report.cleanup.browser = true;
    for (const res of held) res.destroy();
    report.cleanup.heldResponses = true;
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
