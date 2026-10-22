// Source-bound loopback HTTP through production reader, model, tables and filters.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace',
  output = path.join(root, 'tmp/backup-node-scope'),
  compiled = path.join(output, 'static');
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
    'Synthetic collected HTTP observations, not native recovery, guest liveness, #2681 resolution, publication or release proof.',
};
const point = (id, instance, node) => ({
  id,
  instance,
  node,
  storage: 'local',
  type: 'qemu',
  vmid: 100,
  time: '2026-10-21T00:00:00Z',
  format: 'zst',
  volid: `local:backup/${id}.vma.zst`,
  size: 1024,
});
const initialPoints = [
  point('east-file', 'east', 'pve-a'),
  point('west-file', 'west', 'pve-a'),
  point('east-other-file', 'east', 'pve-b'),
  point('case-file', 'East', 'pve-a'),
];
let points, server, browser, current;
async function activate(locator, touch) {
  if (touch) await locator.tap();
  else {
    await locator.focus();
    await locator.press('Enter');
  }
}
async function capture(page, name, locator) {
  if (locator) await locator.scrollIntoViewIfNeeded();
  const file = path.join(output, `${name}.png`);
  await page.screenshot({ path: file, animations: 'disabled' });
  report.captures.push({ file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
  assert(
    await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
    'document horizontal overflow',
  );
}
(async () => {
  try {
    assert.equal(
      report.playwright,
      JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    for (const [file, digest] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
    assert.equal(binding.sources['browser-tests/backup-node-scope.cjs'], report.runner_sha256);
    for (const file of [
      'proxmoxBackupNodeScope.ts',
      'ProxmoxBackupsTable.tsx',
      'proxmoxBackupRecoveryModel.ts',
      'ProxmoxCoverageTable.tsx',
      'ProxmoxRecoverableTable.tsx',
    ])
      assert(binding.sources[`src/features/proxmox/${file}`], `unbound ${file}`);
    const assets = new Map();
    for (const [file, digest] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(compiled, file));
      assert.equal(hash(bytes), digest, file);
      assets.set('/' + file, bytes);
    }
    server = http.createServer((req, res) => {
      const url = new URL(req.url, 'http://fixture.invalid');
      try {
        assert.equal(req.method, 'GET');
        if (url.pathname.startsWith('/api/')) {
          let payload;
          if (url.pathname === '/api/backups/pve')
            payload = { data: { storageBackups: points, guestSnapshots: [], backupTasks: [] } };
          else if (url.pathname === '/api/backups/pbs') payload = { data: { backups: [] } };
          else if (url.pathname === '/api/recovery/postures')
            payload = {
              data: [],
              policy: {
                freshnessWindowSeconds: 604800,
                verificationWindowSeconds: 604800,
                requireVerification: true,
              },
            };
          assert(payload, `unexpected API ${url.pathname}`);
          current.requests.push({
            method: req.method,
            path: url.pathname,
            payload_sha256: hash(JSON.stringify(payload)),
          });
          res.writeHead(200, { 'Content-Type': 'application/json' });
          return res.end(JSON.stringify(payload));
        }
        const key = assets.has(url.pathname)
          ? url.pathname
          : '/browser-tests/backup-node-scope.html';
        assert(assets.has(key));
        res.setHeader(
          'Content-Type',
          { '.js': 'text/javascript', '.css': 'text/css', '.html': 'text/html' }[
            path.extname(key)
          ] || 'application/octet-stream',
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
      current = {
        engine: spec.name,
        browser_version: browser.version(),
        viewport: { width: spec.width, height: spec.height },
        requests: [],
        checks: [],
        errors: [],
      };
      report.cases.push(current);
      points = [...initialPoints];
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
      await page.clock.setFixedTime(new Date('2026-10-22T00:00:00Z'));
      const rows = (kind) => page.locator(`[data-proxmox-backup-row="${kind}"]`);
      await page.goto(`${origin}/proxmox/backups/coverage?node=Shared&keep=node-proof`, {
        waitUntil: 'domcontentloaded',
      });
      const warning = page
        .getByRole('status')
        .filter({ hasText: 'The saved node matches more than one source.' });
      await expect(warning).toBeVisible();
      await expect(rows('coverage')).toHaveCount(0);
      await capture(page, `${spec.name}-ambiguous`, warning);
      const disclosure = page.getByRole('button', { name: /^Filters/ });
      if (spec.touch && (await disclosure.count())) await activate(disclosure, spec.touch);
      await activate(page.getByRole('button', { name: 'Remove Node filter' }), spec.touch);
      await expect(rows('coverage')).toHaveCount(5);
      await page
        .getByRole('combobox', { name: 'Filter', exact: true })
        .selectOption({ label: 'Node: east / Shared (pve-a)' });
      await expect(rows('coverage')).toHaveCount(2);
      await expect(rows('coverage').filter({ hasText: 'east-no-backup' })).toHaveCount(1);
      const chip = page.getByRole('button', { name: 'Node: east / Shared (pve-a)', exact: true });
      await expect(chip).toBeVisible();
      await activate(chip, spec.touch);
      const choices = page.getByRole('listbox', { name: 'Node', exact: true });
      for (const label of [
        'east / Shared (pve-a)',
        'east / Shared (pve-b)',
        'west / Shared (pve-a)',
        'East / Shared (pve-a)',
      ])
        await expect(choices.getByRole('option', { name: label, exact: true })).toBeVisible();
      await capture(page, `${spec.name}-choices`, choices);
      if (spec.touch)
        await choices.getByRole('option', { name: 'east / Shared (pve-a)', exact: true }).tap();
      else {
        await activate(
          choices.getByRole('option', { name: 'east / Shared (pve-a)', exact: true }),
          false,
        );
      }
      await expect(rows('coverage')).toHaveCount(2);
      await capture(page, `${spec.name}-coverage`, rows('coverage').first());
      await activate(page.getByRole('link', { name: 'By date', exact: true }), spec.touch);
      await expect(rows('recoverable')).toHaveCount(1);
      await expect(rows('recoverable')).toContainText('east-vm');
      await activate(
        rows('recoverable').getByRole('button', { name: /Backup evidence/ }),
        spec.touch,
      );
      const dialog = page.getByRole('dialog', { name: 'Backup evidence details' });
      await expect(dialog).toContainText('east-file');
      await expect(dialog).not.toContainText('west-file');
      await capture(page, `${spec.name}-evidence`, dialog);
      await activate(dialog.getByRole('button', { name: 'Close backup evidence' }), spec.touch);
      await page.evaluate(() =>
        document.dispatchEvent(new CustomEvent('fixture-node-observation', { detail: 'rename' })),
      );
      if (
        spec.touch &&
        (await disclosure.count()) &&
        !(await page.getByRole('button', { name: /^Node:/ }).isVisible())
      )
        await activate(disclosure, spec.touch);
      const renamed = page.getByRole('button', {
        name: 'Node: east / Renamed (pve-a)',
        exact: true,
      });
      await expect(renamed).toBeVisible();
      await expect(rows('recoverable')).toHaveCount(1);
      await capture(page, `${spec.name}-renamed`, renamed);
      const selected = new URL(page.url()).searchParams;
      assert.equal(selected.get('node'), 'backup-node-v1:["east","pve-a"]');
      assert.equal(selected.get('keep'), 'node-proof');
      points = initialPoints.filter((value) => value.instance !== 'east' || value.node !== 'pve-a');
      await page.evaluate(() =>
        document.dispatchEvent(new CustomEvent('fixture-node-observation', { detail: 'remove' })),
      );
      await activate(
        page.getByRole('button', { name: 'Refresh backups', exact: true }),
        spec.touch,
      );
      const missing = page
        .getByRole('status')
        .filter({ hasText: 'The saved node is not in the current inventory.' });
      await expect(missing).toBeVisible();
      await expect(rows('recoverable')).toHaveCount(0);
      await capture(page, `${spec.name}-missing`, missing);
      if (
        spec.touch &&
        (await disclosure.count()) &&
        !(await page.getByRole('button', { name: /^Node:/ }).isVisible())
      )
        await activate(disclosure, spec.touch);
      await activate(page.getByRole('button', { name: 'Remove Node filter' }), spec.touch);
      await expect(rows('recoverable')).toHaveCount(3);
      assert.equal(new URL(page.url()).searchParams.get('keep'), 'node-proof');
      current.checks = [
        'ambiguous old URL selects none with visible recovery',
        'exact connection/native scope isolates two workloads including one without a backup',
        'same-name same-connection and case-distinct connection choices remain distinct',
        'keyboard/touch choices, route and query persistence',
        'actual evidence dialog identifies the selected point',
        'rename preserves exact identity',
        'vanished selection does not widen, clear recovers remaining points',
      ];
      assert.deepEqual(current.errors, []);
      await context.close();
      await browser.close();
      browser = null;
    }
    assert.deepEqual(report.unexpected, []);
    report.passed = true;
  } catch (error) {
    report.error = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    report.cleanup.browser_closed = true;
    if (server) await new Promise((resolve) => server.close(resolve));
    report.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(
      JSON.stringify({
        passed: report.passed,
        cases: report.cases.length,
        captures: report.captures.length,
        error: report.error || null,
      }),
    );
  }
})();
