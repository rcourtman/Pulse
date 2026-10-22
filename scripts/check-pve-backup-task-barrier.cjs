// Read-only source/consumer proof. Consume the ordinary scan's emitted state in
// the existing production Backups component; no real provider/guest is used.
// Producer: TestPVEArchiveScanRequiresCurrentTaskInventory, marker
// PVE_ARCHIVE_BARRIER_FRAMES, saved to tmp/pve-archive-barrier/producer.json.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const out = path.resolve('tmp/pve-archive-barrier');
  const bytes = fs.readFileSync(path.join(out, 'producer.json'));
  const frames = JSON.parse(bytes);
  const phases = ['initial', 'unavailable', 'unavailable-again', 'running', 'finished', 'empty'];
  assert.deepEqual(Object.keys(frames).sort(), phases.slice().sort());
  for (const phase of ['unavailable', 'unavailable-again']) {
    assert.deepEqual(frames[phase].storageBackups, frames.initial.storageBackups);
  }
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  process.chdir(root);
  let activePhase = 'initial';
  let activeReads;
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(out, 'vite-cache'),
    logLevel: 'error',
    optimizeDeps: { noDiscovery: true, include: ['solid-js', 'solid-js/web', '@solidjs/router'] },
    server: { host: '127.0.0.1', port: 5198, strictPort: true },
    plugins: [{ name: 'archive-task-barrier-consumer', configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const pathname = new URL(req.url, 'http://fixture.invalid').pathname;
        if (pathname.startsWith('/proxmox/backups/')) req.url = '/browser-tests/backups.html';
        let payload;
        if (pathname === '/fixture/workloads') payload = ['site', 'other'].map((instance) => ({
          id: `${instance}:node:100`, type: 'vm', name: instance === 'site' ? 'fixture-guest' : 'independent-guest',
          displayName: instance === 'site' ? 'fixture-guest' : 'independent-guest', platformId: instance,
          platformType: 'proxmox-pve', sourceType: 'api', status: 'running', lastSeen: Date.now(),
          proxmox: { vmid: 100, node: 'node', instance },
        }));
        else if (pathname === '/api/backups/pve') {
          activeReads[activePhase]++;
          payload = { data: frames[activePhase] };
        } else if (pathname === '/api/backups/pbs') payload = { data: { backups: [] } };
        else if (pathname.startsWith('/api/')) payload = { data: [], policy: { freshnessWindowSeconds: 86400, verificationWindowSeconds: 604800, requireVerification: false } };
        if (payload === undefined) return next();
        res.setHeader('Content-Type', 'application/json');
        res.end(JSON.stringify(payload));
      });
    } }],
  });
  const results = [];
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    for (const width of [1440, 320]) {
      const context = await browser.newContext({ viewport: { width, height: 1000 }, hasTouch: width === 320 });
      const page = await context.newPage();
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      let phase = 'initial';
      const reads = Object.fromEntries(phases.map((p) => [p, 0]));
      activePhase = phase;
      activeReads = reads;
      const rows = page.locator('[data-proxmox-backup-row="recoverable"]');
      const current = page.locator('[data-proxmox-backup-key*="-current.vma.zst"]');
      const snapshots = [];
      const refresh = async () => {
        const count = reads[phase];
        const button = page.getByRole('button', { name: 'Refresh backups', exact: true });
        await button.focus();
        await button.press('Enter');
        await page.waitForFunction(() => document.querySelector('button[title^="Read Pulse"]')?.getAttribute('aria-disabled') === 'false');
        assert(reads[phase] > count, 'refresh must consume the actual selected producer frame');
      };
      await page.goto('http://127.0.0.1:5198/proxmox/backups/date', { waitUntil: 'commit' });
      await page.waitForFunction(() => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === 3);
      await rows.first().waitFor({ state: 'visible' });
      for (const next of phases) {
        phase = next;
        activePhase = phase;
        if (phase !== 'initial') await refresh();
        const count = phase === 'empty' ? 2 : ['running', 'finished'].includes(phase) ? 4 : 3;
        await page.waitForFunction((count) => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === count, count);
        assert.equal(await rows.count(), count);
        assert.equal(await page.locator('[data-proxmox-backup-key="artifact:archive:independent"]').count(), 1, 'same-VMID independent archive must stay visible');
        assert.equal(await current.count(), ['running', 'finished'].includes(phase) ? 1 : 0, 'failed task read must not render a new completed archive');
        const body = await page.locator('main').innerText();
        const completed = phase === 'empty' ? 2 : phase === 'finished' ? 4 : 3;
        assert(body.includes(`${completed} restore points`), 'summary must exclude partial output');
        if (phase === 'running') {
          assert(body.includes('1 incomplete entry'));
          const evidence = current.getByRole('button', { name: 'Backup evidence for fixture-guest: Running', exact: true });
          if (width === 320) await evidence.tap();
          else { await evidence.focus(); await evidence.press('Enter'); }
          await page.getByRole('dialog').waitFor();
          assert((await page.getByRole('dialog').innerText()).includes('not a completed recovery point'));
          await page.keyboard.press('Escape');
          await page.getByRole('dialog').waitFor({ state: 'hidden' });
        }
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
        assert.equal(overflow, false, 'no page-level narrow viewport overflow');
        if (['unavailable-again', 'running', 'finished'].includes(phase)) {
          await rows.first().scrollIntoViewIfNeeded();
          await page.evaluate(() => document.fonts.ready);
          const capture = `${width}-${phase}.png`;
          await page.screenshot({ path: path.join(out, capture), fullPage: true });
          snapshots.push(capture);
        }
      }
      assert.deepEqual(errors, []);
      results.push({ width, reads, phases: phases.length, errors, captures: snapshots, evidenceInteraction: width === 320 ? 'touch' : 'keyboard' });
      await context.close();
    }
    const receipt = { playwright: require('playwright/package.json').version, browser: browser.version(), producerSHA256: crypto.createHash('sha256').update(bytes).digest('hex'), results };
    fs.writeFileSync(path.join(out, 'browser-result.json'), JSON.stringify(receipt, null, 2) + '\n');
    console.log(JSON.stringify(receipt));
  } finally {
    await browser?.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
