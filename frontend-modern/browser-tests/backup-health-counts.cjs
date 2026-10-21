// Production readers, router, health-count action and Coverage, with safe HTTP
// observations only. No provider, guest, backup or recovery operation.
process.env.RAYON_NUM_THREADS = '2';
process.env.UV_THREADPOOL_SIZE = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const http = require('node:http');
const { pathToFileURL } = require('node:url');
const { createRequire } = require('node:module');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const binding = JSON.parse(fs.readFileSync(path.join(root, 'tmp/backup-health-counts/binding.json')));
assert(['before', 'after'].includes(binding.phase));
const output = path.join(root, 'tmp/backup-health-counts', binding.phase);
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const states = ['protected', 'attention', 'unprotected', 'unprotected', 'unknown'];
const guests = states.map((state, index) => ({
  id: `count-${index}`,
  name: `${state}-${index}`,
  type: index % 2 ? 'system-container' : 'vm',
  platformId: 'fixture-east',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'running',
  proxmox: { vmid: 100 + index, instance: 'fixture-east', nodeName: `pve-${index}` },
}));
const fixtures = {
  pve: { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } },
  pbs: {
    data: {
      backups: [{
        id: 'unmatched-backup', instance: 'fixture-pbs', datastore: 'main', namespace: '',
        backupType: 'vm', vmid: '900', backupTime: '2026-10-20T00:00:00Z',
        files: ['manifest.blob'], size: 1024,
      }],
    },
  },
};
const report = {
  binding,
  playwright: require('playwright/package.json').version,
  fixtures,
  guests,
  cases: [],
  captures: [],
  unexpected: [],
  cleanup: {},
  limits: 'Synthetic collected snapshots and server-owned postures through production readers, mapper, router and tables. No native recovery, guest thaw/liveness, publication, production operations or screen-reader speech proof.',
};
let server, browser, current, origin;
const scoped = 'q=missing-workload&node=missing-node&type=host&location=missing-repository';
const clearedKeys = ['q', 'node', 'type', 'location', 'source', 'day', 'view'];
const coverageLimits =
  'Protected rates recorded backup evidence, not current guest health. It does not prove a successful restore, guest thaw or filesystem writes.';
function serve(req, res, next) {
  if (!req.url.startsWith('/api/')) return next();
  try {
    assert.equal(req.method, 'GET');
    const url = new URL(req.url, 'http://fixture.invalid');
    let payload;
    if (url.pathname === '/api/backups/pve') payload = fixtures.pve;
    else if (url.pathname === '/api/backups/pbs') payload = fixtures.pbs;
    else if (url.pathname === '/api/recovery/postures' && current.postureUnavailable) {
      current.requests.push({ path: url.pathname, query: url.search, status: 503 });
      res.writeHead(503, { 'Content-Type': 'application/json' });
      res.end('{"error":"Synthetic unavailable observation"}');
      return;
    }
    else if (url.pathname === '/api/recovery/postures') payload = {
      data: url.searchParams.getAll('resourceId').map((subjectResourceId) => ({
        subjectResourceId,
        state: current.postureOverride?.[subjectResourceId] || states[guests.findIndex((guest) => guest.id === subjectResourceId)] || 'unknown',
        freshness: 'unknown', verification: 'unknown', coverage: 'unknown',
        explanation: 'Synthetic server posture for count navigation, not native recovery evidence.',
        providerStates: [], repositoryResourceIds: [], evidenceIds: [],
        evaluatedAt: '2026-10-20T00:01:00Z',
      })),
      policy: { freshnessWindowSeconds: 604800, verificationWindowSeconds: 604800, requireVerification: true },
    };
    else throw new Error(`Unexpected API ${url.pathname}`);
    current.requests.push({ path: url.pathname, query: url.search, sha256: hash(JSON.stringify(payload)) });
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(payload));
  } catch (error) {
    report.unexpected.push(String(error.stack || error));
    res.writeHead(500, { 'Content-Type': 'application/json' });
    res.end('{}');
  }
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
    for (const [file, digest] of Object.entries(binding.files))
      assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, file);
    assert.equal(report.playwright, JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
    const { build } = await import(pathToFileURL(path.join(frontend, 'node_modules/vite/dist/node/index.js')).href);
    const frontendRequire = createRequire(path.join(frontend, 'package.json'));
    const solid = frontendRequire('vite-plugin-solid');
    const tailwind = (await import(pathToFileURL(frontendRequire.resolve('@tailwindcss/vite')).href)).default;
    const compiled = path.join(output, 'static');
    report.compiled_sources = {};
    await build({
      root: frontend, configFile: false, publicDir: false, logLevel: 'error',
      resolve: { alias: { '@': path.join(frontend, 'src') }, conditions: ['import', 'browser', 'default'] },
      plugins: [tailwind(), solid(), {
        name: 'health-count-source-binding',
        generateBundle() {
          for (const id of this.getModuleIds()) {
            const file = id.split('?')[0];
            if (!file.startsWith(frontend + '/') || file.includes('/node_modules/') || !fs.existsSync(file) || !fs.statSync(file).isFile()) continue;
            const relative = path.relative(root, file), digest = hash(fs.readFileSync(file));
            assert.equal(binding.files[relative], digest, relative);
            report.compiled_sources[relative] = digest;
          }
        },
      }],
      build: { target: 'esnext', outDir: compiled, emptyOutDir: true, rollupOptions: { input: path.join(frontend, 'browser-tests/backup-identity.html') } },
    });
    // The guidance must resolve through the shipped viewer and actual Markdown,
    // not an HTML fallback or a mock document body.
    const doc = 'frontend-modern/public/docs/VM_DISK_MONITORING.md';
    const docBytes = fs.readFileSync(path.join(root, doc));
    assert.equal(hash(docBytes), binding.files[doc]);
    fs.mkdirSync(path.join(compiled, 'docs'), { recursive: true });
    fs.writeFileSync(path.join(compiled, 'docs/VM_DISK_MONITORING.md'), docBytes);
    report.artifacts = {};
    const walk = (dir) => {
      for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const file = path.join(dir, entry.name);
        if (entry.isDirectory()) walk(file);
        else report.artifacts[path.relative(compiled, file)] = hash(fs.readFileSync(file));
      }
    };
    walk(compiled);
    server = http.createServer((req, res) => serve(req, res, () => {
      const requested = new URL(req.url, 'http://fixture.invalid').pathname.slice(1);
      const key = Object.hasOwn(report.artifacts, requested) ? requested : 'browser-tests/backup-identity.html';
      const bytes = fs.readFileSync(path.join(compiled, key));
      assert.equal(hash(bytes), report.artifacts[key]);
      res.setHeader('Content-Type', { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.md': 'text/markdown' }[path.extname(key)] || 'application/octet-stream');
      res.end(bytes);
    }));
    await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
    origin = `http://127.0.0.1:${server.address().port}`;
    for (const spec of [
      { name: 'chromium', engine: chromium, width: 1440, height: 900, touch: false },
      { name: 'webkit', engine: webkit, width: 320, height: 844, touch: true },
    ]) {
      current = { engine: spec.name, viewport: { width: spec.width, height: spec.height }, requests: [], checks: [], errors: [] };
      report.cases.push(current);
      browser = await spec.engine.launch(spec.touch ? { headless: true } : { headless: true, channel: 'chromium', args: ['--no-sandbox'] });
      current.browser_version = browser.version();
      const context = await browser.newContext({ viewport: current.viewport, hasTouch: spec.touch, isMobile: spec.touch, locale: 'en-GB', colorScheme: spec.touch ? 'dark' : 'light' });
      await context.route('**/*', (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== origin || !['GET', 'HEAD'].includes(request.method())) {
          report.unexpected.push(`${request.method()} ${url.origin}${url.pathname}`);
          return route.abort();
        }
        return route.continue();
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      await page.clock.setFixedTime(new Date('2026-10-20T00:02:00Z'));
      if (spec.touch) await page.addInitScript(() => document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.add('dark')));
      page.on('pageerror', (error) => current.errors.push(error.message));
      await page.route('**/*', (route) => {
        const request = route.request(), url = new URL(request.url());
        if (url.origin !== origin || !['GET', 'HEAD'].includes(request.method())) {
          report.unexpected.push(`${request.method()} ${url.origin}${url.pathname}`);
          return route.abort();
        }
        return route.continue();
      });
      const rows = page.locator('[data-proxmox-backup-row="coverage"]');
      const limits = page.locator('[data-backup-coverage-limits]');
      const checkLimits = async () => {
        await expect(page.getByText('Backup coverage', { exact: true })).toBeVisible();
        await expect(limits).toBeVisible();
        await expect(limits).toContainText(coverageLimits);
        await expect(limits.getByRole('link', { name: 'Backup safety guidance' })).toHaveAttribute(
          'href',
          '/docs/VM_DISK_MONITORING',
        );
      };
      const count = (label) => page.getByRole('button', { name: label, exact: true });
      const activate = async (target) => {
        if (spec.touch) await target.tap();
        else { await target.focus(); await target.press('Enter'); }
      };
      const capture = async (name) => {
        await page.evaluate(() => window.scrollTo(0, 0));
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
        const file = `${spec.name}-${name}.png`;
        // The actual viewport contains this count/list interaction. Avoid
        // resizing the page and all responsive/windowed tables for a full-page
        // capture, and give the image encoder its own bounded completion time.
        await page.screenshot({ path: path.join(output, file), fullPage: false, animations: 'disabled', timeout: 30000 });
        report.captures.push({ file, state: name, viewport: current.viewport, sha256: hash(fs.readFileSync(path.join(output, file))) });
      };
      const load = async (route) => {
        await page.goto(`${origin}${route}`, { waitUntil: 'domcontentloaded', timeout: 45000 });
        await page.waitForFunction(() => typeof window.__replaceBackupIdentityWorkloads === 'function');
        await page.evaluate((value) => window.__replaceBackupIdentityWorkloads(value), guests);
        await count('2 unprotected').waitFor();
        await expect(page.getByRole('button', { name: 'Refresh backups', exact: true })).toHaveAttribute('aria-disabled', 'false');
      };
      const initial = `/proxmox/backups/coverage?${scoped}&posture=unprotected&presentation=kept`;
      await load(initial);
      await expect(rows).toHaveCount(0);
      if (binding.phase === 'after') await capture('filtered');
      const reads = current.requests.length;
      await activate(count('2 unprotected'));
      if (binding.phase === 'before') {
        await expect(rows).toHaveCount(0);
        assert.equal(new URL(page.url()).searchParams.get('node'), 'missing-node');
        assert.equal(new URL(page.url()).searchParams.get('posture'), null);
        current.checks.push('Reproduced: global 2 unprotected action toggles posture off, retains four hiding filters, and reveals zero targets.');
        await capture('count-still-empty');
      } else {
        await checkLimits();
        current.checks.push('The coverage summary visibly separates recorded protection from current guest health, restores, thaw and filesystem writes.');
        const expectSelection = async (label, state, ids) => {
          await expect(rows).toHaveCount(ids.length);
          assert.deepEqual(await rows.evaluateAll((items) => items.map((item) => item.dataset.proxmoxCoverageKey).sort()), ids.map((id) => `resource:${id}`).sort());
          const url = new URL(page.url());
          assert.equal(url.pathname, '/proxmox/backups/coverage');
          assert.equal(url.searchParams.get('posture'), state);
          for (const key of clearedKeys) assert.equal(url.searchParams.get(key), null, key);
          assert.equal(url.searchParams.get('presentation'), 'kept');
          await expect(count(label)).toHaveAttribute('aria-pressed', 'true');
          current.checks.push(`${label}: exact counted rows, only posture selected, unrelated URL state preserved.`);
        };
        await expectSelection('2 unprotected', 'unprotected', ['count-2', 'count-3']);
        if (!spec.touch) assert(await count('2 unprotected').evaluate((element) => element === document.activeElement));
        await capture('count-reveals-targets');
        await activate(count('2 unprotected'));
        await expect(rows).toHaveCount(5);
        await expect(count('2 unprotected')).toHaveAttribute('aria-pressed', 'false');
        assert.equal(new URL(page.url()).searchParams.get('posture'), null);
        if (!spec.touch) assert(await count('2 unprotected').evaluate((element) => element === document.activeElement));
        assert.equal(current.requests.length, reads, 'Count selection and toggle must not read inventory or postures.');
        current.checks.push('Toggle off reveals all five rated targets, keeps focus and adds no reads; unmatched evidence stays separate.');
        for (const [route, label, state, ids] of [
          [`/proxmox/backups/date?${scoped}&day=2026-10-19&source=verified&presentation=kept`, '1 attention', 'attention', ['count-1']],
          [`/proxmox/backups?view=coverage&${scoped}&presentation=kept`, '1 unknown', 'unknown', ['count-4']],
          [`/proxmox/backups/coverage?${scoped}&posture=protected&presentation=kept`, '1 protected', 'protected', ['count-0']],
        ]) {
          await load(route);
          const before = current.requests.length;
          await expect(count(label)).toHaveAttribute('aria-pressed', 'false');
          await activate(count(label));
          await expectSelection(label, state, ids);
          assert.equal(current.requests.length, before, 'Navigation must not refresh inventory or posture.');
        }
        await checkLimits();
        await capture('protected-target');
        await activate(page.getByRole('link', { name: 'By date', exact: true }));
        assert.equal(new URL(page.url()).searchParams.get('posture'), null);
        await expect(page.locator('[data-proxmox-backup-row="recoverable"]')).toHaveCount(1);
        await checkLimits();
        await capture('date-coverage-limits');
        current.checks.push('Both Coverage and By date retain the same visible protection limits while ordinary view navigation and evidence remain intact.');
        await load('/proxmox/backups/coverage?posture=protected&node=pve-0&presentation=kept');
        await expect(count('1 protected')).toHaveAttribute('aria-pressed', 'false');
        current.postureOverride = { 'count-0': 'unknown' };
        await activate(page.getByRole('button', { name: 'Refresh backups', exact: true }));
        await count('0 protected').waitFor();
        await expect(count('0 protected')).toHaveAttribute('aria-pressed', 'false');
        await activate(count('0 protected'));
        await expectSelection('0 protected', 'protected', []);
        await capture('active-zero-count');
        await activate(count('0 protected'));
        await expect(rows).toHaveCount(5);
        assert.equal(await count('0 protected').count(), 0);
        current.checks.push('A real inventory/posture refresh to zero keeps the selected count reachable; it clears scopes before toggling off, without an invented healthy target.');
        current.postureOverride = undefined;
        current.postureUnavailable = true;
        await activate(page.getByRole('button', { name: 'Refresh backups', exact: true }));
        await expect(page.getByText(/Protection posture is unavailable\./)).toBeVisible();
        await count('5 unknown').waitFor();
        await checkLimits();
        await capture('unavailable-coverage-limits');
        current.postureUnavailable = false;
        await activate(page.getByRole('button', { name: 'Retry protection posture', exact: true }));
        await count('1 protected').waitFor();
        await checkLimits();
        const readsBeforeDoc = current.requests.length;
        const sourceUrl = page.url();
        const popupPromise = context.waitForEvent('page');
        await activate(limits.getByRole('link', { name: 'Backup safety guidance' }));
        const docPage = await popupPromise;
        await docPage.waitForURL(`${origin}/docs/VM_DISK_MONITORING`);
        await expect(
          docPage.getByRole('heading', { name: 'Backup safety', exact: true }),
        ).toBeVisible();
        await expect(docPage.locator('article')).toContainText(
          'every filesystem covered by the backup',
        );
        assert.equal(await docPage.evaluate(() => window.opener === null), true);
        const file = `${spec.name}-shipped-safety-guidance.png`;
        await docPage.screenshot({
          path: path.join(output, file),
          animations: 'disabled',
          timeout: 30000,
        });
        report.captures.push({
          file,
          state: 'actual-shipped-guidance',
          viewport: current.viewport,
          sha256: hash(fs.readFileSync(path.join(output, file))),
        });
        await docPage.close();
        assert.equal(page.url(), sourceUrl);
        assert.equal(current.requests.length, readsBeforeDoc);
        await checkLimits();
        current.checks.push(
          'Unavailable posture withdraws green verdicts without hiding safety limits; retry restores the server verdict. Keyboard/touch opens real shipped guidance with no opener, extra inventory read, mutation or loss of the backup view.',
        );
      }
      assert.deepEqual(current.errors, []);
      await context.close();
      await browser.close(); browser = undefined;
    }
    assert.deepEqual(report.unexpected, []);
    report.result = binding.phase === 'before' ? 'reproduced' : 'passed';
  } catch (error) {
    report.result = 'failed'; report.failure = String(error.stack || error); process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    report.cleanup.browser_closed = true;
    if (server) await new Promise((resolve) => server.close(resolve));
    report.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify({ result: report.result, failure: report.failure, cases: report.cases.map(({ engine, checks, errors }) => ({ engine, checks, errors })), captures: report.captures, cleanup: report.cleanup }, null, 2));
  }
})();
