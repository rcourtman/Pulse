// Existing GET readers -> production chronology -> By date/Coverage and shipped CSS.
// Synthetic collected evidence only: no backup, guest, collection or provider action.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/backup-clock/browser');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json')));
for (const [file, digest] of Object.entries(binding.files))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Changed source: ${file}`);
const start = Date.parse('2026-10-19T22:59:50Z'); // 23:59:50 Europe/London.
const iso = (time) => new Date(time).toISOString();
const old = '2026-10-13T11:00:00Z';
const recent = '2026-10-19T11:00:00Z';
const archive = (id, time, size) => ({
  id,
  instance: 'east',
  node: 'pve-1',
  storage: 'local',
  vmid: 100,
  type: 'vm',
  time,
  size,
  volid: `local:backup/${id}-qemu-100.vma.zst`,
  format: 'zst',
});
const fixtures = {
  pve: {
    data: {
      backupTasks: [
        {
          id: 'clock-job-100',
          instance: 'east',
          node: 'pve-1',
          vmid: 100,
          type: 'vzdump',
          status: 'OK',
          startTime: iso(start + 5000),
          endTime: iso(start + 10000),
          observedAt: iso(start - 60000),
        },
      ],
      storageBackups: [archive('old', old, 2048), archive('recent', recent, 1024)],
      guestSnapshots: [],
    },
  },
  pbs: {
    data: {
      backups: [
        {
          id: 'clock-pbs-100',
          instance: 'pbs-fixture',
          datastore: 'main',
          namespace: 'pve-1',
          vmid: '100',
          backupType: 'vm',
          backupTime: iso(start + 5000),
          size: 4096,
          files: ['index.json.blob'],
          verified: true,
        },
      ],
    },
  },
  postures: {
    data: [
      {
        subjectResourceId: 'east-100',
        state: 'protected',
        freshness: 'current',
        verification: 'verified',
        coverage: 'complete',
        explanation: 'Server-owned fixture verdict.',
        evaluatedAt: iso(start),
        providerStates: [],
        repositoryResourceIds: [],
        evidenceIds: [],
      },
    ],
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
    assert.equal(req.method, 'GET', 'No mutation, backup or guest requests');
    const url = new URL(req.url, 'http://fixture.invalid');
    assert(
      ['/api/backups/pve', '/api/backups/pbs', '/api/recovery/postures'].includes(url.pathname),
    );
    current.requests.push({ method: req.method, path: url.pathname });
    if (url.pathname === '/api/recovery/postures')
      assert.deepEqual(url.searchParams.getAll('resourceId'), ['east-100']);
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(
      JSON.stringify(
        url.pathname.endsWith('/pve')
          ? fixtures.pve
          : url.pathname.endsWith('/pbs')
            ? fixtures.pbs
            : fixtures.postures,
      ),
    );
  } catch (error) {
    report.errors.push(String(error.stack || error));
    res.end('{}');
  }
}
const dayPattern = /: \d+ backups?(, selected)?$/;
const activityCount = async (page) => {
  const labels = await page
    .getByRole('button', { name: dayPattern })
    .evaluateAll((els) => els.map((el) => el.getAttribute('aria-label')));
  return labels.reduce((sum, label) => sum + Number(label.match(/: (\d+) backups?/)[1]), 0);
};
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
          name: 'backup-clock-http-fixtures',
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
      current = {
        name,
        requests: [],
        pageErrors: [],
        viewport: { width: narrow ? 320 : 1365, height: 1000 },
        timezone: 'Europe/London',
        clockStates: [],
      };
      browser = await engine.launch(
        narrow
          ? { headless: true }
          : { headless: true, channel: 'chromium', args: ['--no-sandbox'] },
      );
      current.browserVersion = browser.version();
      const context = await browser.newContext({
        viewport: current.viewport,
        isMobile: narrow,
        hasTouch: narrow,
        timezoneId: current.timezone,
        locale: 'en-GB',
        colorScheme: narrow ? 'dark' : 'light',
      });
      const page = await context.newPage();
      page.on('pageerror', (error) => current.pageErrors.push(error.message));
      page.on('request', (req) => assert.equal(new URL(req.url()).origin, origin));
      await page.clock.install({ time: new Date(start) });
      await page.clock.pauseAt(new Date(start));
      const activate = async (locator) => {
        if (narrow) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      const capture = async (state) => {
        const geometry = await page.evaluate(() => ({
          overflow: document.documentElement.scrollWidth > innerWidth,
          clipped: [
            ...document.querySelectorAll(
              '[data-backup-activity-scope], [data-backup-volume-evidence], [data-backup-task-detail]',
            ),
          ]
            .filter((el) => el.scrollWidth > el.clientWidth + 1)
            .map((el) => el.textContent),
          localNow: new Date().toString(),
        }));
        assert.equal(geometry.overflow, false, `Overflow: ${state}`);
        assert.deepEqual(geometry.clipped, [], `Clipping: ${state}`);
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
          geometry,
        });
        current.clockStates.push({
          state,
          now: await page.evaluate(() => Date.now()),
          query: new URL(page.url()).search,
          inventoryReads: current.requests.filter((r) => r.path.startsWith('/api/backups/')).length,
        });
      };
      await page.goto(
        `${origin}/browser-tests/backup-location-scope.html?view=date&source=archive&q=east-guest`,
        { waitUntil: 'domcontentloaded', timeout: 60000 },
      );
      if (narrow) await page.evaluate(() => document.documentElement.classList.add('dark'));
      await expect.poll(() => activityCount(page)).toBe(2);
      await activate(page.getByRole('button', { name: '7d', exact: true }));
      const oldDay = page.getByRole('button', { name: /: 1 backup$/ }).first();
      await activate(oldDay);
      await expect(page.getByRole('button', { name: 'Clear date filter' })).toBeVisible();
      assert.equal(new URL(page.url()).searchParams.get('day'), '2026-10-13');
      await expect(page.locator('[data-proxmox-backup-row="recoverable"]')).toHaveCount(1);
      await capture('before-midnight');
      await page.clock.runFor(30000);
      await expect.poll(() => activityCount(page)).toBe(1);
      await expect(page.getByRole('button', { name: dayPattern })).toHaveCount(7);
      await expect(page.getByRole('button', { name: '7d', exact: true })).toHaveAttribute(
        'aria-pressed',
        'true',
      );
      assert.equal(new URL(page.url()).searchParams.get('day'), '2026-10-13');
      await expect(page.locator('[data-proxmox-backup-row="recoverable"]')).toHaveCount(1);
      await activate(page.getByRole('button', { name: 'Clear date filter' }));
      await expect(page.locator('[data-proxmox-backup-row="recoverable"]')).toHaveCount(2);
      await activate(page.getByRole('button', { name: 'Volume', exact: true }));
      await expect(page.getByRole('button', { name: /: 1.00 KB$/ })).toHaveCount(1);
      await expect(page.getByRole('button', { name: /: 2.00 KB$/ })).toHaveCount(0);
      await capture('new-day-volume');
      // Keyboard/touch switch uses the shipped link and keeps one mounted owner.
      await activate(page.getByRole('link', { name: 'Coverage', exact: true }));
      const coverage = page.locator('[data-proxmox-backup-row="coverage"]');
      await expect(coverage).toHaveCount(1);
      // Clear date-view repository/source/search scope before inspecting all evidence.
      const clear = page.getByRole('button', { name: 'Clear filters', exact: true });
      if (await clear.count()) await activate(clear);
      await expect(coverage).toContainText(narrow ? 'Prot.' : 'Protected');
      if (narrow) await coverage.tap();
      else await activate(page.getByRole('button', { name: /expand.*east-guest/i }));
      const detail = page.locator('[data-inline-detail-for="resource:east-100"]');
      await expect(detail).toContainText('Server-owned fixture verdict.');
      await expect(detail.locator('[data-backup-task-duration]')).toHaveText('5s');
      await expect(detail.locator('[data-backup-task-observed] time')).toHaveAttribute(
        'datetime',
        iso(start - 60000),
      );
      await page.clock.setSystemTime(new Date(start - 300000));
      await page.clock.runFor(30000);
      await expect(coverage.locator('[aria-label^="Unknown age"]')).not.toHaveCount(0);
      await expect(detail).toContainText('Latest backup job unknown');
      await expect(coverage).toContainText(narrow ? 'Prot.' : 'Protected');
      await expect(detail).toContainText('Server-owned fixture verdict.');
      await capture('clock-reversed');
      await page.clock.setSystemTime(new Date(start + 60000));
      await page.clock.runFor(30000);
      await expect(coverage.locator('[aria-label^="Unknown age"]')).toHaveCount(0);
      await expect(detail).not.toContainText('Latest backup job unknown');
      await expect(detail.locator('[data-backup-task-duration]')).toHaveText('5s');
      await expect(detail.locator('[data-backup-task-observed] time')).toHaveAttribute(
        'datetime',
        iso(start - 60000),
      );
      await expect(coverage).toContainText(narrow ? 'Prot.' : 'Protected');
      await capture('chronology-reconciled');
      assert.deepEqual(current.pageErrors, []);
      assert.equal(
        current.requests.filter((r) => r.path.startsWith('/api/backups/')).length,
        2,
        'Clock changes, day/range/metric controls and view navigation do not reread inventory',
      );
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
