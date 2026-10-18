// Production HTTP reader -> backup recovery model -> existing Coverage/search.
// Build the shared coverage-workload-identity fixture at this exact source first.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/coverage-workload-identity/static');
const output = path.join(root, 'tmp/backup-failure-status');
const origin = 'http://127.0.0.1:5318';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  binding, protocol_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [], captures: [], responses: [], errors: [], unexpected: [], cleanup: {},
};
const rawFailure = 'ERROR: archive unavailable'; // ordinary poller wire status
const longFailure = rawFailure + '\n' + 'repository-path-'.repeat(18) +
  '\n<img src="/unexpected" onerror="unsafe()"><script>unsafe()</script>';
const task = (extra = {}) => ({
  id: 'latest', instance: 'east', node: 'pve-1', vmid: 100, type: 'vzdump',
  status: rawFailure, startTime: '2026-10-18T19:00:00Z',
  endTime: '2026-10-18T19:01:00Z', observedAt: '2026-10-18T19:02:00Z', ...extra,
});
const guests = ['east', 'west'].map((instance) => ({
  id: `${instance}-100`, name: 'database', type: 'vm', platformId: instance,
  sourceType: 'api', status: 'running', platformScopes: ['proxmox-pve'],
  proxmox: { vmid: 100, node: 'pve-1', instance },
}));
const posture = {
  data: guests.map((g) => ({ subjectResourceId: g.id, state: 'unknown',
    freshness: 'unknown', verification: 'unknown', coverage: 'unknown',
    explanation: 'Synthetic backup posture remains separate from the job result.',
    providerStates: [], repositoryResourceIds: [], evidenceIds: [],
    evaluatedAt: '2026-10-18T20:00:00Z' })),
  policy: { freshnessWindowSeconds: 604800, verificationWindowSeconds: 604800,
    requireVerification: true },
};
let server, browser, tasks;
async function capture(page, name) {
  const file = path.join(output, name + '.png');
  await page.screenshot({ path: file });
  result.captures.push({ file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}
async function geometry(page) {
  return page.evaluate(() => {
    const problems = [];
    for (const n of document.querySelectorAll('[data-backup-task-detail] dd')) {
      const box = n.getBoundingClientRect(), range = document.createRange();
      range.selectNodeContents(n);
      if (n.scrollWidth > n.clientWidth + 1 || [...range.getClientRects()].some(
        (r) => r.left < box.left - 1 || r.right > box.right + 1)) problems.push(n.textContent);
    }
    return { problems, overflow: document.documentElement.scrollWidth > innerWidth };
  });
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.join(root,
    'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
  for (const [file, digest] of Object.entries(binding.sources))
    assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
  for (const file of ['ProxmoxCoverageTable.tsx', 'proxmoxBackupRecoveryModel.ts'])
    assert(binding.sources['src/features/proxmox/' + file]);
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, file); assets.set('/' + file, bytes);
  }
  server = http.createServer((req, res) => {
    const url = new URL(req.url, origin);
    const json = (value) => {
      const bytes = Buffer.from(JSON.stringify(value));
      result.responses.push({ path: url.pathname, sha256: hash(bytes) });
      res.setHeader('Content-Type', 'application/json'); res.end(bytes);
    };
    if (req.method !== 'GET') {
      result.unexpected.push('non-GET'); res.statusCode = 405; return res.end();
    }
    if (url.pathname === '/fixture/workloads') return json(guests);
    if (url.pathname === '/api/backups/pve') return json({ data: {
      backupTasks: tasks, guestSnapshots: [], storageBackups: [] } });
    if (url.pathname === '/api/backups/pbs') return json({ data: { backups: [] } });
    if (url.pathname === '/api/recovery/postures') return json(posture);
    const file = url.pathname === '/proxmox/backups/coverage'
      ? '/browser-tests/coverage-workload-identity.html' : url.pathname;
    const bytes = assets.get(file);
    if (!bytes) { result.unexpected.push(url.pathname); res.statusCode = 404; return res.end(); }
    res.setHeader('Content-Type', { '.js': 'text/javascript', '.css': 'text/css',
      '.html': 'text/html' }[path.extname(file)] || 'application/octet-stream');
    res.end(bytes);
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject); server.listen(5318, '127.0.0.1', resolve);
  });
  for (const [name, engine, width] of [['desktop', chromium, 1440], ['narrow', webkit, 320]]) {
    const phone = width < 768;
    browser = await engine.launch(engine === chromium
      ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
    const context = await browser.newContext({ viewport: { width, height: 1000 },
      hasTouch: phone, isMobile: phone, locale: 'en-GB', timezoneId: 'UTC' });
    const page = await context.newPage(); page.setDefaultTimeout(10000);
    page.on('pageerror', (e) => result.errors.push(String(e)));
    page.on('console', (m) => { if (m.type() === 'error') result.errors.push(m.text()); });
    await context.route('**/*', (route) => {
      if (new URL(route.request().url()).origin === origin) return route.continue();
      result.unexpected.push('off-origin'); return route.abort();
    });
    await page.clock.setFixedTime(new Date('2026-10-18T20:00:00Z'));
    tasks = [task(), task({ id: 'peer', instance: 'west', status: 'OK' })];
    await page.goto(origin + '/proxmox/backups/coverage');
    if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
    const row = page.locator('[data-proxmox-coverage-key="resource:east-100"]');
    const peer = page.locator('[data-proxmox-coverage-key="resource:west-100"]');
    await row.waitFor();
    const search = page.getByPlaceholder('Search backups by workload, node, source, or status');
    for (const term of ['failed', 'archive unavailable', 'error:']) {
      await search.fill(term); await row.waitFor(); assert.equal(await peer.count(), 0);
    }
    await search.fill(''); await peer.waitFor();
    const toggle = row.getByRole('button', { expanded: false });
    if (phone) await row.locator('td').first().tap();
    else { await toggle.focus(); await page.keyboard.press('Enter'); }
    const detail = row.locator('xpath=following-sibling::tr[1]').getByRole('region',
      { name: 'Latest backup job', exact: true });
    await detail.waitFor();
    const field = (label) => detail.locator('dt', { hasText: new RegExp('^' + label + '$') })
      .locator('xpath=following-sibling::dd[1]');
    assert.equal(await field('Result').innerText(), 'Failed');
    assert.match(await field('Result').getAttribute('class'), /text-red-600/);
    assert.equal(await field('Reported status').textContent(), rawFailure);
    assert.equal(await field('Reported error').count(), 0);
    assert.equal(await field('Job connection').innerText(), 'east');
    assert.equal(await field('Job node').innerText(), 'pve-1');
    assert.match(await peer.innerText(), /OK/);
    await row.evaluate((element) => { window.failureStatusRow = element; });
    assert.deepEqual(await geometry(page), { problems: [], overflow: false });
    await detail.scrollIntoViewIfNeeded(); await capture(page, name + '-raw-failure');
    const checks = ['verdict and raw-text search isolate the failed connection',
      'production ERROR: prefix becomes explicit red Failed, with exact raw status and no invented error',
      'same VMID on another connection remains OK; job provenance is unchanged'];
    const refresh = async (next, expected) => {
      tasks = [task(next), task({ id: 'peer', instance: 'west', status: 'OK' })];
      const button = page.getByRole('button', { name: 'Refresh backups', exact: true });
      const response = page.waitForResponse((r) => new URL(r.url()).pathname === '/api/backups/pve');
      if (phone) await button.tap(); else { await button.focus(); await page.keyboard.press('Enter'); }
      await response;
      await page.waitForFunction(() => [...document.querySelectorAll('button[aria-disabled="false"]')]
        .some((b) => b.textContent === 'Refresh backups'));
      await page.waitForFunction((value) => document.querySelector('[data-backup-task-detail] dd')
        ?.textContent === value, expected);
      assert.equal(await row.getByRole('button').getAttribute('aria-expanded'), 'true');
      assert(await row.evaluate((e) => e === window.failureStatusRow));
      if (!phone) assert(await button.evaluate((e) => e === document.activeElement));
    };
    await refresh({ status: longFailure }, 'Failed');
    assert.equal(await field('Reported status').textContent(), longFailure);
    assert.equal(await field('Reported error').count(), 0);
    assert.equal(await detail.locator('img, script').count(), 0);
    assert.deepEqual(await geometry(page), { problems: [], overflow: false });
    await field('Reported status').scrollIntoViewIfNeeded(); await capture(page, name + '-literal-status');
    checks.push('complete multiline raw status wraps; markup stays plain text without network/script execution');
    await refresh({ status: 'OK', error: 'archive unavailable' }, 'Unknown');
    assert.equal(await field('Reported status').innerText(), 'OK');
    assert.equal(await field('Reported error').innerText(), 'archive unavailable');
    assert.equal(await detail.locator('.text-emerald-600').count(), 0);
    assert.equal(await detail.locator('[data-backup-task-status-conflict]').count(), 1);
    await detail.scrollIntoViewIfNeeded(); await capture(page, name + '-contradictory');
    checks.push('successful status plus error stays Unknown, not a Failed or OK invention');
    await refresh({ status: 'ERROR-free' }, 'ERROR-free');
    assert.equal(await field('Reported status').count(), 0);
    assert.equal(await field('Reported error').count(), 0);
    assert.doesNotMatch(await field('Result').getAttribute('class'), /text-red|text-emerald/);
    checks.push('unrecognised lookalike remains literal, not an inferred failure');
    await refresh({ status: 'OK' }, 'OK');
    assert.equal(await field('Reported status').count(), 0);
    assert.equal(await field('Reported error').count(), 0);
    assert.equal(await detail.locator('[data-backup-task-status-conflict]').count(), 0);
    assert.match(await field('Result').getAttribute('class'), /text-base-content/);
    assert(!(await detail.innerText()).includes('archive unavailable'));
    assert.deepEqual(await geometry(page), { problems: [], overflow: false });
    await detail.scrollIntoViewIfNeeded(); await capture(page, name + '-corrected');
    await search.fill('failed'); assert.equal(await row.count(), 0); assert.equal(await peer.count(), 0);
    await search.fill(''); await row.waitFor(); await peer.waitFor();
    checks.push('same-open OK removes stale raw/error/conflict data and no longer matches Failed search',
      'desktop keyboard and narrow touch disclosures/refreshed focus; no horizontal overflow');
    result.cases.push({ viewport: name, width, height: 1000, browser: browser.version(), checks });
    await context.close(); await browser.close(); browser = undefined;
  }
  assert.deepEqual(result.errors, []); assert.deepEqual(result.unexpected, []); result.passed = true;
})().catch((e) => { result.failure = e.stack; process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close(); result.cleanup.browserClosed = true;
  if (server) await new Promise((resolve) => { server.close(resolve); server.closeAllConnections(); });
  result.cleanup.serverClosed = true;
  fs.mkdirSync(output, { recursive: true });
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  console.log(JSON.stringify({ passed: result.passed || false, cases: result.cases.length,
    captures: result.captures.length, failure: result.failure, cleanup: result.cleanup }));
});
