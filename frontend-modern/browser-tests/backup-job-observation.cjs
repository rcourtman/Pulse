// Real offline-node polling projections -> production reader/model/Coverage/CSS.
// Extra malformed/legacy timestamps and clock movement are explicit controls.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/coverage-workload-identity/static');
const output = path.join(root, 'tmp/backup-job-observation/browser');
const origin = 'http://127.0.0.1:5306';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const producerPath = path.join(root, 'tmp/backup-job-observation/producer.json');
const producer = JSON.parse(fs.readFileSync(producerPath));
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  binding, producer: producer.binding,
  producer_input_sha256: hash(fs.readFileSync(producerPath)),
  protocol_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [], captures: [], requests: [], responses: [], geometry: [],
  errors: [], unexpected: [], cleanup: {},
};
let server, browser, payload;
const guests = [
  ['site', 100, 'pve-a'], ['site', 200, 'pve-b'], ['site', 300, 'pve-b'], ['other', 200, 'pve-b'],
].map(([instance, vmid, node]) => ({
  id: `${instance}-${vmid}`, name: 'database', type: 'vm', platformId: instance,
  sourceType: 'api', status: 'running', platformScopes: ['proxmox-pve'],
  proxmox: { vmid, node, nodeName: node, instance },
}));
const postures = {
  data: guests.map((guest) => ({
    subjectResourceId: guest.id, state: 'unknown', freshness: 'unknown',
    verification: 'unknown', coverage: 'unknown',
    explanation: 'Synthetic posture is separate from source observation age.',
    providerStates: [], repositoryResourceIds: [], evidenceIds: [],
    evaluatedAt: '2026-10-18T01:00:00Z',
  })),
  policy: { freshnessWindowSeconds: 604800, verificationWindowSeconds: 604800, requireVerification: true },
};
const projections = producer.observations.direct;
const findTask = (wire, instance, vmid) => wire.data.backupTasks.find((task) => task.instance === instance && task.vmid === vmid);
const initialTime = findTask(projections.initial, 'site', 200).observedAt;
let now = Date.parse(initialTime) + 4 * 3600000;
function control(observedAt) {
  const wire = structuredClone(projections.offline);
  const task = findTask(wire, 'site', 200);
  if (observedAt === undefined) delete task.observedAt;
  else task.observedAt = observedAt;
  return wire;
}
async function geometry(page) {
  return page.evaluate(() => {
    const problems = [];
    for (const node of document.querySelectorAll('[data-backup-task-observed]')) {
      const box = node.getBoundingClientRect();
      const overflow = [];
      const range = document.createRange();
      const walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT);
      while (walker.nextNode()) {
        const text = walker.currentNode;
        for (let i = 0; i < text.textContent.length; i++) {
          if (/\s/u.test(text.textContent[i])) continue;
          range.setStart(text, i); range.setEnd(text, i + 1);
          for (const r of range.getClientRects())
            if (r.width > 0 && r.height > 0 && (r.left < box.left - 1 || r.right > box.right + 1))
              overflow.push({ glyph: text.textContent[i], left: r.left, right: r.right });
        }
      }
      // A span can wrap over multiple lines. Check its containing block as well.
      const parent = node.parentElement;
      if (overflow.length || parent.scrollWidth > parent.clientWidth + 1)
        problems.push({ text: node.textContent, overflow, clientWidth: parent.clientWidth, scrollWidth: parent.scrollWidth });
    }
    return { problems, pageOverflow: document.documentElement.scrollWidth > innerWidth };
  });
}
async function capture(page, name) {
  const file = path.join(output, name + '.png');
  await page.screenshot({ path: file });
  result.captures.push({ file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
  assert.match(producer.binding.source_sha, /^[a-f0-9]{40}$/);
  for (const [file, digest] of Object.entries(producer.binding.files))
    assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, file);
  for (const [file, digest] of Object.entries(binding.sources))
    assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
  for (const file of ['src/features/proxmox/ProxmoxCoverageTable.tsx', 'src/features/proxmox/proxmoxBackupRecoveryModel.ts', 'src/features/proxmox/proxmoxBackupInventory.ts']) assert(binding.sources[file]);
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, file); assets.set('/' + file, bytes);
  }
  server = http.createServer((req, res) => {
    const url = new URL(req.url, origin);
    result.requests.push({ method: req.method, path: url.pathname });
    const json = (body) => { res.setHeader('Content-Type', 'application/json'); res.end(JSON.stringify(body)); };
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
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(5306, '127.0.0.1', resolve); });
  for (const [name, engine, width] of [['desktop', chromium, 1440], ['phone', webkit, 320]]) {
    now = Date.parse(initialTime) + 4 * 3600000;
    const phone = width === 320;
    browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
    const context = await browser.newContext({ viewport: { width, height: 1000 }, hasTouch: phone, isMobile: phone, timezoneId: 'Europe/London', locale: 'en-GB' });
    const page = await context.newPage(); page.setDefaultTimeout(10000);
    page.on('pageerror', (e) => result.errors.push(String(e)));
    page.on('console', (m) => { if (m.type() === 'error') result.errors.push(m.text()); });
    await context.route('**/*', (route) => {
      if (new URL(route.request().url()).origin === origin) return route.continue();
      result.unexpected.push('off-origin'); return route.abort();
    });
    await page.clock.setFixedTime(new Date(now));
    payload = projections.initial;
    await page.goto(origin + '/proxmox/backups/coverage');
    if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
    const row = page.locator('[data-proxmox-coverage-key="resource:site-200"]');
    const sibling = page.locator('[data-proxmox-coverage-key="resource:site-100"]');
    const other = page.locator('[data-proxmox-coverage-key="resource:other-200"]');
    await row.waitFor();
    await row.evaluate((node) => { window.observedRow = node; });
    assert.match(await row.innerText(), /Job observed 4h ago/);
    const toggle = row.getByRole('button');
    if (phone) await row.locator('td').first().tap();
    else { await toggle.focus(); await page.keyboard.press('Enter'); }
    const detail = row.locator('xpath=following-sibling::tr[1]').getByRole('region', { name: 'Latest backup job', exact: true });
    await detail.waitFor();
    const observed = detail.locator('dt', { hasText: /^Last observed$/ }).locator('xpath=following-sibling::dd[1]');
    const resultField = detail.locator('dt', { hasText: /^Result$/ }).locator('xpath=following-sibling::dd[1]');
    assert.equal(await observed.locator('time').getAttribute('datetime'), initialTime);
    assert.match(await observed.innerText(), /4h ago/);
    const refresh = async (next) => {
      payload = next;
      const button = page.getByRole('button', { name: 'Refresh backups', exact: true });
      const wait = page.waitForResponse((r) => new URL(r.url()).pathname === '/api/backups/pve');
      if (phone) await button.tap(); else { await button.focus(); await page.keyboard.press('Enter'); }
      const response = await wait; const body = await response.body();
      assert.deepEqual(JSON.parse(body), next);
      result.responses.push({ browser: name, sha256: hash(body) });
      await page.waitForFunction(() => [...document.querySelectorAll('button[aria-disabled="false"]')].some((button) => button.textContent === 'Refresh backups'));
      assert.equal(await toggle.getAttribute('aria-expanded'), 'true');
      assert(await row.evaluate((node) => node === window.observedRow));
      if (!phone) assert(await button.evaluate((node) => document.activeElement === node));
    };
    for (const phase of ['offline', 'online-empty', 'all-offline', 'no-nodes']) {
      await refresh(projections[phase]);
      assert.equal(await observed.locator('time').getAttribute('datetime'), initialTime);
      assert.match(await observed.innerText(), /4h ago/);
      assert.equal(await resultField.innerText(), 'Warning');
      if (phase === 'offline') {
        assert.match(await sibling.innerText(), /OK/);
        assert.equal(await sibling.locator('[data-backup-task-observed] time').getAttribute('datetime'), findTask(projections.offline, 'site', 100).observedAt);
      }
      assert.equal(await other.locator('[data-backup-task-observed] time').getAttribute('datetime'), findTask(projections.initial, 'other', 200).observedAt);
    }
    result.cases.push({ browser: name, check: 'four real unread-node projections retain the exact age while peer and repeated VMID stay separate' });
    const inventoryRequestCount = () => result.requests.filter((r) => r.path.startsWith('/api/backups/')).length;
    const beforeClockRequests = inventoryRequestCount();
    now += 2 * 3600000;
    await page.clock.setFixedTime(new Date(now));
    // Trigger the shared display clock once; this is not an inventory request.
    await page.clock.runFor(30001);
    await page.waitForFunction(() => document.querySelector('[data-proxmox-coverage-key="resource:site-200"]')?.textContent.includes('Job observed 6h ago'));
    assert.equal(inventoryRequestCount(), beforeClockRequests);
    assert.match(await observed.innerText(), /6h ago/);
    assert.equal(await observed.locator('time').getAttribute('datetime'), initialTime);
    await detail.scrollIntoViewIfNeeded(); await capture(page, name + '-retained-observation');
    result.cases.push({ browser: name, check: 'shared clock ages the retained source timestamp from 4h to 6h without inventory requests' });
    await refresh(projections.resumed);
    assert.equal(await observed.locator('time').getAttribute('datetime'), findTask(projections.resumed, 'site', 200).observedAt);
    for (const [label, value] of [
      ['missing', undefined], ['empty', ''], ['zero', '0001-01-01T00:00:00Z'], ['invalid', 'not-a-time'],
      ['future', new Date(now + 86400000).toISOString()], ['markup', '<img src="/unexpected" onerror="unsafe()">'],
    ]) {
      await refresh(control(value));
      await page.waitForFunction(() => [...document.querySelectorAll('[data-backup-task-detail] dt')].find((node) => node.textContent === 'Last observed')?.nextElementSibling.textContent === 'Unknown');
      assert.equal(await observed.innerText(), 'Unknown');
      assert.equal(await observed.locator('time').count(), 0);
      assert.match(await row.innerText(), /Job observation unknown/);
      assert.equal(await resultField.innerText(), 'Warning');
      result.cases.push({ browser: name, check: label + ' observation stays unknown, result stays separate' });
    }
    await detail.scrollIntoViewIfNeeded(); await capture(page, name + '-unknown-observation');
    await refresh(control(new Date(now - 60000).toISOString()));
    await page.waitForFunction(() => document.querySelector('[data-proxmox-coverage-key="resource:site-200"]')?.textContent.includes('Job observed 1m ago'));
    assert.match(await observed.innerText(), /1m ago/);
    const measured = await geometry(page); result.geometry.push({ browser: name, measured });
    if (measured.problems.length || measured.pageOverflow) await capture(page, name + '-adverse-geometry');
    assert.deepEqual(measured, { problems: [], pageOverflow: false });
    await detail.scrollIntoViewIfNeeded(); await capture(page, name + '-replacement-observation');
    assert.match(await detail.innerText(), /not when the job finished/);
    assert.match(await detail.innerText(), /does not make retained results current/);
    result.cases.push({ browser: name, check: 'same-open known replacement restores only its real observation; no false guest health or fresh posture' });
    result.cases.push({ browser: name, browserVersion: browser.version(), viewport: { width, height: 1000 } });
    await context.close(); await browser.close(); browser = undefined;
  }
  assert.deepEqual(result.errors, []); assert.deepEqual(result.unexpected, []);
  result.result = 'passed';
})().catch((e) => { result.result = 'failed'; result.failure = String(e.stack || e); process.exitCode = 1; })
.finally(async () => {
  if (browser) { await browser.close(); result.cleanup.browserAfterFailure = true; }
  if (server) await new Promise((resolve) => server.close(resolve));
  result.cleanup.serverClosed = true; result.cleanup.browserClosed = true;
  fs.mkdirSync(output, { recursive: true });
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  console.log(JSON.stringify({ result: result.result, failure: result.failure, cases: result.cases.length, captures: result.captures.length, result_file: path.relative(root, path.join(output, 'result.json')) }));
});
