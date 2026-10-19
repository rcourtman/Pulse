// Actual collected-inventory readers -> By date -> production evidence dialog/CSS.
// Synthetic GETs only: no collection, backup, guest or provider operations.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/backup-evidence/browser');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json')));
for (const [file, digest] of Object.entries(binding.files))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Changed source: ${file}`);
const start = Date.parse('2026-10-19T22:59:50Z'); // 23:59:50 Europe/London.
const nativeId = 'pbs-east/main/vm/100/2026-10-19T22:59:55Z/<img src=x onerror=unsafe()>';
const point = {
  id: nativeId,
  instance: 'pbs-east',
  datastore: 'main',
  namespace: 'pve-1',
  backupType: 'vm',
  vmid: '100',
  backupTime: new Date(start + 5000).toISOString(),
  files: ['index.json.blob'],
  size: 1024,
  verified: true,
};
const fixtures = {
  pve: {
    data: {
      backupTasks: [],
      guestSnapshots: [],
      storageBackups: [
        {
          id: 'independent-archive',
          instance: 'east',
          node: 'pve-1',
          storage: 'local',
          vmid: 100,
          type: 'vm',
          time: new Date(start - 60000).toISOString(),
          size: 2048,
          format: 'zst',
          volid: 'local:backup/qemu-100.vma.zst',
        },
      ],
    },
  },
  initial: { data: { backups: [point] } },
  updated: {
    data: {
      backups: [
        { ...point, id: 'newer-point', backupTime: new Date(start + 10000).toISOString() },
        { ...point, verified: false, protected: true },
      ],
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
  protocol_sha256: hash(fs.readFileSync(__filename)),
  fixtures,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  errors: [],
  cleanup: {},
};
let server, browser, current;
function send(res, status, body) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(body));
}
function serve(req, res, next) {
  if (!req.url.startsWith('/api/')) return next();
  try {
    assert.equal(req.method, 'GET', 'No mutation, collection or guest requests');
    const url = new URL(req.url, 'http://fixture.invalid');
    assert(
      ['/api/backups/pve', '/api/backups/pbs', '/api/recovery/postures'].includes(url.pathname),
    );
    current.requests.push({ method: req.method, path: url.pathname });
    if (url.pathname === '/api/recovery/postures') {
      assert.deepEqual(url.searchParams.getAll('resourceId'), ['east-100']);
      return send(res, 200, fixtures.postures);
    }
    if (url.pathname.endsWith('/pve')) return send(res, 200, fixtures.pve);
    if (current.holdPBS) {
      assert(!current.pendingPBS, 'One held PBS read at a time');
      current.pendingPBS = res;
    } else send(res, 200, fixtures.initial);
  } catch (error) {
    report.errors.push(String(error.stack || error));
    if (!res.writableEnded) send(res, 500, {});
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
          name: 'backup-evidence-collected-http',
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
        holdPBS: false,
        viewport: { width: narrow ? 320 : 1365, height: 1000 },
        timezone: 'Europe/London',
      };
      browser = await engine.launch(
        narrow
          ? { headless: true }
          : {
              headless: true,
              channel: 'chromium',
              args: ['--no-sandbox'],
            },
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
      const check = (value) => current.checks.push(value);
      const capture = async (state) => {
        await page.clock.runFor(400); // Allow the production dialog's entry animation to paint.
        const geometry = await page.evaluate(() => {
          const panel = document.querySelector('[role="dialog"]');
          const rect = panel?.getBoundingClientRect();
          return {
            overflow: document.documentElement.scrollWidth > innerWidth,
            clippedFields: [...document.querySelectorAll('[data-backup-evidence-details] dd')]
              .filter((el) => el.scrollWidth > el.clientWidth + 1)
              .map((el) => el.textContent),
            dialogWithinViewport:
              !rect ||
              (rect.left >= 0 &&
                rect.right <= innerWidth &&
                rect.top >= 0 &&
                rect.bottom <= innerHeight),
            focused: document.activeElement?.getAttribute('aria-label'),
            bodyLocked: document.body.style.overflow === 'hidden',
          };
        });
        assert.equal(geometry.overflow, false);
        assert.deepEqual(geometry.clippedFields, []);
        assert.equal(geometry.dialogWithinViewport, true);
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), animations: 'disabled' });
        report.captures.push({
          file,
          state,
          engine: name,
          viewport: current.viewport,
          sha256: hash(fs.readFileSync(path.join(output, file))),
          geometry,
        });
      };
      await page.goto(`${origin}/browser-tests/backup-location-scope.html?view=date`, {
        waitUntil: 'domcontentloaded',
        timeout: 60000,
      });
      if (narrow) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const rows = page.locator('[data-proxmox-backup-row="recoverable"]');
      await expect(rows).toHaveCount(2);
      const trigger = page.getByRole('button', {
        name: 'Backup evidence for east-guest: Verified',
        exact: true,
      });
      const triggerHandle = await trigger.elementHandle();
      const pointRow = await trigger.evaluateHandle((el) => el.closest('tr'));
      await activate(trigger);
      const dialog = page.getByRole('dialog', { name: 'Backup evidence details' });
      await expect(dialog).toContainText(nativeId);
      const dialogHandle = await dialog.elementHandle();
      const close = dialog.getByRole('button', { name: 'Close backup evidence' });
      await close.focus();
      await page.clock.runFor(30000);
      await expect(dialog).toBeVisible();
      await expect(close).toBeFocused();
      assert(await dialogHandle.evaluate((el) => el === document.querySelector('[role="dialog"]')));
      assert(await pointRow.evaluate((el) => el.isConnected));
      await expect(page.getByText('Unknown date', { exact: true })).toHaveCount(0);
      await expect(dialog.locator('img,script')).toHaveCount(0);
      check(
        'Clock/date-group rollover retains the exact open dialog and focused Close without extra inventory reads',
      );
      assert.equal(current.requests.filter((r) => r.path.startsWith('/api/backups/')).length, 2);
      await capture('clock-open');
      await page.keyboard.press('Escape');
      await expect(dialog).toHaveCount(0);
      assert(await triggerHandle.evaluate((el) => document.activeElement === el));
      check('Escape returns keyboard focus to the same original evidence trigger');

      current.holdPBS = true;
      await activate(page.getByRole('button', { name: 'Refresh backups', exact: true }));
      await expect.poll(() => Boolean(current.pendingPBS)).toBe(true);
      await activate(trigger);
      await expect(dialog).toBeVisible();
      const updatingDialog = await dialog.elementHandle();
      await close.focus();
      send(current.pendingPBS, 200, fixtures.updated);
      current.pendingPBS = null;
      await expect(dialog.getByRole('heading', { name: 'Unverified', exact: true })).toBeVisible();
      await expect(dialog.getByRole('heading', { name: 'Verified', exact: true })).toHaveCount(0);
      await expect(dialog.getByRole('heading', { name: 'Prune protected' })).toBeVisible();
      await expect(dialog).toContainText(nativeId);
      await expect(rows).toHaveCount(3);
      assert(
        await updatingDialog.evaluate((el) => el === document.querySelector('[role="dialog"]')),
      );
      assert(
        await pointRow.evaluate(
          (el) =>
            el.isConnected &&
            [...document.querySelectorAll('[data-proxmox-backup-row="recoverable"]')].indexOf(
              el,
            ) === 1,
        ),
      );
      await expect(close).toBeFocused();
      check(
        'Pending HTTP replacement reorders points and updates verification/prune state in the existing focused dialog, not a stale snapshot',
      );
      await capture('updated-open');
      if (narrow) await close.tap();
      else await page.keyboard.press('Escape');
      await expect(dialog).toHaveCount(0);
      assert(await triggerHandle.evaluate((el) => document.activeElement === el));
      await expect(
        page.getByRole('button', {
          name: 'Backup evidence for east-guest: Unverified, prune protected',
          exact: true,
        }),
      ).toBeFocused();
      check(
        'Close/touch and Escape retain focus restoration after the point changes state and order',
      );

      await activate(page.getByRole('button', { name: 'Refresh backups', exact: true }));
      await expect.poll(() => Boolean(current.pendingPBS)).toBe(true);
      await activate(
        page.getByRole('button', {
          name: 'Backup evidence for east-guest: Unverified, prune protected',
          exact: true,
        }),
      );
      await expect(dialog).toBeVisible();
      send(current.pendingPBS, 403, {});
      current.pendingPBS = null;
      await expect(dialog).toHaveCount(0);
      await expect(page.getByText(/PBS backup inventory is unavailable/)).toContainText(
        'Access denied.',
      );
      await expect(rows).toHaveCount(1);
      await page.clock.runFor(30000);
      await expect(dialog).toHaveCount(0);
      assert.equal(await page.evaluate(() => document.body.style.overflow === 'hidden'), false);
      await expect(
        page.getByRole('button', { name: /Backup evidence for east-guest: Unverified/ }),
      ).toHaveCount(0);
      check(
        'Denied source removes its open dialog and points, releases modal isolation and cannot reappear on the next clock tick',
      );
      await capture('source-denied');
      const independent = page.getByRole('button', {
        name: 'Backup evidence for east-guest: Backup file',
        exact: true,
      });
      await activate(independent);
      await expect(dialog).toContainText('independent-archive');
      await expect(dialog).not.toContainText(nativeId);
      await activate(close);
      await expect(independent).toBeFocused();
      check(
        'The independent PVE source stays readable and interactive without borrowing the withdrawn PBS record',
      );
      assert.equal(current.requests.filter((r) => r.path.startsWith('/api/backups/')).length, 6);
      assert.deepEqual(current.pageErrors, []);
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
    report.failedCase = current && { ...current, pendingPBS: Boolean(current.pendingPBS) };
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    if (server) await server.close();
    report.cleanup = { browserClosed: true, serverClosed: true };
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2));
  }
})();
