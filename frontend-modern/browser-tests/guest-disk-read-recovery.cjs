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
const output = path.join(root, 'tmp/guest-disk-read-deadline');
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
function hold(req, res, kind, record) {
  held.add(res);
  record.closed = false;
  current.held.push({ res, kind, record });
  res.on('close', () => {
    held.delete(res);
    record.closed = true;
  });
  if (kind === 'body' || kind === 'error-body') {
    res.writeHead(kind === 'body' ? 200 : 503, { 'Content-Type': 'application/json' });
    res.write(kind === 'body' ? '{"resources":[' : '{"error":"PRIVATE_');
  }
}
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
    const record = { path: req.url, method: req.method, org, mode: current.mode };
    current.requests.push(record);
    if (url.pathname === '/api/resources') {
      assert.equal(url.searchParams.get('type'), 'physical_disk');
      assert.equal(url.searchParams.get('parent'), 'vm-resource-101');
      const page = Number(url.searchParams.get('page'));
      assert.equal(url.searchParams.get('limit'), '100');
      if (org === 'other-org') return reply(res, 200, inventory(37, 'Other organisation disk'));
      if (['headers', 'body', 'error-body', 'pagination'].includes(current.mode)) {
        return hold(req, res, current.mode, record);
      }
      assert.equal(page, 1);
      if (current.mode === 'rate-limit') {
        res.writeHead(429, { 'Content-Type': 'application/json', 'Retry-After': '60' });
        return res.end(JSON.stringify({ error: 'PRIVATE_RATE_LIMIT' }));
      }
      if (['503', '403'].includes(current.mode))
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
          name: 'offline-guest-disk-read-deadline',
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
      current = { name, mode: 'headers', temperature: 48, requests: [], held: [] };
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
      const now = new Date('2026-10-19T12:00:00Z');
      await page.clock.install({ time: now });
      await page.clock.pauseAt(now);
      const capture = async (state) => {
        const file = `${name}-${state}.png`;
        const local = path.join(output, file);
        assert(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          'No horizontal page overflow',
        );
        await page.screenshot({ path: local, fullPage: true, animations: 'disabled' });
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
      const resourceRequests = () =>
        current.requests.filter((r) => r.path.startsWith('/api/resources?'));
      const resourceReads = () => resourceRequests().length;
      const historyReads = () =>
        current.requests.filter((r) => r.path.startsWith('/api/metrics-store/history?')).length;
      await page.goto(`${origin}/browser-tests/guest-disk-read-recovery.html`, {
        waitUntil: 'domcontentloaded',
      });
      if (narrow) await page.evaluate(() => document.documentElement.classList.add('dark'));
      // Cold headers stall: an absent card must become an actionable read failure.
      await expect.poll(resourceReads).toBe(1);
      await page.clock.runFor(19_999);
      await expect(retry()).toHaveCount(0);
      await page.clock.runFor(1);
      await expect(retry()).toBeEnabled();
      await expect(page.getByRole('status')).toContainText('20 seconds');
      await expect(card()).toHaveCount(0);
      await expect.poll(() => current.requests[0].closed).toBe(true);
      await capture('initial-timeout');
      const afterInitial = resourceReads();
      await page.clock.runFor(65_000);
      assert.equal(resourceReads(), afterInitial, 'No timeout retry loop');

      current.mode = 'normal';
      await activate(retry());
      await expect(card().getByText('48°C', { exact: true })).toBeVisible();
      await expect(card().getByText('Guest SATA', { exact: true })).toHaveCount(1);
      assert.equal(
        await page.getByTestId('guest-physical-disk').count(),
        1,
        'Unrelated host disk excluded',
      );
      await activate(page.locator('[data-testid="guest-physical-disk"] summary'));
      await expect(page.getByTestId('disk-detail-history')).toBeVisible();
      await expect.poll(historyReads).toBeGreaterThan(0);
      await capture('recovered-expanded');

      // A complete response body, not just its headers, shares the deadline.
      current.mode = 'body';
      const beforeBody = resourceReads();
      await refresh();
      await expect.poll(resourceReads).toBe(beforeBody + 1);
      await page.clock.runFor(20_000);
      await expect(retry()).toBeEnabled();
      await expect(card()).toHaveCount(0);
      await expect(page.getByTestId('disk-detail-history')).toHaveCount(0);
      await expect(page.locator('[data-temperature-reading]')).toHaveCount(0);
      await expect.poll(() => resourceRequests().at(-1).closed).toBe(true);
      const historyAtFailure = historyReads();
      await capture('background-body-timeout');

      // A stalled error body also releases one explicit retry without leaking it.
      current.mode = 'error-body';
      const beforeErrorBody = resourceReads();
      await activate(retry());
      await expect(retry()).toBeDisabled();
      await expect(retry()).toHaveText('Retrying…');
      if (!narrow) await page.keyboard.press('Enter');
      await expect.poll(resourceReads).toBe(beforeErrorBody + 1);
      const box = await retry().boundingBox();
      if (narrow) assert(box.height >= 44 && box.width >= 44, '44px touch Retry');
      await capture('retry-pending');
      await page.clock.runFor(20_000);
      await expect(retry()).toBeEnabled();
      await expect(card()).toHaveCount(0);
      await expect.poll(() => resourceRequests().at(-1).closed).toBe(true);
      assert.equal(historyReads(), historyAtFailure, 'Withdrawn SMART history stays stopped');
      assert(
        !/PRIVATE_/.test(await page.locator('body').innerText()),
        'No provider/error-body text',
      );
      await capture('retry-timeout');

      // One allowance covers the first page and the complete parallel fan-out.
      current.mode = 'pagination';
      const beforePages = resourceReads();
      await activate(retry());
      await expect.poll(resourceReads).toBe(beforePages + 1);
      await page.clock.runFor(15_000);
      const firstPage = current.held.find((h) => h.kind === 'pagination' && !h.record.closed);
      reply(firstPage.res, 200, { ...inventory(81, 'INCOMPLETE_PAGE'), meta: { totalPages: 3 } });
      await expect.poll(resourceReads).toBe(beforePages + 3);
      const pages = current.requests.filter((r) => r.mode === 'pagination');
      assert.deepEqual(pages.map((r) => new URL(r.path, origin).searchParams.get('page')).sort(), [
        '1',
        '2',
        '3',
      ]);
      await page.clock.runFor(4_999);
      await expect(retry()).toBeDisabled();
      await page.clock.runFor(1);
      await expect(retry()).toBeEnabled();
      await expect(card()).toHaveCount(0);
      await expect(page.getByText('INCOMPLETE_PAGE')).toHaveCount(0);
      await expect.poll(() => pages.every((r) => r.closed)).toBe(true);
      await capture('pagination-timeout');

      // The actual API client's Retry-After wait is within the same read budget.
      current.mode = 'rate-limit';
      const beforeRateLimit = resourceReads();
      const rateResponse = page.waitForResponse(
        (response) => response.url().includes('/api/resources?') && response.status() === 429,
      );
      await activate(retry());
      await expect.poll(resourceReads).toBe(beforeRateLimit + 1);
      await (await rateResponse).finished();
      await page.evaluate(() => Promise.resolve());
      await page.clock.runFor(20_000);
      await expect(retry()).toBeEnabled();
      await page.clock.runFor(65_000);
      assert.equal(
        resourceReads(),
        beforeRateLimit + 1,
        'Aborted Retry-After sends no later request',
      );
      await capture('rate-limit-timeout');

      current.mode = '403';
      await activate(retry());
      await expect(retry()).toBeEnabled();
      await expect(card()).toHaveCount(0);
      assert(!/PRIVATE_/.test(await page.locator('body').innerText()));

      // Failed cache remount + explicit retry share one new bounded request.
      current.mode = 'headers';
      const beforeRemount = resourceReads();
      await activate(page.getByRole('button', { name: 'Close guest details' }));
      await activate(page.getByRole('button', { name: 'Reopen guest details' }));
      await expect.poll(resourceReads).toBe(beforeRemount + 1);
      await expect(retry()).toBeDisabled();
      await expect(card()).toHaveCount(0);
      await page.clock.runFor(20_000);
      await expect(retry()).toBeEnabled();
      assert.equal(resourceReads(), beforeRemount + 1);

      current.mode = 'normal';
      current.temperature = 49;
      await activate(retry());
      await expect(card().getByText('49°C', { exact: true })).toBeVisible();
      assert.equal(
        await page.getByTestId('guest-physical-disk').evaluate((r) => r.open),
        false,
        'Recovered detail stays collapsed',
      );
      await expect(page.getByTestId('disk-detail-history')).toHaveCount(0);
      await capture('read-recovered');
      current.mode = 'empty';
      const beforeEmpty = resourceReads();
      await refresh();
      await expect.poll(resourceReads).toBe(beforeEmpty + 1);
      await expect(card()).toHaveCount(0);
      await expect(retry()).toHaveCount(0);

      // The existing org fence still retires an old source while its deadline runs.
      current.mode = '503';
      await refresh();
      await expect(retry()).toBeEnabled();
      current.mode = 'headers';
      const beforeOrg = resourceReads();
      await activate(retry());
      await expect.poll(resourceReads).toBe(beforeOrg + 1);
      const oldOrg = current.held.at(-1);
      await activate(page.getByRole('button', { name: 'Switch organisation' }));
      await expect(card().getByText('Other organisation disk', { exact: true })).toBeVisible();
      reply(oldOrg.res, 200, inventory(99, 'OLD_ORG_DISK'));
      await page.clock.runFor(20_000);
      await expect(card().getByText('37°C', { exact: true })).toBeVisible();
      await expect(page.getByText('OLD_ORG_DISK', { exact: true })).toHaveCount(0);
      await expect(retry()).toHaveCount(0);
      await capture('organisation-fenced');
      report.cases.push({
        browser: name,
        version: browser.version(),
        viewport: page.viewportSize(),
        requests: current.requests,
        completeReadDeadlineMs: 20_000,
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
