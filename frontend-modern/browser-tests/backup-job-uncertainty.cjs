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
const output = path.join(root, 'tmp/backup-job-uncertainty');
const origin = 'http://127.0.0.1:5320';
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
// Exercise the same large-inventory window as ordinary fleets, not a tiny mock table.
const guests = [
  ...['east', 'west'].map((instance) => ({
  id: `${instance}-100`, name: 'database', type: 'vm', platformId: instance,
  sourceType: 'api', status: 'running', platformScopes: ['proxmox-pve'],
  proxmox: { vmid: 100, node: 'pve-1', instance },
})),
  ...Array.from({ length: 198 }, (_, i) => ({
    id: `fleet-${i}`, name: `zz-synthetic-${i}`, type: 'vm', platformId: 'east',
    sourceType: 'api', status: 'running', platformScopes: ['proxmox-pve'],
    proxmox: { vmid: 1000 + i, node: 'pve-1', instance: 'east' },
  })),
];
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
  assert.deepEqual(await geometry(page), { problems: [], overflow: false });
  const file = path.join(output, name + '.png');
  await page.screenshot({ path: file });
  result.captures.push({ file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}
async function geometry(page) {
  return page.evaluate(() => {
    const problems = [];
    for (const n of document.querySelectorAll('[data-backup-task-detail] dd, [data-backup-task-evidence] dd')) {
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
    server.once('error', reject); server.listen(5320, '127.0.0.1', resolve);
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
    const peerTask = task({ id: 'peer', instance: 'west', status: 'OK' });
    const known = task({ id: 'known-ok', status: 'OK' });
    const uncertain = task({ id: 'undated-failed', node: 'pve-previous', status: longFailure, startTime: '',
      error: 'Distinct reported error retained independently.' });
    tasks = [uncertain, known, peerTask];
    await page.goto(origin + '/proxmox/backups/coverage');
    if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
    const row = page.locator('[data-proxmox-coverage-key="resource:east-100"]');
    const peer = page.locator('[data-proxmox-coverage-key="resource:west-100"]');
    await row.waitFor();
    const search = page.getByPlaceholder('Search backups by workload, node, source, or status');
    for (const term of ['failed', 'archive unavailable', 'undated-failed', 'distinct reported error', '2026-10-18T19:02:00Z']) {
      await search.fill(term); await row.waitFor(); assert.equal(await peer.count(), 0);
    }
    await search.fill(''); await peer.waitFor();
    assert.equal(await row.locator('[data-backup-task-unknown]').count(), 1);
    assert.match(await peer.innerText(), /OK/);
    const toggle = row.getByRole('button', { expanded: false });
    if (phone) await row.locator('td').first().tap();
    else { await toggle.focus(); await page.keyboard.press('Enter'); }
    const open = row.locator('xpath=following-sibling::tr[1]');
    const collected = open.getByRole('region', { name: 'Collected backup jobs', exact: true });
    await collected.waitFor();
    const jobs = collected.locator('[data-backup-task-evidence]');
    const latest = open.getByRole('region', { name: 'Latest backup job', exact: true });
    const field = (job, label) => job.locator('dt', { hasText: new RegExp('^' + label + '$') })
      .locator('xpath=following-sibling::dd[1]');
    assert.equal(await jobs.count(), 2); assert.equal(await latest.count(), 0);
    assert.match(await collected.innerText(), /do not establish which job is latest/);
    assert.match(await collected.innerText(), /not a job timeline/);
    assert.equal(await field(jobs.nth(0), 'Job ID').innerText(), 'undated-failed');
    assert.equal(await field(jobs.nth(0), 'Result').innerText(), 'Failed');
    assert.equal(await field(jobs.nth(0), 'Reported status').textContent(), longFailure);
    assert.equal(await field(jobs.nth(0), 'Reported error').innerText(), uncertain.error);
    assert.equal(await field(jobs.nth(0), 'Started').innerText(), 'Unknown · not reported');
    assert.equal(await field(jobs.nth(0), 'Job connection').innerText(), 'east');
    assert.equal(await field(jobs.nth(0), 'Job node').innerText(), 'pve-previous');
    assert.equal(await field(jobs.nth(0), 'Last observed').locator('time').getAttribute('datetime'), uncertain.observedAt);
    assert.equal(await field(jobs.nth(1), 'Result').innerText(), 'OK');
    assert.equal(await collected.locator('img, script').count(), 0);
    await row.evaluate((element) => { window.uncertainJobRow = element; });
    await collected.locator('h4').first().scrollIntoViewIfNeeded();
    await capture(page, name + '-unresolved');
    await field(jobs.nth(0), 'Reported status').scrollIntoViewIfNeeded();
    await capture(page, name + '-raw-failure');
    const checks = ['Unknown latest result keeps both collected jobs, full failure status, separate error and recorded provenance',
      'failure/error/job-ID searches isolate the correct connection; equal-name/VMID peer remains OK',
      'missing start time is explicit, literal markup never executes, full wrapping is readable without hover'];
    const refresh = async (next, predicate, value) => {
      tasks = [...next, peerTask];
      const button = page.getByRole('button', { name: 'Refresh backups', exact: true });
      const read = page.waitForResponse((r) => new URL(r.url()).pathname === '/api/backups/pve');
      if (phone) await button.tap(); else { await button.focus(); await page.keyboard.press('Enter'); }
      const response = await read;
      assert.deepEqual(await response.json(), { data: { backupTasks: tasks, guestSnapshots: [], storageBackups: [] } });
      await page.waitForFunction(predicate, value);
      await page.waitForFunction(() => [...document.querySelectorAll('button[aria-disabled="false"]')]
        .some((b) => b.textContent === 'Refresh backups'));
      assert.equal(await row.getByRole('button').getAttribute('aria-expanded'), 'true');
      assert(await row.evaluate((e) => e === window.uncertainJobRow));
      if (!phone) assert(await button.evaluate((e) => e === document.activeElement));
    };
    const startVisible = (value) => [...document.querySelectorAll('[data-backup-task-start-unknown]')]
      .some((e) => e.textContent.includes(value));
    const future = '2026-10-19T19:00:00Z';
    await refresh([known, task({ id: 'future-failed', startTime: future })], startVisible, future);
    assert.equal(await latest.count(), 0); assert.equal(await jobs.count(), 2);
    assert.equal(await field(jobs.nth(1), 'Started').locator('time').count(), 0);
    assert.equal(await row.locator('[data-backup-task-unknown]').count(), 1);
    const rawStart = '<img src="/unexpected"><script>unsafe()</script>';
    await refresh([task({ id: 'invalid-start', startTime: rawStart }), known], startVisible, rawStart);
    assert.equal(await latest.count(), 0); assert.equal(await collected.locator('img, script').count(), 0);
    await field(jobs.nth(0), 'Started').scrollIntoViewIfNeeded();
    await capture(page, name + '-invalid-start');
    checks.push('future and invalid provider times remain Unknown, preserve exact literal start text and do not create a dated latest verdict');
    const tied = [task({ id: 'same-id', status: 'OK' }),
      task({ id: 'same-id', status: 'failed', error: 'Same-ID conflicting diagnostic' })];
    await refresh(tied, () => document.querySelector('[data-backup-task-chronology]')?.textContent.includes('same newest start time'));
    assert.equal(await jobs.count(), 2); assert.equal(await latest.count(), 0);
    assert.equal(await field(jobs.nth(0), 'Job ID').innerText(), 'same-id');
    assert.equal(await field(jobs.nth(1), 'Job ID').innerText(), 'same-id');
    assert.equal(await field(jobs.nth(0), 'Result').innerText(), 'OK');
    assert.equal(await field(jobs.nth(1), 'Result').innerText(), 'Failed');
    assert(!(await collected.innerText()).includes('archive unavailable'));
    await collected.scrollIntoViewIfNeeded(); await capture(page, name + '-conflict');
    await refresh([...tied].reverse(), () => document.querySelector('[data-backup-task-evidence] dd:nth-of-type(2)')?.textContent === 'Failed');
    assert.equal(await field(jobs.nth(0), 'Result').innerText(), 'Failed');
    assert.equal(await field(jobs.nth(1), 'Result').innerText(), 'OK');
    assert.equal(await jobs.count(), 2);
    checks.push('conflicting same-ID observations both survive, including reversed response order, without choosing a winner');
    await refresh([known], () => document.querySelector('[data-backup-task-detail] dd')?.textContent === 'OK');
    assert.equal(await collected.count(), 0); assert.equal(await latest.count(), 1);
    assert.equal(await row.locator('[data-backup-task-unknown]').count(), 0);
    assert(!(await open.innerText()).includes('conflicting diagnostic'));
    assert(!(await open.innerText()).includes('archive unavailable'));
    await latest.scrollIntoViewIfNeeded(); await capture(page, name + '-corrected');
    await search.fill('failed'); assert.equal(await row.count(), 0); assert.equal(await peer.count(), 0);
    await search.fill(''); await row.waitFor(); await peer.waitFor();
    await refresh([], () => !document.querySelector('[data-backup-task-detail], [data-backup-task-unresolved], [data-backup-task-chronology]'));
    assert.equal(await latest.count(), 0); assert.equal(await collected.count(), 0);
    assert.equal(await row.locator('[data-backup-task-unknown]').count(), 0);
    checks.push('ordinary OK then empty replacement removes stale evidence/search matches while preserving the logical open workload',
      'desktop keyboard and 320px WebKit touch use the actual HTTP reader, table and CSS; no horizontal overflow');
    // The PVE per-node task-list bound is 200. All accepted observations must
    // remain reachable inside the measured logical workload, even at that bound.
    const many = Array.from({ length: 200 }, (_, i) => task({ id: `collected-${i}`,
      status: i === 199 ? 'failed' : 'OK', startTime: i === 199 ? '' : known.startTime,
      error: i === 199 ? 'Last collected diagnostic remains reachable.' : undefined }));
    await refresh(many, () => document.querySelectorAll('[data-backup-task-evidence]').length === 200);
    await field(jobs.last(), 'Reported error').scrollIntoViewIfNeeded();
    assert.equal(await field(jobs.last(), 'Job ID').innerText(), 'collected-199');
    assert.match(await field(jobs.last(), 'Reported error').innerText(), /remains reachable/);
    assert(await field(jobs.last(), 'Reported error').isVisible());
    assert(await row.evaluate((e) => e === window.uncertainJobRow));
    await capture(page, name + '-last-collected');
    await collected.locator('h4').first().scrollIntoViewIfNeeded();
    assert(await collected.locator('h4').first().isVisible());
    assert.equal(await row.locator('[data-backup-task-unknown]').count(), 1);
    assert.equal(await jobs.count(), 200);
    checks.push('200-workload window retains the full 200-job collected list: final diagnostic and return to first evidence are reachable without row eviction');
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
