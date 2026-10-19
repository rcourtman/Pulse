// Real HTTP inventory reader -> recovery model -> By date chart and shipped CSS.
// Synthetic, GET-only observations; no backup, guest or provider operation.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/coverage-workload-identity/static');
const output = path.join(root, 'tmp/backup-activity-volume');
const origin = 'http://127.0.0.1:5321';
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
const guest = {
  id: 'east-100',
  name: 'recovery-vm',
  type: 'vm',
  platformId: 'east',
  sourceType: 'api',
  status: 'running',
  platformScopes: ['proxmox-pve'],
  proxmox: { vmid: 100, node: 'pve-1', instance: 'east' },
};
const pbs = (extra = {}) => ({
  id: 'pbs-point',
  instance: 'pbs',
  datastore: 'main',
  backupType: 'vm',
  vmid: '100',
  backupTime: today,
  files: ['index.json.blob'],
  verified: true,
  size: 0,
  ...extra,
});
const archive = (extra = {}) => ({
  id: 'pve-file',
  instance: 'east',
  node: 'pve-1',
  vmid: 100,
  type: 'qemu',
  time: today,
  storage: 'local',
  volid: 'local:backup/vzdump-qemu-100.tar.zst',
  ...extra,
});
const snapshot = (extra = {}) => ({
  id: 'guest-snapshot',
  instance: 'east',
  node: 'pve-1',
  vmid: 100,
  type: 'qemu',
  time: today,
  name: 'pre-upgrade',
  ...extra,
});
let state, server, browser;
function observations() {
  if (state === 'missing')
    return { backups: [pbs()], archives: [archive()], snapshots: [snapshot()] };
  if (state === 'partial')
    return {
      backups: [
        pbs(),
        pbs({ id: 'running', inProgress: true }),
        pbs({ id: 'future', backupTime: '2026-10-20T10:00:00Z' }),
        pbs({ id: 'undated', backupTime: '' }),
        pbs({ id: 'old', backupTime: '2020-01-01T10:00:00Z' }),
      ],
      archives: [archive({ size: 1024 })],
      snapshots: [snapshot({ time: yesterday })],
    };
  if (state === 'measured') return { backups: [pbs({ size: 2048 })], archives: [], snapshots: [] };
  return {
    backups: [pbs({ backupTime: '2026-08-30T10:00:00Z', size: 4096 })],
    archives: [],
    snapshots: [],
  };
}
async function capture(page, name) {
  const geometry = await page.evaluate(() => {
    const clipped = [];
    for (const node of document.querySelectorAll(
      '[data-backup-volume-evidence], [aria-label="Activity metric"], [aria-label="Activity range"]',
    )) {
      const box = node.getBoundingClientRect(),
        range = document.createRange();
      range.selectNodeContents(node);
      if (
        node.scrollWidth > node.clientWidth + 1 ||
        [...range.getClientRects()].some(
          (rect) => rect.left < box.left - 1 || rect.right > box.right + 1,
        )
      )
        clipped.push(node.textContent);
    }
    return { clipped, overflow: document.documentElement.scrollWidth > innerWidth };
  });
  result.geometry.push({ name, ...geometry });
  assert.deepEqual(geometry, { clipped: [], overflow: false });
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
  for (const [file, digest] of Object.entries(binding.sources))
    assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
  for (const file of [
    'ProxmoxBackupsTable.tsx',
    'BackupActivityChart.tsx',
    'proxmoxBackupActivityPresentation.ts',
    'proxmoxBackupInventory.ts',
    'proxmoxBackupRecoveryModel.ts',
  ])
    assert(binding.sources['src/features/proxmox/' + file]);
  assert.equal(binding.sources['browser-tests/backup-activity-volume.cjs'], result.protocol_sha256);
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
      result.responses.push({ method: req.method, path: url.pathname, state, sha256: hash(bytes) });
      res.setHeader('Content-Type', 'application/json');
      res.end(bytes);
    };
    if (req.method !== 'GET') {
      result.unexpected.push('non-GET');
      res.statusCode = 405;
      return res.end();
    }
    if (url.pathname === '/fixture/workloads') return json([guest]);
    const data = observations();
    if (url.pathname === '/api/backups/pve')
      return json({
        data: {
          backupTasks: [],
          storageBackups: data.archives,
          guestSnapshots: data.snapshots,
        },
      });
    if (url.pathname === '/api/backups/pbs') return json({ data: { backups: data.backups } });
    if (url.pathname === '/api/recovery/postures')
      return json({
        data: [
          {
            subjectResourceId: guest.id,
            state: 'unknown',
            freshness: 'unknown',
            verification: 'unknown',
            coverage: 'unknown',
            providerStates: [],
            repositoryResourceIds: [],
            evidenceIds: [],
            explanation: 'Synthetic protection posture is separate from activity.',
            evaluatedAt: today,
          },
        ],
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
    server.listen(5321, '127.0.0.1', resolve);
  });
  for (const [name, engine, width] of [
    ['desktop', chromium, 1365],
    ['narrow', webkit, 320],
  ]) {
    state = 'missing';
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
    await page.goto(origin + '/proxmox/backups/date');
    if (width === 320) await page.evaluate(() => document.documentElement.classList.add('dark'));
    const button = (label) => page.getByRole('button', { name: label, exact: true });
    await page.getByRole('button', { name: /: 3 backups$/ }).waitFor();
    await capture(page, name + '-count');
    if (width === 320) await button('Volume').tap();
    else {
      for (
        let i = 0;
        i < 20 && !(await button('Volume').evaluate((el) => el === document.activeElement));
        i++
      )
        await page.keyboard.press('Tab');
      assert(
        await button('Volume').evaluate((el) => el === document.activeElement),
        'native Tab reaches Volume',
      );
      await page.keyboard.press('Enter');
      assert(
        await button('Volume').evaluate((el) => el === document.activeElement),
        'metric retains focus',
      );
    }
    await page
      .getByText(
        'Completed restore points exist in this window, but no positive sizes were reported. Use Count to view activity.',
        { exact: true },
      )
      .waitFor();
    assert.equal(await button('Volume').getAttribute('aria-pressed'), 'true');
    assert.equal(await button('Count').count(), 1);
    assert.equal(await page.getByText('0 B', { exact: true }).count(), 0);
    assert.match(
      await page.locator('[data-backup-volume-evidence]').innerText(),
      /3 completed restore points/,
    );
    await capture(page, name + '-missing-volume');
    if (width === 320) await button('Count').tap();
    else {
      await page.keyboard.press('Shift+Tab');
      await page.keyboard.press('Space');
    }
    await page.getByRole('button', { name: /: 3 backups$/ }).waitFor();
    if (width === 320) await button('Volume').tap();
    else {
      await page.keyboard.press('Tab');
      await page.keyboard.press('Enter');
    }
    state = 'partial';
    await button('Refresh backups').click();
    const partial = page.getByRole('button', {
      name: /: 1.00 KB reported; 1 backup without a positive size$/,
    });
    await partial.waitFor();
    const missingDay = page.getByRole('button', {
      name: /: Size unavailable; 1 backup without a positive size$/,
    });
    await missingDay.waitFor();
    assert.match(
      await page.locator('[data-backup-volume-evidence]').innerText(),
      /2 completed restore points/,
    );
    assert.equal(await button('Volume').getAttribute('aria-pressed'), 'true');
    if (width !== 320) {
      await missingDay.focus();
      await page
        .getByText('Size unavailable; 1 backup without a positive size', { exact: true })
        .waitFor();
    }
    await capture(page, name + '-partial-volume');
    await button('Volume').focus(); // leave the tooltip before replacing inventory
    state = 'measured';
    await button('Refresh backups').click();
    await page.getByRole('button', { name: /: 2.00 KB$/ }).waitFor();
    assert.doesNotMatch(
      await page.locator('[data-backup-volume-evidence]').innerText(),
      /no positive size reported/,
    );
    assert.equal(await page.getByRole('button', { name: /without a positive size/ }).count(), 0);
    state = 'old';
    await button('Refresh backups').click();
    await page
      .getByText('No completed restore-point activity in the selected 30-day window.', {
        exact: true,
      })
      .waitFor();
    await capture(page, name + '-empty-window');
    if (width === 320) await button('90d').tap();
    else await button('90d').click();
    const oldDay = page.getByRole('button', { name: /: 4.00 KB$/ });
    await oldDay.waitFor();
    if (width === 320) await oldDay.tap();
    else {
      await oldDay.focus();
      await page.keyboard.press('Enter');
    }
    assert.match(page.url(), /day=2026-08-30/);
    await button('Clear date filter').click();
    assert(!page.url().includes('day='));
    await button('7d').click();
    await page
      .getByText('No completed restore-point activity in the selected 7-day window.', {
        exact: true,
      })
      .waitFor();
    await button('Count').click();
    assert.equal(await button('Count').getAttribute('aria-pressed'), 'true');
    assert.equal(await button('Volume').count(), 1);
    result.cases.push({
      name,
      width,
      browser: browser.version(),
      checks: [
        'native metric activation and return',
        'absent/zero sizes across PBS/PVE/guest snapshots',
        'partial and unknown daily size labels',
        'incomplete/future/undated/old exclusion',
        'ordinary measured replacement',
        'empty-window range recovery',
        'day filter and clear',
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
