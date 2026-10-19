// Real HTTP -> production backup decoder/model/Coverage -> shipped CSS.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/unmatched-backup-jobs/browser');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json')));
for (const [file, digest] of Object.entries(binding.files))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Source changed: ${file}`);
const datedError = 'Cannot start backup <img src=x onerror="window.__jobInjected=true">';
const undatedError = 'retired-storage-unavailable / original job evidence';
const tasks = [
  {
    id: 'ambiguous-job-100',
    type: 'vzdump',
    vmid: 100,
    instance: '',
    node: 'shared-node',
    status: 'ERROR: cannot start backup',
    error: datedError,
    startTime: '2026-10-19T09:00:00Z',
    endTime: '2026-10-19T09:01:00Z',
    observedAt: '2026-10-19T10:00:00Z',
  },
  {
    id: 'retired-job-300',
    type: 'vzdump',
    vmid: 300,
    instance: 'retired-connection',
    node: 'old-node',
    status: 'error',
    error: undatedError,
    startTime: '',
    observedAt: '2026-10-19T10:00:00Z',
  },
  {
    id: 'aggregate-parent',
    type: 'vzdump',
    vmid: 0,
    instance: 'east',
    node: 'shared-node',
    status: 'error',
    startTime: '2026-10-19T08:00:00Z',
  },
];
const pve = { data: { backupTasks: tasks, storageBackups: [], guestSnapshots: [] } };
const pbs = {
  data: {
    backups: [
      {
        id: 'pbs-main/east/vm/100',
        instance: 'pbs-main',
        datastore: 'main',
        namespace: 'east',
        vmid: '100',
        backupType: 'vm',
        backupTime: '2026-10-19T02:00:00Z',
        files: ['index.json.blob'],
        size: 1024,
      },
    ],
  },
};
const posture = {
  data: [],
  policy: {
    freshnessWindowSeconds: 604800,
    verificationWindowSeconds: 604800,
    requireVerification: true,
  },
};
const report = {
  binding,
  playwright: require('playwright/package.json').version,
  fixtures: { pve, pbs, posture },
  cases: [],
  captures: [],
  errors: [],
  cleanup: {},
};
let server, browser, current;
function serve(req, res, next) {
  if (!req.url.startsWith('/api/')) return next();
  try {
    assert.equal(req.method, 'GET', 'No product mutation, backup or guest operation');
    const url = new URL(req.url, 'http://fixture.invalid');
    assert(
      ['/api/backups/pve', '/api/backups/pbs', '/api/recovery/postures'].includes(url.pathname),
    );
    current.requests.push(url.pathname);
    const value = url.pathname.endsWith('/pve')
      ? pve
      : url.pathname.endsWith('/pbs')
        ? pbs
        : posture;
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(value));
  } catch (error) {
    report.errors.push(String(error.stack || error));
    res.writeHead(500, { 'Content-Type': 'application/json' });
    res.end('{}');
  }
}
(async () => {
  try {
    assert.equal(
      report.playwright,
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
          name: 'offline-unmatched-jobs',
          configureServer(s) {
            s.middlewares.use(serve);
          },
        },
      ],
      server: { host: '127.0.0.1', port: 0, strictPort: false },
      optimizeDeps: { noDiscovery: true, include: ['solid-js', 'solid-js/web', '@solidjs/router'] },
    });
    await server.listen();
    const origin = `http://127.0.0.1:${server.httpServer.address().port}`;
    for (const [name, engine, narrow] of [
      ['chromium', chromium, false],
      ['webkit', webkit, true],
    ]) {
      current = { name, requests: [] };
      browser = await engine.launch(
        narrow
          ? { headless: true }
          : { headless: true, channel: 'chromium', args: ['--no-sandbox'] },
      );
      current.browserVersion = browser.version();
      current.viewport = { width: narrow ? 320 : 1365, height: 1000 };
      const context = await browser.newContext({
        viewport: current.viewport,
        isMobile: narrow,
        hasTouch: narrow,
        locale: 'en-GB',
        timezoneId: 'UTC',
        colorScheme: narrow ? 'dark' : 'light',
      });
      const page = await context.newPage();
      page.on('pageerror', (error) => report.errors.push(`${name}: ${error.message}`));
      page.on('request', (request) => assert.equal(new URL(request.url()).origin, origin));
      const now = new Date('2026-10-19T12:00:00Z');
      await page.clock.install({ time: now });
      await page.clock.pauseAt(now);
      await page.goto(`${origin}/browser-tests/unmatched-backup-jobs.html?view=coverage`, {
        waitUntil: 'domcontentloaded',
      });
      if (narrow) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const activate = async (locator) => {
        if (narrow) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      const capture = async (state) => {
        const file = `${name}-${state}.png`;
        await page.screenshot({
          path: path.join(output, file),
          fullPage: true,
          animations: 'disabled',
        });
        report.captures.push({
          file,
          state,
          engine: name,
          viewport: current.viewport,
          sha256: hash(fs.readFileSync(path.join(output, file))),
        });
      };
      const row = (text) =>
        page.locator('[data-proxmox-backup-row="coverage"]').filter({ hasText: text });
      const toggleRow = async (text) => {
        if (narrow) await row(text).tap();
        else await activate(row(text).getByRole('button'));
      };
      const counts = async (targets) => {
        await expect(
          page.getByText(new RegExp(`${targets} targets ·\\s*1 restore points`)),
        ).toBeVisible();
        await expect(page.getByText('aggregate-parent', { exact: true })).toHaveCount(0);
      };
      await counts(2);
      await activate(page.getByRole('button', { name: /2 unmatched backup targets/ }));
      await expect(row('Guest 100')).toContainText('Failed');
      await expect(row('east-guest')).not.toContainText('Failed');
      await expect(row('west-guest')).not.toContainText('Failed');
      await toggleRow('Guest 100');
      const latest = page.getByRole('region', { name: 'Latest backup job' });
      await expect(latest).toContainText('ambiguous-job-100');
      await expect(latest).toContainText(datedError);
      await expect(latest).toContainText('shared-node');
      await expect(latest.locator('dd').filter({ hasText: /^Not reported$/ })).toBeVisible();
      await expect(latest.locator('img')).toHaveCount(0);
      assert.equal(await page.evaluate(() => window.__jobInjected), undefined);
      await capture('unmatched-dated');
      await toggleRow('Guest 100');
      await toggleRow('Guest 300');
      const collected = page.getByRole('region', { name: 'Collected backup jobs' });
      await expect(collected).toContainText('retired-job-300');
      await expect(collected).toContainText(undatedError);
      await expect(collected).toContainText('retired-connection');
      await expect(page.getByRole('region', { name: 'Latest backup job' })).toHaveCount(0);
      await capture('unmatched-undated');
      const search = page.getByPlaceholder('Search backups by workload, node, source, or status');
      await search.fill('retired-job-300');
      await expect(row('Guest 300')).toBeVisible();
      await expect(row('Guest 100')).toHaveCount(0);
      await counts(2);
      await capture('searched-evidence');
      await search.fill('');
      await activate(page.getByRole('button', { name: 'Fixture: resolve ambiguity' }));
      await counts(1);
      await expect(page.getByRole('button', { name: /1 unmatched backup target/ })).toBeVisible();
      await expect(row('Guest 100')).toHaveCount(0);
      await expect(row('east-guest')).toContainText('Failed');
      await toggleRow('east-guest');
      await expect(page.getByRole('region', { name: 'Latest backup job' })).toContainText(
        'ambiguous-job-100',
      );
      await capture('resolved-owner');
      await activate(page.getByRole('button', { name: 'Fixture: restore ambiguity' }));
      await counts(2);
      await expect(row('east-guest')).not.toContainText('Failed');
      await expect(row('west-guest')).not.toContainText('Failed');
      await expect(row('Guest 100')).toContainText('Failed');
      assert.equal(current.requests.filter((p) => p === '/api/backups/pve').length, 1);
      assert.equal(current.requests.filter((p) => p === '/api/backups/pbs').length, 1);
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
        false,
      );
      current.pageErrors = [...report.errors];
      await context.close();
      current.contextClosed = true;
      await browser.close();
      browser = undefined;
      current.browserClosed = true;
      report.cases.push(current);
    }
    assert.deepEqual(report.errors, []);
  } catch (error) {
    report.failure = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) {
      await browser.close();
      report.cleanup.failureBrowserClosed = true;
    }
    if (server) {
      await server.close();
      report.cleanup.serverClosed = true;
    }
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2));
    console.log(
      JSON.stringify({
        cases: report.cases.length,
        captures: report.captures.length,
        failure: report.failure ?? null,
        cleanup: report.cleanup,
      }),
    );
  }
})();
