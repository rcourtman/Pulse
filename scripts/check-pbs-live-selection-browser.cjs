// Source-proof-emitted full PBS inventory -> existing production Backups UI.
// Closed HTTP fixtures only. No real PBS, guest, backup or recovery operation.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const http = require('node:http');
const { createRequire } = require('node:module');
const { pathToFileURL } = require('node:url');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');

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
  // The existing backup-identity proof compiles this same consumer before
  // navigation. Reuse that route instead of racing cold dev-module requests
  // against page.goto. No product or primary assertion is changed.
  const { build } = await import(pathToFileURL(path.join(root, 'node_modules/vite/dist/node/index.js')).href);
  const frontendRequire = createRequire(path.join(root, 'package.json'));
  const solid = frontendRequire('vite-plugin-solid');
  const tailwind = (await import(pathToFileURL(frontendRequire.resolve('@tailwindcss/vite')).href)).default;
  const compiled = path.join(out, 'static');
  const compiledSources = {};
  await build({
    root, configFile: false, publicDir: false, logLevel: 'error',
    resolve: { alias: { '@': path.join(root, 'src') }, conditions: ['import', 'browser', 'default'] },
    plugins: [tailwind(), solid(), {
      name: 'pbs-live-selection-compiled-source-binding',
      generateBundle() {
        for (const id of this.getModuleIds()) {
          const file = id.split('?')[0];
          if (!file.startsWith(root + '/') || file.includes('/node_modules/')) continue;
          if (fs.existsSync(file) && fs.statSync(file).isFile()) compiledSources[path.relative(root, file)] = hash(fs.readFileSync(file));
        }
      },
    }],
    build: { target: 'esnext', outDir: compiled, emptyOutDir: true,
      rollupOptions: { input: path.join(root, 'browser-tests/backups.html') } },
  });
  const compiledArtifacts = {};
  const walk = dir => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const file = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(file);
      else compiledArtifacts[path.relative(compiled, file)] = hash(fs.readFileSync(file));
    }
  };
  walk(compiled);
  let activePhase = 'initial';
  let reads;
  const server = http.createServer((req, res) => {
    const pathname = new URL(req.url, 'http://fixture.invalid').pathname;
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
    if (payload !== undefined) {
      res.setHeader('Content-Type', 'application/json');
      return res.end(JSON.stringify(payload));
    }
    if (pathname === '/favicon.ico') { res.statusCode = 204; return res.end(); }
    const key = pathname.startsWith('/proxmox/backups/') ? 'browser-tests/backups.html' : decodeURIComponent(pathname).slice(1);
    if (!Object.hasOwn(compiledArtifacts, key)) { res.statusCode = 404; return res.end('Not found'); }
    const artifact = fs.readFileSync(path.join(compiled, key));
    assert.equal(hash(artifact), compiledArtifacts[key], 'served bytes must match this compiled consumer');
    res.setHeader('Content-Type', { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' }[path.extname(key)] || 'application/octet-stream');
    res.end(artifact);
  });
  let browser;
  const results = [];
  try {
    await new Promise(resolve => server.listen(5198, '127.0.0.1', resolve));
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
    const receipt = { source, compiledSources, compiledArtifacts, playwright: require('playwright/package.json').version, browser: browser.version(), producerSHA256: crypto.createHash('sha256').update(bytes).digest('hex'), results };
    fs.writeFileSync(path.join(out, 'browser-result.json'), JSON.stringify(receipt, null, 2) + '\n');
    console.log(JSON.stringify(receipt));
  } finally {
    await browser?.close();
    await new Promise(resolve => server.close(resolve));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
