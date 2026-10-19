// Real HTTP -> production backup decoder, exact-location resolver, tables and CSS.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/backup-location-scope/browser-v4');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json')));
for (const [file, digest] of Object.entries(binding.files))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Source changed: ${file}`);
const pbsPoint = (datastore, hour) => ({
  id: `pbs-server/${datastore}/vm/100`,
  instance: 'pbs-server',
  datastore,
  namespace: '',
  backupType: 'vm',
  vmid: '100',
  backupTime: `2026-10-19T0${hour}:00:00Z`,
  files: ['index.json.blob'],
  size: 1024,
  verified: true,
});
const archive = (node) => ({
  id: `east/local/${node}/100`,
  instance: 'east',
  node: 'pve-1',
  storageNode: node,
  storage: 'local',
  vmid: 100,
  type: 'vm',
  time: '2026-10-19T01:00:00Z',
  size: 2048,
  volid: `local:backup/vzdump-qemu-100-${node}.vma.zst`,
  format: 'zst',
});
const fixtures = {
  pbs: { data: { backups: [pbsPoint('main', 2), pbsPoint('Main', 3)] } },
  pve: {
    data: {
      storageBackups: [archive('node-a'), archive('node-b')],
      guestSnapshots: [],
      backupTasks: [],
    },
  },
  postures: {
    data: [],
    policy: {
      freshnessWindowSeconds: 604800,
      verificationWindowSeconds: 604800,
      requireVerification: true,
    },
  },
};
const report = {
  binding,
  playwright: require('playwright/package.json').version,
  fixtures,
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
      ? fixtures.pve
      : url.pathname.endsWith('/pbs')
        ? fixtures.pbs
        : fixtures.postures;
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(value));
  } catch (error) {
    report.errors.push(String(error.stack || error));
    res.writeHead(500);
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
          name: 'offline-location-scope',
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
    // Desktop completed at c4bdb7a45c with these identical product bytes.
    // Finish only the narrow journey after correcting its collapsed-menu precondition.
    for (const [name, engine, narrow] of [['webkit', webkit, true]]) {
      current = { name, requests: [], phases: [], localRequests: [] };
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
      page.on('request', (req) => {
        assert.equal(new URL(req.url()).origin, origin);
        current.localRequests.push(new URL(req.url()).pathname);
      });
      const now = new Date('2026-10-19T12:00:00Z');
      await page.clock.install({ time: now });
      await page.clock.pauseAt(now);
      const activate = async (locator) => {
        if (narrow) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      const showFilters = async () => {
        const menu = page.getByRole('combobox', { name: 'Filter', exact: true });
        if (!(await menu.isVisible()))
          await activate(page.getByRole('button', { name: /^Filters/ }));
        await expect(menu).toBeVisible();
      };
      const capture = async (state) => {
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
          false,
          `Overflow: ${state}`,
        );
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
      const rows = () => page.locator('[data-proxmox-backup-row="recoverable"]');
      const visit = async (selected) => {
        current.phases.push({ phase: 'navigation-start', selected });
        await page.goto(
          `${origin}/browser-tests/backup-location-scope.html?view=date${selected ? `&location=${encodeURIComponent(selected)}` : ''}`,
          { waitUntil: 'domcontentloaded', timeout: 60000 },
        );
        current.phases.push({ phase: 'dom-loaded' });
        if (narrow) await page.evaluate(() => document.documentElement.classList.add('dark'));
        await expect(
          page.getByRole('button', { name: 'Refresh backups', exact: true }),
        ).toBeVisible();
        await showFilters();
      };
      await visit('');
      await expect(rows()).toHaveCount(4);
      const option = page.getByRole('option', {
        name: 'Backup location: pbs-server / main',
        exact: true,
      });
      const value = await option.getAttribute('value');
      await page.getByRole('combobox', { name: 'Filter', exact: true }).selectOption(value);
      await expect(rows()).toHaveCount(1);
      await expect(rows()).toHaveAttribute(
        'data-proxmox-backup-key',
        'artifact:pbs:pbs-server/main/vm/100',
      );
      await expect(page.getByText('pbs-server / main / (root)', { exact: true })).toBeVisible();
      await expect(page.getByText('pbs-server / Main / (root)', { exact: true })).toHaveCount(0);
      const selected = new URL(page.url()).searchParams.get('location');
      assert.equal(selected, 'backup-location-v2:["pbs","pbs-server","main"]');
      await activate(page.getByRole('link', { name: 'Coverage', exact: true }));
      const coverage = page.locator('[data-proxmox-backup-row="coverage"]');
      await expect(coverage).toHaveCount(1);
      if (narrow) await coverage.tap();
      else await activate(coverage.getByRole('button'));
      await expect(page.getByText('pbs-server / main / (root)', { exact: true })).toBeVisible();
      await expect(page.getByText('pbs-server / Main / (root)', { exact: true })).toHaveCount(0);
      assert.equal(new URL(page.url()).searchParams.get('location'), selected);
      await capture('exact-lower-coverage');
      // Existing saved views cannot guess which of these stores was intended.
      await visit('pbs:pbs-server:main');
      await expect(
        page
          .getByRole('status')
          .filter({ hasText: 'saved backup location matches more than one source' }),
      ).toBeVisible();
      await expect(rows()).toHaveCount(0);
      await capture('ambiguous-saved-location');
      await activate(
        page.getByRole('button', {
          name: 'Backup location: Ambiguous saved location',
          exact: true,
        }),
      );
      const filterValues = page.getByRole('textbox', { name: 'Filter Backup location values' });
      await expect(filterValues).toBeFocused();
      // Case-insensitive search can return both; choosing the exact option is
      // the operation that must not pool their inventories.
      await filterValues.fill('pbs-server / Main');
      const exactUpper = page.getByRole('option', { name: 'pbs-server / Main', exact: true });
      await activate(exactUpper);
      await expect(rows()).toHaveCount(1);
      await expect(rows()).toHaveAttribute(
        'data-proxmox-backup-key',
        'artifact:pbs:pbs-server/Main/vm/100',
      );
      await expect(page.getByText('pbs-server / Main / (root)', { exact: true })).toBeVisible();
      await expect(page.getByText('pbs-server / main / (root)', { exact: true })).toHaveCount(0);
      await expect(
        page
          .getByRole('status')
          .filter({ hasText: 'saved backup location matches more than one source' }),
      ).toHaveCount(0);
      assert.equal(
        new URL(page.url()).searchParams.get('location'),
        'backup-location-v2:["pbs","pbs-server","Main"]',
      );
      await capture('exact-upper-recovered');
      // A unique legacy aggregate keeps both local copies, not either PBS store.
      await visit('archive:east:local');
      await expect(rows()).toHaveCount(2);
      await expect(rows().nth(0)).toHaveAttribute(
        'data-proxmox-backup-key',
        'artifact:archive:east/local/node-a/100',
      );
      await expect(rows().nth(1)).toHaveAttribute(
        'data-proxmox-backup-key',
        'artifact:archive:east/local/node-b/100',
      );
      await expect(page.getByText('east / local / node-a', { exact: true })).toBeVisible();
      await expect(page.getByText('east / local / node-b', { exact: true })).toBeVisible();
      await expect(
        page.getByRole('button', { name: 'Backup location: east / local', exact: true }),
      ).toBeVisible();
      await capture('legacy-local-group');
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
    report.failedCase = current;
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
