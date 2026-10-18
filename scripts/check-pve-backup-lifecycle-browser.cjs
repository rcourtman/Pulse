// Consumes TestPVEBackupLifecycleHTTP's source-bound emitted inventories in the
// existing production Backups component. It does not contact a real provider.
// Run from the assigned root with pulse-worker-browser after placing the decoded
// BACKUP_BROWSER_PRODUCER in tmp/pve-backup-lifecycle/producer.json.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const out = path.resolve('tmp/pve-backup-lifecycle');
  const bytes = fs.readFileSync(path.join(out, 'producer.json'));
  const producer = JSON.parse(bytes);
  for (const kind of ['backupTasks', 'storageBackups', 'guestSnapshots']) {
    assert.equal(producer.retired[kind].length, 0);
    assert.equal(producer.current[kind].length, 1);
  }
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  process.chdir(root);
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5198, strictPort: true },
    plugins: [{
      name: 'backup-lifecycle-consumer',
      configureServer(server) {
        server.middlewares.use((req, _res, next) => {
          if (req.url?.startsWith('/proxmox/backups/')) req.url = '/browser-tests/backups.html';
          next();
        });
      },
    }],
  });
  let browser;
  const results = [];
  try {
    await server.listen();
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    for (const width of [1440, 320]) {
      const context = await browser.newContext({ viewport: { width, height: 900 } });
      const page = await context.newPage();
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      let phase = 'current';
      const reads = { current: 0, retired: 0 };
      await page.route('**/fixture/workloads', (route) => route.fulfill({ json: phase === 'retired' ? [] : [{
        id: 'site:node:100', type: 'virtual-machine', name: 'fixture-guest', displayName: 'fixture-guest',
        platformId: 'site', platformType: 'proxmox-pve', sourceType: 'api', status: 'running',
        lastSeen: Date.now(), proxmox: { vmid: 100, node: 'node', instance: 'site' },
      }] }));
      await page.route((url) => url.pathname.startsWith('/api/'), (route) => {
        const pathname = new URL(route.request().url()).pathname;
        if (pathname === '/api/backups/pve') {
          reads[phase] += 1;
          return route.fulfill({ json: { data: producer[phase] } });
        }
        if (pathname === '/api/backups/pbs') return route.fulfill({ json: { data: { backups: [] } } });
        return route.fulfill({ json: { data: [], policy: {
          freshnessWindowSeconds: 86400, verificationWindowSeconds: 604800, requireVerification: false,
        } } });
      });
      const rows = page.locator('[data-proxmox-backup-row="recoverable"]');
      const checkRows = async (count) => {
        await page.waitForFunction((count) => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === count, count);
        assert.equal(await rows.count(), count);
        assert(!(await rows.evaluateAll((rows) => rows.map((row) => row.outerHTML).join(''))).includes('withdrawn'));
      };
      const capture = async (name) => {
        await page.evaluate(() => document.fonts.ready);
        await page.screenshot({ path: path.join(out, `${width}-${name}.png`), fullPage: true });
      };
      const refresh = async () => {
        const previous = reads[phase];
        const button = page.getByRole('button', { name: 'Refresh backups', exact: true });
        await button.waitFor();
        await button.focus();
        await button.press('Enter');
        await page.waitForFunction(() => document.querySelector('button[title^="Read Pulse"]')?.getAttribute('aria-disabled') === 'false');
        assert(reads[phase] > previous, 'refresh must consume the selected producer result');
      };
      await page.goto('http://127.0.0.1:5198/proxmox/backups/date');
      await checkRows(2);
      await rows.first().scrollIntoViewIfNeeded();
      await capture('current');
      phase = 'retired';
      await refresh();
      await checkRows(0);
      // Another ordinary read cannot resurrect the late retired observation.
      await refresh();
      await checkRows(0);
      await capture('retired');
      phase = 'current';
      await refresh();
      await checkRows(2);
      await page.goto('http://127.0.0.1:5198/proxmox/backups/coverage');
      const expand = page.getByRole('button', { name: 'Expand details for fixture-guest', exact: true });
      await expand.waitFor();
      await expand.focus();
      await expand.press('Enter');
      await page.getByText('Restore evidence', { exact: true }).waitFor();
      await page.getByText('Restore evidence', { exact: true }).scrollIntoViewIfNeeded();
      await capture('coverage');
      assert.deepEqual(errors, []);
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
      assert.equal(overflow, false, 'page must not overflow the viewport');
      results.push({ width, reads, currentArtifacts: 2, retiredArtifacts: 0, keyboardExpansion: true, pageErrors: errors, pageOverflow: overflow });
      await context.close();
    }
    const result = { playwright: require('playwright/package.json').version, chromium: browser.version(), fixtureSHA256: crypto.createHash('sha256').update(bytes).digest('hex'), results };
    fs.writeFileSync(path.join(out, 'browser-result.json'), `${JSON.stringify(result, null, 2)}\n`);
    console.log(JSON.stringify(result));
  } finally {
    await browser?.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
