// Actual poller/store/posture frames -> production Backups table and API hooks.
// Static offline preview; no guest/backup commands or external service access.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/pve-reconciliation-static');
const output = path.join(root, 'tmp/pve-reconciliation-browser');
const origin = 'http://127.0.0.1:5373';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const producer = JSON.parse(fs.readFileSync(path.join(root, 'tmp/pve-reconciliation-producer.json')));
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  result: 'incomplete',
  producer_sha256: hash(fs.readFileSync(path.join(root, 'tmp/pve-reconciliation-producer.json'))),
  consumer: binding,
  harness: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [], requests: [], captures: [], cleanup: { browser: false, server: false },
  limits: 'Real PVE storage poll, recovery ingest and posture reader through production Backups/API hooks. Synthetic HTTP envelope and workloads. Not full application, installed recovery, archive availability, guest thaw or filesystem liveness.',
};
let browser, server;

(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
  assert.equal(producer.runtime_sha256, hash(fs.readFileSync(path.join(root, 'internal/monitoring/monitor_backups.go'))));
  assert.equal(binding.graph, hash(fs.readFileSync(path.join(root, 'frontend-modern/package-lock.json'))));
  assert.equal(binding.builder, hash(fs.readFileSync(path.join(root, 'scripts/build-pve-backup-reconciliation.mjs'))));
  for (const [file, digest] of Object.entries(binding.sources)) {
    assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
  }
  for (const file of ['src/hooks/useProtectionPostures.ts', 'src/features/proxmox/ProxmoxBackupsTable.tsx', 'src/features/proxmox/ProxmoxCoverageTable.tsx']) {
    assert(binding.sources[file], `production consumer not compiled: ${file}`);
  }
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, file);
    assets.set('/' + file, bytes);
  }
  server = http.createServer((request, response) => {
    const pathname = new URL(request.url, origin).pathname;
    const isEntry = pathname === '/proxmox/backups/coverage';
    const bytes = assets.get(isEntry ? '/browser-tests/backups.html' : pathname);
    if (!bytes || request.method !== 'GET') return response.writeHead(404).end();
    response.writeHead(200, { 'Content-Type': isEntry ? 'text/html' : ({ '.js': 'text/javascript', '.css': 'text/css' }[path.extname(pathname)] || 'application/octet-stream') });
    response.end(bytes);
  });
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(5373, '127.0.0.1', resolve); });
  for (const [name, engine, width, height] of [['desktop', chromium, 1280, 900], ['narrow', webkit, 320, 812]]) {
    browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
    const page = await browser.newPage({ viewport: { width, height }, isMobile: width < 768, hasTouch: width < 768, locale: 'en-GB', timezoneId: 'UTC' });
    page.setDefaultTimeout(10000);
    await page.clock.setFixedTime(new Date(producer.observed_at));
    const record = { name, width, height, browser: browser.version(), errors: [], phases: [] };
    page.on('pageerror', (error) => record.errors.push(error.message));
    let phase = 'observed';
    const responses = [];
    page.on('response', async (response) => {
      if (new URL(response.url()).pathname === '/api/recovery/postures') responses.push(await response.json());
    });
    await page.route('**/*', async (route) => {
      const request = route.request(), url = new URL(request.url());
      assert.equal(url.origin, origin, 'external request');
      assert.equal(request.method(), 'GET', 'unexpected mutation');
      const workloads = producer.resource_ids.map((id, index) => ({ id, type: 'vm', name: `guest-${101 + index}`, platformId: 'cluster', platformType: 'proxmox-pve', sourceType: 'api', status: 'running', lastSeen: Date.parse(producer.observed_at), proxmox: { vmid: 101 + index, node: index === 0 ? 'node-a' : 'node-b', instance: 'cluster' } }));
      let body;
      if (url.pathname === '/fixture/workloads') body = workloads;
      else if (url.pathname === '/api/backups/pve') body = producer.frames[phase].pve;
      else if (url.pathname === '/api/backups/pbs') body = { data: { backups: [] } };
      else if (url.pathname === '/api/recovery/postures') {
        assert.deepEqual(url.searchParams.getAll('resourceId').sort(), [...producer.resource_ids].sort());
        body = producer.frames[phase].postures;
      } else {
        assert(!url.pathname.startsWith('/api/'), `unexpected API read: ${url.pathname}`);
        return route.continue();
      }
      result.requests.push({ viewport: name, phase, path: url.pathname });
      return route.fulfill({ json: body });
    });
    for (phase of ['observed', 'warm-partial', 'partial', 'denied', 'recovered', 'confirmed-empty']) {
      if (phase === 'observed') await page.goto(origin + '/proxmox/backups/coverage');
      else await page.reload();
      const expand = page.getByRole('button', { name: 'Expand details for guest-102', exact: true });
      await expand.waitFor();
      await expand.focus();
      await expand.press('Enter');
      const panel = page.getByTestId('proxmox-coverage-detail');
      const posture = producer.frames[phase].postures.data.find((item) => item.subjectResourceId === producer.resource_ids[1]);
      await panel.getByText(posture.explanation, { exact: true }).waitFor();
      await page.waitForFunction(() => document.fonts.status === 'loaded');
      const body = await panel.innerText();
      for (const provider of posture.providerStates) {
        const cap = (s) => s[0].toUpperCase() + s.slice(1);
        assert(body.includes(`History ${cap(provider.historyCompleteness)}`));
        assert(body.includes(`Access ${cap(provider.permissions)}`));
      }
      if (phase === 'warm-partial' || phase === 'partial' || phase === 'denied') {
        assert.notEqual(posture.state, 'protected');
        assert.equal(posture.providerStates[0].historyCompleteness, 'partial');
        assert.equal(Object.keys(producer.frames[phase].points).length, 2, 'unseen historical point disappeared');
      }
      assert.deepEqual(responses.at(-1), producer.frames[phase].postures, 'actual hook response changed producer evidence');
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
      assert.equal(overflow, false, `${name}/${phase}: horizontal overflow`);
      const file = path.join(output, `${name}-${phase}.png`);
      await page.screenshot({ path: file, fullPage: true });
      result.captures.push({ name, phase, file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
      record.phases.push({ phase, state: posture.state, body, overflow });
      await page.getByRole('button', { name: 'Collapse details for guest-102', exact: true }).press('Enter');
      await panel.waitFor({ state: 'hidden' });
    }
    assert.deepEqual(record.errors, []);
    result.cases.push(record);
    await browser.close(); browser = null;
  }
  result.result = 'passed';
})().catch((error) => { result.error = error.stack; process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close();
  result.cleanup.browser = true;
  if (server) await new Promise((resolve) => server.close(resolve));
  result.cleanup.server = true;
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
});
