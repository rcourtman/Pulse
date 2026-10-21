// Synthetic collected HTTP observations through the real reader/model/tables.
// No backup, guest, thaw, restore, provider or production operation.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const compiled = path.join(root, 'tmp/backup-point-state/static');
const output = path.join(root, 'tmp/backup-inventory-identity');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const report = {
  source_sha: binding.source_sha,
  binding,
  runner_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  passed: false,
  cases: [],
  captures: [],
  unexpected: [],
  cleanup: {},
  limits:
    'Synthetic HTTP through production reader, model, router and tables. No native guest/backup recovery, independent thaw, filesystem liveness, installed acceptance, publication or release proof.',
};
const archive = {
  id: 'fixture-archive',
  instance: 'fixture-pve',
  node: 'pve-a',
  storage: 'local',
  type: 'qemu',
  vmid: 101,
  time: '2026-10-20T10:00:00Z',
  format: 'zst',
  volid: 'local:backup/vzdump-qemu-101-original.vma.zst',
  size: 1024,
};
const snapshot = {
  id: 'fixture-snapshot',
  instance: 'fixture-pve',
  node: 'pve-a',
  type: 'qemu',
  vmid: 101,
  time: '2026-10-20T09:00:00Z',
  name: 'original-snapshot',
};
const pbs = {
  id: 'fixture-pbs',
  instance: 'pbs-east',
  datastore: 'main',
  namespace: '',
  backupType: 'vm',
  vmid: '101',
  backupTime: '2026-10-20T08:00:00Z',
  files: ['index.json.blob'],
  verified: true,
  size: 2048,
};
const emptyPVE = () => ({ backupTasks: [], storageBackups: [], guestSnapshots: [] });
let server, browser, current, sources, heldRetry;
const prefix = (kind) =>
  kind === 'pbs' ? 'pbs' : kind === 'storageBackups' ? 'archive' : 'snapshot';
const sourceFor = (kind) => (kind === 'pbs' ? 'pbs' : 'pve');
const pointFor = (kind) => (kind === 'pbs' ? pbs : kind === 'storageBackups' ? archive : snapshot);

function payloads(kind, records) {
  const pve = emptyPVE();
  if (kind === 'pbs') pve.storageBackups = [archive];
  else pve[kind] = records;
  return { pve: { data: pve }, pbs: { data: { backups: kind === 'pbs' ? records : [pbs] } } };
}
function holdSource(source) {
  let release;
  const promise = new Promise((resolve) => {
    release = resolve;
  });
  return { source, promise, release };
}
async function activate(button, touch) {
  if (touch) await button.tap();
  else {
    await button.focus();
    await button.press('Enter');
  }
}
async function capture(page, name) {
  const file = path.join(output, `${name}.png`);
  await page.screenshot({ path: file, fullPage: false, animations: 'disabled' });
  report.captures.push({ file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}

(async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
    assert.match(binding.source_sha, /^[a-f0-9]{40}$/);
    assert.equal(
      report.playwright,
      JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    for (const [file, digest] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
    assert.equal(
      binding.sources['browser-tests/backup-inventory-identity.cjs'],
      report.runner_sha256,
    );
    for (const file of [
      'proxmoxBackupInventory.ts',
      'proxmoxBackupRecoveryModel.ts',
      'ProxmoxBackupsTable.tsx',
      'ProxmoxCoverageTable.tsx',
      'ProxmoxRecoverableTable.tsx',
    ])
      assert(binding.sources[`src/features/proxmox/${file}`], `Unbound consumer: ${file}`);
    const assets = new Map();
    for (const [file, digest] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(compiled, file));
      assert.equal(hash(bytes), digest, file);
      assets.set('/' + file, bytes);
    }
    server = http.createServer(async (req, res) => {
      const url = new URL(req.url, 'http://fixture.invalid');
      try {
        assert.equal(req.method, 'GET');
        if (url.pathname.startsWith('/api/')) {
          const key =
            url.pathname === '/api/backups/pve'
              ? 'pve'
              : url.pathname === '/api/backups/pbs'
                ? 'pbs'
                : null;
          const payload = key
            ? sources[key]
            : url.pathname === '/api/recovery/postures'
              ? { data: [], policy: {} }
              : null;
          assert(payload, `Unexpected API: ${url.pathname}`);
          current.requests.push({
            method: req.method,
            path: url.pathname,
            sha256: hash(JSON.stringify(payload)),
          });
          if (heldRetry?.source === key) await heldRetry.promise;
          res.writeHead(200, { 'Content-Type': 'application/json' });
          return res.end(JSON.stringify(payload));
        }
        const key = assets.has(url.pathname)
          ? url.pathname
          : '/browser-tests/backup-point-state.html';
        assert(assets.has(key));
        res.setHeader(
          'Content-Type',
          {
            '.js': 'text/javascript',
            '.css': 'text/css',
            '.md': 'text/markdown',
            '.html': 'text/html',
          }[path.extname(key)] || 'application/octet-stream',
        );
        res.end(assets.get(key));
      } catch (error) {
        report.unexpected.push(String(error.stack || error));
        res.writeHead(500);
        res.end('{}');
      }
    });
    await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const spec of [
      { name: 'desktop', engine: chromium, width: 1440, height: 900, touch: false },
      { name: 'phone', engine: webkit, width: 320, height: 844, touch: true },
    ]) {
      browser = await spec.engine.launch(
        spec.touch
          ? { headless: true }
          : { headless: true, channel: 'chromium', args: ['--no-sandbox'] },
      );
      for (const kind of ['pbs', 'storageBackups', 'guestSnapshots']) {
        const source = sourceFor(kind),
          point = pointFor(kind);
        current = {
          engine: spec.name,
          browser_version: browser.version(),
          kind,
          source,
          viewport: { width: spec.width, height: spec.height },
          checks: [],
          requests: [],
          errors: [],
        };
        report.cases.push(current);
        sources = payloads(kind, [point]);
        const context = await browser.newContext({
          viewport: current.viewport,
          isMobile: spec.touch,
          hasTouch: spec.touch,
          locale: 'en-GB',
          timezoneId: 'UTC',
          colorScheme: spec.touch ? 'dark' : 'light',
        });
        await context.route('**/*', (route) => {
          const req = route.request(),
            url = new URL(req.url());
          if (url.origin !== origin || req.method() !== 'GET') {
            report.unexpected.push(`${req.method()} ${url.origin}${url.pathname}`);
            return route.abort();
          }
          return route.continue();
        });
        if (spec.touch)
          await context.addInitScript(() =>
            document.addEventListener('DOMContentLoaded', () =>
              document.documentElement.classList.add('dark'),
            ),
          );
        const page = await context.newPage();
        page.setDefaultTimeout(10000);
        page.on('pageerror', (error) => current.errors.push(error.message));
        await page.clock.setFixedTime(new Date('2026-10-20T12:00:00Z'));
        await page.goto(`${origin}/proxmox/backups/date?keep=identity-proof`, {
          waitUntil: 'domcontentloaded',
        });
        const rows = () => page.locator('[data-proxmox-backup-row="recoverable"]');
        const notice = () =>
          page
            .getByRole('status')
            .filter({ hasText: `${source.toUpperCase()} backup inventory is unavailable` });
        const artifactRow = () =>
          page.locator(
            `[data-proxmox-backup-row="recoverable"][data-proxmox-backup-key="artifact:${prefix(kind)}:${point.id}"]`,
          );
        await expect(rows()).toHaveCount(2);
        await expect(
          page.getByRole('button', { name: 'Refresh backups', exact: true }),
        ).toBeEnabled();
        await activate(artifactRow().getByRole('button', { name: /Backup evidence/ }), spec.touch);
        const dialog = page.getByRole('dialog', { name: 'Backup evidence details' });
        await expect(dialog).toBeVisible();
        await expect(dialog).toContainText(point.id);
        // Close before using the underlying refresh, preserving normal modal access.
        if (spec.touch) await dialog.getByRole('button', { name: 'Close backup evidence' }).tap();
        else await page.keyboard.press('Escape');
        await expect(dialog).toHaveCount(0);
        const changed = {
          ...point,
          instance: 'SYNTHETIC_PRIVATE_SOURCE_SENTINEL',
          vmid: kind === 'pbs' ? '999' : 999,
          verified: false,
        };
        sources = payloads(kind, [point, changed]);
        heldRetry = holdSource(source);
        await activate(
          page.getByRole('button', { name: 'Refresh backups', exact: true }),
          spec.touch,
        );
        await expect(
          page.getByRole('button', { name: 'Reading backups…', exact: true }),
        ).toBeDisabled();
        await activate(artifactRow().getByRole('button', { name: /Backup evidence/ }), spec.touch);
        await expect(dialog).toBeVisible();
        await expect(dialog).toContainText(point.id);
        await expect
          .poll(() => current.requests.filter((r) => r.path === `/api/backups/${source}`).length)
          .toBe(2);
        heldRetry.release();
        heldRetry = null;
        await expect(notice()).toBeVisible();
        await expect(notice()).toContainText('The response format is invalid');
        await expect(rows()).toHaveCount(1);
        await expect(artifactRow()).toHaveCount(0);
        await expect(dialog).toHaveCount(0);
        await expect(page.getByText('No backups yet', { exact: true })).toHaveCount(0);
        await expect(
          page.getByRole('status').filter({ hasText: '1 backup entries read' }),
        ).toBeVisible();
        await expect(page.locator('body')).not.toContainText('SYNTHETIC_PRIVATE_SOURCE_SENTINEL');
        await expect(page.getByRole('alert')).toHaveCount(0);
        assert.equal(new URL(page.url()).searchParams.get('keep'), 'identity-proof');
        await notice().scrollIntoViewIfNeeded();
        await capture(page, `${spec.name}-${kind}-duplicate-withdrawn`);
        current.checks.push(
          'A conflicting duplicate invalidates only its source; no first/last winner, stale artifact, private source text, false-empty state or ErrorBoundary replaces independent evidence.',
        );
        // Retry is held to inspect the pending state, not timed favourable repeats.
        const other = source === 'pbs' ? 'pve' : 'pbs';
        const otherReads = current.requests.filter(
          (r) => r.path === `/api/backups/${other}`,
        ).length;
        const valid =
          kind === 'pbs'
            ? { ...point, verified: false }
            : kind === 'storageBackups'
              ? { ...point, volid: 'local:backup/vzdump-qemu-101-recovered.vma.zst' }
              : { ...point, name: 'recovered-snapshot', description: 'Recovered snapshot record' };
        sources = payloads(kind, [valid]);
        heldRetry = holdSource(source);
        const retry = page.getByRole('button', {
          name: `Retry ${source.toUpperCase()} inventory`,
          exact: true,
        });
        await activate(retry, spec.touch);
        await expect(retry).toBeDisabled();
        await expect(notice()).toBeVisible();
        await expect(rows()).toHaveCount(1);
        await expect
          .poll(() => current.requests.filter((r) => r.path === `/api/backups/${source}`).length)
          .toBe(3);
        heldRetry.release();
        heldRetry = null;
        await expect(notice()).toHaveCount(0);
        await expect(rows()).toHaveCount(2);
        assert.equal(
          current.requests.filter((r) => r.path === `/api/backups/${other}`).length,
          otherReads,
        );
        await activate(artifactRow().getByRole('button', { name: /Backup evidence/ }), spec.touch);
        await expect(dialog).toBeVisible();
        await expect(dialog).toContainText(point.id);
        await expect(dialog).toContainText(
          kind === 'pbs' ? 'Unverified' : kind === 'storageBackups' ? valid.volid : point.id,
        );
        await expect(dialog).not.toContainText('SYNTHETIC_PRIVATE_SOURCE_SENTINEL');
        await capture(page, `${spec.name}-${kind}-recovered-evidence`);
        if (spec.touch) await dialog.getByRole('button', { name: 'Close backup evidence' }).tap();
        else await page.keyboard.press('Escape');
        await expect(artifactRow().getByRole('button', { name: /Backup evidence/ })).toBeFocused();
        current.checks.push(
          'Keyboard/touch isolated retry keeps its warning until accepted data; the same opaque ID recovers its current, correctly scoped evidence and modal focus return.',
        );
        await activate(page.getByRole('link', { name: 'Coverage', exact: true }), spec.touch);
        await expect(page.locator('[data-proxmox-backups-table="coverage"]')).toBeVisible();
        assert.equal(new URL(page.url()).searchParams.get('keep'), 'identity-proof');
        await expect(page.getByText('Backup inventory is incomplete', { exact: true })).toHaveCount(
          0,
        );
        current.checks.push(
          'Coverage remains usable after recovery and view navigation preserves unrelated query state.',
        );
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        assert.deepEqual(current.errors, []);
        assert.equal(current.requests.filter((r) => r.path === `/api/backups/${source}`).length, 3);
        assert.equal(current.requests.filter((r) => r.path === `/api/backups/${other}`).length, 2);
        await context.close();
      }
      await browser.close();
      browser = null;
    }
    assert.deepEqual(report.unexpected, []);
    report.passed = true;
  } finally {
    if (heldRetry) {
      heldRetry.release();
      heldRetry = null;
      report.cleanup.pending_response_released = true;
    }
    if (browser) await browser.close();
    report.cleanup.browsers_closed = true;
    if (server) await new Promise((resolve) => server.close(resolve));
    report.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
  }
  console.log(
    `passed ${report.cases.length} source-local duplicate identity journeys; ${report.captures.length} captures; browser/server closed`,
  );
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
