// Actual collected-inventory GET readers -> Backups -> shipped activity chart/CSS.
// Synthetic fixtures only; no collection, backup, guest or provider operations.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/backup-keyboard/browser');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json')));
for (const [file, digest] of Object.entries(binding.files))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Changed source: ${file}`);
const start = Date.parse('2026-10-19T22:59:50Z'); // 23:59:50 Europe/London.
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
      backupTasks: [],
      guestSnapshots: [],
      storageBackups: [
        archive('old', '2026-10-18T12:00:00Z', 2048),
        archive('recent', '2026-10-19T12:00:00Z', 1024),
      ],
    },
  },
  pbs: { data: { backups: [] } },
  postures: {
    data: [
      {
        subjectResourceId: 'east-100',
        state: 'protected',
        freshness: 'current',
        verification: 'verified',
        coverage: 'complete',
        explanation: 'Server-owned fixture verdict.',
        evaluatedAt: new Date(start).toISOString(),
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
    assert.equal(req.method, 'GET', 'No mutation, collection or guest requests');
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
          name: 'backup-keyboard-collected-http',
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
        checks: [],
        viewport: { width: narrow ? 320 : 1365, height: 1000 },
        timezone: 'Europe/London',
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
          clippedHelp: [...document.querySelectorAll('[id$="-keyboard-help"]')]
            .filter((el) => el.scrollWidth > el.clientWidth + 1)
            .map((el) => el.textContent),
          focused: document.activeElement?.getAttribute('aria-label'),
          dayButtons: document.querySelectorAll('[data-backup-activity-days] button').length,
          dayTabStops: document.querySelectorAll('[data-backup-activity-days] button[tabindex="0"]')
            .length,
        }));
        assert.equal(geometry.overflow, false, `Viewport overflow: ${state}`);
        assert.deepEqual(geometry.clippedHelp, []);
        assert.equal(geometry.dayTabStops, 1);
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
      };
      const check = (name) => current.checks.push(name);
      await page.goto(
        `${origin}/browser-tests/backup-location-scope.html?view=date&source=archive`,
        { waitUntil: 'domcontentloaded', timeout: 60000 },
      );
      if (narrow) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const group = page.getByRole('group', { name: 'Backup activity days' });
      const days = group.getByRole('button');
      const oneYear = page.getByRole('button', { name: '1y', exact: true });
      const search = page.getByPlaceholder('Search backups by workload, node, source, or status');
      await expect(group).toBeVisible();
      await activate(oneYear);
      await expect(days).toHaveCount(365);
      await expect(group.locator('button[tabindex="0"]')).toHaveCount(1);
      await oneYear.focus();
      await page.keyboard.press('Tab');
      await expect(days.last()).toBeFocused();
      await page.keyboard.press('Tab');
      await expect(search).toBeFocused();
      await page.keyboard.press('Shift+Tab');
      await expect(days.last()).toBeFocused();
      check(
        'One natural Tab enters all 365 days; the next reaches the existing search, Shift+Tab returns',
      );
      await page.keyboard.press('ArrowLeft');
      await expect(days.nth(363)).toBeFocused();
      assert.equal(new URL(page.url()).searchParams.get('day'), null);
      await page.keyboard.press('ArrowRight');
      await expect(days.last()).toBeFocused();
      await page.keyboard.press('Home');
      await expect(days.first()).toBeFocused();
      await page.keyboard.press('ArrowLeft');
      await expect(days.first()).toBeFocused();
      await page.keyboard.press('End');
      await expect(days.last()).toBeFocused();
      await page.keyboard.press('ArrowRight');
      await expect(days.last()).toBeFocused();
      assert.equal(new URL(page.url()).searchParams.get('day'), null);
      check(
        'Arrow/Home/End focus does not select or wrap; native Enter selects the existing date filter',
      );
      await page.keyboard.press('Enter');
      const remembered = group.getByRole('button', { name: /^Mon, Oct 19:/ });
      await expect(remembered).toHaveAttribute('aria-pressed', 'true');
      assert.equal(new URL(page.url()).searchParams.get('day'), '2026-10-19');
      await expect(page.locator('[data-proxmox-backup-row="recoverable"]')).toHaveCount(1);
      if (!narrow) await expect(page.locator('[data-tooltip-portal="true"]')).toHaveCount(1);
      await page.keyboard.press('Escape');
      await expect(page.locator('[data-tooltip-portal="true"]')).toHaveCount(0);
      await expect(remembered).toBeFocused();
      await capture('year-selected');
      await remembered.evaluate((el) => {
        window.__rememberedBackupDay = el;
      });
      await page.clock.runFor(30000);
      await expect(remembered).toBeFocused();
      assert(await remembered.evaluate((el) => el === window.__rememberedBackupDay));
      await expect(days.nth(363)).toBeFocused();
      assert.equal(new URL(page.url()).searchParams.get('day'), '2026-10-19');
      check(
        'Local midnight replaces observed points but preserves the focused calendar-day DOM node and selected filter',
      );
      await capture('live-window-focus');
      await activate(page.getByRole('button', { name: 'Volume', exact: true }));
      await expect(remembered).toHaveAccessibleName('Mon, Oct 19: 1.00 KB, selected');
      assert(await remembered.evaluate((el) => el === window.__rememberedBackupDay));
      await oneYear.focus();
      await page.keyboard.press('Tab');
      await expect(remembered).toBeFocused();
      if (!narrow)
        await expect(page.locator('[data-tooltip-portal="true"]')).toContainText('1.00 KB');
      check('Volume preserves day identity/re-entry and updates accessible and hover evidence');
      await capture('volume-focus');
      await page.keyboard.press('Escape');
      await expect(page.locator('[data-tooltip-portal="true"]')).toHaveCount(0);
      await page.keyboard.press('Space');
      assert.equal(new URL(page.url()).searchParams.get('day'), null);
      await expect(page.locator('[data-proxmox-backup-row="recoverable"]')).toHaveCount(2);
      await page.keyboard.press('Tab');
      await expect(search).toBeFocused();
      check('Native Space clears the day and Tab still reaches filters without a trap');
      await activate(page.getByRole('button', { name: '7d', exact: true }));
      const old = group.getByRole('button', { name: /^Sun, Oct 18:/ });
      await activate(old);
      await expect(old).toHaveAttribute('aria-pressed', 'true');
      await expect(group.locator('button[tabindex="0"]')).toHaveAccessibleName(
        'Sun, Oct 18: 2.00 KB, selected',
      );
      assert.equal(new URL(page.url()).searchParams.get('day'), '2026-10-18');
      assert.equal(new URL(page.url()).searchParams.get('source'), 'archive');
      await expect(page.locator('[data-proxmox-backup-row="recoverable"]')).toHaveCount(1);
      check(
        narrow
          ? 'Touch retains ordinary seven-day selection and source scope'
          : 'Existing seven-day activation and source scope remain',
      );
      await capture('short-range-selection');
      assert.deepEqual(current.pageErrors, []);
      assert.equal(current.requests.filter((r) => r.path.startsWith('/api/backups/')).length, 2);
      check(
        'Only the original two collected inventory GETs, no action dispatch or extra inventory reads',
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
