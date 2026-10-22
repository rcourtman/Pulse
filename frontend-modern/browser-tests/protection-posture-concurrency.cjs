// Held synthetic HTTP -> production posture owner -> Coverage and evidence.
// This never contacts a provider, starts collection or performs a guest operation.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const compiled = path.join(root, 'tmp/posture-concurrency/static');
const output = path.join(root, 'tmp/posture-concurrency/browser');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const policy = {
  freshnessWindowSeconds: 604800,
  verificationWindowSeconds: 604800,
  requireVerification: true,
};
const posture = (ids, state = 'protected') => ({
  data: ids.map((subjectResourceId) => ({
    subjectResourceId,
    state,
    freshness: 'current',
    verification: 'verified',
    coverage: 'complete',
    explanation:
      state === 'protected' ? 'Synthetic current verified backup.' : 'Synthetic attention result.',
    evaluatedAt: '2026-10-22T00:00:00Z',
    providerStates: [],
    repositoryResourceIds: [],
    evidenceIds: [],
  })),
  policy,
});
const inventory = {
  pve: { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } },
  pbs: {
    data: {
      backups: [
        {
          id: 'synthetic-pbs/main/vm/100',
          instance: 'synthetic-pbs',
          datastore: 'main',
          vmid: '100',
          backupType: 'vm',
          backupTime: '2026-10-21T23:00:00Z',
          files: ['index.json.blob'],
          verified: true,
        },
      ],
    },
  },
};
const report = {
  binding,
  runner_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  passed: false,
  cases: [],
  captures: [],
  errors: [],
  cleanup: {},
  limits:
    'Synthetic per-evaluation read bound, not a global/multi-tab rate limit, installed load relief, native recovery or release availability.',
};
let server, browser, current, wave;
const openResponses = new Set();
const beginWave = (name) => {
  wave = { name, entries: [], active: 0, peak: 0 };
  current.waves.push(wave);
  return wave;
};
const send = (res, value, status = 200) => {
  if (res.destroyed) return;
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(value));
};
const release = (entry, state = 'protected') => {
  if (entry.released) return;
  entry.released = true;
  send(entry.res, posture(entry.ids, state));
};
const activate = async (page, locator, touch) => {
  if (touch) await locator.tap();
  else {
    await locator.focus();
    await page.keyboard.press('Enter');
  }
};
(async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
    assert.equal(
      report.playwright,
      JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    for (const [file, digest] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
    assert.equal(
      binding.sources['browser-tests/protection-posture-concurrency.cjs'],
      report.runner_sha256,
    );
    assert(binding.sources['src/hooks/useProtectionPostures.ts']);
    assert(binding.sources['src/features/proxmox/ProxmoxBackupsTable.tsx']);
    assert(binding.sources['src/features/proxmox/ProxmoxCoverageTable.tsx']);
    const assets = new Map();
    for (const [file, digest] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(compiled, file));
      assert.equal(hash(bytes), digest, file);
      assets.set('/' + file, bytes);
    }
    server = http.createServer((req, res) => {
      try {
        const url = new URL(req.url, 'http://fixture.invalid');
        assert.equal(req.method, 'GET', 'Read-only fixture');
        if (url.pathname === '/api/recovery/postures') {
          const ids = url.searchParams.getAll('resourceId');
          assert(ids.length >= 1 && ids.length <= 200);
          assert.equal(url.searchParams.get('limit'), '200');
          const owner = wave;
          const entry = { ids, res, released: false, closed: false };
          owner.entries.push(entry);
          owner.active += 1;
          owner.peak = Math.max(owner.peak, owner.active);
          openResponses.add(res);
          res.on('close', () => {
            entry.closed = true;
            owner.active -= 1;
            openResponses.delete(res);
          });
          return;
        }
        if (url.pathname.startsWith('/api/')) {
          current.inventoryRequests.push(url.pathname);
          if (url.pathname === '/api/backups/pve') return send(res, inventory.pve);
          if (url.pathname === '/api/backups/pbs') return send(res, inventory.pbs);
          if (url.pathname === '/api/license/runtime-capabilities')
            return send(res, {
              capabilities: [],
              limits: [],
              max_history_days: 7,
              runtime: { build: 'community' },
            });
          throw new Error(`Unexpected API: ${url.pathname}`);
        }
        const file = assets.has(url.pathname)
          ? url.pathname
          : '/browser-tests/protection-posture-concurrency.html';
        assert(assets.has(file));
        res.setHeader(
          'Content-Type',
          { '.js': 'text/javascript', '.css': 'text/css', '.html': 'text/html' }[
            path.extname(file)
          ] || 'application/octet-stream',
        );
        res.end(assets.get(file));
      } catch (error) {
        report.errors.push(String(error.stack || error));
        send(res, {}, 500);
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
        version: browser.version(),
        viewport: { width: spec.width, height: spec.height },
        waves: [],
        inventoryRequests: [],
        checks: [],
      };
      report.cases.push(current);
      const context = await browser.newContext({
        viewport: current.viewport,
        isMobile: spec.touch,
        hasTouch: spec.touch,
        locale: 'en-GB',
        timezoneId: 'UTC',
        colorScheme: spec.touch ? 'dark' : 'light',
      });
      await context.route('**/*', (route) =>
        new URL(route.request().url()).origin === origin ? route.continue() : route.abort(),
      );
      const page = await context.newPage();
      page.on('pageerror', (error) => report.errors.push(`${spec.name}: ${error.message}`));
      const now = new Date('2026-10-22T00:00:00Z');
      await page.clock.install({ time: now });
      await page.clock.pauseAt(now);
      const initial = beginWave('initial');
      await page.goto(`${origin}/proxmox/backups/coverage?q=edge-backup-reader&retained=1`, {
        waitUntil: 'domcontentloaded',
      });
      if (spec.touch) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const row = () => page.locator('[data-proxmox-backup-row="coverage"]').first();
      const expectPosture = (label) =>
        expect(row().getByText(label, { exact: true })).toBeVisible();
      const capture = async (state) => {
        await page.evaluate(() => document.fonts.ready);
        assert(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          'No document horizontal overflow',
        );
        const file = `${spec.name}-${state}.png`;
        await page.screenshot({
          path: path.join(output, file),
          fullPage: true,
          animations: 'disabled',
        });
        report.captures.push({
          file,
          state,
          viewport: current.viewport,
          sha256: hash(fs.readFileSync(path.join(output, file))),
        });
      };
      const awaitFour = async (owner) => {
        await expect.poll(() => owner.entries.length).toBe(4);
        assert.equal(owner.peak, 4);
      };
      const completeExceptFirst = async (owner, state) => {
        while (owner.entries.length < 11) {
          const count = owner.entries.length;
          owner.entries.slice(1).forEach((entry) => release(entry, state));
          await expect.poll(() => owner.entries.length).toBeGreaterThan(count);
          assert(owner.peak <= 4);
        }
        owner.entries.slice(1).forEach((entry) => release(entry, state));
        await expect.poll(() => owner.active).toBe(1);
        assert.equal(new Set(owner.entries.flatMap((entry) => entry.ids)).size, 2001);
        assert.equal(owner.entries.flatMap((entry) => entry.ids).length, 2001);
      };
      await awaitFour(initial);
      await expectPosture(spec.touch ? 'Check' : 'Checking');
      // Completing a later batch refills exactly one slot, not the whole queue.
      release(initial.entries[3]);
      await expect.poll(() => initial.entries.length).toBe(5);
      await completeExceptFirst(initial, 'protected');
      await expectPosture(spec.touch ? 'Check' : 'Checking');
      await capture('queued-no-partial-verdict');
      release(initial.entries[0]);
      await expectPosture(spec.touch ? 'Prot.' : 'Protected');
      await capture('complete-protected');
      current.checks.push(
        'All 2,001 subjects join atomically from 11 exact-once batches; active HTTP peak 4, including out-of-order slot refill.',
      );
      const failed = beginWave('malformed-poll');
      await page.clock.fastForward(30_000);
      await awaitFour(failed);
      await expectPosture(spec.touch ? 'Prot.' : 'Protected');
      failed.entries[1].released = true;
      send(failed.entries[1].res, { private: 'SYNTHETIC_PRIVATE_BODY' });
      const notice = page
        .getByRole('status')
        .filter({ hasText: 'Protection posture is unavailable' });
      await expect(notice).toBeVisible();
      await expectPosture('Unknown');
      await expect.poll(() => failed.active).toBe(0);
      assert.equal(failed.entries.length, 4);
      assert(!(await page.locator('body').innerText()).includes('SYNTHETIC_PRIVATE_BODY'));
      await capture('failed-unknown');
      // Independent backup evidence stays inspectable after the verdict fails.
      if (spec.touch) await row().tap();
      else await activate(page, row().getByRole('button').first(), false);
      const detail = page.locator('[data-inline-detail-for="resource:vm:0000"]');
      await expect(detail).toContainText('synthetic-pbs / main / (root)');
      await activate(page, detail.getByRole('button', { name: /Backup evidence for/ }), spec.touch);
      const dialog = page.getByRole('dialog', { name: 'Backup evidence details' });
      await expect(dialog).toContainText('synthetic-pbs/main/vm/100');
      await capture('independent-evidence');
      if (spec.touch)
        await activate(page, dialog.getByRole('button', { name: 'Close backup evidence' }), true);
      else await page.keyboard.press('Escape');
      await expect(dialog).toHaveCount(0);
      const retry = beginWave('isolated-retry');
      const inventoriesBefore = current.inventoryRequests.length;
      await activate(
        page,
        page.getByRole('button', { name: 'Retry protection posture', exact: true }),
        spec.touch,
      );
      await awaitFour(retry);
      await completeExceptFirst(retry, 'attention');
      await expectPosture('Unknown');
      release(retry.entries[0], 'attention');
      await expect(notice).toHaveCount(0);
      await expectPosture(spec.touch ? 'Attn.' : 'Attention');
      assert.equal(current.inventoryRequests.length, inventoriesBefore);
      await capture('retry-attention');
      current.checks.push(
        'Malformed batch cancels its active siblings and never starts the remaining seven; Unknown withdraws the old verdict, independent evidence opens, and posture-only retry publishes all 2,001 new results without rereading inventories.',
      );
      const disposed = beginWave('disposed-poll');
      await page.clock.fastForward(30_000);
      await awaitFour(disposed);
      await activate(
        page,
        page.getByRole('button', { name: 'Fixture: hide backup view', exact: true }),
        spec.touch,
      );
      await expect(row()).toHaveCount(0);
      await expect.poll(() => disposed.active).toBe(0);
      disposed.entries.forEach((entry) => release(entry));
      const remount = beginWave('remount');
      await activate(
        page,
        page.getByRole('button', { name: 'Fixture: show backup view', exact: true }),
        spec.touch,
      );
      await awaitFour(remount);
      await completeExceptFirst(remount, 'attention');
      release(remount.entries[0], 'attention');
      await expectPosture(spec.touch ? 'Attn.' : 'Attention');
      assert.equal(disposed.entries.length, 4);
      assert(page.url().includes('retained=1'));
      current.checks.push(
        'Disposal closes all four active HTTP reads, retired responses admit no queued old work, remount recovers at the same bound and unrelated URL state survives.',
      );
      await context.close();
      await browser.close();
      browser = null;
    }
    assert.deepEqual(report.errors, []);
    report.passed = true;
  } catch (error) {
    report.errors.push(String(error.stack || error));
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    report.cleanup.browserClosed = true;
    for (const res of openResponses) res.destroy();
    if (server) {
      server.closeAllConnections();
      await new Promise((resolve) => server.close(resolve));
    }
    report.cleanup.serverClosed = true;
    // Response objects are nonserializable. Retain their observed facts only.
    for (const item of report.cases)
      for (const owner of item.waves)
        owner.entries = owner.entries.map(({ res, ...entry }) => entry);
    fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(
      JSON.stringify({
        passed: report.passed,
        captures: report.captures.length,
        errors: report.errors,
      }),
    );
  }
})();
