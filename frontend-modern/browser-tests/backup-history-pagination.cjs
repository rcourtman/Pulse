// Contradictory offset-page history -> ordinary Go polling -> shipped Coverage.
// The consumer input is the actual emitted producer JSON, never a rewritten verdict.
// All provider/API data is synthetic. No command, guest, backup or database is run.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/coverage-workload-identity/static');
const output = path.join(root, 'tmp/backup-history-pagination/browser');
const origin = 'http://127.0.0.1:5312';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const producerPath = path.join(root, 'tmp/backup-history-pagination/producer.json');
const producer = JSON.parse(fs.readFileSync(producerPath));
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  binding, producer: producer.binding,
  producer_input_sha256: hash(fs.readFileSync(producerPath)),
  protocol_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [], captures: [], responses: [], geometry: [], requests: [],
  errors: [], unexpected: [], cleanup: {},
};
let browser, server, payload;
const guests = [ ['site', 100, 'pve-a'], ['site', 200, 'pve-b'], ['other', 100, 'pve-a'] ].map(
  ([instance, vmid, node]) => ({
    id: `${instance}-${vmid}`, name: 'database', type: 'vm', platformId: instance,
    sourceType: 'api', status: 'running', platformScopes: ['proxmox-pve'],
    proxmox: { vmid, node, nodeName: node, instance },
  }),
);
const postures = {
  data: guests.map((guest) => ({
    subjectResourceId: guest.id, state: 'unknown', freshness: 'unknown', verification: 'unknown',
    coverage: 'unknown', providerStates: [], repositoryResourceIds: [], evidenceIds: [],
    explanation: 'Synthetic independent protection posture, not a task outcome or restore assurance.',
    evaluatedAt: '2026-10-18T21:00:00Z',
  })),
  policy: { freshnessWindowSeconds: 604800, verificationWindowSeconds: 604800, requireVerification: true },
};
async function capture(page, name) {
  const measured = await page.evaluate(() => {
    const overflow = [];
    for (const node of document.querySelectorAll('[data-backup-task-detail] dd')) {
      if (node.scrollWidth > node.clientWidth + 1) overflow.push(node.textContent);
    }
    return { overflow, pageOverflow: document.documentElement.scrollWidth > innerWidth };
  });
  result.geometry.push({ name, measured });
  assert.deepEqual(measured, { overflow: [], pageOverflow: false });
  const file = path.join(output, `${name}.png`);
  await page.screenshot({ path: file });
  result.captures.push({ file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
  assert.equal(binding.source_sha, producer.binding.source_sha);
  assert.equal(hash(fs.readFileSync(__filename)), hash(fs.readFileSync(path.join(root, 'frontend-modern/browser-tests/backup-history-pagination.cjs'))));
  for (const [file, digest] of Object.entries(producer.binding.files)) {
    // Preserve the producer's original control-file identity separately when
    // this browser protocol is repaired. All runtime inputs still have to
    // match the actual checkout; the running protocol is checked just above.
    const input = file === 'frontend-modern/browser-tests/backup-history-pagination.cjs'
      ? 'tmp/backup-history-pagination/producer-protocol.cjs'
      : file;
    assert.equal(hash(fs.readFileSync(path.join(root, input))), digest, file);
  }
  for (const [file, digest] of Object.entries(binding.sources))
    assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
  for (const file of [ 'src/features/proxmox/ProxmoxCoverageTable.tsx', 'src/features/proxmox/proxmoxBackupRecoveryModel.ts', 'src/features/proxmox/proxmoxBackupInventory.ts' ]) assert(binding.sources[file], file);
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, file);
    assets.set('/' + file, bytes);
  }
  server = http.createServer((req, res) => {
    const url = new URL(req.url, origin);
    result.requests.push({ method: req.method, path: url.pathname });
    const json = (data) => { res.setHeader('Content-Type', 'application/json'); res.end(JSON.stringify(data)); };
    if (req.method !== 'GET') { result.unexpected.push('non-GET'); res.statusCode = 405; return res.end(); }
    if (url.pathname === '/fixture/workloads') return json(guests);
    if (url.pathname === '/api/backups/pve') return json(payload);
    if (url.pathname === '/api/backups/pbs') return json({ data: { backups: [] } });
    if (url.pathname === '/api/recovery/postures') return json(postures);
    const file = url.pathname === '/proxmox/backups/coverage' ? '/browser-tests/coverage-workload-identity.html' : url.pathname;
    const bytes = assets.get(file);
    if (!bytes) { result.unexpected.push(url.pathname); res.statusCode = 404; return res.end(); }
    res.setHeader('Content-Type', { '.js': 'text/javascript', '.css': 'text/css', '.html': 'text/html' }[path.extname(file)] || 'application/octet-stream');
    res.end(bytes);
  });
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(5312, '127.0.0.1', resolve); });
  for (const client of ['direct', 'cluster']) {
    for (const [name, engine, width] of [ ['desktop', chromium, 1440], ['phone', webkit, 320] ]) {
      const phone = width === 320;
      browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width, height: 1000 }, isMobile: phone, hasTouch: phone, locale: 'en-GB', timezoneId: 'UTC' });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      page.on('pageerror', (e) => result.errors.push(String(e)));
      page.on('console', (msg) => { if (msg.type() === 'error') result.errors.push(msg.text()); });
      await context.route('**/*', (route) => {
        if (new URL(route.request().url()).origin === origin) return route.continue();
        result.unexpected.push('off-origin'); return route.abort();
      });
      const observations = producer.observations[client];
      const lastSeen = Math.max(...observations.initial.data.backupTasks.map((t) => Date.parse(t.observedAt)));
      await page.clock.setFixedTime(new Date(lastSeen + 60000));
      payload = observations.initial;
      await page.goto(origin + '/proxmox/backups/coverage');
      if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const row = page.locator('[data-proxmox-coverage-key="resource:site-100"]');
      await row.waitFor();
      await row.evaluate((element) => { window.historyPaginationRow = element; });
      const toggle = row.getByRole('button', { expanded: false });
      if (phone) await row.locator('td').first().tap();
      else { await toggle.focus(); await page.keyboard.press('Enter'); }
      const detail = row.locator('xpath=following-sibling::tr[1]').getByRole('region', { name: 'Latest backup job', exact: true });
      await detail.waitFor();
      const field = (label) => detail.locator('dt', { hasText: new RegExp('^' + label + '$') }).locator('xpath=following-sibling::dd[1]');
      assert.equal(await field('Result').innerText(), 'Failed');
      assert.equal(await field('Reported status').textContent(), 'ERROR: archive unavailable');
      assert.equal(await field('Job node').innerText(), 'pve-a');
      const checks = ['initial production failed task and pve-a provenance render'];
      for (const [phase, expected] of [ ['conflict', 'Failed'], ['conflict-reverse', 'Failed'], ['conflict-guest', 'Failed'], ['conflict-kind', 'Failed'], ['recovered', 'OK'] ]) {
        payload = observations[phase];
        const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
        const read = page.waitForResponse((r) => new URL(r.url()).pathname === '/api/backups/pve');
        if (phone) await refresh.tap();
        else { await refresh.focus(); await page.keyboard.press('Enter'); }
        const response = await read, bytes = await response.body();
        assert.deepEqual(JSON.parse(bytes), payload);
        result.responses.push({ client, viewport: name, phase, sha256: hash(bytes) });
        await page.waitForFunction(() => [...document.querySelectorAll('button[aria-disabled="false"]')].some((button) => button.textContent === 'Refresh backups'));
        // Job ID is the first description value. Wait for the named Result
        // field rather than treating that unrelated value as an outcome.
        await field('Result').filter({ hasText: new RegExp('^' + expected + '$') }).waitFor();
        assert.equal(await field('Result').innerText(), expected);
        assert.equal(await field('Job node').innerText(), 'pve-a');
        assert.equal(await field('Job connection').innerText(), 'site');
        assert.equal(await row.getByRole('button').getAttribute('aria-expanded'), 'true');
        assert(await row.evaluate((element) => element === window.historyPaginationRow));
        if (!phone) assert(await refresh.evaluate((button) => button === document.activeElement));
        assert.match(await page.locator('[data-proxmox-coverage-key="resource:site-200"]').innerText(), /OK/);
        assert.match(await page.locator('[data-proxmox-coverage-key="resource:other-100"]').innerText(), /OK/);
        if (phase !== 'recovered') {
          assert.equal(await field('Reported error').count(), 0, 'the producer reports its raw error as status, not a separate error field');
          assert.equal(await field('Result').innerText(), 'Failed');
          assert.equal(await field('Reported status').textContent(), 'ERROR: archive unavailable');
          assert.equal(await detail.locator('.text-emerald-600').count(), 0);
          checks.push(phase + ': both ambiguous history versions are withheld; the current failed live result and scope remain; readable peer and reused VMID remain OK');
        } else {
          assert.equal(await field('Reported status').count(), 0);
          assert.equal(await field('Reported error').count(), 0);
          checks.push('ordinary complete history updates the outcome to OK without stale error');
        }
        if (phase === 'conflict-kind' || phase === 'recovered') {
          await detail.scrollIntoViewIfNeeded();
          await capture(page, `${client}-${name}-${phase}`);
        }
      }
      const search = page.getByPlaceholder('Search backups by workload, node, source, or status');
      await search.fill('pve-a');
      await row.waitFor();
      assert.equal(await page.locator('[data-proxmox-coverage-key="resource:site-200"]').count(), 0);
      await search.fill('');
      await page.locator('[data-proxmox-coverage-key="resource:site-200"]').waitFor();
      checks.push('node search and clearing work after read-only refresh');
      result.cases.push({ client, viewport: name, width, height: 1000, browser: browser.version(), checks });
      await context.close(); await browser.close(); browser = undefined;
    }
  }
  assert.deepEqual(result.errors, []); assert.deepEqual(result.unexpected, []);
  result.passed = true;
})().catch((e) => { result.failure = e.stack; process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close(); result.cleanup.browserClosed = true;
  if (server) await new Promise((resolve) => { server.close(resolve); server.closeAllConnections(); });
  result.cleanup.serverClosed = true;
  fs.mkdirSync(output, { recursive: true });
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  console.log(JSON.stringify({ passed: result.passed || false, cases: result.cases.length, captures: result.captures.length, failure: result.failure, cleanup: result.cleanup }));
});
