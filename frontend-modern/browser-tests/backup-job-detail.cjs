// Existing HTTP reader -> recovery mapper -> actual Coverage disclosure/CSS.
// Prepare with build-coverage-workload-identity.mjs <observed full source SHA>.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/coverage-workload-identity/static');
const output = path.join(root, 'tmp/backup-job-detail/final');
const origin = 'http://127.0.0.1:5304';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  binding,
  protocol_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  requests: [],
  errors: [],
  unexpected: [],
  cleanup: {},
};
let server, browser, tasks;
const error =
  'Backup failed: repository is unavailable.\n' +
  'repository-path-'.repeat(18) +
  '\n<img src="/unexpected" onerror="unsafe()">';
const task = (extra = {}) => ({
  id: 'latest',
  instance: 'east',
  node: 'pve-1',
  vmid: 100,
  type: 'vzdump',
  status: 'failed',
  startTime: '2026-10-16T17:00:00Z',
  error,
  ...extra,
});
const guests = ['east', 'west'].map((instance) => ({
  id: `${instance}-100`,
  name: 'database',
  type: 'vm',
  platformId: instance,
  sourceType: 'api',
  status: 'running',
  platformScopes: ['proxmox-pve'],
  proxmox: { vmid: 100, node: 'pve-1', instance },
}));
const posture = () => ({
  data: guests.map((guest) => ({
    subjectResourceId: guest.id,
    state: 'unknown',
    freshness: 'unknown',
    verification: 'unknown',
    coverage: 'unknown',
    explanation: 'Synthetic provider posture is separate from job results.',
    providerStates: [],
    repositoryResourceIds: [],
    evidenceIds: [],
    evaluatedAt: '2026-10-16T18:00:00Z',
  })),
  policy: {
    freshnessWindowSeconds: 604800,
    verificationWindowSeconds: 604800,
    requireVerification: true,
  },
});
async function capture(page, name) {
  const file = path.join(output, name + '.png');
  await page.screenshot({ path: file });
  result.captures.push({ file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}
async function geometry(page) {
  return page.evaluate(() => {
    const problems = [];
    for (const node of document.querySelectorAll(
      '[data-backup-task-detail] dd, [data-backup-task-status-conflict]',
    )) {
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
(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  for (const [file, digest] of Object.entries(binding.sources))
    assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
  assert(binding.sources['src/features/proxmox/ProxmoxCoverageTable.tsx']);
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, file);
    assets.set('/' + file, bytes);
  }
  server = http.createServer((req, res) => {
    const url = new URL(req.url, origin);
    const json = (value) => {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(value));
    };
    result.requests.push({ method: req.method, path: url.pathname });
    if (req.method !== 'GET') {
      result.unexpected.push('non-GET');
      res.statusCode = 405;
      return res.end();
    }
    if (url.pathname === '/fixture/workloads') return json(guests);
    if (url.pathname === '/api/backups/pve')
      return json({
        data: {
          backupTasks: tasks,
          guestSnapshots: [],
          storageBackups: [],
        },
      });
    if (url.pathname === '/api/backups/pbs') return json({ data: { backups: [] } });
    if (url.pathname === '/api/recovery/postures') return json(posture());
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
    server.listen(5304, '127.0.0.1', resolve);
  });
  for (const [name, engine, width] of [
    ['desktop', chromium, 1440],
    ['phone', webkit, 320],
  ]) {
    const phone = width < 768;
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
    await page.clock.setFixedTime(new Date('2026-10-16T18:00:00Z'));
    tasks = [task(), task({ id: 'other', instance: 'west', error: 'OTHER CONNECTION ERROR' })];
    await page.goto(origin + '/proxmox/backups/coverage');
    if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
    const row = page.locator('[data-proxmox-coverage-key="resource:east-100"]');
    await row.waitFor();
    await row.evaluate((element) => {
      window.statusEvidenceRow = element;
    });
    const toggle = row.getByRole('button', { expanded: false });
    // The compact view deliberately makes the whole identity cell the touch
    // target; its small keyboard disclosure is visually hidden until focused.
    if (phone) await row.locator('td').first().tap();
    else {
      await toggle.focus();
      await page.keyboard.press('Enter');
    }
    const detail = page.getByRole('region', { name: 'Latest backup job', exact: true });
    await detail.waitFor();
    assert.match(await detail.innerText(), /Failed/);
    assert.equal(await detail.locator('dd').last().textContent(), error);
    assert.equal(await detail.locator('img, script').count(), 0);
    assert.equal(await detail.locator('time').getAttribute('datetime'), '2026-10-16T17:00:00Z');
    assert(!(await detail.innerText()).includes('OTHER CONNECTION ERROR'));
    assert.deepEqual(await geometry(page), { problems: [], overflow: false });
    await detail.scrollIntoViewIfNeeded();
    await capture(page, name + '-failed');
    const checks = [
      'full literal multiline error',
      'workload isolation',
      'exact start time',
      'wrapping and no overflow',
    ];
    const refresh = async (next) => {
      tasks = next;
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
      await page.waitForFunction(() =>
        document
          .querySelector('button[aria-disabled="false"]')
          ?.textContent.includes('Refresh backups'),
      );
      assert.equal(await row.getByRole('button').getAttribute('aria-expanded'), 'true');
      if (!phone) assert(await button.evaluate((e) => document.activeElement === e));
    };
    await refresh([task({ status: 'running', error: '' })]);
    await page.waitForFunction(
      () => document.querySelector('[data-backup-task-detail] dd')?.textContent === 'Running',
    );
    assert(!(await detail.innerText()).includes('Reported error'));
    checks.push('same-open Running removes old error and retains disclosure/keyboard focus');
    const headers = await page
      .locator('[data-proxmox-backups-table="coverage"] thead th')
      .allTextContents();
    const jobIndex = headers.findIndex((label) => label.trim() === 'Job');
    assert(jobIndex >= 0);
    const jobCell = row.locator('td').nth(jobIndex);
    for (const status of ['OK', 'success', 'completed']) {
      await refresh([task({ status, error })]);
      await page.waitForFunction(
        () => document.querySelector('[data-backup-task-detail] dd')?.textContent === 'Unknown',
      );
      assert.match(await jobCell.innerText(), /Unknown/);
      assert.equal(await jobCell.locator('.bg-emerald-500').count(), 0);
      assert.equal(await jobCell.locator('.bg-amber-500').count(), 1);
      assert.equal(
        await detail.locator('dt:has-text("Reported status") + dd').textContent(),
        status,
      );
      assert.equal(await detail.locator('dd').last().textContent(), error);
      assert.equal(await detail.locator('img, script').count(), 0);
      assert.match(
        await detail.locator('[data-backup-task-status-conflict]').innerText(),
        /reported job status and error disagree.*result is unknown/s,
      );
      assert(await row.evaluate((element) => window.statusEvidenceRow === element));
      assert.deepEqual(await geometry(page), { problems: [], overflow: false });
      checks.push(
        `${status} plus error stays Unknown, with literal original evidence and no green success`,
      );
    }
    await detail.scrollIntoViewIfNeeded();
    await capture(page, name + '-contradictory');
    await refresh([task({ status: 'OK', error: '' })]);
    await page.waitForFunction(
      () => document.querySelector('[data-backup-task-detail] dd')?.textContent === 'OK',
    );
    assert(!(await detail.innerText()).includes('Backup failed'));
    assert(!(await detail.innerText()).includes('Reported status'));
    assert.equal(await detail.locator('[data-backup-task-status-conflict]').count(), 0);
    assert.equal(await jobCell.locator('.bg-emerald-500').count(), 1);
    await detail.scrollIntoViewIfNeeded();
    await capture(page, name + '-corrected');
    checks.push('OK result is not a stale failure');
    await refresh([
      task({ status: 'OK', error: '' }),
      task({ id: 'undated', startTime: '', error }),
    ]);
    await page.locator('[data-backup-task-chronology]').waitFor();
    assert.equal(await detail.count(), 0);
    assert.match(
      await page.locator('[data-backup-task-chronology]').innerText(),
      /unavailable or in the future/,
    );
    checks.push('unknown chronology does not expose an older job');
    await refresh([task({ status: 'OK', error: '' }), task({ id: 'conflict', error })]);
    await page.waitForFunction(() =>
      document
        .querySelector('[data-backup-task-chronology]')
        ?.textContent.includes('same newest start time'),
    );
    assert.equal(await detail.count(), 0);
    checks.push('conflicting outcomes retain Unknown');
    await page.locator('[data-backup-task-chronology]').scrollIntoViewIfNeeded();
    await capture(page, name + '-uncertain');
    await refresh([]);
    await page.waitForFunction(() => !document.querySelector('[data-backup-task-chronology]'));
    assert.equal(await detail.count(), 0);
    assert.equal(await row.getByRole('button').getAttribute('aria-expanded'), 'true');
    assert.deepEqual(await geometry(page), { problems: [], overflow: false });
    checks.push('no jobs removes old detail without closing workload');
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
