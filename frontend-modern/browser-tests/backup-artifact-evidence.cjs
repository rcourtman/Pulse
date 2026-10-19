// Actual inventory readers -> recovery model -> By date and expanded Coverage.
// Only synthetic GETs on guest-local loopback; no backup or guest operation.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/coverage-workload-identity/static');
const output = path.join(root, 'tmp/backup-artifact-evidence');
const origin = 'http://127.0.0.1:5322';
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
const rawId =
  'pbs-east/main/pve-1/vm/100/2026-10-19T11:00:00Z/' +
  'long-provider-record-'.repeat(4) +
  '<img src="/unexpected" onerror="unsafe()"><script>unsafe()</script>';
const guest = {
  id: 'east-100',
  name: 'database',
  type: 'vm',
  platformId: 'east',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'running',
  proxmox: { vmid: 100, nodeName: 'pve-1', instance: 'east' },
};
const point = (id, extra = {}) => ({
  id,
  instance: 'pbs-east',
  datastore: 'main',
  namespace: 'pve-1',
  backupType: 'vm',
  vmid: '100',
  backupTime: '2026-10-19T11:00:00Z',
  size: 1024,
  files: ['index.json.blob'],
  protected: false,
  ...extra,
});
const points = [
  point(rawId, { verified: true, protected: true }),
  point('unverified', { verified: false, protected: true }),
  point('unknown'),
  point('running', {
    verified: true,
    inProgress: true,
    writeActivityObserved: true,
    writeActive: true,
  }),
  point('failed', {
    verified: true,
    inProgress: true,
    writeActivityObserved: true,
    writeActive: false,
  }),
];
const pve = {
  data: {
    backupTasks: [],
    storageBackups: [
      {
        id: 'archive',
        instance: 'east',
        node: 'pve-1',
        type: 'vm',
        vmid: 100,
        time: '2026-10-19T10:00:00Z',
        storage: 'local',
        protected: false,
        volid: 'local:backup/database.vma.zst',
      },
    ],
    guestSnapshots: [
      {
        id: 'snapshot',
        instance: 'east',
        node: 'pve-1',
        type: 'vm',
        vmid: 100,
        time: '2026-10-19T09:00:00Z',
        name: 'before-update',
      },
    ],
  },
};
const expectedText = {
  Verified: 'does not prove a restore or guest thaw',
  Unverified: 'does not prove that the verification job failed',
  Unknown: 'no verification result reported',
  Running: 'not a completed recovery point',
  Failed: 'not a completed recovery point',
  Snapshot: 'not a separate backup',
  'Backup file': 'no verification result reported',
};
let server,
  browser,
  replacement = false;
const apiReads = () =>
  result.responses.filter((response) => response.path.startsWith('/api/')).length;
async function capture(page, name) {
  const geometry = await page.evaluate(() => {
    const clipped = [];
    for (const node of document.querySelectorAll(
      '[data-backup-evidence-details] dd, [data-backup-evidence-details] p',
    )) {
      const box = node.getBoundingClientRect(),
        range = document.createRange();
      range.selectNodeContents(node);
      if (
        node.scrollWidth > node.clientWidth + 1 ||
        [...range.getClientRects()].some((r) => r.left < box.left - 1 || r.right > box.right + 1)
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
    'ProxmoxRecoverableTable.tsx',
    'ProxmoxCoverageTable.tsx',
    'proxmoxBackupsTableShared.tsx',
    'proxmoxBackupRecoveryModel.ts',
    'proxmoxBackupInventory.ts',
  ])
    assert(binding.sources['src/features/proxmox/' + file]);
  assert.equal(
    binding.sources['browser-tests/backup-artifact-evidence.cjs'],
    result.protocol_sha256,
  );
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
    if (url.pathname === '/fixture/workloads') return json([guest]);
    if (url.pathname === '/api/backups/pve') return json(pve);
    if (url.pathname === '/api/backups/pbs')
      return json({
        data: {
          backups: replacement
            ? points.map((p) =>
                p.id === rawId ? { ...p, verified: undefined, protected: false } : p,
              )
            : points,
        },
      });
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
            explanation: 'Collected inventory does not establish guest recovery.',
            evaluatedAt: '2026-10-19T12:00:00Z',
          },
        ],
      });
    const file = url.pathname.startsWith('/proxmox/backups/')
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
    server.listen(5322, '127.0.0.1', resolve);
  });
  for (const [name, engine, width] of [
    ['desktop', chromium, 1365],
    ['narrow', webkit, 320],
  ]) {
    replacement = false;
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
    if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
    const evidenceButton = (scope, label, protectedFlag = false) =>
      scope.getByRole('button', {
        name: `Backup evidence for database: ${label}${protectedFlag ? ', prune protected' : ''}`,
        exact: true,
      });
    const verified = evidenceButton(page, 'Verified', true);
    await verified.waitFor();
    await verified.scrollIntoViewIfNeeded();
    const height = await verified.evaluate((el) => el.getBoundingClientRect().height);
    if (phone) assert(height >= 44, 'phone target is at least 44px');
    await capture(page, name + '-date-affordance');
    const checks = [];
    async function disclose(button, label, protectedFlag, captureName, keyboardKey = 'Enter') {
      const readsBefore = apiReads();
      if (phone) await button.tap();
      else {
        await button.focus();
        await page.keyboard.press('Shift+Tab');
        await page.keyboard.press('Tab');
        assert(await button.evaluate((el) => el === document.activeElement));
        await page.keyboard.press(keyboardKey);
      }
      const dialog = page.getByRole('dialog', { name: 'Backup evidence details' });
      await dialog.waitFor();
      assert.equal(await button.getAttribute('aria-expanded'), 'true');
      assert.match(
        await dialog.locator('[data-backup-verification-evidence]').innerText(),
        new RegExp(expectedText[label]),
      );
      assert.match(
        await dialog.innerText(),
        /do not establish guest responsiveness or a successful restore/,
      );
      assert.equal(
        await dialog.locator('[data-backup-prune-evidence]').count(),
        protectedFlag ? 1 : 0,
      );
      assert.equal(await dialog.locator('img, script').count(), 0);
      const close = dialog.getByRole('button', { name: 'Close backup evidence' });
      await close.waitFor();
      await page.waitForFunction(
        () => document.activeElement?.getAttribute('aria-label') === 'Close backup evidence',
      );
      await page.keyboard.press('Tab');
      assert(await close.evaluate((el) => el === document.activeElement));
      await page.keyboard.press('Shift+Tab');
      assert(await close.evaluate((el) => el === document.activeElement));
      assert(await page.locator('#root').evaluate((el) => el.hasAttribute('inert')));
      if (captureName) await capture(page, name + '-' + captureName);
      if (phone) await close.tap();
      else await page.keyboard.press('Escape');
      await dialog.waitFor({ state: 'detached' });
      assert(await button.evaluate((el) => el === document.activeElement));
      assert.equal(await button.getAttribute('aria-expanded'), 'false');
      assert.equal(await page.locator('#root').getAttribute('inert'), null);
      assert.equal(apiReads(), readsBefore, 'evidence disclosure issues no API requests');
      checks.push(
        `${label}: native activation, explanation, prune=${protectedFlag}, focus trap/return, no request`,
      );
    }
    // Inspect full long/untrusted identity in the actual consuming dialog.
    if (phone) await verified.tap();
    else {
      await verified.focus();
      await page.keyboard.press('Enter');
    }
    const dialog = page.getByRole('dialog');
    await dialog.waitFor();
    assert.equal(await dialog.locator('dd').last().textContent(), rawId);
    assert.match(await dialog.innerText(), /pbs-east \/ main \/ pve-1/);
    assert.equal(await dialog.locator('img, script').count(), 0);
    const close = dialog.getByRole('button', { name: 'Close backup evidence' });
    if (phone) await close.tap();
    else await page.keyboard.press('Escape');
    await dialog.waitFor({ state: 'detached' });
    await disclose(verified, 'Verified', true, 'date-verified');
    for (const [label, prune] of [
      ['Unverified', true],
      ['Unknown', false],
      ['Running', false],
      ['Failed', false],
      ['Backup file', false],
      ['Snapshot', false],
    ])
      await disclose(
        evidenceButton(page, label, prune),
        label,
        prune,
        label === 'Snapshot' ? 'date-snapshot' : null,
        'Space',
      );

    // Replace the previously successful/prune-protected record via the ordinary reader.
    replacement = true;
    const read = page.waitForResponse(
      (response) => new URL(response.url()).pathname === '/api/backups/pbs',
    );
    const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
    if (phone) await refresh.tap();
    else {
      await refresh.focus();
      await page.keyboard.press('Enter');
    }
    await read;
    await verified.waitFor({ state: 'detached' });
    const replaced = page
      .locator('[data-proxmox-backup-row="recoverable"]')
      .filter({
        has: page.locator('[data-backup-artifact-state]', { hasText: 'Unknown' }),
      })
      .filter({ has: page.locator('[data-backup-artifact-details]', { hasText: '1 PBS file' }) });
    // Two unknown points share a workload but keep separate record context.
    const unknownButtons = evidenceButton(page, 'Unknown');
    assert.equal(await unknownButtons.count(), 2);
    let replacementButton;
    for (const button of await unknownButtons.all()) {
      if (phone) await button.tap();
      else {
        await button.focus();
        await page.keyboard.press('Enter');
      }
      const current = page.getByRole('dialog');
      if ((await current.locator('dd').last().textContent()) === rawId) {
        assert.equal(
          await current.getByRole('heading', { name: 'Verified', exact: true }).count(),
          0,
        );
        assert.equal(await current.locator('[data-backup-prune-evidence]').count(), 0);
        replacementButton = button;
        await capture(page, name + '-date-replaced-unknown');
      }
      if (phone) await current.getByRole('button', { name: 'Close backup evidence' }).tap();
      else await page.keyboard.press('Escape');
    }
    assert(replacementButton);
    assert.equal(await replaced.count(), 2);
    checks.push(
      'ordinary refresh withdraws Verified/prune protection; matching record stays literal and unknown',
    );

    // Same shared action inside the existing nested Coverage evidence table.
    const coverage = page.getByRole('link', { name: /Coverage/ });
    if (phone) await coverage.tap();
    else {
      await coverage.focus();
      await page.keyboard.press('Enter');
    }
    const row = page.locator('[data-proxmox-coverage-key="resource:east-100"]');
    await row.waitFor();
    if (phone) await row.locator('td').first().tap();
    else {
      await row.getByRole('button', { expanded: false }).focus();
      await page.keyboard.press('Enter');
    }
    const expanded = row.locator('xpath=following-sibling::tr[1]');
    await disclose(evidenceButton(expanded, 'Failed'), 'Failed', false, 'coverage-failed');
    assert.equal(await row.getByRole('button', { expanded: true }).count(), 1);
    checks.push(
      'Coverage nested-table disclosure does not collapse the workload or alter its unknown posture',
    );
    assert.equal(result.errors.length, 0);
    assert.equal(result.unexpected.length, 0);
    result.cases.push({ name, width, browser: browser.version(), checks });
    await context.close();
    await browser.close();
    browser = undefined;
    result.cleanup[name] = true;
  }
})()
  .catch((error) => {
    result.failure = String(error.stack || error);
    process.exitCode = 1;
  })
  .finally(async () => {
    if (browser) await browser.close().catch(() => {});
    if (server) await new Promise((resolve) => server.close(resolve));
    result.cleanup.server = !server?.listening;
    fs.mkdirSync(output, { recursive: true });
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        cases: result.cases,
        failure: result.failure,
        errors: result.errors,
        unexpected: result.unexpected,
        cleanup: result.cleanup,
        captures: result.captures,
      }),
    );
  });
