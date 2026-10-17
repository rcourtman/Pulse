// Synthetic HTTP snapshots; production Workloads mapper, drawers, charts and alert settings.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/native-guest-filesystem-power/browser';
const origin = 'http://127.0.0.1:5361';
const hash = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const point = (timestamp, value) => ({timestamp, value, min: value, max: value});
const disk = (used = 9 * 1024 ** 3) => ({mountpoint: '/data', device: '/dev/vda1', total: 10 * 1024 ** 3, used, usage: used / (10 * 1024 ** 3) * 100, type: 'ext4', filesystem: 'ext4'});
function resource(state) {
  const disks = state.capacity ? [{...disk(), used: undefined, usage: undefined}] : [disk(state.used)];
  return {
    id: 'guest:site-a:105', type: state.kind || 'vm', name: 'backup-guest', displayName: 'backup-guest',
    platformId: 'site-a', platformType: 'proxmox-pve', platformScopes: ['proxmox-pve'],
    status: 'online', vmid: 105, node: 'pve-a', instance: 'site-a', lastSeen: new Date().toISOString(),
    metrics: {
      cpu: {percent: 10}, memory: {percent: 25, used: 1024 ** 3, total: 4 * 1024 ** 3, observation: {state: 'current', source: 'status-mem', observedAt: new Date().toISOString()}},
      disk: state.capacity ? {total: 10 * 1024 ** 3} : {percent: disks[0].usage, used: disks[0].used, total: 10 * 1024 ** 3},
    },
    metricsTarget: {resourceType: 'vm', resourceId: 'guest:site-a:105'},
    proxmox: {vmid: 105, node: 'pve-a', instance: 'site-a', runtimeStatus: state.power, disks: state.linked ? undefined : disks, diskStatusReason: state.reason || ''},
    agent: state.linked ? {agentId: 'fixture-agent', stale: Boolean(state.stale), disks} : undefined,
  };
}
const settle = page => page.evaluate(async () => {
  await document.fonts.ready;
  await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
});
(async () => {
  fs.mkdirSync(output, {recursive: true});
  const runtime = 'frontend-modern/src/utils/workloadGuestPresentation.ts';
  const result = {result: 'incomplete', playwright: require('playwright/package.json').version, runtime_hashes: {[runtime]: hash('/workspace/' + runtime)}, cases: [], screenshots: [], cleanup: {browser: false, server: false}, limits: 'Synthetic HTTP/resource fixtures with real production mappers, row, full guest/canonical drawers, History and alert settings. No provider, QGA, backup, guest action, installed recovery or release proof.'};
  assert.equal(result.playwright, JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
  const {createServer} = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({root, configFile: root + '/vite.config.ts', cacheDir: output + '/vite-cache', optimizeDeps: {entries: ['browser-tests/paused-filesystem.html', 'browser-tests/resource-stopped-filesystem.html']}, server: {host: '127.0.0.1', port: 5361, strictPort: true}});
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, height, dark] of [['desktop', chromium, 1440, 1000, false], ['narrow', webkit, 320, 900, true]]) {
      browser = await engine.launch(engine === chromium ? {headless: true, channel: 'chromium', args: ['--no-sandbox']} : {headless: true});
      const context = await browser.newContext({viewport: {width, height}, isMobile: width < 768, hasTouch: width < 768, locale: 'en-GB', timezoneId: 'UTC'});
      const page = await context.newPage();
      page.setDefaultTimeout(12000);
      const record = {name, width, height, dark, browser: browser.version(), errors: [], requests: [], observations: []};
      result.cases.push(record);
      let state = {power: 'unknown'}, stored = false;
      page.on('pageerror', error => record.errors.push(error.message));
      await page.routeWebSocket(/.*/, socket => socket.close());
      await page.addInitScript(dark => {
        document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.toggle('dark', dark));
        window.__trustedInputs = [];
        for (const type of ['click', 'keydown']) document.addEventListener(type, event => window.__trustedInputs.push({type, trusted: event.isTrusted}), true);
      }, dark);
      await page.route('**/*', route => {
        const request = route.request(), url = new URL(request.url());
        if (url.origin !== origin) {record.requests.push({path: url.pathname, method: request.method(), blocked: true}); return route.abort();}
        if (!url.pathname.startsWith('/api/')) return route.continue();
        record.requests.push({path: url.pathname, method: request.method()});
        assert.equal(request.method(), 'GET');
        if (url.pathname === '/api/proof/resources') return route.fulfill({json: [resource(state)]});
        if (url.pathname === '/api/metrics-store/history') {
          const end = Date.now() - 60000, start = end - 3600000;
          return route.fulfill({json: {resourceType: 'vm', resourceId: url.searchParams.get('resourceId'), range: url.searchParams.get('range'), start, end, metrics: stored ? {disk: [point(start, 25), point(end, 50)]} : {}, source: 'store'}});
        }
        return route.fulfill({json: url.pathname.includes('runtime-capabilities') ? {capabilities: [], limits: [], max_history_days: 90, runtime: {build: 'community'}} : {success: true, data: [], alerts: [], anomalies: [], config: null, relationships: [], available: false}});
      });
      const capture = async (label, locator) => {
        await locator.scrollIntoViewIfNeeded(); await settle(page);
        const file = `${name}-${label}.png`;
        await locator.screenshot({path: path.join(output, file)});
        result.screenshots.push({file, sha256: hash(path.join(output, file)), label, name});
      };
      await page.goto(origin + '/browser-tests/paused-filesystem.html', {waitUntil: 'commit'});
      const row = page.locator('[data-guest-id]');
      await row.waitFor();
      const diskCell = row.locator('[data-workload-col="disk"]');
      assert.match(await diskCell.innerText(), /Last known/);
      assert.match(await diskCell.getAttribute('title'), /VM running state is unconfirmed/);
      await capture('workload-row', row);
      if (width < 768) await row.getByText('backup-guest', {exact: true}).tap();
      else {await row.getByRole('button', {name: 'Expand backup-guest', exact: true}).focus(); await page.keyboard.press('Enter');}
      const drawer = page.getByRole('region', {name: 'Guest details'});
      await drawer.waitFor();
      const details = page.getByTestId('guest-technical-details');
      await details.locator('summary').click();
      assert.match(await details.innerText(), /Last known 90%/);
      assert.match(await details.innerText(), /VM running state is unconfirmed/);
      assert.equal(await details.getByRole('progressbar', {name: 'Filesystem /data utilization'}).count(), 0);
      const chart = drawer.locator('[data-history-group="utilization"]');
      await chart.locator('[data-history-last-known="disk"]').waitFor();
      assert.equal(await chart.locator('[data-history-current="disk"]').count(), 0);
      // The fixture uses a labelled section, which may expose region only in some engines.
      const settings = page.locator('section[aria-label="Guest filesystem alert settings"]');
      assert.match(await settings.innerText(), /Last known 9.0 \/ 10.0 GB · 90.0%/);
      assert.match(await settings.innerText(), /95/);
      await capture('unconfirmed-details', details);
      await capture('unconfirmed-history', chart);
      await capture('alert-thresholds', settings);
      const apply = async value => {state = value; await page.evaluate(() => window.loadSnapshot()); await settle(page);};
      for (const power of ['starting', 'stopping', 'private-unrecognised-state']) {
        await apply({power});
        assert.match(await diskCell.innerText(), /Last known/);
        assert.match(await details.innerText(), /VM running state is unconfirmed/);
        assert.doesNotMatch(await details.innerText(), /private-unrecognised-state/);
        assert.equal(await chart.locator('[data-history-current="disk"]').count(), 0);
      }
      await apply({power: 'unknown', capacity: true});
      assert.equal(await chart.locator('[data-history-last-known="disk"]').count(), 0);
      assert.doesNotMatch(await details.innerText(), /Last known 0%/);
      await apply({power: 'unknown', reason: 'permission-denied'});
      assert.match(await details.innerText(), /Permission denied/);
      assert.doesNotMatch(await details.innerText(), /VM running state is unconfirmed/);
      await apply({power: 'unknown', linked: true});
      assert.equal(await details.getByRole('progressbar', {name: 'Filesystem /data utilization'}).getAttribute('aria-valuenow'), '90');
      assert.match(await chart.locator('[data-history-current="disk"]').innerText(), /90.0%/);
      await apply({power: 'unknown', linked: true, stale: true});
      assert.match(await details.innerText(), /stopped reporting/);
      await apply({power: 'running', used: 0});
      assert.equal(await details.getByRole('progressbar', {name: 'Filesystem /data utilization'}).getAttribute('aria-valuenow'), '0');
      assert.match(await chart.locator('[data-history-current="disk"]').innerText(), /0.0%/);
      assert.doesNotMatch(await details.innerText(), /VM running state is unconfirmed/);
      assert.match(await settings.innerText(), /95/);
      await capture('fresh-zero', details);
      record.observations.push('HTTP -> useWorkloads -> same-open row, filesystem details, History and alert settings qualify unconfirmed native values; explicit error, capacity-only, independent agent/staleness and fresh zero controls passed. Saved threshold remains 95%.');
      await details.locator('summary').focus(); await page.keyboard.press('Enter');
      assert.equal(await details.evaluate(el => el.open), false);
      record.trustedInput = await page.evaluate(() => window.__trustedInputs);
      assert(record.trustedInput.some(event => event.trusted));
      // The canonical drawer consumes the same shared qualification independently.
      stored = true;
      await page.goto(origin + '/browser-tests/resource-stopped-filesystem.html', {waitUntil: 'commit'});
      await page.waitForFunction(() => Boolean(window.__resourceStoppedFilesystem));
      await page.evaluate(() => window.__resourceStoppedFilesystem.apply({power: 'unknown'}));
      const canonicalChart = page.locator('[data-history-group="utilization"]');
      await canonicalChart.locator('path[data-history-series]').first().waitFor();
      const paths = await canonicalChart.locator('path[data-history-series]').evaluateAll(items => items.map(item => item.getAttribute('d')));
      assert.equal(await canonicalChart.locator('[data-history-current="disk"]').count(), 0);
      assert.match(await canonicalChart.locator('[data-history-deferred="disk"]').innerText(), /VM running state is unconfirmed/);
      await capture('canonical-stored', canonicalChart);
      await page.evaluate(() => window.__resourceStoppedFilesystem.apply({power: 'running', usage: 0}));
      assert.deepEqual(await canonicalChart.locator('path[data-history-series]').evaluateAll(items => items.map(item => item.getAttribute('d'))), paths);
      assert.equal(await canonicalChart.locator('[data-history-deferred="disk"]').count(), 0);
      record.observations.push('Canonical drawer withdraws current disk at native unknown state without changing stored paths; ordinary running replacement clears the notice.');
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      assert.deepEqual(record.errors, []);
      assert.equal(record.requests.filter(request => !['GET', 'HEAD'].includes(request.method)).length, 0);
      await context.close(); await browser.close(); browser = null;
    }
    result.result = 'passed';
  } catch (error) {result.error = String(error.stack || error); throw error;}
  finally {
    if (browser) await browser.close(); result.cleanup.browser = true;
    await server.close(); result.cleanup.server = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
