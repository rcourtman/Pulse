// Real HTTP inventory readers -> recovery model -> scoped chart/list and CSS.
// Synthetic GET-only observations; no backup, guest or provider operation.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/coverage-workload-identity/static');
const output = path.join(root, 'tmp/backup-activity-scope');
const origin = 'http://127.0.0.1:5323';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  binding,
  protocol_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  geometry: [],
  responses: [],
  errors: [],
  unexpected: [],
  cleanup: {},
};
const today = '2026-10-19T10:00:00Z';
const yesterday = '2026-10-18T10:00:00Z';
const earlier = '2026-10-17T10:00:00Z';
const guest = (id, instance, nodeName, vmid) => ({
  id,
  name: id,
  type: 'vm',
  platformId: instance,
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'running',
  proxmox: { vmid, nodeName, instance },
});
const workloads = [
  guest('east-workload', 'east', 'pve-1', 100),
  guest('west-workload', 'west', 'pve-2', 200),
];
const pbs = (id, extra = {}) => ({
  id,
  instance: 'pbs-server',
  datastore: 'main',
  namespace: 'pve-1',
  backupType: 'vm',
  vmid: '100',
  backupTime: today,
  size: 2048,
  files: ['index.json.blob'],
  protected: false,
  verified: true,
  ...extra,
});
let replacement = false,
  server,
  browser;
function inventory() {
  return {
    pve: {
      backupTasks: [],
      storageBackups: [
        {
          id: 'file-100',
          instance: 'east',
          node: 'pve-1',
          type: 'vm',
          vmid: 100,
          time: today,
          size: 1024,
          storage: 'local',
          format: 'zst',
          volid: 'local:backup/vzdump-qemu-100.vma.zst',
          isPBS: false,
        },
      ],
      guestSnapshots: [
        {
          id: 'snapshot-100',
          instance: 'east',
          node: 'pve-1',
          type: 'vm',
          vmid: 100,
          time: yesterday,
          name: 'before-update',
          vmstate: false,
        },
      ],
    },
    pbs: {
      backups: [
        pbs('east-main', { size: replacement ? 4096 : 2048 }),
        pbs('east-offsite', { instance: 'pbs-offsite', backupTime: earlier, size: 4096 }),
        pbs('west-offsite', {
          instance: 'pbs-offsite',
          namespace: 'pve-2',
          vmid: '200',
          backupTime: yesterday,
          size: 8192,
          verified: false,
        }),
        pbs('west-undated', {
          instance: 'pbs-other',
          namespace: 'pve-2',
          vmid: '200',
          backupTime: '',
          size: 0,
        }),
        pbs('west-running', {
          instance: 'pbs-offsite',
          namespace: 'pve-2',
          vmid: '200',
          inProgress: true,
          size: 16384,
          verified: false,
        }),
      ],
    },
  };
}
const dayPattern = /: \d+ backups?(, selected)?$/;
async function count(page) {
  const labels = await page
    .getByRole('button', { name: dayPattern })
    .evaluateAll((els) => els.map((el) => el.getAttribute('aria-label')));
  return labels.reduce((total, label) => total + Number(label.match(/: (\d+) backups?/)[1]), 0);
}
const rowCount = (page) => page.locator('[data-proxmox-backup-row="recoverable"]').count();
async function capture(page, name) {
  await page.evaluate(() => window.scrollTo(0, 0));
  const geometry = await page.evaluate(() => ({
    overflow: document.documentElement.scrollWidth > innerWidth,
    clipped: [
      ...document.querySelectorAll('[data-backup-activity-scope], [data-backup-volume-evidence]'),
    ]
      .filter((el) => el.scrollWidth > el.clientWidth + 1)
      .map((el) => el.textContent),
  }));
  result.geometry.push({ name, ...geometry });
  assert.deepEqual(geometry, { overflow: false, clipped: [] });
  const file = path.join(output, name + '.png');
  await page.screenshot({ path: file, fullPage: true });
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
  for (const [file, digest] of Object.entries(binding.sources))
    assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
  assert.equal(binding.sources['browser-tests/backup-activity-scope.cjs'], result.protocol_sha256);
  for (const file of [
    'ProxmoxBackupsTable.tsx',
    'BackupActivityChart.tsx',
    'proxmoxBackupInventory.ts',
    'proxmoxBackupRecoveryModel.ts',
  ])
    assert(binding.sources['src/features/proxmox/' + file]);
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, file);
    assets.set('/' + file, bytes);
  }
  server = http.createServer((req, res) => {
    const url = new URL(req.url, origin);
    const json = (value) => {
      const bytes = Buffer.from(JSON.stringify(value));
      result.responses.push({
        method: req.method,
        path: url.pathname,
        replacement,
        sha256: hash(bytes),
      });
      res.setHeader('Content-Type', 'application/json');
      res.end(bytes);
    };
    if (req.method !== 'GET') {
      result.unexpected.push('non-GET');
      res.statusCode = 405;
      return res.end();
    }
    if (url.pathname === '/fixture/workloads') return json(workloads);
    const data = inventory();
    if (url.pathname === '/api/backups/pve') return json({ data: data.pve });
    if (url.pathname === '/api/backups/pbs') return json({ data: data.pbs });
    if (url.pathname === '/api/recovery/postures')
      return json({
        data: [],
        policy: {
          freshnessWindowSeconds: 604800,
          verificationWindowSeconds: 604800,
          requireVerification: true,
        },
      });
    const file =
      url.pathname === '/proxmox/backups/date'
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
    server.listen(5323, '127.0.0.1', resolve);
  });
  for (const [name, engine, width] of [
    ['desktop', chromium, 1365],
    ['narrow', webkit, 320],
  ]) {
    replacement = false;
    browser = await engine.launch(
      engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const context = await browser.newContext({
      viewport: { width, height: 1000 },
      hasTouch: width === 320,
      isMobile: width === 320,
      locale: 'en-GB',
      timezoneId: 'UTC',
    });
    if (width === 320)
      await context.addInitScript(() =>
        document.addEventListener(
          'DOMContentLoaded',
          () => document.documentElement.classList.add('dark'),
          { once: true },
        ),
      );
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
    await page.clock.setFixedTime(new Date('2026-10-19T12:00:00Z'));
    const button = (label) => page.getByRole('button', { name: label, exact: true });
    const activate = async (control) => {
      if (width === 320) await control.tap();
      else {
        await control.focus();
        await page.keyboard.press('Enter');
      }
    };
    const matrices = [];
    for (const [query, total, rows, undated] of [
      ['', 5, 7, true],
      ['?node=pve-1', 4, 4, false],
      ['?type=vm&q=west-workload', 1, 3, true],
      ['?q=east-workload', 4, 4, false],
      ['?source=pbs', 3, 5, true],
      ['?source=archive', 1, 1, false],
      ['?source=snapshot', 1, 1, false],
      ['?source=verified', 2, 3, true],
      ['?source=unverified', 1, 1, false],
      ['?location=pbs%3Apbs-offsite%3Amain', 2, 3, false],
      [
        '?node=pve-1&type=vm&source=pbs&location=pbs%3Apbs-offsite%3Amain&q=east-workload',
        1,
        1,
        false,
      ],
      ['?type=ct', 0, 0, false],
    ]) {
      await page.goto(origin + '/proxmox/backups/date' + query);
      await button('Refresh backups').waitFor();
      assert.equal(await count(page), total, query);
      assert.equal(await rowCount(page), rows, query);
      assert.equal(
        Boolean(
          await page
            .getByText(/Backup entries with unavailable or future dates remain listed/)
            .count(),
        ),
        undated,
        query,
      );
      matrices.push({ query, total, rows, undated });
      if (!query) await capture(page, name + '-all-count');
      if (query.startsWith('?node=pve-1&type=')) await capture(page, name + '-repository-count');
      if (query === '?type=ct') await capture(page, name + '-empty-scope');
    }
    await page.goto(origin + '/proxmox/backups/date');
    await button('Refresh backups').waitFor();
    const requestsBeforeFilters = result.responses.length;
    const search = page.getByPlaceholder('Search backups by workload, node, source, or status');
    await search.fill('east-workload');
    await page.waitForURL((url) => url.searchParams.get('q') === 'east-workload');
    assert.equal(await count(page), 4);
    assert.equal(await rowCount(page), 4);
    const labels = async () =>
      (
        await page
          .getByRole('button', { name: dayPattern })
          .evaluateAll((els) => els.map((el) => el.getAttribute('aria-label')))
      ).map((label) => label.replace(/, selected$/, ''));
    const timeline = await labels();
    await activate(page.getByRole('button', { name: /: 2 backups$/ }));
    await page.waitForURL((url) => url.searchParams.get('day') === '2026-10-19');
    assert.equal(new URL(page.url()).searchParams.get('day'), '2026-10-19');
    assert.equal(await rowCount(page), 2);
    assert.deepEqual(await labels(), timeline);
    await activate(button('Clear date filter'));
    await page.waitForURL((url) => !url.searchParams.has('day'));
    assert.equal(await rowCount(page), 4);
    await activate(button('Volume'));
    await page.getByRole('button', { name: /: 3.00 KB$/ }).waitFor();
    assert.match(
      await page.locator('[data-backup-volume-evidence]').innerText(),
      /1 completed restore point/,
    );
    assert.equal(await page.getByRole('button', { name: /: 8.00 KB$/ }).count(), 0);
    assert.equal(result.responses.length, requestsBeforeFilters, 'filters must not fetch again');
    await capture(page, name + '-scoped-volume');
    replacement = true;
    await activate(button('Refresh backups'));
    await page.getByRole('button', { name: /: 5.00 KB$/ }).waitFor();
    assert.equal(await search.inputValue(), 'east-workload');
    assert.equal(await button('Volume').getAttribute('aria-pressed'), 'true');
    const requestsAfterRefresh = result.responses.length;
    assert.equal(
      requestsAfterRefresh - requestsBeforeFilters,
      3,
      'only normal PVE/PBS/posture GETs',
    );
    await search.fill('west-workload');
    await page.waitForURL((url) => url.searchParams.get('q') === 'west-workload');
    await page.getByRole('button', { name: /: 8.00 KB$/ }).waitFor();
    assert.doesNotMatch(
      await page.locator('[data-backup-volume-evidence]').innerText(),
      /no positive size reported/,
    );
    if (width === 320) await activate(page.getByRole('button', { name: /^Filters/ }));
    await activate(page.getByRole('button', { name: /Clear filters/ }));
    await page.waitForURL((url) => !url.searchParams.has('q'));
    await page
      .getByRole('button', { name: /: 8.00 KB reported\. 1 backup without a positive size$/ })
      .waitFor();
    await page.getByRole('button', { name: /: 5.00 KB$/ }).waitFor();
    assert.equal(result.responses.length, requestsAfterRefresh);
    await activate(button('7d'));
    await activate(button('Count'));
    assert.equal(await count(page), 5);
    result.cases.push({
      name,
      width,
      browser: browser.version(),
      matrices,
      checks: [
        'saved scope matrix',
        'same-page search',
        'keyboard/touch day selection and clear',
        'full timeline retained after day selection',
        'scoped Count/Volume/missing sizes',
        'same-scope ordinary refresh',
        'filter clear and range/metric retention',
        'no filter-triggered read',
      ],
    });
    await context.close();
    await browser.close();
    browser = null;
    result.cleanup[name] = true;
  }
  assert.deepEqual(result.errors, []);
  assert.deepEqual(result.unexpected, []);
  result.passed = true;
})()
  .catch((error) => {
    result.failure = String(error.stack || error);
    process.exitCode = 1;
  })
  .finally(async () => {
    if (browser) {
      await browser.close();
      result.cleanup.failure_browser = true;
    }
    if (server) {
      await new Promise((resolve) => server.close(resolve));
      result.cleanup.server = true;
    }
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        passed: result.passed,
        failure: result.failure,
        cases: result.cases,
        cleanup: result.cleanup,
      }),
    );
  });
