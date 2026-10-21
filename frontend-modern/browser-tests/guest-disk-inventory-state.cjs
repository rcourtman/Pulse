// The real guest drawer and scoped inventory reader, with safe HTTP fixtures.
// Run the existing hardware fixture compiler in source proof first.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const compiled = path.join(root, 'tmp/guest-hardware-proof/static');
const output = path.join(root, 'tmp/guest-disk-inventory-proof');
const origin = 'http://127.0.0.1:5353';
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const report = {
  result: 'incomplete',
  playwright: require('playwright/package.json').version,
  profiles: [],
  captures: [],
  cleanup: { browsers_created: 0, browsers_closed: 0, contexts_created: 0, contexts_closed: 0, server_closed: false },
  limits: 'Synthetic scoped HTTP inventory through useUnifiedResources and the full GuestDrawer. No guest probe, collection, disk health, missing-Windows-disk recovery, thaw, filesystem liveness, installed result or release acceptance.',
};
let server, browser, context, lastPage;
const save = () => fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
(async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
    assert.equal(report.playwright, JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
    const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
    report.binding = { source_sha: binding.source_sha, graph: binding.graph, sha256: hash(fs.readFileSync(path.join(compiled, 'binding.json'))) };
    // A driver-only repair changes no compiled app bytes. Bind its own exact
    // execution revision separately; all other imported/source/asset hashes
    // must still match the completed compiler proof, including the full CSS input.
    const driverPath = 'browser-tests/guest-disk-inventory-state.cjs';
    const driverHash = hash(fs.readFileSync(path.join(frontend, driverPath)));
    if (driverHash !== binding.sources[driverPath]) {
      const execution = JSON.parse(fs.readFileSync(path.join(root, 'tmp/guest-disk-inventory-driver-binding.json')));
      assert.equal(execution.version, 1);
      assert.match(execution.source_sha, /^[a-f0-9]{40}$/);
      assert.equal(execution.compiled_source_sha, binding.source_sha);
      assert.equal(execution.compiled_binding_sha256, report.binding.sha256);
      assert.equal(execution.driver_path, driverPath);
      assert.equal(execution.driver_sha256, driverHash);
      report.execution_binding = execution;
    }
    for (const [file, digest] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(frontend, file))), file === driverPath && report.execution_binding ? report.execution_binding.driver_sha256 : digest, file);
    for (const file of ['src/components/Workloads/GuestPhysicalDisks.tsx', 'src/components/Workloads/GuestDrawer.tsx', 'src/hooks/useUnifiedResources.ts', 'browser-tests/guest-disk-inventory-state.cjs'])
      assert(binding.sources[file], `unbound source: ${file}`);
    const assets = new Map();
    for (const [file, digest] of Object.entries(binding.artifacts)) {
      assert(!path.isAbsolute(file) && !file.split('/').includes('..'));
      const bytes = fs.readFileSync(path.join(compiled, file));
      assert.equal(hash(bytes), digest, file);
      assets.set('/' + file, bytes);
    }
    server = http.createServer((req, res) => {
      if (!['GET', 'HEAD'].includes(req.method)) { res.writeHead(405); return res.end(); }
      const pathname = new URL(req.url, origin).pathname;
      const file = pathname === '/' ? '/browser-tests/guest-hardware-observation.html' : pathname;
      const bytes = assets.get(file);
      if (!bytes) { res.writeHead(404); return res.end(); }
      res.setHeader('Content-Type', file.endsWith('.html') ? 'text/html' : file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'application/octet-stream');
      res.setHeader('Cache-Control', 'no-store');
      res.end(req.method === 'HEAD' ? undefined : bytes);
    });
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(5353, '127.0.0.1', resolve); });
    for (const [name, engine, width, dark] of [['desktop', chromium, 1365, false], ['narrow', webkit, 320, true]]) {
      browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      report.cleanup.browsers_created++;
      context = await browser.newContext({ viewport: { width, height: 900 }, isMobile: width < 768, hasTouch: width < 768, locale: 'en-GB', timezoneId: 'UTC', reducedMotion: 'reduce' });
      report.cleanup.contexts_created++;
      const page = lastPage = await context.newPage();
      page.setDefaultTimeout(15000);
      const record = { name, width, height: 900, dark, browser: browser.version(), cases: [], requests: [], responses: [], page_errors: [], console_errors: [], failed: [] };
      report.profiles.push(record);
      page.on('pageerror', error => record.page_errors.push(error.message));
      page.on('console', message => { if (message.type() === 'error') record.console_errors.push(message.text()); });
      page.on('requestfailed', request => record.failed.push({ url: request.url(), path: new URL(request.url()).pathname, failure: request.failure() }));
      await page.routeWebSocket('**/*', socket => socket.close());
      let mode = 'held-empty', release;
      let gate = new Promise(resolve => release = resolve);
      const disk = temperature => ({ id: 'disk-101-a', parentId: 'vm-resource-101', type: 'physical_disk', name: 'Guest SATA', platformId: 'fixture', platformType: 'proxmox-pve', sourceType: 'agent', sources: ['agent'], status: 'online', lastSeen: Date.now(), health: { verdict: 'ok', reasons: [] }, physicalDisk: { devPath: '/dev/sda', model: 'Guest SATA', sizeBytes: 1_000_000_000, health: 'PASSED', temperature, smart: { reallocatedSectors: 0, pendingSectors: 0, udmaCrcErrors: 0 } } });
      await page.route('**/*', async route => {
        const request = route.request(), url = new URL(request.url());
        assert.equal(url.origin, origin, 'no external request');
        assert(['GET', 'HEAD'].includes(request.method()), 'read only');
        if (url.pathname === '/favicon.ico') return route.fulfill({ status: 204, body: '' });
        if (!url.pathname.startsWith('/api/')) return route.continue();
        const requestedMode = mode;
        record.requests.push({ path: url.pathname, query: url.search, method: request.method(), mode: requestedMode });
        assert(!/diagnostics|run-command|guest-agent|qemu/.test(url.pathname), 'no guest operation');
        if (url.pathname === '/api/resources') {
          assert.equal(url.searchParams.get('type'), 'physical_disk');
          assert.equal(url.searchParams.get('parent'), 'vm-resource-101');
          if (requestedMode.startsWith('held-')) await gate;
          const status = requestedMode === 'failed' ? 503 : 200;
          const rows = ['current', 'recovered'].includes(requestedMode)
            ? [disk(requestedMode === 'current' ? 45 : 42)]
            : [{ ...disk(88), id: 'unrelated-disk', parentId: 'another-machine', name: 'Unrelated disk' }, { ...disk(88), id: 'not-a-disk', type: 'vm' }];
          record.responses.push({ mode: requestedMode, status, ids: status === 200 ? rows.map(row => row.id) : [] });
          return route.fulfill({ status, json: status === 200 ? { resources: rows, meta: { totalPages: 1 } } : { error: 'PRIVATE_FIXTURE_ERROR' } });
        }
        if (url.pathname === '/api/metrics-store/history') {
          // Match the production single/all envelope and the requested owner
          // and range. Invalid fixture data must not masquerade as read recovery.
          const metric = url.searchParams.get('metric');
          const body = { resourceType: url.searchParams.get('resourceType'), resourceId: url.searchParams.get('resourceId'), range: url.searchParams.get('range') || '24h', source: 'store', start: Date.now() - 3600000, end: Date.now() };
          if (metric) { body.metric = metric; body.points = []; }
          else body.metrics = {};
          return route.fulfill({ json: body });
        }
        if (url.pathname === '/api/alerts/active') return route.fulfill({ json: [] });
        assert(['/api/license/runtime-capabilities', '/api/alerts/config', '/api/security/status', '/api/metrics-store/stats', '/api/patrol/config'].includes(url.pathname), `unexpected API: ${url.pathname}`);
        return route.fulfill({ json: url.pathname.startsWith('/api/license/') ? { capabilities: [], limits: [], max_history_days: 7, runtime: { build: 'community' } } : { enabled: true, activationState: 'active' } });
      });
      // Observe, never trigger, History's AbortSignal. Every cancelled request
      // must match a distinct observed client abort; all other failures fail.
      await page.addInitScript(() => {
        const audit = [];
        window.__diskInventoryFetchAudit = audit;
        const original = window.fetch;
        window.fetch = function(input, init) {
          const url = new URL(typeof input === 'string' || input instanceof URL ? input : input.url, window.location.href).href;
          if (new URL(url).pathname !== '/api/metrics-store/history') return original.call(this, input, init);
          const signal = init?.signal || (input instanceof Request ? input.signal : undefined);
          const item = { id: audit.length, url, signal_observed: Boolean(signal), aborted: Boolean(signal?.aborted) };
          audit.push(item);
          signal?.addEventListener('abort', () => { item.aborted = true; }, { once: true });
          return original.call(this, input, init);
        };
      });
      await page.addInitScript(dark => document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.toggle('dark', dark), { once: true }), dark);
      const capture = async (label, target) => {
        await target.scrollIntoViewIfNeeded();
        await page.evaluate(async () => { await document.fonts.ready; await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))); });
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), 'no horizontal overflow');
        const file = `${name}-${label}.png`;
        await target.screenshot({ path: path.join(output, file), animations: 'disabled' });
        report.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))) });
        save();
      };
      const reread = async next => { mode = next; await page.evaluate(() => window.__guestHardware.reread()); };
      const statusCard = page.getByTestId('guest-physical-disk-inventory-status');
      const row = page.getByTestId('guest-physical-disk');
      await page.goto(origin, { waitUntil: 'commit' });
      await page.waitForFunction(() => window.__guestHardware?.evidence().projected);
      await expect(statusCard.getByRole('status')).toContainText('Loading physical disk inventory');
      await expect(statusCard).toContainText("Pulse's collected inventory, not the guest");
      await expect(statusCard).not.toContainText('No physical disks');
      await expect(row).toHaveCount(0);
      await capture('pending', statusCard);
      release();
      await expect(statusCard).toContainText('No physical disks were reported for this guest');
      await expect(statusCard).toContainText('Missing inventory does not establish disk absence or health');
      await expect(row).toHaveCount(0);
      await expect(page.getByText('Unrelated disk', { exact: true })).toHaveCount(0);
      record.cases.push('Pending scoped read is visible, not an empty result; successful foreign/non-disk-only response reports missing inventory, not no hardware or healthy hardware.');
      await capture('empty', statusCard);
      await reread('current');
      await expect(statusCard).toHaveCount(0);
      await expect(row).toContainText('45°C');
      await expect(row).toContainText('Healthy');
      await reread('empty');
      await expect(statusCard).toContainText('No physical disks were reported');
      await expect(row).toHaveCount(0);
      record.cases.push('Accepted inventory replacement withdraws the previous disk and exposes the empty result without a phantom healthy disk.');
      await reread('current');
      await expect(row).toContainText('45°C');
      await reread('failed');
      const error = page.getByRole('status').filter({ hasText: 'Physical disk details are unavailable' });
      await expect(error).toBeVisible();
      await expect(statusCard).toHaveCount(0);
      await expect(row).toHaveCount(0);
      await expect(page.getByText('PRIVATE_FIXTURE_ERROR', { exact: false })).toHaveCount(0);
      mode = 'held-retry';
      gate = new Promise(resolve => release = resolve);
      const before = record.requests.filter(item => item.path === '/api/resources').length;
      const retry = page.getByRole('button', { name: 'Retry physical disk details' });
      if (width < 768) await retry.tap();
      else { await retry.focus(); await page.keyboard.press('Enter'); }
      await expect(retry).toBeDisabled();
      await expect(retry).toHaveText('Retrying…');
      await expect(error).toBeVisible();
      await expect(statusCard).toHaveCount(0);
      await capture('failed-retry', error.locator('..'));
      assert.equal(record.requests.filter(item => item.path === '/api/resources').length, before + 1);
      release();
      await expect(error).toHaveCount(0);
      await expect(statusCard).toContainText('No physical disks were reported');
      await expect(retry).toHaveCount(0);
      await expect(row).toHaveCount(0);
      record.cases.push('503 retains explicit failure through one keyboard/touch Retry, hides prior readings/private error, and a successful empty retry becomes missing inventory rather than failure or health.');
      await reread('recovered');
      await expect(statusCard).toHaveCount(0);
      await expect(row).toContainText('42°C');
      const summary = row.locator('summary');
      if (width < 768) await summary.tap();
      else { await summary.focus(); await page.keyboard.press('Enter'); }
      await expect(row).toHaveAttribute('open', '');
      await expect(row.getByTestId('disk-detail-health')).toContainText('Healthy');
      await expect(row).not.toContainText('45°C');
      record.cases.push('A new successful scoped response restores only its replacement 42°C disk, and its real SMART detail opens with keyboard/touch. This is presentation, not native disk recovery.');
      await capture('replacement', row);
      assert.deepEqual(record.page_errors, []);
      record.client_fetch_audit = await page.evaluate(() => window.__diskInventoryFetchAudit);
      const matched = new Set();
      for (const failed of record.failed) {
        assert.equal(failed.path, '/api/metrics-store/history', 'non-History failure');
        assert(['net::ERR_ABORTED', 'cancelled', 'Cancelled', 'Load cancelled', 'Load request cancelled'].includes(failed.failure?.errorText), 'unexpected request failure');
        const owner = record.client_fetch_audit.find(item => item.signal_observed && item.aborted && item.url === failed.url && !matched.has(item.id));
        assert(owner, 'failed request has no matching observed client abort');
        matched.add(owner.id);
      }
      record.verified_client_aborts = matched.size;
      assert(record.console_errors.every(message => /Failed to load resource: the server responded with a status of 503/.test(message)), 'unexpected console error');
      assert.equal(record.responses.filter(item => item.status === 503).length, 1);
      await context.close(); context = null; report.cleanup.contexts_closed++;
      await browser.close(); browser = null; report.cleanup.browsers_closed++;
    }
    report.result = 'passed';
  } catch (error) {
    report.error = String(error.stack || error);
    if (lastPage && !lastPage.isClosed()) {
      report.diagnostic = await lastPage.locator('body').innerText().catch(() => 'unavailable');
      await lastPage.screenshot({ path: path.join(output, 'failure.png') }).catch(() => {});
    }
    throw error;
  } finally {
    if (context) { await context.close(); report.cleanup.contexts_closed++; }
    if (browser) { await browser.close(); report.cleanup.browsers_closed++; }
    if (server) { await new Promise(resolve => server.close(resolve)); report.cleanup.server_closed = true; }
    save();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
