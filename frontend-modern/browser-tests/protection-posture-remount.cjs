// Real HTTP -> production posture/query/cache -> Backups/Coverage -> shipped CSS.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/protection-posture-remount');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json')));
for (const [file, digest] of Object.entries(binding.files))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Source changed: ${file}`);
const policy = {
  freshnessWindowSeconds: 604800,
  verificationWindowSeconds: 604800,
  requireVerification: true,
};
const posture = (state = 'protected') => ({
  data: [
    {
      subjectResourceId: 'ct-112',
      state,
      freshness: 'current',
      verification: 'verified',
      coverage: 'complete',
      providerStates: [],
      repositoryResourceIds: [],
      evidenceIds: ['point-1'],
      explanation:
        state === 'protected'
          ? 'A current verified backup is available.'
          : 'Current evaluation needs attention.',
      evaluatedAt: '2026-10-19T12:00:00Z',
    },
  ],
  policy,
});
const pve = {
  data: {
    backupTasks: [],
    guestSnapshots: [],
    storageBackups: [
      {
        id: 'archive-112',
        instance: 'pve-a',
        node: 'pve-a',
        storage: 'local',
        type: 'ct',
        vmid: 112,
        time: '2026-10-19T01:00:00Z',
        format: 'zst',
        volid: 'local:backup/vzdump-lxc-112.tar.zst',
      },
    ],
  },
};
const pbs = {
  data: {
    backups: [
      {
        id: 'pbs-main/main/ct/112',
        instance: 'pbs-main',
        datastore: 'main',
        vmid: '112',
        backupType: 'ct',
        backupTime: '2026-10-19T02:00:00Z',
        files: ['index.json.blob'],
      },
    ],
  },
};
const report = {
  binding,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  errors: [],
  cleanup: {},
};
let server, browser, current;
const held = new Set();
const send = (res, value, status = 200) => {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(value));
};
function serve(req, res, next) {
  if (!req.url.startsWith('/api/')) return next();
  try {
    assert.equal(req.method, 'GET', 'No product mutation or guest operation');
    const url = new URL(req.url, 'http://fixture.invalid');
    assert(
      ['/api/backups/pve', '/api/backups/pbs', '/api/recovery/postures'].includes(url.pathname),
    );
    const entry = {
      path: url.pathname,
      org: req.headers['x-org-id'] ?? null,
      mode: current.mode,
      closed: false,
    };
    current.requests.push(entry);
    req.on('close', () => {
      entry.closed = true;
    });
    if (url.pathname === '/api/backups/pve') return send(res, pve);
    if (url.pathname === '/api/backups/pbs') return send(res, pbs);
    assert.deepEqual(url.searchParams.getAll('resourceId'), ['ct-112']);
    assert.equal(url.searchParams.get('limit'), '200');
    if (current.mode === 'headers' || current.mode === 'body') {
      if (current.mode === 'body') {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.flushHeaders();
      }
      held.add(res);
      current.held.push({ res, entry });
      res.on('close', () => held.delete(res));
      return;
    }
    if (current.mode === 'denied') return send(res, {}, 403);
    return send(res, posture(current.mode));
  } catch (error) {
    report.errors.push(String(error.stack || error));
    send(res, {}, 500);
  }
}
(async () => {
  try {
    assert.equal(
      report.playwright,
      JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    const { createServer } = await import(
      pathToFileURL(path.join(frontend, 'node_modules/vite/dist/node/index.js')).href
    );
    server = await createServer({
      root: frontend,
      configFile: path.join(frontend, 'vite.config.ts'),
      cacheDir: path.join(output, 'vite-cache'),
      logLevel: 'error',
      plugins: [
        {
          name: 'offline-posture-remount',
          configureServer(s) {
            s.middlewares.use(serve);
          },
        },
      ],
      server: { host: '127.0.0.1', port: 0, strictPort: false },
      optimizeDeps: { noDiscovery: true, include: ['solid-js', 'solid-js/web', '@solidjs/router'] },
    });
    await server.listen();
    const origin = `http://127.0.0.1:${server.httpServer.address().port}`;
    for (const [name, engine, narrow] of [
      ['chromium', chromium, false],
      ['webkit', webkit, true],
    ]) {
      current = { mode: 'protected', requests: [], held: [] };
      browser = await engine.launch(
        narrow
          ? { headless: true }
          : { headless: true, channel: 'chromium', args: ['--no-sandbox'] },
      );
      const viewport = { width: narrow ? 320 : 1365, height: 1000 };
      const context = await browser.newContext({
        viewport,
        isMobile: narrow,
        hasTouch: narrow,
        locale: 'en-GB',
        timezoneId: 'UTC',
        colorScheme: narrow ? 'dark' : 'light',
      });
      const page = await context.newPage();
      page.on('pageerror', (error) => report.errors.push(`${name}: ${error.message}`));
      page.on('request', (request) => assert.equal(new URL(request.url()).origin, origin));
      const now = new Date('2026-10-19T12:00:00Z');
      await page.clock.install({ time: now });
      await page.clock.pauseAt(now);
      await page.goto(
        `${origin}/browser-tests/protection-posture-remount.html?view=coverage&q=backup-guest`,
        { waitUntil: 'domcontentloaded' },
      );
      if (narrow) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const activate = async (locator) => {
        if (narrow) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      const capture = async (state) => {
        await page.evaluate(() => document.fonts.ready);
        assert(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          'No horizontal overflow',
        );
        const file = `${name}-${state}.png`;
        await page.screenshot({
          path: path.join(output, file),
          fullPage: true,
          animations: 'disabled',
        });
        report.captures.push({
          file,
          state,
          engine: name,
          viewport,
          sha256: hash(fs.readFileSync(path.join(output, file))),
        });
      };
      const count = () =>
        current.requests.filter((r) => r.path === '/api/recovery/postures').length;
      const inventoryCount = () =>
        current.requests.filter((r) => r.path.startsWith('/api/backups/')).length;
      const row = () => page.locator('[data-proxmox-backup-row="coverage"]').first();
      const notice = () =>
        page.getByRole('status').filter({ hasText: 'Protection posture is unavailable' });
      const retry = () =>
        page.getByRole('button', { name: 'Retry protection posture', exact: true });
      const expectPosture = async (label) =>
        expect(row().getByText(label, { exact: true })).toBeVisible();
      const expand = async () => {
        if (narrow) await row().tap();
        else await activate(row().getByRole('button'));
        await expect(page.locator('[data-inline-detail-for="resource:ct-112"]')).toContainText(
          'backup-guest',
        );
        await expect(page.locator('[data-inline-detail-for="resource:ct-112"]')).toContainText(
          'PBS snapshot',
        );
      };
      await expectPosture(narrow ? 'Prot.' : 'Protected');
      await expect.poll(count).toBe(1);
      assert.equal(inventoryCount(), 2);
      await capture('current');
      current.mode = 'headers';
      await page.clock.fastForward(30_000);
      await expect.poll(count).toBe(2);
      await page.clock.fastForward(150_000);
      await expect.poll(count).toBe(3);
      await expect(notice()).toBeVisible();
      await expectPosture('Unknown');
      await expand();
      await capture('timeout');

      // Remount within the same browser/cache while the replacement hangs.
      await activate(page.getByRole('button', { name: 'Fixture: hide backup view', exact: true }));
      await expect(row()).toHaveCount(0);
      current.mode = 'body';
      await activate(page.getByRole('button', { name: 'Fixture: show backup view', exact: true }));
      await expect.poll(count).toBe(4);
      await expect(notice()).toBeVisible();
      await expectPosture('Unknown');
      for (const { res } of current.held.slice(0, 2)) res.end(JSON.stringify(posture()));
      await expect(notice()).toBeVisible();
      await expand();
      await capture('remounted-unknown');
      const beforeRetry = inventoryCount();
      await activate(retry());
      await expect.poll(count).toBe(5);
      await expect(
        page.getByRole('button', { name: 'Retrying protection posture…', exact: true }),
      ).toBeDisabled();
      await expectPosture('Unknown');
      assert.equal(inventoryCount(), beforeRetry, 'Scoped retry does not refetch inventories');
      current.held.at(-1).res.end(JSON.stringify(posture('unprotected')));
      await expect(notice()).toHaveCount(0);
      await expectPosture(narrow ? 'Unprot.' : 'Unprotected');
      await capture('recovered-current');

      // A scope switch clears the retained failure/verdict; an old read stays retired.
      current.mode = 'headers';
      await activate(page.getByRole('button', { name: 'Refresh backups', exact: true }));
      await expect.poll(count).toBe(6);
      const priorOrg = current.held.at(-1).res;
      await page.evaluate(() => window.__switchBackupOrg('next-org'));
      await expect.poll(count).toBe(7);
      priorOrg.end(JSON.stringify(posture()));
      await expectPosture(narrow ? 'Check' : 'Checking');
      await expect(notice()).toHaveCount(0);
      current.held.at(-1).res.end(JSON.stringify(posture('attention')));
      await expectPosture(narrow ? 'Attn.' : 'Attention');
      await expect(
        page.getByPlaceholder('Search backups by workload, node, source, or status'),
      ).toHaveValue('backup-guest');
      await capture('new-org-current');
      report.cases.push({
        engine: name,
        browserVersion: browser.version(),
        viewport,
        requests: current.requests,
        checks: [
          'Warm Protected',
          'Existing stall fallback produces Unknown',
          'Failed state survives same-cache remount',
          'Late aborted responses do not restore Protected',
          'Independent PVE/PBS evidence remains inspectable',
          'Keyboard/touch scoped retry is single-flight',
          'Current evaluation alone restores posture',
          'Organisation fence and saved search remain intact',
        ],
      });
      await context.close();
      report.cleanup[`${name}Context`] = true;
      await browser.close();
      browser = null;
    }
    assert.deepEqual(report.errors, []);
    report.passed = true;
  } catch (error) {
    report.failure = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    report.cleanup.browser = true;
    for (const response of held) response.destroy();
    held.clear();
    report.cleanup.heldResponses = true;
    if (server) await server.close();
    report.cleanup.server = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(
      JSON.stringify({
        passed: report.passed ?? false,
        journeys: report.cases.length,
        captures: report.captures.length,
        failure: report.failure,
      }),
    );
  }
})();
