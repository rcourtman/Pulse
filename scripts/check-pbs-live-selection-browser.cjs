// Source-proof-emitted full PBS inventory -> existing production Backups UI.
// Closed HTTP fixtures only. No real PBS, guest, backup or recovery operation.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const out = path.resolve('tmp/pbs-live-selection');
  const bytes = fs.readFileSync(path.join(out, 'producer.json'));
  const frames = JSON.parse(bytes);
  const source = JSON.parse(fs.readFileSync(path.join(out, 'source.json')));
  assert.match(source.head, /^[0-9a-f]{40}$/);
  assert.equal(source.head, source.producerSource);
  const phases = ['initial', 'changed', 'transient', 'denied', 'recovered', 'empty', 'restored'];
  assert.deepEqual(Object.keys(frames).sort(), phases.slice().sort());
  assert.deepEqual(frames.changed.backups, frames.transient.backups);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  process.chdir(root);
  let activePhase = 'initial';
  let reads;
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'), cacheDir: path.join(out, 'vite-cache'), logLevel: 'error',
    optimizeDeps: { noDiscovery: true, include: ['solid-js', 'solid-js/web', '@solidjs/router'] },
    server: { host: '127.0.0.1', port: 5198, strictPort: true },
    plugins: [{ name: 'pbs-live-selection-consumer', configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const pathname = new URL(req.url, 'http://fixture.invalid').pathname;
        if (pathname.startsWith('/proxmox/backups/')) req.url = '/browser-tests/backups.html';
        let payload;
        if (pathname === '/fixture/workloads') payload = [100, 101].map((vmid, index) => ({
          id: frames.initial.postures[index].subjectResourceId, type: 'vm', name: `guest-${vmid}`, displayName: `guest-${vmid}`,
          platformId: 'cluster', platformType: 'proxmox-pve', sourceType: 'api', status: 'running', lastSeen: Date.now(),
          proxmox: { vmid, node: `node-${index}`, instance: 'cluster' },
        }));
        else if (pathname === '/api/backups/pbs') {
          reads[activePhase]++;
          payload = { data: { backups: frames[activePhase].backups } };
        } else if (pathname === '/api/backups/pve') payload = { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } };
        else if (pathname.startsWith('/api/')) payload = { data: pathname === '/api/recovery/postures' ? frames[activePhase].postures : [], policy: { freshnessWindowSeconds: 86400, verificationWindowSeconds: 604800, requireVerification: false } };
        if (payload === undefined) return next();
        res.setHeader('Content-Type', 'application/json');
        res.end(JSON.stringify(payload));
      });
    } }],
  });
  let browser;
  const results = [];
  try {
    await server.listen();
    await server.transformRequest('/browser-tests/backups.tsx');
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    for (const width of [1440, 320]) {
      const context = await browser.newContext({ viewport: { width, height: 1000 }, hasTouch: width === 320 });
      const page = await context.newPage();
      const errors = [];
      page.on('pageerror', error => errors.push(error.message));
      reads = Object.fromEntries(phases.map(phase => [phase, 0]));
      activePhase = 'initial';
      await page.goto('http://127.0.0.1:5198/proxmox/backups/date?q=guest-100', { waitUntil: 'commit' });
      const snapshots = [];
      const observations = [];
      const capture = async (name, subject) => {
        await subject.scrollIntoViewIfNeeded();
        await page.evaluate(() => document.fonts.ready);
        // Visibility is not settled pixels: the empty-state fade may still
        // be running, and a narrow table row can sit below the viewport.
        await page.waitForFunction(() => !document.getAnimations().some(animation =>
          animation.playState === 'running' && Number.isFinite(animation.effect?.getComputedTiming().iterations)));
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        await page.screenshot({ path: path.join(out, name), fullPage: true });
        snapshots.push(name);
      };
      for (const phase of phases) {
        activePhase = phase;
        if (phase !== 'initial') {
          const before = reads[phase];
          const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
          if (width === 320) await refresh.tap();
          else { await refresh.focus(); await refresh.press('Enter'); }
          await page.waitForFunction(() => document.querySelector('button[title^="Read Pulse"]')?.getAttribute('aria-disabled') === 'false');
          assert(reads[phase] > before, 'refresh must consume this exact producer frame');
        }
        const count = frames[phase].backups.length;
        try {
          await page.waitForFunction(count => document.querySelector('main')?.innerText.includes(`${count} restore points`), count);
        } catch (error) {
          await page.screenshot({ path: path.join(out, 'failure.png'), fullPage: true });
          fs.writeFileSync(path.join(out, 'failure.json'), JSON.stringify({ width, phase, count, reads, errors, text: await page.locator('body').innerText() }, null, 2));
          throw error;
        }
        const target = frames[phase].backups.find(backup => backup.vmid === '100');
        let rowText = '';
        if (target) {
          const row = page.locator(`[data-proxmox-backup-key="artifact:pbs:${target.id}"]`);
          await row.waitFor({ state: 'visible' });
          rowText = await row.innerText();
          assert(rowText.includes('guest-100'));
          const state = target.verified ? 'Verified' : 'Unverified';
          const evidence = row.getByRole('button', { name: `Backup evidence for guest-100: ${state}`, exact: true });
          if (phase === 'changed' || phase === 'recovered') {
            if (width === 320) await evidence.tap();
            else { await evidence.focus(); await evidence.press('Enter'); }
            await page.getByRole('dialog').waitFor();
            const text = await page.getByRole('dialog').innerText();
            assert(text.includes(state));
            assert(text.includes('guest responsiveness') && text.includes('successful restore'), 'evidence must retain its safety limit');
            await capture(`${width}-${phase}-evidence.png`, page.getByRole('dialog'));
            await page.keyboard.press('Escape');
            await page.getByRole('dialog').waitFor({ state: 'hidden' });
            await evidence.focus();
          }
          if (width === 1440) assert(rowText.includes(phase === 'changed' || phase === 'transient' ? '4.00 KB' : '1.00 KB'));
        } else {
          assert.equal(await page.locator('[data-proxmox-backup-key*="--vm-100-"]').count(), 0, 'denied/empty target must not be invented');
        }
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
        observations.push({ phase, fullProducerRows: count, selectedTarget: target?.id ?? null, rowText });
        if (['changed', 'denied', 'recovered'].includes(phase)) {
          const subject = target
            ? page.locator(`[data-proxmox-backup-key="artifact:pbs:${target.id}"]`)
            : page.getByText('No recoverable artifacts match current filters', { exact: true });
          await capture(`${width}-${phase}.png`, subject);
        }
      }
      assert.deepEqual(errors, []);
      results.push({ width, reads, observations, errors, captures: snapshots, interaction: width === 320 ? 'touch' : 'keyboard' });
      await context.close();
    }
    const receipt = { source, playwright: require('playwright/package.json').version, browser: browser.version(), producerSHA256: crypto.createHash('sha256').update(bytes).digest('hex'), results };
    fs.writeFileSync(path.join(out, 'browser-result.json'), JSON.stringify(receipt, null, 2) + '\n');
    console.log(JSON.stringify(receipt));
  } finally {
    await browser?.close();
    await server.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
