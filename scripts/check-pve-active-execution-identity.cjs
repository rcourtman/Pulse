// Actual GET-only Go scan snapshots -> unchanged compiled production Backups.
// Independent provider inventories are synthetic; no guest or backup is run.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const http = require('node:http');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const out = path.join(root, 'tmp/pve-active-identity');
const compiled = path.join(root, 'tmp/coverage-workload-identity/static');
const origin = 'http://127.0.0.1:5313';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const producerBytes = fs.readFileSync(path.join(out, 'producer.json'));
const producer = JSON.parse(producerBytes);
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const phases = ['initial', 'conflict-guest', 'conflict-parent', 'conflict-start', 'running', 'finished', 'empty'];
const result = {
  producer: producer.binding, producerSHA256: hash(producerBytes),
  compilerBinding: binding, protocolSHA256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [], responses: [], captures: [], requests: [], geometry: [], errors: [], unexpected: [], cleanup: {},
};
const guests = ['site', 'other'].map((instance) => ({
  id: `${instance}-100`, type: 'vm', name: instance === 'site' ? 'fixture-guest' : 'independent-guest',
  displayName: instance === 'site' ? 'fixture-guest' : 'independent-guest',
  platformId: instance, platformType: 'proxmox-pve', platformScopes: ['proxmox-pve'], sourceType: 'api', status: 'running',
  proxmox: { vmid: 100, node: 'node', nodeName: 'node', instance },
}));
const postures = { data: [], policy: { freshnessWindowSeconds: 86400, verificationWindowSeconds: 604800, requireVerification: false } };
let payload, server, browser;
(async () => {
  fs.mkdirSync(out, { recursive: true });
  assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
  for (const [file, digest] of Object.entries(producer.binding.files))
    assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, file);
  for (const [file, digest] of Object.entries(binding.sources))
    assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
  for (const file of ['src/features/proxmox/ProxmoxBackupsTable.tsx', 'src/features/proxmox/proxmoxBackupRecoveryModel.ts', 'src/features/proxmox/proxmoxBackupInventory.ts']) assert(binding.sources[file], file);
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, file); assets.set('/' + file, bytes);
  }
  server = http.createServer((req, res) => {
    const url = new URL(req.url, origin);
    result.requests.push({ method: req.method, path: url.pathname });
    const json = (data) => { res.setHeader('Content-Type', 'application/json'); res.end(JSON.stringify(data)); };
    if (req.method !== 'GET') { result.unexpected.push('non-GET'); res.statusCode = 405; return res.end(); }
    if (url.pathname === '/fixture/workloads') return json(guests);
    if (url.pathname === '/api/backups/pve') return json({ data: payload });
    if (url.pathname === '/api/backups/pbs') return json({ data: { backups: [] } });
    if (url.pathname === '/api/recovery/postures') return json(postures);
    const file = url.pathname === '/proxmox/backups/date' ? '/browser-tests/coverage-workload-identity.html' : url.pathname;
    const bytes = assets.get(file);
    if (!bytes) { result.unexpected.push(url.pathname); res.statusCode = 404; return res.end(); }
    res.setHeader('Content-Type', { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' }[path.extname(file)] || 'application/octet-stream'); res.end(bytes);
  });
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(5313, '127.0.0.1', resolve); });
  for (const client of ['direct', 'cluster']) {
    const frames = producer.observations[client];
    assert.deepEqual(Object.keys(frames).sort(), phases.slice().sort());
    for (const phase of phases.filter((phase) => phase.startsWith('conflict-')))
      assert.deepEqual(frames[phase].storageBackups, frames.initial.storageBackups, 'contradictions preserve original archive evidence');
    for (const [name, engine, width] of [['desktop', chromium, 1440], ['phone', webkit, 320]]) {
      const phone = width === 320;
      browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width, height: 1000 }, isMobile: phone, hasTouch: phone, locale: 'en-GB', timezoneId: 'UTC' });
      const page = await context.newPage(); page.setDefaultTimeout(10000);
      page.on('pageerror', (error) => result.errors.push(String(error)));
      page.on('console', (msg) => { if (msg.type() === 'error') result.errors.push(msg.text()); });
      await context.route('**/*', (route) => {
        if (new URL(route.request().url()).origin === origin) return route.continue();
        result.unexpected.push('off-origin'); return route.abort();
      });
      payload = frames.initial;
      await page.clock.setFixedTime(new Date(Date.parse(frames.finished.storageBackups.find((b) => b.instance === 'site' && b.id.includes('-current.vma.zst')).time) + 60000));
      if (phone) await page.addInitScript(() => document.documentElement.classList.add('dark'));
      await page.goto(origin + '/proxmox/backups/date');
      const rows = page.locator('[data-proxmox-backup-row="recoverable"]');
      const current = page.locator('[data-proxmox-backup-key*="-current.vma.zst"]');
      const checks = [];
      for (const phase of phases) {
        payload = frames[phase];
        if (phase !== 'initial') {
          const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
          const read = page.waitForResponse((r) => new URL(r.url()).pathname === '/api/backups/pve');
          if (phone) await refresh.tap(); else { await refresh.focus(); await refresh.press('Enter'); }
          const response = await read, bytes = await response.body();
          assert.deepEqual(JSON.parse(bytes), { data: payload });
          result.responses.push({ client, viewport: name, phase, sha256: hash(bytes) });
          await page.waitForFunction(() => [...document.querySelectorAll('button[aria-disabled="false"]')].some((button) => button.textContent === 'Refresh backups'));
        }
        const count = phase === 'empty' ? 2 : ['running', 'finished'].includes(phase) ? 4 : 3;
        await page.waitForFunction((count) => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === count, count);
        assert.equal(await rows.count(), count);
        assert.equal(await page.locator('[data-proxmox-backup-key="artifact:archive:independent"]').count(), 1, 'same VMID independent connection persists');
        assert.equal(await current.count(), ['running', 'finished'].includes(phase) ? 1 : 0, 'ambiguous identity cannot render newly discovered output as a completed archive');
        const body = await page.locator('main').innerText();
        const completed = phase === 'empty' ? 2 : phase === 'finished' ? 4 : 3;
        assert(body.includes(`${completed} restore points`), 'summary excludes unfinished or withheld output');
        assert(body.includes(`snapshot-${phase}`), 'independent snapshot collection still progresses');
        if (phase === 'running') {
          assert(body.includes('1 incomplete entry'));
          const evidence = current.getByRole('button', { name: 'Backup evidence for fixture-guest: Running', exact: true });
          if (phone) await evidence.tap(); else { await evidence.focus(); await evidence.press('Enter'); }
          const dialog = page.getByRole('dialog'); await dialog.waitFor();
          assert((await dialog.innerText()).includes('not a completed recovery point'));
          await page.keyboard.press('Escape'); await dialog.waitFor({ state: 'hidden' });
          if (!phone) assert(await evidence.evaluate((button) => button === document.activeElement), 'evidence dialog returns keyboard focus');
        }
        checks.push(`${phase}: ${count} rows/${completed} restore points, exact snapshot and independent connection`);
        if (['conflict-start', 'running', 'finished'].includes(phase)) {
          if (phase === 'running') await current.scrollIntoViewIfNeeded(); else await rows.first().scrollIntoViewIfNeeded();
          await page.evaluate(() => document.fonts.ready);
          const geometry = await page.evaluate(() => ({ pageOverflow: document.documentElement.scrollWidth > innerWidth }));
          assert.equal(geometry.pageOverflow, false);
          result.geometry.push({ client, viewport: name, phase, ...geometry });
          const file = path.join(out, `${client}-${name}-${phase}.png`);
          await page.screenshot({ path: file, fullPage: true });
          result.captures.push({ file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
        }
      }
      result.cases.push({ client, viewport: name, width, browser: browser.version(), checks });
      await context.close(); await browser.close(); browser = undefined;
    }
  }
  assert.deepEqual(result.errors, []); assert.deepEqual(result.unexpected, []); result.passed = true;
})().catch((e) => { result.failure = e.stack; process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close(); result.cleanup.browsersClosed = true;
  if (server) await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  result.cleanup.serverClosed = true;
  fs.writeFileSync(path.join(out, 'browser-result.json'), JSON.stringify(result, null, 2) + '\n');
  console.log(JSON.stringify({ passed: result.passed || false, failure: result.failure, cases: result.cases.length, captures: result.captures.length, cleanup: result.cleanup }));
});
