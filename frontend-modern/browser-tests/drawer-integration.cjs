// Exact compiled production consumers; read-only synthetic fixtures, never guest commands.
const fs = require('node:fs'), path = require('node:path'), http = require('node:http');
const crypto = require('node:crypto'), assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const root = '/workspace', origin = 'http://127.0.0.1:5335';
const compiled = root + '/tmp/drawer-integration/final-static';
const output = root + '/tmp/drawer-integration/results';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const binding = JSON.parse(fs.readFileSync(compiled + '/binding.json'));
  const result = { result: 'incomplete', source: binding.source_sha, driver: hash(fs.readFileSync(__filename)), playwright: require('playwright/package.json').version, binding, cases: [], captures: [], cleanup: { browser: false, server: false }, limits: 'Read-only synthetic canonical observations through actual useWorkloads, GuestRow, GuestDrawer, NodeDrawer, charts and Docs. Not native guest/PBS/thaw/restart/installed or reporter acceptance.' };
  let server, browser;
  try {
    assert.match(result.source, /^[0-9a-f]{40}$/);
    assert.equal(result.playwright, JSON.parse(fs.readFileSync(root + '/tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
    for (const [file, digest] of Object.entries(binding.sources)) assert.equal(hash(fs.readFileSync(root + '/frontend-modern/' + file)), digest, file);
    const assets = new Map();
    for (const [file, digest] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(compiled + '/' + file); assert.equal(hash(bytes), digest, file); assets.set('/' + file, bytes);
    }
    for (const doc of ['VM_DISK_MONITORING.md', 'TROUBLESHOOTING.md']) assert.equal(hash(fs.readFileSync(root + '/docs/' + doc)), binding.artifacts['docs/' + doc]);
    server = http.createServer((request, response) => {
      if (!['GET', 'HEAD'].includes(request.method)) { response.writeHead(405); return response.end(); }
      let pathname = new URL(request.url, origin).pathname;
      if (['/', '/guest', '/node', '/docs/VM_DISK_MONITORING', '/docs/TROUBLESHOOTING'].includes(pathname)) pathname = '/browser-tests/drawer-integration.html';
      const bytes = assets.get(pathname);
      if (!bytes) { response.writeHead(404); return response.end(); }
      response.setHeader('Content-Type', { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.md': 'text/markdown; charset=utf-8' }[path.extname(pathname)] || 'application/octet-stream');
      response.setHeader('Cache-Control', 'no-store'); response.end(request.method === 'HEAD' ? undefined : bytes);
    });
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(5335, '127.0.0.1', resolve); });
    for (const [engine, width, theme] of [[chromium, 1365, 'light'], [chromium, 1365, 'dark'], [webkit, 390, 'light'], [webkit, 390, 'dark']]) {
      const name = `${engine === chromium ? 'chromium' : 'webkit'}-${width}-${theme}`;
      browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width, height: 900 }, isMobile: width < 768, hasTouch: width < 768, colorScheme: theme, locale: 'en-GB', reducedMotion: 'reduce' });
      await context.addInitScript((theme) => {
        window.__trustedInputs = [];
        document.addEventListener('click', (event) => window.__trustedInputs.push({ trusted: event.isTrusted, label: event.target.closest('a,button,summary')?.textContent }), true);
        document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.toggle('dark', theme === 'dark'));
      }, theme);
      const page = await context.newPage(); page.setDefaultTimeout(8000);
      const record = { name, browser: browser.version(), width, theme, requests: [], errors: [], states: [] }; result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      await page.clock.install({ time: new Date('2026-10-13T13:00:00Z') });
      let historyMode = 'empty';
      await context.route('**/*', (route) => {
        const request = route.request(), url = new URL(request.url());
        record.requests.push({ method: request.method(), path: url.pathname, search: url.search });
        assert.equal(url.origin, origin); assert.equal(request.method(), 'GET');
        assert(!/\/(?:diagnostics|run-command|guest-agent|qemu)(?:\/|$)/.test(url.pathname));
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          const range = url.searchParams.get('range'); assert(['24h', '7d'].includes(range));
          const end = Date.parse('2026-10-13T13:00:00Z'), start = end - (range === '7d' ? 7 : 1) * 86400000;
          const metrics = Object.fromEntries(['cpu','memory','disk','temperature','netin','netout','diskread','diskwrite'].map((metric) => [metric, historyMode === 'empty' ? [] : [
            { timestamp: start, value: 20, min: 10, max: 40 }, { timestamp: end, value: 30, min: 20, max: 90 },
          ]]));
          return route.fulfill({ json: { resourceType: url.searchParams.get('resourceType'), resourceId: url.searchParams.get('resourceId'), range, start, end, metrics, source: 'mock_synthetic' } });
        }
        if (url.pathname === '/api/license/runtime-capabilities') return route.fulfill({ json: { capabilities: [], hosted_mode: false, max_history_days: 7, runtime: { build: 'free' }, blocked_capabilities: [] } });
        return route.fulfill({ json: { success: true, data: [], alerts: [], anomalies: [], config: null } });
      });
      const capture = async (suffix, locator) => {
        await page.evaluate(async () => { await document.fonts.ready; await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))); });
        const file = `${name}-${suffix}.png`;
        if (locator) await locator.screenshot({ path: output + '/' + file });
        else await page.screenshot({ path: output + '/' + file, fullPage: false });
        result.captures.push({ file, bytes: fs.statSync(output + '/' + file).size, sha256: hash(fs.readFileSync(output + '/' + file)) });
      };
      const noOverflow = async () => assert.equal(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth), 0);
      const activate = async (control) => { if (width < 768) await control.tap(); else { await control.focus(); await page.keyboard.press('Enter'); } };
      await page.goto(origin + '/guest', { waitUntil: 'networkidle' });
      const row = page.locator('[data-guest-id="fixture:pve1:101"]');
      const memoryCell = row.locator('[data-column-id="memory"]'), diskCell = row.locator('[data-column-id="disk"]');
      // A quiet low-pressure cache reading and a setup gap add no redundant caption/button.
      await memoryCell.getByText('25%', { exact: true }).waitFor();
      assert.equal(await memoryCell.getByRole('button', { name: /^Memory reading/ }).count(), 0);
      assert.equal(await diskCell.getByRole('button', { name: /^Disk reading/ }).count(), 0);
      assert.match(await diskCell.getAttribute('title'), /not running|not responding|guest agent/i);
      await noOverflow(); await capture('quiet-row', row);
      await page.evaluate(() => window.__integration.memory('cache-high'));
      const cache = memoryCell.getByRole('button', { name: /^Memory reading/ }); await activate(cache);
      const memoryDialog = page.getByRole('dialog', { name: 'Memory reading for integration-guest' });
      await memoryDialog.waitFor(); assert.match(await memoryDialog.textContent(), /may include cache/i);
      await noOverflow(); await capture('cache-dialog');
      await page.keyboard.press('Escape'); await memoryDialog.waitFor({ state: 'hidden' });
      await page.evaluate(() => { window.__integration.memory('stale'); window.__integration.mode('retained'); });
      await activate(diskCell.getByRole('button', { name: /^Disk reading/ }));
      const diskDialog = page.getByRole('dialog', { name: 'Disk reading for integration-guest' });
      await diskDialog.waitFor(); assert.match(await diskDialog.textContent(), /last known|retained|paused/i);
      await capture('disk-dialog'); await page.keyboard.press('Escape');
      if (width < 768) await row.getByText('integration-guest', { exact: true }).tap();
      else await activate(row.locator('[data-row-action]'));
      const overview = page.getByTestId('guest-drawer-overview'); await overview.waitFor();
      assert.equal(await page.getByRole('tab', { name: 'History', exact: true }).count(), 0);
      const details = page.getByTestId('guest-technical-details'); assert.equal(await details.getAttribute('open'), null);
      const chart = overview.locator('[data-history-group="utilization"]'); await chart.waitFor();
      await chart.locator('[data-history-deferred="memory"]').waitFor();
      assert.match(await chart.locator('[data-history-deferred="memory"]').textContent(), /stopped reporting/);
      assert.equal(await chart.locator('[data-history-current="memory"]').count(), 0);
      assert.match(await chart.locator('[data-history-deferred="disk"]').textContent(), /last known|paused/i);
      await overview.getByText('/data', { exact: true }).waitFor();
      const os = (await page.evaluate(() => window.__integration.evidence())).os;
      const fact = overview.getByTestId('guest-drawer-facts').getByText(os, { exact: true });
      await fact.waitFor(); assert.equal(await fact.evaluate((e) => e.scrollWidth <= e.clientWidth), true);
      await chart.scrollIntoViewIfNeeded(); await noOverflow(); await capture('retained-charts', overview);
      // A new selected current observation, not a cooldown, withdraws the qualifications.
      await page.evaluate(() => { window.__integration.memory('current-zero'); window.__integration.mode('fresh'); });
      await chart.locator('[data-history-deferred="memory"]').waitFor({ state: 'hidden' });
      await chart.locator('[data-history-deferred="disk"]').waitFor({ state: 'hidden' });
      assert.match(await chart.locator('[data-history-current="memory"]').textContent(), /0%/);
      await page.evaluate(() => window.__integration.display('sparklines'));
      assert.equal(await row.getByRole('button', { name: /^(Memory|Disk) reading/ }).count(), 0);
      record.states.push('Quiet captions, keyboard/touch disclosures, stale-Agent/prior disk qualifications, current measured zero and sparkline withdrawal');
      historyMode = 'stored';
      await activate(overview.getByRole('button', { name: 'Refresh history', exact: true }));
      await chart.locator('[data-history-band="memory"]').waitFor({ state: 'attached' });
      assert(await chart.locator('[data-history-band="memory"]').getAttribute('d'));
      const inspector = chart.getByRole('slider', { name: 'Inspect Utilization history' });
      await inspector.focus(); await page.keyboard.press('End');
      await chart.locator('[data-history-peak="memory"]').waitFor();
      assert.match(await chart.locator('[data-history-peak="memory"]').textContent(), /90%/);
      assert.match(await chart.getByTestId('guest-history-hover-time').getAttribute('aria-label'), /Stored observation/);
      await noOverflow(); await capture('stored-bands', chart);
      await overview.getByLabel('History range', { exact: true }).selectOption('7d');
      assert(record.requests.some((r) => r.path === '/api/metrics-store/history' && r.search.includes('range=7d')));
      await activate(page.getByRole('tab', { name: 'Manage', exact: true }));
      const reads = record.requests.filter((r) => r.path === '/api/metrics-store/history').length;
      await page.clock.runFor(60001);
      assert.equal(record.requests.filter((r) => r.path === '/api/metrics-store/history').length, reads);
      await activate(page.getByRole('tab', { name: 'Overview', exact: true }));
      record.states.push('Overview-only History, finite min/max spread and stored peak, range selection; hidden Manage does not poll');
      // Missing usage stays missing and the shipped guidance is usable without hovering.
      await page.evaluate(() => { window.__integration.mode('unknown'); window.__integration.memory('powered-off'); });
      await overview.getByRole('link', { name: 'Filesystem reading guidance' }).waitFor();
      await activate(overview.getByRole('link', { name: 'Filesystem reading guidance' }));
      await page.waitForURL('**/docs/VM_DISK_MONITORING#*');
      await page.getByRole('heading', { name: 'A missing reading is not an installation diagnosis', exact: true }).waitFor();
      await noOverflow(); await capture('safe-guide');
      record.states.push('Unknown usage and powered-off memory kept distinct; actual shipped safe guidance reached');

      historyMode = 'empty';
      await page.goto(origin + '/node', { waitUntil: 'networkidle' });
      const node = page.getByTestId('node-drawer-overview'), nodeDetails = page.getByTestId('node-technical-details');
      await node.waitFor(); assert.equal(await nodeDetails.getAttribute('open'), null);
      assert.equal(await page.getByRole('tab', { name: 'History', exact: true }).count(), 0);
      const thermals = node.locator('[data-history-group="thermals"]');
      await thermals.locator('[data-history-current="temperature"]').waitFor();
      assert.match(await thermals.locator('[data-history-current="temperature"]').textContent(), /55/);
      assert.doesNotMatch(await thermals.textContent(), /91°C|95°C/);
      const helpTrigger = node.getByRole('button', { name: 'Why update checks are unavailable' });
      await activate(helpTrigger);
      const help = page.getByRole('dialog', { name: 'Proxmox update-check access' }); await help.waitFor();
      assert.match(await help.textContent(), /Sys.Modify/); assert.match(await help.textContent(), /broader than read-only monitoring/);
      await help.evaluate((e) => { window.__retainedHelp = e; }); await helpTrigger.evaluate((e) => { window.__retainedTrigger = e; });
      await page.evaluate(() => window.__integration.update({ cpu: 0.2 }));
      await page.clock.runFor(60001);
      await page.evaluate(() => window.__integration.update({ pendingUpdatesStatus: 'unavailable' }));
      assert.equal(await help.evaluate((e) => window.__retainedHelp === e), true);
      assert.equal(await helpTrigger.evaluate((e) => window.__retainedTrigger === e), true);
      const more = help.getByRole('link', { name: 'Learn more', exact: true });
      assert.equal(await more.getAttribute('href'), '/docs/TROUBLESHOOTING#proxmox-pending-update-access');
      await noOverflow(); await capture('node-help');
      await page.keyboard.press('Escape');
      await activate(nodeDetails.locator('summary'));
      await nodeDetails.getByText('Usage unavailable', { exact: true }).waitFor();
      await nodeDetails.getByText('0 B of 10.0 GB', { exact: true }).waitFor();
      await nodeDetails.getByText('5.00 GB of 10.0 GB', { exact: true }).waitFor();
      assert.equal(await nodeDetails.getByRole('button', { name: 'Why update checks are unavailable' }).count(), 0);
      await noOverflow(); await capture('node-disks', nodeDetails);
      await page.evaluate(() => window.__integration.update({ cpuTemperature: undefined }));
      await thermals.locator('[data-history-current="temperature"]').waitFor({ state: 'hidden' });
      assert.doesNotMatch(await thermals.textContent(), /91°C|95°C/);
      record.states.push('Primary CPU only; missing primary not replaced by hottest sensor; per-disk missing versus measured zero; update help stable outside closed Details');
      assert.equal(record.errors.length, 0); record.trustedInputs = await page.evaluate(() => window.__trustedInputs);
      assert(record.trustedInputs.length > 0 && record.trustedInputs.every((e) => e.trusted));
      await context.close(); await browser.close(); browser = null;
    }
    result.result = 'passed';
  } catch (error) { result.error = String(error.stack || error); process.exitCode = 1; }
  finally {
    if (browser) await browser.close(); result.cleanup.browser = true;
    if (server) await new Promise((resolve) => server.close(resolve)); result.cleanup.server = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({ result: result.result, cases: result.cases.length, captures: result.captures.length, error: result.error || null }));
  }
})();
