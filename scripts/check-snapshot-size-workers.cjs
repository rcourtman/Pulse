// Source-produced snapshot/alert frames -> existing production consumers.
// Closed, read-only HTTP fixtures. No live guest, storage or backup operation.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const http = require('node:http');
const { chromium } = require('playwright');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');

(async () => {
  const root = path.resolve('frontend-modern');
  const out = path.resolve('tmp/snapshot-size-workers');
  const bytes = fs.readFileSync(path.join(out, 'producer.json'));
  const frames = JSON.parse(bytes);
  const source = JSON.parse(fs.readFileSync(path.join(out, 'source.json')));
  const phases = ['initial', 'unread', 'missing-size', 'recovered', 'empty'];
  assert.match(source.head, /^[0-9a-f]{40}$/);
  assert.match(source.producerSource, /^[0-9a-f]{40}$/);
  assert.equal(source.helperSHA256, hash(fs.readFileSync(__filename)));
  for (const [file, digest] of Object.entries(source.producerFiles))
    assert.equal(hash(fs.readFileSync(file)), digest, 'producer bytes differ from proved source');
  assert([1440, 320].includes(source.viewportWidth), 'one preselected viewport per bounded execution');
  assert.equal(source.producerSHA256, hash(bytes));
  assert.deepEqual(Object.keys(frames).sort(), phases.slice().sort());
  const playwright = require('playwright/package.json').version;
  assert.equal(playwright, JSON.parse(fs.readFileSync('tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
  const compiled = path.join(out, 'static');
  const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
  assert.equal(binding.source, source.compilerSource);
  const compiledSources = binding.compiledSources, compiledArtifacts = binding.artifacts, servedArtifacts = {};
  for (const [file, digest] of Object.entries(compiledSources))
    assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, 'compiled bytes differ from containing source');
  for (const [file, digest] of Object.entries(compiledArtifacts))
    assert.equal(hash(fs.readFileSync(path.join(compiled, file))), digest, 'compiled artifact changed');
  // Preserve build/source binding even if the execution transport cannot
  // return its final result. A checkpoint is partial evidence, not a pass.
  fs.writeFileSync(path.join(out, `compiled-binding-${source.viewportWidth}.json`), JSON.stringify({ source, playwright, compiledSources, compiledArtifacts }, null, 2) + '\n');
  const checkpoint = (stage, details = {}) => fs.writeFileSync(path.join(out, `browser-progress-${source.viewportWidth}.json`), JSON.stringify({ source, stage, ...details }, null, 2) + '\n');
  checkpoint('compiled');
  let phase = 'initial', reads;
  const requests = [], allRequests = [], unexpected = [];
  const server = http.createServer((req, res) => {
    const pathname = new URL(req.url, 'http://fixture.invalid').pathname;
    allRequests.push({ method: req.method, path: pathname, phase });
    if (req.method !== 'GET') { unexpected.push(`${req.method} ${pathname}`); res.statusCode = 405; return res.end('{}'); }
    let payload;
    if (pathname === '/fixture/workloads') payload = [
      { id: 'site:a-slow:100', type: 'vm', name: 'database', displayName: 'database', platformId: 'site', platformType: 'proxmox-pve', sourceType: 'api', status: 'running', lastSeen: Date.now(), proxmox: { vmid: 100, node: 'a-slow', instance: 'site' } },
      { id: 'site:b-peer:200', type: 'system-container', name: 'worker', displayName: 'worker', platformId: 'site', platformType: 'proxmox-pve', sourceType: 'api', status: 'running', lastSeen: Date.now(), proxmox: { vmid: 200, node: 'b-peer', instance: 'site' } },
    ];
    else if (pathname === '/api/backups/pve') { reads[phase]++; payload = { data: frames[phase].pve }; }
    else if (pathname === '/api/backups/pbs') payload = { data: { backups: [] } };
    else if (pathname === '/api/alerts/delivery-diagnosis') payload = [];
    else if (pathname.startsWith('/api/')) payload = { data: [], policy: { freshnessWindowSeconds: 86400, verificationWindowSeconds: 604800, requireVerification: false } };
    if (payload !== undefined) {
      requests.push({ path: pathname, phase });
      res.setHeader('Content-Type', 'application/json');
      return res.end(JSON.stringify(payload));
    }
    if (pathname === '/favicon.ico') { res.statusCode = 204; return res.end(); }
    const key = pathname.startsWith('/proxmox/backups/') ? 'browser-tests/backups.html' : decodeURIComponent(pathname).slice(1);
    if (!Object.hasOwn(compiledArtifacts, key)) { unexpected.push(`GET ${pathname}`); res.statusCode = 404; return res.end('Not found'); }
    const artifact = fs.readFileSync(path.join(compiled, key));
    assert.equal(hash(artifact), compiledArtifacts[key], 'served bytes match this build');
    servedArtifacts[key] = hash(artifact);
    res.setHeader('Content-Type', { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' }[path.extname(key)] || 'application/octet-stream');
    res.end(artifact);
  });
  const results = [], captures = [];
  let browser, browserVersion, failure = null;
  const cleanup = { browserClosed: false, serverClosed: false };
  const overflowCensus = [];
  try {
    await new Promise(resolve => server.listen(5198, '127.0.0.1', resolve));
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    browserVersion = browser.version();
    for (const width of [source.viewportWidth]) {
      const context = await browser.newContext({ viewport: { width, height: 1000 }, hasTouch: width === 320, reducedMotion: 'reduce', colorScheme: width === 320 ? 'dark' : 'light' });
      if (width === 320) await context.addInitScript(() => document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.add('dark')));
      const page = await context.newPage();
      const alertPage = await context.newPage();
      const errors = [], consoleErrors = [], requestFailures = [], teardownRequests = [];
      let closing = false;
      for (const view of [page, alertPage]) {
        view.on('pageerror', error => errors.push(error.message));
        view.on('console', message => { if (message.type() === 'error') consoleErrors.push(message.text()); });
        view.on('requestfailed', request => {
          const observation = { path: new URL(request.url()).pathname, error: request.failure()?.errorText };
          (closing ? teardownRequests : requestFailures).push(observation);
        });
        await view.route('**/*', route => {
          const req = route.request(), url = new URL(req.url());
          if (url.origin !== 'http://127.0.0.1:5198' || req.method() !== 'GET') {
            unexpected.push(`${req.method()} ${url.origin}${url.pathname}`);
            return route.abort();
          }
          return route.continue();
        });
      }
      reads = Object.fromEntries(phases.map(name => [name, 0]));
      phase = 'initial';
      await page.goto('http://127.0.0.1:5198/proxmox/backups/date', { waitUntil: 'commit' });
      await alertPage.goto('http://127.0.0.1:5198/browser-tests/alert-delivery-freshness.html', { waitUntil: 'commit' });
      await alertPage.waitForFunction(() => window.__deliveryFreshness);
      const capture = async (view, name, subject) => {
        await subject.evaluate(element => element.scrollIntoView({ block: 'center' }));
        await view.evaluate(() => document.fonts.ready);
        await view.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert.equal(await view.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, 'no horizontal overflow');
        await view.screenshot({ path: path.join(out, name), fullPage: true, animations: 'disabled' });
        captures.push({ file: name, sha256: hash(fs.readFileSync(path.join(out, name))) });
      };
      const observations = [];
      for (const name of phases) {
        checkpoint('phase-start', { width, phase: name, results, captures });
        phase = name;
        if (name !== 'initial') {
          const before = reads[name];
          const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
          if (width === 320) await refresh.tap();
          else { await refresh.focus(); await refresh.press('Enter'); }
          await page.waitForFunction(() => document.querySelector('button[title^="Read Pulse"]')?.getAttribute('aria-disabled') === 'false');
          assert(reads[name] > before, 'refresh consumes the selected source-produced frame');
        }
        const expected = frames[name].pve.guestSnapshots.length;
        await page.waitForFunction(count => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === count, expected);
        const slow = page.locator('[data-proxmox-backup-key="artifact:snapshot:site-a-slow-100-weekly"]');
        const peer = page.locator('[data-proxmox-backup-key="artifact:snapshot:site-b-peer-200-weekly"]');
        await peer.waitFor({ state: 'visible' });
        assert((await peer.innerText()).includes('worker'));
        const slowText = await slow.count() ? await slow.innerText() : null;
        const peerText = await peer.innerText();
        if (width === 1440) {
          const size = frames[name].pve.guestSnapshots.find(snap => snap.node === 'b-peer').sizeBytes / (1 << 30);
          assert(new RegExp(`${size}(?:\\.0+)? GB`).test(peerText), `current peer size ${size} GB must render: ${peerText}`);
          if (['unread', 'missing-size'].includes(name)) assert(slowText.includes('No size'), 'unknown total is not the readable disk partial sum');
        }
        if (name === 'empty') assert.equal(await slow.count(), 0, 'authoritative removal cannot leave a ghost row');
        await alertPage.evaluate(active => window.__deliveryFreshness.setAlerts(Object.fromEntries(active.map(alert => [alert.id, alert]))), frames[name].active || []);
        if (['initial', 'unread', 'missing-size'].includes(name)) {
          const message = frames[name].active[0].message;
          await alertPage.waitForFunction(message => document.body.innerText.includes(message), message);
          assert((await alertPage.locator('body').innerText()).includes('Acknowledged'));
        } else await alertPage.getByText('No active alerts', { exact: true }).waitFor();
        for (const [surface, view] of [['backups', page], ['alerts', alertPage]]) {
          const overflow = await view.evaluate(() => ({ width: innerWidth, documentWidth: document.documentElement.scrollWidth,
            overflowing: Array.from(document.querySelectorAll('main, main *')).filter(el => el.clientWidth > 0 && el.scrollWidth > el.clientWidth + 1).map(el => ({tag:el.tagName,classes:el.className,width:el.clientWidth,scroll:el.scrollWidth})).slice(0,50) }));
          overflowCensus.push({ width, phase: name, surface, ...overflow });
          assert(overflow.documentWidth <= width, 'no document overflow in any phase');
        }
        observations.push({ phase: name, expectedSnapshots: expected, slowText, peerText, active: frames[name].active?.map(alert => ({ id: alert.id, value: alert.value, message: alert.message, acknowledged: alert.acknowledged })) ?? [] });
        if (['unread', 'recovered', 'empty'].includes(name)) await capture(page, `${width}-${name}-backups.png`, name === 'unread' ? slow : peer);
        if (['unread', 'recovered'].includes(name)) await capture(alertPage, `${width}-${name}-alerts.png`, alertPage.locator('main'));
        checkpoint('state-observed', { width, phase: name, observations, results, captures, errors });
        if (name === 'recovered') {
          const evidence = slow.getByRole('button', { name: 'Backup evidence for database: Snapshot', exact: true });
          checkpoint('evidence-open-start', { width, phase: name, results, captures });
          if (width === 320) await evidence.tap();
          else { await evidence.focus(); await evidence.press('Enter'); }
          const dialog = page.getByRole('dialog');
          await dialog.waitFor();
          const text = await dialog.innerText();
          assert(text.includes('not a separate backup') && text.includes('guest responsiveness') && text.includes('successful restore'));
          checkpoint('evidence-observed', { width, phase: name, text, results, captures });
          await capture(page, `${width}-recovered-evidence.png`, dialog);
          await page.keyboard.press('Escape');
          await dialog.waitFor({ state: 'hidden' });
        }
      }
      assert.deepEqual(errors, []);
      assert.deepEqual(consoleErrors, []);
      assert.deepEqual(requestFailures, []);
      assert(phases.every(name => reads[name] === 1), 'one actual PVE inventory read per phase and viewport');
      results.push({ width, dark: width === 320, interaction: width === 320 ? 'touch' : 'keyboard', reads, observations, errors, consoleErrors, requestFailures, teardownRequests });
      checkpoint('viewport-complete', { width, results, captures });
      closing = true;
      await context.close();
    }
    assert.deepEqual(unexpected, []);
  } catch (error) {
    failure = { name: error.name, message: error.message };
    process.exitCode = 1;
  } finally {
    if (browser) { await browser.close(); cleanup.browserClosed = true; }
    server.closeAllConnections();
    await new Promise(resolve => server.close(resolve)); cleanup.serverClosed = true;
    const receipt = { source, playwright, browser: browserVersion, compiledSources, compiledArtifacts, servedArtifacts,
      producerSHA256: hash(bytes), results, captures, requests, allRequests, unexpected, overflowCensus, cleanup, failure };
    fs.writeFileSync(path.join(out, `browser-result-${source.viewportWidth}.json`), JSON.stringify(receipt, null, 2) + '\n');
    console.log(JSON.stringify({ width: source.viewportWidth, result: failure ? 'fail' : 'pass', failure, cleanup, captures: captures.length }));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
