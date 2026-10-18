// Existing Backups consumer and HTTP readers; rows/postures are emitted by
// the ordinary foreground/background namespace-discovery source tests.
// Synthetic workloads only: no authenticated app, provider, guest or backup.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const out = path.resolve('.ai-coordination/pbs-namespace-discovery');
  const input = fs.readFileSync(path.join(out, 'producer.json'));
  const producer = JSON.parse(input);
  const phases = {
    initial: producer.transient_initial,
    partial: producer.transient_partial,
    denied: producer.denied_partial,
    complete: producer.transient_complete,
  };
  const counts = { initial: 3, partial: 2, denied: 1, complete: 1 };
  const summaries = { initial: '3 protected', partial: '3 unknown', denied: '3 attention', complete: '1 protected' };
  for (const [name, data] of Object.entries(phases)) {
    assert.equal(data.backups.length, counts[name]);
    assert.equal(data.postures.length, 3);
  }
  assert.deepEqual(phases.initial.postures.map(row => row.state).sort(), ['protected', 'protected', 'protected']);
  assert.deepEqual(phases.partial.postures.map(row => row.state).sort(), ['unknown', 'unknown', 'unknown']);
  assert.deepEqual(phases.denied.postures.map(row => row.state).sort(), ['attention', 'attention', 'attention']);
  assert.deepEqual(phases.complete.postures.map(row => row.state).sort(), ['protected', 'unknown', 'unknown']);
  // Match the fixture's actual node-independent canonical guest identities,
  // not the store's lexicographic posture ordering (100, 102, 101).
  const guestID = vmid => `vm-${crypto.createHash('sha256').update(`proxmox-guest:cluster:${vmid}`).digest('hex').slice(0, 16)}`;
  assert.deepEqual(phases.initial.postures.map(row => row.subjectResourceId).sort(), [100, 101, 102].map(guestID).sort());
  assert.equal(phases.complete.postures.find(row => row.state === 'protected').subjectResourceId, guestID(102));
  const held = phases.initial.backups.find(row => row.vmid === '100');
  assert.equal(held.namespace, 'team/production');
  assert.deepEqual(phases.partial.backups.find(row => row.vmid === '100'), held, 'transient loss retains exact original artifact');
  assert(!phases.denied.backups.some(row => row.vmid === '100'), 'access-rejected raw scope is withdrawn');
  assert(!phases.complete.backups.some(row => row.vmid === '100'), 'complete enumeration confirms deletion');
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  process.chdir(root);
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5198, strictPort: true },
    plugins: [{ name: 'pbs-namespace-consumer', configureServer(server) {
      server.middlewares.use((req, _res, next) => {
        if (req.url?.startsWith('/proxmox/backups/')) req.url = '/browser-tests/backups.html';
        next();
      });
    } }],
  });
  const result = {
    playwright: require('playwright/package.json').version,
    producerSHA256: crypto.createHash('sha256').update(input).digest('hex'),
    cases: [], captures: [], cleanup: {},
  };
  assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.resolve('..', 'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    result.chromium = browser.version();
    for (const width of [1440, 320]) {
      const context = await browser.newContext({ viewport: { width, height: 1000 } });
      const page = await context.newPage();
      const pageErrors = [], requests = [], unexpected = [];
      page.on('pageerror', error => pageErrors.push(error.message));
      let phase = 'initial';
      const workloads = [100, 101, 102].map((vmid, i) => ({
        id: guestID(vmid), type: 'vm', name: `guest-${vmid}`, displayName: `guest-${vmid}`,
        platformId: 'cluster', platformType: 'proxmox-pve', sourceType: 'api', status: 'running',
        lastSeen: Date.now(), proxmox: { vmid, node: `node-${i}`, instance: 'cluster' },
      }));
      await page.route('**/fixture/workloads', route => route.fulfill({ json: workloads }));
      await page.route(url => url.pathname.startsWith('/api/'), route => {
        const url = new URL(route.request().url());
        requests.push({ phase, method: route.request().method(), path: url.pathname });
        if (route.request().method() !== 'GET') { unexpected.push(url.pathname); return route.abort(); }
        if (url.pathname === '/api/backups/pbs') return route.fulfill({ json: { data: { backups: phases[phase].backups } } });
        if (url.pathname === '/api/backups/pve') return route.fulfill({ json: { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } } });
        if (url.pathname === '/api/recovery/postures') return route.fulfill({ json: { data: phases[phase].postures, policy: {
          freshnessWindowSeconds: 86400, verificationWindowSeconds: 604800, requireVerification: true,
        } } });
        unexpected.push(url.pathname);
        return route.abort();
      });
      const capture = async label => {
        await page.evaluate(() => document.fonts.ready);
        await page.evaluate(() => Promise.all(document.getAnimations().filter(a => a.effect?.getTiming().iterations !== Infinity).map(a => a.finished.catch(() => {}))));
        const file = path.join(out, `${width}-${label}.png`);
        await page.screenshot({ path: file, fullPage: true });
        result.captures.push({ path: path.relative(path.resolve('..'), file), sha256: crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex') });
      };
      const assertDate = async () => {
        await page.waitForFunction(count => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === count, counts[phase]);
        await page.getByRole('button', { name: summaries[phase], exact: true }).waitFor();
        const rows = page.locator('[data-proxmox-backup-row="recoverable"]');
        const text = (await rows.allTextContents()).join(' ');
        assert.equal(await rows.count(), counts[phase]);
        assert(text.includes('guest-102'));
        assert.equal(text.includes('guest-100'), phase === 'initial' || phase === 'partial');
        if (phase !== 'initial') assert(!text.includes('guest-101'), 'root emptiness replaces only root raw rows');
        if (width === 1440 && phase !== 'initial') assert.match(text, /4\.00 KB/, 'readable sibling advances while discovery is unavailable');
        return text;
      };
      const facts = [];
      for (const name of Object.keys(phases)) {
        phase = name;
        if (name === 'partial') {
          const before = requests.length;
          const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
          await refresh.focus();
          await refresh.press('Enter');
          await page.waitForFunction(() => document.querySelector('button[title^="Read Pulse"]')?.getAttribute('aria-disabled') === 'false');
          assert(requests.slice(before).some(r => r.phase === 'partial' && r.path === '/api/recovery/postures'));
        } else {
          await page.goto('http://127.0.0.1:5198/proxmox/backups/date', { waitUntil: 'commit' });
        }
        const dateRows = await assertDate();
        await capture(`${name}-date`);
        const fact = { phase, dateRows, expectedArtifacts: counts[phase] };
        if (name === 'partial' || name === 'denied') {
          await page.goto('http://127.0.0.1:5198/proxmox/backups/coverage', { waitUntil: 'commit' });
          await page.getByRole('button', { name: summaries[phase], exact: true }).waitFor();
          const expand = page.getByRole('button', { name: 'Expand details for guest-100 (VM 100)', exact: true });
          await expand.focus();
          await expand.press('Enter');
          const card = page.locator('[data-backup-provider-evidence]').first();
          await card.waitFor();
          const text = (await card.innerText()).replace(/\s+/g, ' ');
          assert.match(text, /History Partial/);
          assert.match(text, /Result Warning/);
          assert.match(text, name === 'partial' ? /Access Unknown/ : /Access Partial/);
          await card.scrollIntoViewIfNeeded();
          await capture(`${name}-coverage`);
          fact.provider = text;
          fact.keyboardExpansion = true;
        }
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
        facts.push(fact);
      }
      assert.deepEqual(pageErrors, []);
      assert.deepEqual(unexpected, []);
      result.cases.push({ width, workloadBindings: workloads.map(row => ({ id: row.id, vmid: row.proxmox.vmid })), facts, requests, pageErrors, unexpected, keyboardRefresh: true, pageOverflow: false });
      await context.close();
    }
  } finally {
    await browser?.close(); result.cleanup.browserClosed = true;
    await server.close(); result.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(out, 'browser-result.json'), JSON.stringify(result, null, 2) + '\n');
  }
  console.log(JSON.stringify(result));
})().catch(error => { console.error(error); process.exitCode = 1; });
