// Safe HTTP snapshots -> production inventory readers, recovery mapper and both
// Backups views. No provider, guest, backup, restore or installed-recovery work.
process.env.RAYON_NUM_THREADS = '2';
process.env.UV_THREADPOOL_SIZE = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { execFileSync } = require('node:child_process');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/backup-connection-identity/browser');
const binding = JSON.parse(fs.readFileSync(path.join(root, 'tmp/backup-connection-identity/binding.json')));
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const guest = (id, instance, node) => ({
  id, name: 'database', type: 'vm', platformId: instance,
  platformType: 'proxmox-pve', sourceType: 'api', status: 'running',
  lastSeen: Date.parse('2026-10-19T12:00:00Z'),
  proxmox: { vmid: 100, instance, nodeName: node },
});
const upper = guest('upper-100', 'East', 'pve-upper');
const lower = guest('lower-100', 'east', 'pve-lower');
const pve = { data: {
  storageBackups: ['upper', 'lower'].map(kind => ({
    id: `file-${kind}`, instance: kind === 'upper' ? 'East' : 'east',
    node: kind === 'upper' ? 'pve-lower' : 'pve-upper', type: 'vm', vmid: 100,
    storage: 'local', time: '2026-10-19T10:00:00Z', size: 1024, format: 'zst',
    volid: `local:backup/${kind}-archive-marker.vma.zst`, protected: false,
  })),
  guestSnapshots: ['upper', 'lower'].map(kind => ({
    id: `snapshot-${kind}`, instance: kind === 'upper' ? 'East' : 'east',
    node: kind === 'upper' ? 'pve-lower' : 'pve-upper', type: 'vm', vmid: 100,
    time: '2026-10-19T10:30:00Z', name: `${kind}-snapshot-marker`,
    description: `${kind}-snapshot-marker`, vmstate: false,
  })),
  backupTasks: ['upper', 'lower'].map(kind => ({
    id: `${kind}-job`, instance: kind === 'upper' ? 'East' : 'east',
    node: kind === 'upper' ? 'pve-lower' : 'pve-upper',
    type: kind === 'upper' ? 'vzdump' : 'vm', vmid: 100,
    status: kind === 'upper' ? 'warning' : 'OK',
    startTime: '2026-10-19T11:00:00Z', endTime: '2026-10-19T11:05:00Z',
    observedAt: '2026-10-19T11:06:00Z',
  })),
} };
const pbs = { data: { backups: ['upper', 'lower'].map(kind => ({
  id: `pbs-${kind}`, instance: 'central-backups', datastore: 'main',
  namespace: kind === 'upper' ? 'East' : 'east', backupType: 'vm', vmid: '100',
  backupTime: '2026-10-19T09:00:00Z', size: 2048,
  files: ['manifest.blob'], protected: true, verified: true,
})) } };
const result = {
  passed: false, source: binding.source, tree: binding.tree,
  protocol_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [], captures: [], unexpected: [], cleanup: {},
  limits: 'Synthetic collected snapshots and resource replacement through actual HTTP decoder, router, mapper, tables and CSS. PVE exact-source attribution and PBS exact connection-name hints; existing unique-VMID PBS fallback is unchanged. No native collection, backup/restore, guest thaw/liveness, installed recovery, release or screen-reader speech proof.',
};
let server, browser, current, origin;
function checkBinding() {
  for (const [file, digest] of Object.entries(binding.files))
    assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, file);
}
function serve(req, res, next) {
  if (!req.url.startsWith('/api/')) return next();
  assert.equal(req.method, 'GET');
  const url = new URL(req.url, 'http://fixture.invalid');
  let payload;
  if (url.pathname === '/api/backups/pve') payload = pve;
  else if (url.pathname === '/api/backups/pbs')
    payload = current.phase === 'orphan' ? { data: { backups: [] } } : pbs;
  else if (url.pathname === '/api/recovery/postures') payload = {
    data: url.searchParams.getAll('resourceId').map(subjectResourceId => ({
      subjectResourceId, state: 'unknown', freshness: 'unknown', verification: 'unknown',
      coverage: 'unknown', explanation: 'Synthetic evidence, not a recovery verdict.',
      providerStates: [], repositoryResourceIds: [], evidenceIds: [],
      evaluatedAt: '2026-10-19T12:00:00Z',
    })),
    policy: { freshnessWindowSeconds: 604800, verificationWindowSeconds: 604800, requireVerification: true },
  };
  else if (url.pathname.includes('runtime-capabilities')) payload = {
    capabilities: [], limits: [], max_history_days: 7, hosted_mode: false,
    runtime: { build: 'community' }, blocked_capabilities: [],
  };
  else payload = { data: [], alerts: [], anomalies: [], enabled: false };
  current.reads.push({ path: url.pathname, phase: current.phase, sha256: hash(JSON.stringify(payload)) });
  res.setHeader('Content-Type', 'application/json');
  res.end(JSON.stringify(payload));
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
    checkBinding();
    assert.equal(result.playwright,
      JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json')))
        .packages['node_modules/@playwright/test'].version);
    // Prepare the admitted lock from this assignment's verified offline cache,
    // inside the browser capability, not through the source shell's listeners.
    execFileSync('npm', ['ci', '--offline', '--cache', path.join(root, 'tmp/backup-connection-identity/cache/npm')],
      { cwd: frontend, stdio: 'pipe', timeout: 60000 });
    result.dependency_install = 'npm ci --offline, exact bound manifest/lock';
    const { createServer } = await import(pathToFileURL(path.join(frontend, 'node_modules/vite/dist/node/index.js')).href);
    server = await createServer({
      root: frontend, configFile: path.join(frontend, 'vite.config.ts'),
      cacheDir: path.join(output, 'vite-cache'), logLevel: 'error',
      optimizeDeps: { entries: ['browser-tests/backup-identity.html'] },
      plugins: [{ name: 'exact-connection-reads', configureServer(s) { s.middlewares.use(serve); } }],
      server: { host: '127.0.0.1', port: 0, strictPort: false, proxy: {} },
    });
    await server.listen();
    origin = `http://127.0.0.1:${server.httpServer.address().port}`;
    for (const spec of [
      { name: 'chromium', engine: chromium, width: 1440, touch: false },
      { name: 'webkit', engine: webkit, width: 320, touch: true },
    ]) {
      current = { engine: spec.name, viewport: { width: spec.width, height: 1000 }, phase: 'both', reads: [], checks: [], pageErrors: [] };
      result.cases.push(current);
      browser = await spec.engine.launch(spec.touch ? { headless: true } :
        { headless: true, channel: 'chromium', args: ['--no-sandbox'] });
      current.browser_version = browser.version();
      const context = await browser.newContext({ viewport: current.viewport, hasTouch: spec.touch,
        isMobile: spec.touch, locale: 'en-GB', timezoneId: 'UTC', colorScheme: spec.touch ? 'dark' : 'light' });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      await page.clock.setFixedTime(new Date('2026-10-19T12:01:00Z'));
      if (spec.touch) await page.addInitScript(() => document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.add('dark')));
      page.on('pageerror', error => current.pageErrors.push(error.message));
      await page.route('**/*', route => {
        const req = route.request(), url = new URL(req.url());
        if (url.origin !== origin || !['GET', 'HEAD'].includes(req.method())) {
          result.unexpected.push(`${req.method()} ${url.origin}${url.pathname}`);
          return route.abort();
        }
        return route.continue();
      });
      const check = (condition, message) => { assert(condition, message); current.checks.push(message); };
      const activate = async target => {
        if (spec.touch) await target.tap();
        else { await target.focus(); await target.press('Enter'); }
      };
      const row = id => page.locator(`[data-proxmox-coverage-key="resource:${id}"]`);
      const detail = id => page.locator(`[data-inline-detail-for="resource:${id}"]`);
      const replace = workloads => page.evaluate(next => window.__replaceBackupIdentityWorkloads(next), workloads);
      const capture = async state => {
        const geometry = await page.evaluate(() => ({ overflow: document.documentElement.scrollWidth > innerWidth,
          focus: document.activeElement?.getAttribute('aria-label') || document.activeElement?.textContent?.slice(0, 100) }));
        assert.equal(geometry.overflow, false);
        const file = `${spec.name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), fullPage: true });
        result.captures.push({ file, state, viewport: current.viewport, geometry, sha256: hash(fs.readFileSync(path.join(output, file))) });
      };
      await page.goto(`${origin}/browser-tests/backup-identity.html?view=coverage`, { waitUntil: 'domcontentloaded' });
      await page.waitForFunction(() => typeof window.__replaceBackupIdentityWorkloads === 'function');
      await replace([upper, lower]);
      await row('upper-100').waitFor({ timeout: 45000 });
      await activate(page.getByRole('link', { name: 'Coverage', exact: true }));
      await page.waitForURL('**/proxmox/backups/coverage');
      await row('upper-100').getByText(spec.touch ? 'Latest task: Warning' : 'Warning', { exact: true }).waitFor();
      await row('lower-100').getByText(spec.touch ? 'Latest task: OK' : 'OK', { exact: true }).waitFor();
      for (const [kind, connection] of [['upper', 'East'], ['lower', 'east']]) {
        await activate(row(`${kind}-100`).getByRole('button'));
        const text = await detail(`${kind}-100`).innerText();
        check(text.includes(`${kind}-job`) && text.includes(`${kind}-archive-marker`) && text.includes(`${kind}-snapshot-marker`), `${connection}: job, archive and snapshot stay with exact connection despite opposing historical node`);
        check(text.includes('PBS') && !text.includes(`${kind === 'upper' ? 'lower' : 'upper'}-job`), `${connection}: exact PBS namespace hint and independent latest job`);
      }
      const toggle = row('upper-100').getByRole('button');
      await toggle.focus();
      await replace([lower, upper]);
      check(await toggle.evaluate(element => element === document.activeElement), 'Reordered observations retain focused row owner');
      check(await toggle.getAttribute('aria-expanded') === 'true', 'Open source evidence survives replacement');
      await capture('coverage-exact');
      await activate(page.getByRole('link', { name: 'By date', exact: true }));
      await page.waitForURL('**/proxmox/backups/date');
      const points = page.locator('[data-proxmox-backup-row="recoverable"]');
      await points.first().waitFor();
      check(await points.count() === 6, 'All six source-specific restore observations remain listed');
      const identities = await points.evaluateAll(nodes => nodes.map(node => ({
        key: node.dataset.proxmoxBackupKey,
        connection: node.querySelector('[data-backup-point-workload-connection]')?.textContent.trim(),
      })));
      for (const point of identities) check(point.connection === `Connection: ${point.key.endsWith('upper') ? 'East' : 'east'}`, `${point.key}: By date carries its correct consuming workload connection`);
      const upperPoint = page.locator('[data-proxmox-backup-key="artifact:pbs:pbs-upper"]');
      await activate(upperPoint.getByRole('button', { name: /Backup evidence/ }));
      const dialog = page.getByRole('dialog', { name: 'Backup evidence details' });
      await dialog.waitFor();
      const evidence = await dialog.innerText();
      check(evidence.includes('pbs-upper') && evidence.includes('central-backups / main / East'), 'Actual point dialog preserves the independent PBS record and repository');
      await capture('point-exact');
      if (spec.touch) await activate(dialog.getByRole('button', { name: 'Close backup evidence', exact: true }));
      else await page.keyboard.press('Escape');
      await dialog.waitFor({ state: 'hidden' });
      check(await upperPoint.getByRole('button', { name: /Backup evidence/ }).evaluate(element => element === document.activeElement), 'Closing point evidence restores its invoking focus');
      // Independent next PBS observation is empty; this scenario isolates PVE
      // removed-source ownership, not the unchanged PBS unique-VMID fallback.
      current.phase = 'orphan';
      await replace([lower]);
      await activate(page.getByRole('button', { name: 'Refresh backups', exact: true }));
      await page.getByRole('button', { name: 'Refresh backups', exact: true }).waitFor();
      await activate(page.getByRole('link', { name: 'Coverage', exact: true }));
      await row('lower-100').getByText(spec.touch ? 'Latest task: OK' : 'OK', { exact: true }).waitFor();
      check(!(await row('lower-100').innerText()).includes('Warning'), 'A removed East source cannot lend its job to the remaining east guest');
      const unmatched = page.getByRole('button', { name: /1 unmatched backup target/ });
      await activate(unmatched);
      const orphan = page.locator('[data-proxmox-backup-row="coverage"]').filter({ hasText: 'Connection: East' });
      await orphan.getByText(spec.touch ? 'N/A' : 'Not evaluated', { exact: true }).waitFor();
      await activate(orphan.getByRole('button'));
      const key = await orphan.getAttribute('data-proxmox-coverage-key');
      const orphanDetail = page.locator(`[data-inline-detail-for="${key}"]`);
      const orphanText = await orphanDetail.innerText();
      check(orphanText.includes('upper-job') && orphanText.includes('upper-archive-marker') && !orphanText.includes('lower-job'), 'Collected removed-source evidence remains inspectable, unrated and isolated');
      await capture('removed-source');
      await replace([upper, lower]);
      await row('upper-100').getByText(spec.touch ? 'Latest task: Warning' : 'Warning', { exact: true }).waitFor();
      check(await page.getByRole('button', { name: /unmatched backup target/ }).count() === 0, 'Unchanged PVE evidence returns to its exact guest after ordinary resource recovery');
      check(current.reads.filter(read => read.path === '/api/backups/pve').length === 2, 'Only initial/manual inventory reads; view and resource replacement add none');
      assert.deepEqual(current.pageErrors, []);
      await context.close();
      await browser.close(); browser = undefined;
    }
    checkBinding();
    assert.deepEqual(result.unexpected, []);
    result.passed = true;
  } catch (error) {
    result.failure = { name: error.name, message: error.message, stack: error.stack };
    process.exitCode = 1;
  } finally {
    if (browser) { await browser.close(); result.cleanup.browser = true; }
    else result.cleanup.browser = true;
    if (server) { await server.close(); result.cleanup.server = true; }
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({ passed: result.passed, cases: result.cases.length, captures: result.captures.length, failure: result.failure, cleanup: result.cleanup }));
  }
})();
