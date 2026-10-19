// Read-only partial HTTP body -> production posture owner -> existing Coverage.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/posture-read/browser');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json')));
for (const [file, digest] of Object.entries(binding.files))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Changed source: ${file}`);
const fixtures = {
  pbs: { data: { backups: [] } },
  pve: {
    data: {
      backupTasks: [],
      guestSnapshots: [],
      storageBackups: [
        {
          id: 'east/local/100',
          instance: 'east',
          node: 'pve-1',
          storage: 'local',
          vmid: 100,
          type: 'vm',
          time: '2026-10-19T01:00:00Z',
          size: 2048,
          volid: 'local:backup/vzdump-qemu-100.vma.zst',
          format: 'zst',
        },
      ],
    },
  },
};
const posture = (state) => ({
  data: [
    {
      subjectResourceId: 'east-100',
      state,
      freshness: 'current',
      verification: 'verified',
      coverage: 'complete',
      explanation:
        state === 'protected'
          ? 'A current verified backup is available.'
          : 'A new evaluation needs attention.',
      evaluatedAt: '2026-10-19T12:00:00Z',
      providerStates: [],
      repositoryResourceIds: [],
      evidenceIds: ['point-1'],
    },
  ],
  policy: {
    freshnessWindowSeconds: 604800,
    verificationWindowSeconds: 604800,
    requireVerification: true,
  },
});
const report = {
  binding,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  errors: [],
  cleanup: {},
};
let server,
  browser,
  current,
  currentPage,
  mode = 'success',
  finishRetry;
const openResponses = new Set();
function serve(req, res, next) {
  if (!req.url.startsWith('/api/')) return next();
  try {
    assert.equal(req.method, 'GET', 'No backup, guest or mutation requests');
    const url = new URL(req.url, 'http://fixture.invalid');
    assert(
      ['/api/backups/pve', '/api/backups/pbs', '/api/recovery/postures'].includes(url.pathname),
    );
    const entry = { path: url.pathname, mode };
    current.requests.push(entry);
    res.writeHead(200, { 'Content-Type': 'application/json' });
    if (url.pathname === '/api/recovery/postures') {
      assert.deepEqual(url.searchParams.getAll('resourceId'), ['east-100']);
      if (mode === 'stall' || mode === 'retry') {
        // Actual headers arrive, but JSON body remains incomplete.
        res.write('{"data":');
        openResponses.add(res);
        res.on('close', () => {
          openResponses.delete(res);
          entry.closed = true;
        });
        if (mode === 'retry')
          finishRetry = () =>
            res.end(JSON.stringify(posture('attention')).slice('{"data":'.length));
        return;
      }
      res.end(JSON.stringify(posture('protected')));
      return;
    }
    res.end(JSON.stringify(url.pathname.endsWith('/pve') ? fixtures.pve : fixtures.pbs));
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
          name: 'posture-partial-http',
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
    // The desktop journey completed at 3ef1e8a815 with identical product bytes.
    // Continue only the unfinished compact/touch consumer.
    for (const [name, engine, narrow] of [['webkit', webkit, true]]) {
      mode = 'success';
      finishRetry = undefined;
      current = {
        name,
        requests: [],
        pageErrors: [],
        viewport: { width: narrow ? 320 : 1365, height: 1000 },
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
        locale: 'en-GB',
        timezoneId: 'UTC',
        colorScheme: narrow ? 'dark' : 'light',
      });
      const page = await context.newPage();
      currentPage = page;
      await page.addInitScript(() => {
        window.__postureReads = [];
        const original = window.fetch;
        window.fetch = (...args) => {
          if (String(args[0]).includes('/api/'))
            window.__postureReads.push({ url: String(args[0]), at: Date.now() });
          return original(...args);
        };
      });
      page.on('pageerror', (error) => current.pageErrors.push(error.message));
      page.on('request', (req) => assert.equal(new URL(req.url()).origin, origin));
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
      await page.goto(`${origin}/browser-tests/backup-location-scope.html?view=coverage`, {
        waitUntil: 'domcontentloaded',
        timeout: 60000,
      });
      if (narrow) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const coverage = page.locator('[data-proxmox-backup-row="coverage"]');
      await expect(coverage).toHaveCount(1);
      await expect(coverage).toContainText(narrow ? 'Prot.' : 'Protected');
      const expand = page.getByRole('button', { name: /expand.*east-guest/i });
      if (narrow) await coverage.tap();
      else await activate(expand);
      const detail = page.locator('[data-inline-detail-for="resource:east-100"]');
      await expect(detail).toContainText('vzdump-qemu-100.vma.zst');
      await capture('protected');
      mode = 'stall';
      await activate(page.getByRole('button', { name: 'Refresh backups', exact: true }));
      await expect(page.getByRole('button', { name: 'Reading backups…' })).toHaveAttribute(
        'aria-disabled',
        'true',
      );
      // Observe the existing foreground replacement plus its background stall
      // budget. The standalone transport tests pin the exact 150-second budget;
      // this consuming journey keeps ordinary polling active throughout.
      await page.clock.runFor(180000);
      const warning = page
        .getByRole('status')
        .filter({ hasText: 'Protection posture is unavailable' });
      await expect(warning).toBeVisible();
      await expect(warning).toContainText('Each read stops waiting after 150 seconds.');
      await expect(warning).toContainText(
        "Retry reads Pulse's collected evidence, not a guest probe.",
      );
      await expect(coverage).toContainText('Unknown');
      await expect(coverage).not.toContainText(narrow ? 'Prot.' : 'Protected');
      await expect(detail).toContainText('vzdump-qemu-100.vma.zst');
      await expect(
        page.getByRole('button', { name: 'Refresh backups', exact: true }),
      ).toHaveAttribute('aria-disabled', 'false');
      await capture('unavailable');
      const inventoryReads = current.requests.filter((r) =>
        r.path.startsWith('/api/backups/'),
      ).length;
      mode = 'retry';
      await activate(page.getByRole('button', { name: 'Retry protection posture', exact: true }));
      const busy = page.getByRole('button', { name: 'Retrying protection posture…', exact: true });
      await expect(busy).toBeDisabled();
      await expect(coverage).toContainText('Unknown');
      await expect.poll(() => typeof finishRetry).toBe('function');
      const before = current.requests.length;
      await busy.click({ force: true });
      assert.equal(current.requests.length, before, 'No duplicate pending retry');
      await capture('retrying');
      finishRetry();
      await expect(coverage).toContainText(narrow ? 'Attn.' : 'Needs attention');
      await expect(warning).toHaveCount(0);
      await expect(detail).toContainText('A new evaluation needs attention.');
      await expect(detail).toContainText('vzdump-qemu-100.vma.zst');
      assert.equal(
        current.requests.filter((r) => r.path.startsWith('/api/backups/')).length,
        inventoryReads,
        'Posture retry must not reread backup inventory',
      );
      await capture('recovered');
      assert.deepEqual(current.pageErrors, []);
      current.readTrace = await page.evaluate(() => window.__postureReads);
      await context.close();
      current.contextClosed = true;
      await browser.close();
      browser = undefined;
      current.browserClosed = true;
      await expect.poll(() => openResponses.size).toBe(0);
      report.cases.push(current);
    }
    assert.deepEqual(report.errors, []);
  } catch (error) {
    if (currentPage && !currentPage.isClosed()) {
      current.readTrace = await currentPage.evaluate(() => window.__postureReads).catch(() => null);
    }
    report.failure = String(error.stack || error);
    report.failedCase = current;
    process.exitCode = 1;
  } finally {
    if (browser) {
      await browser.close();
      report.cleanup.failureBrowserClosed = true;
    }
    for (const res of openResponses) res.destroy();
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
