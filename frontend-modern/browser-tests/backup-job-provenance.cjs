// Real poller projections -> production HTTP reader/model/Coverage table/CSS.
// The producer.json input is extracted unchanged from the bound Go proof.
// Extra missing/long-field controls are explicitly synthetic, not producer facts.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/coverage-workload-identity/static');
const output = path.join(root, 'tmp/backup-job-provenance/browser');
const origin = 'http://127.0.0.1:5305';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const producerPath = path.join(root, 'tmp/backup-job-provenance/producer.json');
const producer = JSON.parse(fs.readFileSync(producerPath));
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  binding,
  producer: producer.binding,
  producer_input_sha256: hash(fs.readFileSync(producerPath)),
  protocol_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  requests: [],
  errors: [],
  unexpected: [],
  cleanup: {},
};
let server, browser, payload;
const guests = [
  ['site', 100, 'vm', 'current-node'],
  ['site', 200, 'system-container', 'current-node'],
  ['west', 100, 'vm', 'pve1'],
].map(([instance, vmid, type, node]) => ({
  id: `${instance}-${vmid}`,
  name: 'repeated-workload-name',
  type,
  platformId: instance,
  sourceType: 'api',
  status: 'running',
  platformScopes: ['proxmox-pve'],
  proxmox: { vmid, node, nodeName: node, instance },
}));
const postures = {
  data: guests.map((guest) => ({
    subjectResourceId: guest.id,
    state: 'unknown',
    freshness: 'unknown',
    verification: 'unknown',
    coverage: 'unknown',
    explanation: 'Synthetic posture is independent of observed job results.',
    providerStates: [],
    repositoryResourceIds: [],
    evidenceIds: [],
    evaluatedAt: '2026-10-18T11:00:00Z',
  })),
  policy: {
    freshnessWindowSeconds: 604800,
    verificationWindowSeconds: 604800,
    requireVerification: true,
  },
};
const synthetic = (extra) => {
  const task = producer.observations.warning.data.backupTasks.find((item) => item.vmid === 100);
  return { data: { backupTasks: [{ ...task, ...extra }], guestSnapshots: [], storageBackups: [] } };
};
async function geometry(page) {
  return page.evaluate(() => {
    const problems = [];
    for (const node of document.querySelectorAll('[data-backup-task-detail] dd')) {
      const box = node.getBoundingClientRect(),
        range = document.createRange();
      range.selectNodeContents(node);
      if (
        node.scrollWidth > node.clientWidth + 1 ||
        [...range.getClientRects()].some((r) => r.left < box.left - 1 || r.right > box.right + 1)
      )
        problems.push(node.textContent);
    }
    return { problems, overflow: document.documentElement.scrollWidth > innerWidth };
  });
}
async function capture(page, name) {
  const file = path.join(output, name + '.png');
  await page.screenshot({ path: file });
  result.captures.push({ file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  assert.equal(producer.binding.source_sha, binding.source_sha);
  for (const [file, digest] of Object.entries(binding.sources))
    assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
  for (const file of [
    'src/features/proxmox/ProxmoxCoverageTable.tsx',
    'src/features/proxmox/proxmoxBackupRecoveryModel.ts',
  ])
    assert(binding.sources[file]);
  assert(!JSON.stringify(producer.observations).includes('synthetic-private-guest-detail'));
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, file);
    assets.set('/' + file, bytes);
  }
  server = http.createServer((req, res) => {
    const url = new URL(req.url, origin);
    result.requests.push({ method: req.method, path: url.pathname });
    const json = (value) => {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(value));
    };
    if (req.method !== 'GET') {
      result.unexpected.push('non-GET');
      res.statusCode = 405;
      return res.end();
    }
    if (url.pathname === '/fixture/workloads') return json(guests);
    if (url.pathname === '/api/backups/pve') return json(payload);
    if (url.pathname === '/api/backups/pbs') return json({ data: { backups: [] } });
    if (url.pathname === '/api/recovery/postures') return json(postures);
    const file =
      url.pathname === '/proxmox/backups/coverage'
        ? '/browser-tests/coverage-workload-identity.html'
        : url.pathname;
    const bytes = assets.get(file);
    if (!bytes) {
      result.unexpected.push(url.pathname);
      res.statusCode = 404;
      return res.end();
    }
    res.setHeader(
      'Content-Type',
      { '.js': 'text/javascript', '.css': 'text/css', '.html': 'text/html' }[path.extname(file)] ||
        'application/octet-stream',
    );
    res.end(bytes);
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(5305, '127.0.0.1', resolve);
  });
  for (const [name, engine, width] of [
    ['desktop', chromium, 1440],
    ['phone', webkit, 320],
  ]) {
    const phone = width === 320;
    browser = await engine.launch(
      engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const context = await browser.newContext({
      viewport: { width, height: 1000 },
      hasTouch: phone,
      isMobile: phone,
      timezoneId: 'UTC',
      locale: 'en-GB',
    });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    page.on('pageerror', (e) => result.errors.push(String(e)));
    page.on('console', (m) => {
      if (m.type() === 'error') result.errors.push(m.text());
    });
    await context.route('**/*', (route) => {
      if (new URL(route.request().url()).origin === origin) return route.continue();
      result.unexpected.push('off-origin');
      return route.abort();
    });
    await page.clock.setFixedTime(new Date('2026-10-18T11:00:00Z'));
    payload = producer.observations.running;
    await page.goto(origin + '/proxmox/backups/coverage');
    if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
    const row = page.locator('[data-proxmox-coverage-key="resource:site-100"]');
    await row.waitFor();
    await row.evaluate((element) => {
      window.provenanceRow = element;
    });
    const toggle = row.getByRole('button', { expanded: false });
    if (phone) await row.locator('td').first().tap();
    else {
      await toggle.focus();
      await page.keyboard.press('Enter');
    }
    const detail = row
      .locator('xpath=following-sibling::tr[1]')
      .getByRole('region', { name: 'Latest backup job', exact: true });
    await detail.waitFor();
    const field = (label) =>
      detail
        .locator('dt', { hasText: new RegExp('^' + label + '$') })
        .locator('xpath=following-sibling::dd[1]');
    assert.equal(await field('Result').innerText(), 'Running');
    assert.equal(await field('Job connection').innerText(), 'site');
    assert.equal(await field('Job node').innerText(), 'pve1');
    assert.match(await row.innerText(), /current-node/);
    const refresh = async (next, expected) => {
      payload = next;
      const button = page.getByRole('button', { name: 'Refresh backups', exact: true });
      const response = page.waitForResponse(
        (r) => new URL(r.url()).pathname === '/api/backups/pve',
      );
      if (phone) await button.tap();
      else {
        await button.focus();
        await page.keyboard.press('Enter');
      }
      await response;
      await page.waitForFunction(
        (value) => document.querySelector('[data-backup-task-detail] dd')?.textContent === value,
        expected,
      );
      assert.equal(await row.getByRole('button').getAttribute('aria-expanded'), 'true');
      assert(await row.evaluate((element) => window.provenanceRow === element));
      if (!phone) assert(await button.evaluate((element) => document.activeElement === element));
      assert.deepEqual(await geometry(page), { problems: [], overflow: false });
    };
    const checks = ['running job keeps original site/pve1 beside current guest node'];
    for (const phase of ['warning', 'cached']) {
      await refresh(producer.observations[phase], 'Warning');
      assert.equal(await field('Job connection').innerText(), 'site');
      assert.equal(await field('Job node').innerText(), 'pve1');
      assert.equal(
        await field('Reported warning').innerText(),
        'Backup completed with warnings; review the original Proxmox task log.',
      );
      assert.equal(await field('Reported error').count(), 0);
      assert.equal(await detail.locator('.text-emerald-600').count(), 0);
      assert.match(await detail.innerText(), /workload may now be on another node/);
      const sibling = page.locator('[data-proxmox-coverage-key="resource:site-200"]');
      assert.match(await sibling.innerText(), /OK/);
      const otherConnection = page.locator('[data-proxmox-coverage-key="resource:west-100"]');
      assert(!(await otherConnection.innerText()).includes('Warning'));
      checks.push(
        `${phase}: warning and original provenance survive; clean sibling and reused west VMID remain separate`,
      );
    }
    await detail.scrollIntoViewIfNeeded();
    await capture(page, name + '-producer-warning');
    const search = page.getByPlaceholder('Search backups by workload, node, source, or status');
    await search.fill('pve1');
    await row.waitFor();
    assert.match(await row.innerText(), /Warning/);
    checks.push('old job node search still finds the migrated guest');
    await search.fill('');
    await refresh(producer.observations['next-clean-job'], 'OK');
    assert.equal(await field('Reported warning').count(), 0);
    assert.equal(await field('Job node').innerText(), 'pve1');
    checks.push('next clean poll clears warning but preserves recorded origin');
    await refresh(synthetic({ node: '' }), 'Warning');
    assert.equal(await field('Job node').innerText(), 'Not reported');
    assert.equal(await field('Job connection').innerText(), 'site');
    await detail.scrollIntoViewIfNeeded();
    await capture(page, name + '-missing-node');
    await refresh(synthetic({ instance: '', node: 'current-node' }), 'Warning');
    assert.equal(await field('Job connection').innerText(), 'Not reported');
    assert.equal(await field('Job node').innerText(), 'current-node');
    checks.push(
      'synthetic missing fields never borrow current workload connection/node or retain old values',
    );
    const longNode =
      'original-backup-node-'.repeat(10) + '<img src="/unexpected" onerror="unsafe()">';
    await refresh(synthetic({ node: longNode }), 'Warning');
    assert.equal(await field('Job node').textContent(), longNode);
    assert.equal(await detail.locator('img, script').count(), 0);
    await detail.scrollIntoViewIfNeeded();
    await capture(page, name + '-literal-long-node');
    checks.push('synthetic long HTML-shaped node stays literal and wraps without clipping');
    result.cases.push({ name, width, height: 1000, browser: browser.version(), checks });
    await context.close();
    await browser.close();
    browser = undefined;
  }
  assert.deepEqual(result.errors, []);
  assert.deepEqual(result.unexpected, []);
  result.passed = true;
})()
  .catch((e) => {
    result.failure = e.stack;
    process.exitCode = 1;
  })
  .finally(async () => {
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    if (server)
      await new Promise((resolve) => {
        server.close(resolve);
        server.closeAllConnections();
      });
    result.cleanup.serverClosed = true;
    fs.mkdirSync(output, { recursive: true });
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        passed: result.passed || false,
        cases: result.cases.length,
        captures: result.captures.length,
        failure: result.failure,
        cleanup: result.cleanup,
      }),
    );
  });
