// Synthetic read-only HTTP inventory -> production Backups reader, model, table
// and disclosure. No provider, guest, backup, restore or recovery operation.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/pbs-counts/browser');
const binding = JSON.parse(fs.readFileSync(path.join(root, 'tmp/pbs-counts/binding.json')));
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const backup = (instance, datastore, id) => ({
  id,
  instance,
  datastore,
  namespace: '',
  backupType: 'vm',
  vmid: '100',
  backupTime: '2026-10-19T01:00:00Z',
  size: 100,
  protected: false,
  verified: true,
  files: ['manifest.blob'],
});
const other = [backup('pbs-backup', 'tank', 'other-1'), backup('pbs-backup', 'tank', 'other-2')];
const legacy = backup('legacy-archive', 'tank', 'legacy-1');
const frames = {
  initial: { data: { backups: [backup('backup', 'offsite', 'own-offsite'), ...other, legacy] } },
  empty: { data: { backups: [...other, legacy] } },
  failure: { data: { backups: 'INVALID_RESPONSE_SENTINEL' } },
  recovered: { data: { backups: [backup('backup', 'tank', 'own-tank'), ...other, legacy] } },
};
const result = {
  passed: false,
  source: binding.source,
  tree: binding.tree,
  playwright: require('playwright/package.json').version,
  protocol_sha256: hash(fs.readFileSync(__filename)),
  binding_sha256: hash(fs.readFileSync(path.join(root, 'tmp/pbs-counts/binding.json'))),
  cases: [],
  captures: [],
  unexpected: [],
  cleanup: {},
  limits:
    'Safe synthetic inventory and resource observations; actual page/query/decoder/count producer/rows/CSS. No production embed, native PBS, guest liveness, restore, thaw, installed recovery, release or screen-reader speech proof.',
};
let server, browser, current, origin, delayed;
const checkBinding = () => {
  for (const [file, digest] of Object.entries(binding.files))
    assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, file);
};
function serve(req, res, next) {
  if (!req.url.startsWith('/api/')) return next();
  assert.equal(req.method, 'GET');
  const url = new URL(req.url, 'http://fixture.invalid');
  let payload;
  if (url.pathname === '/api/backups/pbs') {
    const phase = current.phase;
    payload = frames[phase];
    current.reads.push({ path: url.pathname, phase, sha256: hash(JSON.stringify(payload)) });
    const send = () => {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(payload));
    };
    if (current.hold) {
      delayed = send;
      return;
    }
    send();
    return;
  }
  if (url.pathname === '/api/backups/pve')
    payload = { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } };
  else if (url.pathname.includes('runtime-capabilities'))
    payload = {
      capabilities: [],
      limits: [],
      max_history_days: 7,
      hosted_mode: false,
      runtime: { build: 'community' },
      blocked_capabilities: [],
    };
  else if (url.pathname.includes('/metrics-store/history'))
    payload = {
      resourceType: url.searchParams.get('resourceType'),
      resourceId: url.searchParams.get('resourceId'),
      range: url.searchParams.get('range'),
      start: 1,
      end: 2,
      metrics: {},
    };
  else payload = { data: [], alerts: [], anomalies: [], enabled: false };
  res.setHeader('Content-Type', 'application/json');
  res.end(JSON.stringify(payload));
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
    checkBinding();
    assert.equal(
      result.playwright,
      JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    const { createServer } = await import(
      pathToFileURL(path.join(frontend, 'node_modules/vite/dist/node/index.js')).href
    );
    server = await createServer({
      root: frontend,
      configFile: path.join(frontend, 'vite.config.ts'),
      cacheDir: path.join(output, 'vite-cache'),
      logLevel: 'error',
      plugins: [
        {
          name: 'pbs-count-reads',
          configureServer(s) {
            s.middlewares.use(serve);
          },
        },
      ],
      server: { host: '127.0.0.1', port: 0, strictPort: false, proxy: {} },
    });
    await server.listen();
    origin = `http://127.0.0.1:${server.httpServer.address().port}`;
    for (const spec of [
      {
        name: 'chromium',
        engine: chromium,
        width: 1440,
        height: 1000,
        touch: false,
        transport: 'direct',
      },
      {
        name: 'webkit',
        engine: webkit,
        width: 320,
        height: 1000,
        touch: true,
        transport: 'canonical',
      },
    ]) {
      current = {
        engine: spec.name,
        viewport: { width: spec.width, height: spec.height },
        transport: spec.transport,
        phase: 'initial',
        hold: false,
        reads: [],
        checks: [],
        pageErrors: [],
      };
      result.cases.push(current);
      browser = await spec.engine.launch(
        spec.touch
          ? { headless: true }
          : { headless: true, channel: 'chromium', args: ['--no-sandbox'] },
      );
      current.browserVersion = browser.version();
      const context = await browser.newContext({
        viewport: current.viewport,
        hasTouch: spec.touch,
        isMobile: spec.touch,
        locale: 'en-GB',
        timezoneId: 'UTC',
        colorScheme: spec.touch ? 'dark' : 'light',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(12000);
      await page.clock.setFixedTime(new Date('2026-10-19T12:01:00Z'));
      if (spec.touch)
        await page.addInitScript(() =>
          document.addEventListener('DOMContentLoaded', () =>
            document.documentElement.classList.add('dark'),
          ),
        );
      await page.route('**/*', (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin || !['GET', 'HEAD'].includes(req.method())) {
          result.unexpected.push(`${req.method()} ${url.origin}${url.pathname}`);
          return route.abort();
        }
        return route.continue();
      });
      page.on('pageerror', (error) => current.pageErrors.push(error.message));
      const table = page.locator('[data-proxmox-backups-table="servers"]');
      const row = (name, store) =>
        table
          .locator('tr')
          .filter({ has: page.locator(`td[title="${name} · ${store}"]`) })
          .first();
      const detail = (name, store) =>
        page.getByRole('region', {
          name: `Datastore capacity for ${name} / ${store}`,
          exact: true,
        });
      const check = (condition, text) => {
        assert(condition, text);
        current.checks.push(text);
      };
      const activate = async (button) => {
        if (spec.touch) await button.tap();
        else {
          await button.focus();
          await button.press('Enter');
        }
      };
      const open = async (name, store) => {
        const target = row(name, store);
        if (spec.touch) await target.tap();
        else
          await activate(
            target.getByRole('button', { name: `Expand details for ${name}`, exact: true }),
          );
        await detail(name, store).waitFor();
      };
      const expectCount = async (name, store, count) => {
        const content = detail(name, store)
          .locator('dl div')
          .filter({ hasText: 'Backup entries:' });
        await content.getByText(String(count), { exact: true }).waitFor();
        check(
          (await content.innerText()).includes(`Backup entries: ${count}`),
          `${name}/${store}: ${count}`,
        );
        if (!spec.touch) {
          const index = await table
            .locator('th')
            .evaluateAll((heads) =>
              heads.findIndex((head) => head.getAttribute('aria-label') === 'Backups'),
            );
          assert.equal(
            (await row(name, store).locator('td').nth(index).innerText()).trim(),
            String(count),
          );
        }
      };
      const capture = async (state) => {
        const geometry = await page.evaluate(() => ({
          overflow: document.documentElement.scrollWidth > innerWidth,
          clippedDetails: [
            ...document.querySelectorAll('section[aria-label^="Datastore capacity"]'),
          ]
            .filter((el) => {
              const r = el.getBoundingClientRect();
              return r.left < -1 || r.right > innerWidth + 1;
            })
            .map((el) => el.getAttribute('aria-label')),
          focus:
            document.activeElement?.getAttribute('aria-label') ||
            document.activeElement?.textContent?.slice(0, 100),
        }));
        assert.equal(geometry.overflow, false);
        assert.deepEqual(geometry.clippedDetails, []);
        const file = `${spec.name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), fullPage: true });
        result.captures.push({
          file,
          sha256: hash(fs.readFileSync(path.join(output, file))),
          engine: spec.name,
          viewport: current.viewport,
          state,
          geometry,
        });
      };
      await page.goto(
        `${origin}/browser-tests/pbs-count-sources.html?transport=${spec.transport}`,
        { waitUntil: 'networkidle' },
      );
      await table.waitFor();
      await open('backup', 'tank');
      await expectCount('backup', 'tank', 0);
      await open('pbs-backup', 'tank');
      await expectCount('pbs-backup', 'tank', 2);
      await open('archive', 'tank');
      await expectCount('archive', 'tank', 1);
      await capture('isolated');
      current.phase = 'empty';
      await activate(page.getByRole('button', { name: 'Refresh backups', exact: true }));
      await expectCount('backup', 'tank', 0);
      await expectCount('pbs-backup', 'tank', 2);
      await page.getByRole('button', { name: 'Refresh backups', exact: true }).waitFor();
      current.phase = 'failure';
      await activate(page.getByRole('button', { name: 'Refresh backups', exact: true }));
      await detail('backup', 'tank').getByText('Unavailable', { exact: true }).waitFor();
      check(
        !(await page.locator('body').innerText()).includes('INVALID_RESPONSE_SENTINEL'),
        'Invalid response bytes never reach the page',
      );
      await capture('unavailable');
      current.phase = 'recovered';
      current.hold = true;
      await activate(page.getByRole('button', { name: 'Retry PBS inventory', exact: true }));
      const started = Date.now();
      while (!delayed) {
        if (Date.now() - started > 5000) throw new Error('Missing held inventory read');
        await new Promise((resolve) => setTimeout(resolve, 15));
      }
      const toggle = row('backup', 'tank').getByRole('button', {
        name: 'Collapse details for backup',
        exact: true,
      });
      await toggle.focus();
      await toggle.evaluate((el) => {
        window.__pbsFocusedToggle = el;
      });
      current.hold = false;
      delayed();
      delayed = undefined;
      await expectCount('backup', 'tank', 1);
      await expectCount('pbs-backup', 'tank', 2);
      check(
        await toggle.evaluate(
          (el) =>
            el === window.__pbsFocusedToggle &&
            document.activeElement === el &&
            el.getAttribute('aria-expanded') === 'true',
        ),
        'Retry recovery keeps the same open disclosure and actual focus',
      );
      await capture('recovered');
      await page.evaluate(() => window.__pbsCountSources.setAmbiguous(true));
      await open('duplicate', 'tank');
      await expectCount('duplicate', 'tank', 'Unknown');
      await capture('ambiguous');
      assert.deepEqual(current.pageErrors, []);
      assert.equal(current.reads.length, 4);
      await context.close();
      current.contextClosed = true;
      await browser.close();
      current.browserClosed = true;
      browser = undefined;
    }
    result.loadedModules = [];
    for (const [file] of server.moduleGraph.fileToModulesMap) {
      if (file.startsWith(frontend + '/src/') && fs.existsSync(file)) {
        const relative = path.relative(root, file),
          digest = hash(fs.readFileSync(file));
        assert.equal(binding.files[relative], digest, relative);
        result.loadedModules.push({ file: relative, sha256: digest });
      }
    }
    for (const file of [
      'ProxmoxBackupsTable.tsx',
      'proxmoxBackupInventory.ts',
      'ProxmoxBackupServersTable.tsx',
    ])
      assert(
        result.loadedModules.some((module) => module.file.endsWith('/' + file)),
        file,
      );
    checkBinding();
    assert.deepEqual(result.unexpected, []);
    result.passed = true;
  } catch (error) {
    result.error = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (delayed) delayed();
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    if (server) await server.close();
    result.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        passed: result.passed,
        captures: result.captures.length,
        error: result.error,
        cleanup: result.cleanup,
      }),
    );
  }
})();
