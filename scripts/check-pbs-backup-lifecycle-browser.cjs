// Existing production Backups consumer, with source-proof-emitted PBS rows.
// Input: .ai-coordination/pbs-backup-lifecycle/producer.json, never live PBS.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const out = path.resolve('.ai-coordination/pbs-backup-lifecycle');
  const bytes = fs.readFileSync(path.join(out, 'producer.json'));
  const producer = JSON.parse(bytes);
  assert.equal(producer.held.length, 1);
  assert.equal(producer.current.length, 1);
  assert.equal(producer.retired.length, 0);
  assert.equal(producer.held[0].size, 1024);
  assert.equal(producer.current[0].size, 4096);
  assert.equal(producer.held[0].backupTime, producer.current[0].backupTime);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  process.chdir(root);
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5198, strictPort: true },
    plugins: [{ name: 'pbs-lifecycle-consumer', configureServer(server) {
      server.middlewares.use((req, _res, next) => {
        if (req.url?.startsWith('/proxmox/backups/')) req.url = '/browser-tests/backups.html';
        next();
      });
    } }],
  });
  let browser;
  const results = [];
  try {
    await server.listen();
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    for (const width of [1440, 320]) {
      const context = await browser.newContext({ viewport: { width, height: 900 } });
      const page = await context.newPage();
      const pageErrors = [];
      page.on('pageerror', error => pageErrors.push(error.message));
      let phase = 'held';
      const reads = { held: 0, retired: 0, current: 0 };
      await page.route('**/fixture/workloads', route => route.fulfill({ json: [{
        id: 'cluster:node-0:100', type: 'vm', name: 'guest-100', displayName: 'guest-100',
        platformId: 'cluster', platformType: 'proxmox-pve', sourceType: 'api', status: 'running',
        lastSeen: Date.now(), proxmox: { vmid: 100, node: 'node-0', instance: 'cluster' },
      }] }));
      await page.route(url => url.pathname.startsWith('/api/'), route => {
        const name = new URL(route.request().url()).pathname;
        if (name === '/api/backups/pbs') {
          reads[phase] += 1;
          return route.fulfill({ json: { data: { backups: producer[phase] } } });
        }
        if (name === '/api/backups/pve') return route.fulfill({ json: { data: {
          backupTasks: [], storageBackups: [], guestSnapshots: [],
        } } });
        return route.fulfill({ json: { data: [], policy: {
          freshnessWindowSeconds: 86400, verificationWindowSeconds: 604800, requireVerification: false,
        } } });
      });
      const rows = page.locator('[data-proxmox-backup-row="recoverable"]');
      const checkRows = async count => {
        await page.waitForFunction(count => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === count, count);
        assert.equal(await rows.count(), count);
        if (count) await page.getByText('guest-100', { exact: true }).first().waitFor();
        assert(!(await rows.allTextContents()).join(' ').includes('withdrawn'));
      };
      const capture = async label => {
        await page.evaluate(() => document.fonts.ready);
        await page.screenshot({ path: path.join(out, `${width}-${label}.png`), fullPage: true });
      };
      const refresh = async () => {
        const before = reads[phase];
        const button = page.getByRole('button', { name: 'Refresh backups', exact: true });
        await button.focus();
        await button.press('Enter');
        await page.waitForFunction(() => document.querySelector('button[title^="Read Pulse"]')?.getAttribute('aria-disabled') === 'false');
        assert(reads[phase] > before, 'refresh must consume this producer state');
      };
      await page.goto('http://127.0.0.1:5198/proxmox/backups/date', { waitUntil: 'commit' });
      await checkRows(1);
      await rows.first().scrollIntoViewIfNeeded();
      const heldKey = await rows.first().getAttribute('data-proxmox-backup-key');
      const heldRow = await rows.first().innerText();
      if (width === 1440) assert.match(heldRow, /1(?:\.0)?\s*KB/);
      await capture('held');
      phase = 'retired';
      await refresh();
      await checkRows(0);
      await refresh();
      await checkRows(0);
      await page.getByText(/No restore points/).first().waitFor();
      await capture('retired');
      phase = 'current';
      await refresh();
      await checkRows(1);
      const currentKey = await rows.first().getAttribute('data-proxmox-backup-key');
      const currentRow = await rows.first().innerText();
      assert.equal(currentKey, heldKey, 'same snapshot identity must update in place');
      if (width === 1440) assert.match(currentRow, /4(?:\.0)?\s*KB/);
      await rows.first().scrollIntoViewIfNeeded();
      await capture('current');
      await page.goto('http://127.0.0.1:5198/proxmox/backups/coverage', { waitUntil: 'commit' });
      const expand = page.getByRole('button', { name: 'Expand details for guest-100 (VM 100)', exact: true });
      await expand.waitFor();
      await expand.focus();
      await expand.press('Enter');
      await page.getByText('Restore evidence', { exact: true }).waitFor();
      await page.getByText('Restore evidence', { exact: true }).scrollIntoViewIfNeeded();
      await capture('coverage');
      assert.deepEqual(pageErrors, []);
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
      assert.equal(overflow, false);
      results.push({ width, reads, heldRow, currentRow, heldKey, currentKey, retiredArtifacts: 0, keyboardRefresh: true, keyboardCoverageExpansion: true, pageErrors, pageOverflow: overflow });
      await context.close();
    }
    const result = { playwright: require('playwright/package.json').version, chromium: browser.version(), fixtureSHA256: crypto.createHash('sha256').update(bytes).digest('hex'), results };
    fs.writeFileSync(path.join(out, 'browser-result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    await browser?.close();
    await server.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
