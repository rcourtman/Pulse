// Existing Backups HTTP readers/component/router consume actual ordinary-poll
// rows and stored protection emitted by TestPBSRootDatastoreDiscoveryRegression.
// Synthetic read-only providers/workloads; not an authenticated or native lab.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const out = path.resolve('.ai-coordination/pbs-datastore-discovery');
  const input = fs.readFileSync(path.join(out, 'producer.json'));
  const producer = JSON.parse(input);
  const phases = {
    initial: producer.transient_initial,
    unavailable: producer.transient_observed,
    denied: producer.denied_observed,
    empty: producer.empty_observed,
    recovered: producer.transient_recovered,
  };
  const counts = { initial: 2, unavailable: 2, denied: 0, empty: 0, recovered: 2 };
  const summaries = { initial: '2 protected', unavailable: '2 unknown', denied: '2 attention', empty: '2 unknown', recovered: '2 protected' };
  const guestID = vmid => `vm-${crypto.createHash('sha256').update(`proxmox-guest:cluster:${vmid}`).digest('hex').slice(0, 16)}`;
  for (const [name, data] of Object.entries(phases)) {
    assert.equal(data.backups.length, counts[name]);
    assert.equal(data.postures.length, 2);
    assert.deepEqual(data.postures.map(p => p.subjectResourceId).sort(), [100, 101].map(guestID).sort());
  }
  assert.deepEqual(phases.unavailable.backups, phases.initial.backups, 'unavailable discovery cannot re-observe raw backup evidence');
  for (const name of ['unavailable', 'denied']) for (const posture of phases[name].postures) {
    assert.equal(posture.state, name === 'denied' ? 'attention' : 'unknown');
    assert.equal(posture.providerStates[0].historyCompleteness, 'unavailable');
    assert.equal(posture.providerStates[0].permissions, name === 'denied' ? 'denied' : 'unknown');
  }
  for (const posture of phases.empty.postures) { assert.equal(posture.state, 'unknown'); assert.deepEqual(posture.providerStates, []); }
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  process.chdir(root);
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5198, strictPort: true },
    plugins: [{ name: 'pbs-datastore-consumer', configureServer(server) {
      server.middlewares.use((req, _res, next) => {
        if (req.url?.startsWith('/proxmox/backups/')) req.url = '/browser-tests/backups.html';
        next();
      });
    } }],
  });
  const result = { playwright: require('playwright/package.json').version, producerSHA256: crypto.createHash('sha256').update(input).digest('hex'), cases: [], captures: [], cleanup: {} };
  assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.resolve('..', 'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    result.chromium = browser.version();
    for (const width of [1440, 320]) {
      const context = await browser.newContext({ viewport: { width, height: 1000 } });
      const page = await context.newPage();
      const errors = [], requests = [], unexpected = [], facts = [];
      page.on('pageerror', error => errors.push(error.message));
      let phase = 'initial';
      const workloads = [100, 101].map((vmid, i) => ({ id: guestID(vmid), type: 'vm', name: `guest-${vmid}`, displayName: `guest-${vmid}`, platformId: 'cluster', platformType: 'proxmox-pve', sourceType: 'api', status: 'running', lastSeen: Date.now(), proxmox: { vmid, node: `node-${i}`, instance: 'cluster' } }));
      await page.route('**/fixture/workloads', route => route.fulfill({ json: workloads }));
      await page.route(url => url.pathname.startsWith('/api/'), route => {
        const url = new URL(route.request().url());
        requests.push({ phase, method: route.request().method(), path: url.pathname });
        if (route.request().method() !== 'GET') { unexpected.push(url.pathname); return route.abort(); }
        if (url.pathname === '/api/backups/pbs') return route.fulfill({ json: { data: { backups: phases[phase].backups } } });
        if (url.pathname === '/api/backups/pve') return route.fulfill({ json: { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } } });
        if (url.pathname === '/api/recovery/postures') return route.fulfill({ json: { data: phases[phase].postures, policy: { freshnessWindowSeconds: 86400, verificationWindowSeconds: 604800, requireVerification: true } } });
        unexpected.push(url.pathname); return route.abort();
      });
      const capture = async label => {
        await page.evaluate(() => document.fonts.ready);
        await page.evaluate(() => Promise.all(document.getAnimations().filter(a => a.effect?.getTiming().iterations !== Infinity).map(a => a.finished.catch(() => {}))));
        const file = path.join(out, `${width}-${label}.png`);
        await page.screenshot({ path: file, fullPage: true });
        result.captures.push({ path: path.relative(path.resolve('..'), file), sha256: crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex') });
      };
      for (const name of Object.keys(phases)) {
        phase = name;
        if (name === 'unavailable') {
          const before = requests.length;
          const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
          await refresh.focus(); await refresh.press('Enter');
          await page.waitForFunction(() => document.querySelector('button[title^="Read Pulse"]')?.getAttribute('aria-disabled') === 'false');
          assert(requests.slice(before).some(r => r.phase === phase && r.path === '/api/recovery/postures'));
        } else await page.goto('http://127.0.0.1:5198/proxmox/backups/date', { waitUntil: 'commit' });
        await page.getByRole('button', { name: summaries[phase], exact: true }).waitFor();
        await page.waitForFunction(count => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === count, counts[phase]);
        const rows = page.locator('[data-proxmox-backup-row="recoverable"]');
        const dateText = (await rows.allTextContents()).join(' ');
        if (counts[phase]) { assert.match(dateText, /guest-100/); assert.match(dateText, /guest-101/); }
        if (name === 'recovered' && width === 1440) assert.match(dateText, /4\.00 KB/);
        const fact = { phase, expectedArtifacts: counts[phase], dateText };
        if (name === 'unavailable' || name === 'empty' || name === 'recovered') await capture(`${name}-date`);
        if (name === 'unavailable' || name === 'denied' || name === 'empty') {
          await page.goto('http://127.0.0.1:5198/proxmox/backups/coverage', { waitUntil: 'commit' });
          await page.getByRole('button', { name: summaries[phase], exact: true }).waitFor();
          const expand = page.getByRole('button', { name: 'Expand details for guest-100 (VM 100)', exact: true });
          await expand.focus(); await expand.press('Enter');
          if (name !== 'empty') {
            const card = page.locator('[data-backup-provider-evidence]').first();
            await card.waitFor();
            const text = (await card.innerText()).replace(/\s+/g, ' ');
            assert.match(text, /History Unavailable/); assert.match(text, /Result Failed/);
            assert.match(text, name === 'denied' ? /Access Denied/ : /Access Unknown/);
            await card.scrollIntoViewIfNeeded(); fact.providerText = text;
          } else assert.equal(await page.locator('[data-backup-provider-evidence]').count(), 0, 'complete empty history cannot retain provider success');
          await capture(`${name}-coverage`); fact.keyboardExpansion = true;
        }
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
        facts.push(fact);
      }
      assert.deepEqual(errors, []); assert.deepEqual(unexpected, []);
      result.cases.push({ width, facts, requests, errors, unexpected, keyboardRefresh: true, pageOverflow: false });
      await context.close();
    }
  } finally {
    await browser?.close(); result.cleanup.browserClosed = true;
    await server.close(); result.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(out, 'browser-result.json'), JSON.stringify(result, null, 2) + '\n');
  }
  console.log(JSON.stringify(result));
})().catch(error => { console.error(error); process.exitCode = 1; });
