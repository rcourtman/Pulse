// Actual backup readers/API client/router/table/CSS; local synthetic HTTP only.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/backup-read-deadline');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json')));
for (const [file, digest] of Object.entries(binding.files))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Source changed: ${file}`);
const LIMIT = 150_000;
const emptyPVE = { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } };
const pve = {
  data: {
    ...emptyPVE.data,
    storageBackups: [
      {
        id: 'archive-112',
        instance: 'pve-a',
        node: 'pve-a',
        storage: 'local',
        type: 'ct',
        vmid: 112,
        time: '2026-10-19T01:00:00Z',
        format: 'zst',
        volid: 'local:backup/vzdump-lxc-112.tar.zst',
      },
    ],
  },
};
const pbs = {
  data: {
    backups: [
      {
        id: 'pbs-main/main/ct/112',
        instance: 'pbs-main',
        datastore: 'main',
        vmid: '112',
        backupType: 'ct',
        backupTime: '2026-10-19T02:00:00Z',
        files: ['index.json.blob'],
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
  cases: [],
  captures: [],
  errors: [],
  cleanup: {},
};
let server, browser, current;
const held = new Set();
function serve(req, res, next) {
  if (!req.url.startsWith('/api/')) return next();
  try {
    assert.equal(req.method, 'GET', 'Inventory actions are read-only');
    const source =
      req.url === '/api/backups/pve' ? 'pve' : req.url === '/api/backups/pbs' ? 'pbs' : null;
    assert(
      source || req.url.startsWith('/api/recovery/postures?'),
      'No guest, collection or mutation endpoint',
    );
    const mode = source ? current.modes[source] : 'normal';
    current.requests.push({ path: req.url, method: req.method, mode });
    const reply = (status, value, headers = {}) => {
      res.writeHead(status, { 'Content-Type': 'application/json', ...headers });
      res.end(JSON.stringify(value));
    };
    if (mode === 'headers' || mode === 'body') {
      current.responses.push(res);
      held.add(res);
      res.on('close', () => held.delete(res));
      if (mode === 'body') {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.write('{"data":');
        res.flushHeaders();
      }
      return;
    }
    if (mode === 'denied') return reply(403, { error: 'Synthetic access refusal' });
    if (mode === 'rate-limit') {
      current.modes[source] = 'normal';
      return reply(429, {}, { 'Retry-After': '120' });
    }
    reply(200, source === 'pve' ? pve : source === 'pbs' ? pbs : posture);
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
          name: 'offline-backup-deadline',
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
      browser = await engine.launch(
        narrow
          ? { headless: true }
          : { headless: true, channel: 'chromium', args: ['--no-sandbox'] },
      );
      const viewport = { width: narrow ? 320 : 1365, height: 1000 };
      const visit = async (modes) => {
        current = { modes, requests: [], responses: [] };
        const context = await browser.newContext({
          viewport,
          isMobile: narrow,
          hasTouch: narrow,
          locale: 'en-GB',
          timezoneId: 'UTC',
          colorScheme: narrow ? 'dark' : 'light',
        });
        const page = await context.newPage();
        page.on('pageerror', (error) => report.errors.push(error.message));
        page.on('request', (request) => assert.equal(new URL(request.url()).origin, origin));
        const now = new Date('2026-10-19T12:00:00Z');
        await page.clock.install({ time: now });
        await page.clock.pauseAt(now);
        await page.goto(`${origin}/browser-tests/backup-inventory.html?view=date&q=backup-guest`, {
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
          await page.evaluate(() => document.fonts.ready);
          assert(
            await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
            'No horizontal page overflow',
          );
          await page.screenshot({
            path: path.join(output, file),
            fullPage: true,
            animations: 'disabled',
          });
          report.captures.push({
            file,
            engine: name,
            viewport,
            state,
            sha256: hash(fs.readFileSync(path.join(output, file))),
          });
        };
        return { context, page, activate, capture };
      };
      for (const source of ['pve', 'pbs']) {
        const { context, page, activate, capture } = await visit({
          pve: 'normal',
          pbs: 'normal',
          [source]: 'headers',
        });
        const count = (key) =>
          current.requests.filter((r) => r.path === `/api/backups/${key}`).length;
        const rows = () => page.locator('[data-proxmox-backup-row="recoverable"]');
        const notice = () =>
          page
            .getByRole('status')
            .filter({ hasText: `${source.toUpperCase()} backup inventory is unavailable` });
        const retry = () =>
          page.getByRole('button', {
            name: `Retry ${source.toUpperCase()} inventory`,
            exact: true,
          });
        await expect.poll(() => count(source)).toBe(1);
        await expect(rows()).toHaveCount(1);
        await page.clock.fastForward(LIMIT - 1);
        await expect(
          page.getByRole('button', { name: 'Reading backups…', exact: true }),
        ).toHaveAttribute('aria-disabled', 'true');
        await expect(notice()).toHaveCount(0);
        await page.clock.fastForward(1);
        await expect(notice()).toContainText(
          'The inventory read timed out. Missing restore points do not mean no backups exist.',
        );
        await expect(notice()).toContainText('it does not start a backup or contact guests');
        await expect(
          page.getByRole('button', { name: 'Refresh backups', exact: true }),
        ).toHaveAttribute('aria-disabled', 'false');
        await expect(retry()).toBeEnabled();
        await expect(rows()).toHaveCount(1);
        assert.equal(count(source), 1, 'No retry after expiry');
        await capture(`${source}-header-timeout`);

        // Release an obsolete response after expiry, then use an explicit retry.
        for (const response of current.responses)
          response.end(JSON.stringify(source === 'pve' ? pve : pbs));
        await expect(notice()).toContainText('timed out');
        current.modes[source] = 'normal';
        await activate(retry());
        await expect(rows()).toHaveCount(2);
        await expect(notice()).toHaveCount(0);
        assert.equal(count(source), 2);
        assert.equal(
          count(source === 'pve' ? 'pbs' : 'pve'),
          1,
          'An isolated retry leaves the other source alone',
        );

        // A partial JSON body is covered by the same deadline as headers.
        current.modes[source] = 'body';
        await activate(page.getByRole('button', { name: 'Refresh backups', exact: true }));
        await expect.poll(() => count(source)).toBe(3);
        await expect(rows()).toHaveCount(2);
        await page.clock.fastForward(LIMIT);
        await expect(notice()).toContainText('The inventory read timed out.');
        await expect(rows()).toHaveCount(
          1,
          'Timed-out source points are withdrawn, independent evidence remains',
        );
        await capture(`${source}-body-timeout`);
        current.modes[source] = 'denied';
        await activate(retry());
        await expect(notice()).toContainText('Access denied.');
        await expect(notice()).not.toContainText('timed out');
        await expect(rows()).toHaveCount(1);
        current.modes[source] = 'normal';
        await activate(retry());
        await expect(notice()).toHaveCount(0);
        await expect(rows()).toHaveCount(2);
        await expect(
          page.getByPlaceholder('Search backups by workload, node, source, or status'),
        ).toHaveValue('backup-guest');
        await capture(`${source}-explicit-recovery`);

        // Preserve the client's existing two-minute idempotent 429 wait.
        current.modes[source] = 'rate-limit';
        await activate(page.getByRole('button', { name: 'Refresh backups', exact: true }));
        await expect.poll(() => count(source)).toBe(6);
        await page.clock.fastForward(119_999);
        assert.equal(count(source), 6);
        await page.clock.fastForward(1);
        await expect.poll(() => count(source)).toBe(7);
        await expect(
          page.getByRole('button', { name: 'Refresh backups', exact: true }),
        ).toHaveAttribute('aria-disabled', 'false');
        await expect(notice()).toHaveCount(0);
        report.cases.push({
          source,
          engine: name,
          browserVersion: browser.version(),
          viewport,
          requests: current.requests,
          checks: [
            'Exact header deadline',
            'Partial body deadline',
            'No timeout retry; explicit isolated recovery',
            'Failure withdrawal and independent source preservation',
            '403 is denial, not timeout',
            'Saved search survives refresh/retry',
            'Existing two-minute Retry-After remains usable',
          ],
        });
        await context.close();
      }
      const { context, page, capture } = await visit({ pve: 'headers', pbs: 'body' });
      await expect
        .poll(() => current.requests.filter((r) => r.path.startsWith('/api/backups/')).length)
        .toBe(2);
      await page.clock.fastForward(LIMIT);
      await expect(
        page.getByText('Could not load Proxmox backup inventory', { exact: true }),
      ).toBeVisible();
      await expect(page.getByText('No backups yet', { exact: true })).toHaveCount(0);
      await expect(page.getByRole('button', { name: 'Refresh', exact: true })).toBeEnabled();
      await expect(page.locator('[data-proxmox-backups-table="servers"]')).toContainText(
        'pbs-main',
      );
      await capture('both-unavailable');
      report.cases.push({
        source: 'both',
        engine: name,
        viewport,
        checks: [
          'Both unanswered inventories settle unavailable without an empty-backup claim',
          'Separately known PBS server remains mounted',
          'Refresh is usable',
        ],
        requests: current.requests,
      });
      await context.close();
      await browser.close();
      browser = null;
    }
    assert.deepEqual(report.errors, []);
    report.passed = true;
  } catch (error) {
    report.failure = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    report.cleanup.browser = true;
    for (const response of held) response.destroy();
    held.clear();
    report.cleanup.heldResponses = true;
    if (server) await server.close();
    report.cleanup.server = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(
      JSON.stringify({
        passed: report.passed ?? false,
        cases: report.cases.length,
        captures: report.captures.length,
        failure: report.failure,
      }),
    );
  }
})();
