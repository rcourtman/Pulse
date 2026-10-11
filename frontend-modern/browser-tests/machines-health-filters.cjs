// Exact parent and final use the same production page, fixture and interaction matrix.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const start = Date.parse('2026-10-11T17:00:00Z');
const origin = 'http://127.0.0.1:5351';
const hash = (p) => crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
(async () => {
  const binding = JSON.parse(fs.readFileSync('/workspace/tmp/health-filters-binding.json'));
  const root = '/workspace/' + binding.frontend_root;
  const output = '/workspace/tmp/health-filters/' + (binding.run ?? binding.mode);
  fs.mkdirSync(output, { recursive: true });
  for (const [p, h] of Object.entries(binding.content_sha256))
    assert.equal(hash(root + '/' + p.replace('frontend-modern/', '')), h, p);
  const result = {
    binding,
    playwright: require('playwright/package.json').version,
    cases: [],
    captures: [],
    cleanup: {},
    completed: false,
    limits:
      'Synthetic production Machines page/router/table, not full application, physical phone, native recovery, publication or release.',
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const { createServer } = await import(root + '/node_modules/vite/dist/node/index.js');
  const server = await createServer({
    root,
    configFile: root + '/vite.config.ts',
    cacheDir: output + '/vite-cache',
    server: { host: '127.0.0.1', port: 5351, strictPort: true, watch: null },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  let browser, page;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1680, false],
      ['phone', webkit, 390, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      page = await browser.newPage({
        viewport: { width, height: 950 },
        isMobile: width < 500,
        hasTouch: width < 500,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(10000);
      const record = {
        name,
        browser: browser.version(),
        width,
        dark,
        reads: [],
        writes: [],
        unexpectedReads: [],
        offOrigin: [],
        errors: [],
        consoleErrors: [],
        checks: [],
      };
      result.cases.push(record);
      page.on('pageerror', (err) => record.errors.push(err.message));
      page.on('console', (msg) => {
        if (msg.type() === 'error') record.consoleErrors.push(msg.text().slice(0, 600));
      });
      await page.clock.install({ time: new Date(start - 1000) });
      await page.clock.pauseAt(new Date(start));
      await page.addInitScript((dark) => {
        localStorage.clear();
        document.addEventListener(
          'DOMContentLoaded',
          () => document.documentElement.classList.toggle('dark', dark),
          { once: true },
        );
      }, dark);
      await page.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url()),
          p = url.pathname;
        if (url.origin !== origin) {
          record.offOrigin.push(url.origin);
          return route.abort();
        }
        if (!p.startsWith('/api/')) return route.continue();
        if (req.method() !== 'GET') {
          record.writes.push({ p, method: req.method() });
          return route.fulfill({ status: 403, json: { error: 'Fixture refuses writes' } });
        }
        record.reads.push(p);
        let data;
        if (p === '/api/resources')
          data = {
            data: [
              {
                id: 'fixture-agent',
                name: 'Silent host',
                displayName: 'Silent host',
                type: 'agent',
                platformId: 'fixture-agent',
                platformType: 'agent',
                sourceType: 'agent',
                sources: ['agent'],
                status: 'online',
                lastSeen: new Date(start - 60_000).toISOString(),
                cpu: { current: 24 },
                identity: { hostname: 'fixture-host' },
                agent: { agentId: 'fixture-agent', stale: false },
              },
            ],
            meta: { totalPages: 1 },
          };
        else if (p === '/api/agents/metadata') data = {};
        else if (p === '/api/security/status')
          data = { hasAuthentication: true, requiresAuth: false, sessionCapabilities: {} };
        else if (p === '/api/health') data = { status: 'ok' };
        else if (p === '/api/license/runtime-capabilities')
          data = {
            capabilities: [],
            limits: [],
            max_history_days: 7,
            hosted_mode: false,
            blocked_capabilities: [],
            runtime: { build: 'community' },
          };
        else {
          record.unexpectedReads.push(p);
          return route.fulfill({ status: 404, json: { error: 'Unexpected fixture read' } });
        }
        return route.fulfill({ json: data });
      });
      const capture = async (state) => {
        const p = `${output}/${name}-${state}.png`;
        // A frozen page clock also freezes route/CSS paint. Settle that
        // presentation before accepting a filtered-empty screenshot.
        await page.clock.runFor(500);
        await page.screenshot({ path: p, fullPage: true, animations: 'disabled' });
        result.captures.push({ path: p.replace('/workspace/', ''), sha256: hash(p) });
      };
      const row = page.locator('[data-agents-machine-row="fixture-agent"]');
      const button = (label) => page.getByRole('button', { name: new RegExp(`^${label}(?:,|$)`) });
      const counts = async (healthy, degraded) => {
        assert.equal(await button('Healthy').getAttribute('aria-label'), `Healthy, ${healthy}`);
        assert.equal(await button('Degraded').getAttribute('aria-label'), `Degraded, ${degraded}`);
        assert.equal(await button('Offline').getAttribute('aria-label'), 'Offline, 0');
      };
      await page.goto(origin + '/browser-tests/machines-health-filters.html', {
        waitUntil: 'domcontentloaded',
        timeout: 60000,
      });
      await row.waitFor();
      await page.clock.runFor(20000); // Complete normal initial canonical REST revalidation.
      if (width < 500) await page.getByRole('button', { name: /^Filters/ }).tap();
      await counts(1, 0);
      const search = page.getByRole('textbox', { name: 'Search machines' });
      await search.fill('Silent');
      await page.waitForURL((url) => url.searchParams.get('q') === 'Silent');
      await row.evaluate((el) => {
        window.__originalRow = el;
      });
      const details = row.getByRole('button', { name: /details/i }).first();
      await details.focus();
      const snapshot = await page.evaluate(() => window.__healthFilters.snapshot());
      const reads = record.reads.length;
      await page.clock.runFor(220000); // Report age exactly five minutes: still Healthy.
      await counts(1, 0);
      assert.match(
        await row.locator('span[title="online"], span[role="img"]').getAttribute('class'),
        /bg-emerald-500/,
      );
      await page.clock.runFor(30000);
      assert.match(
        await row.locator('span[title="online"], span[role="img"]').getAttribute('class'),
        /bg-amber-500/,
      );
      await counts(binding.mode === 'parent' ? 1 : 0, binding.mode === 'parent' ? 0 : 1);
      assert.equal(await page.evaluate(() => window.__healthFilters.snapshot()), snapshot);
      assert.equal(record.reads.length, reads);
      assert.equal(await row.evaluate((el) => el === window.__originalRow), true);
      assert.equal(await details.evaluate((el) => el === document.activeElement), true);
      if (binding.mode !== 'parent') {
        assert.equal(
          await row.getByRole('img', { name: 'Stale', exact: true }).getAttribute('title'),
          'Stale',
        );
      }
      await capture('held-all');
      await button('Healthy').click();
      await page.waitForURL((url) => url.searchParams.get('status') === 'online');
      assert.equal(new URL(page.url()).searchParams.get('status'), 'online');
      assert.equal(await button('Healthy').getAttribute('aria-pressed'), 'true');
      assert.equal(await row.count(), binding.mode === 'parent' ? 1 : 0);
      await capture('held-healthy');
      await button('Degraded').click();
      await page.waitForURL((url) => url.searchParams.get('status') === 'degraded');
      assert.equal(new URL(page.url()).searchParams.get('status'), 'degraded');
      assert.equal(await button('Degraded').getAttribute('aria-pressed'), 'true');
      assert.equal(await row.count(), binding.mode === 'parent' ? 0 : 1);
      await capture('held-degraded');
      await page.evaluate(() => window.__healthFilters.report());
      await counts(1, 0);
      assert.equal(await row.count(), 0); // Selected Degraded must not retain recovered rows.
      assert.equal(new URL(page.url()).searchParams.get('status'), 'degraded');
      assert.equal(await search.inputValue(), 'Silent');
      await button('Healthy').click();
      await page.waitForURL((url) => url.searchParams.get('status') === 'online');
      await row.waitFor();
      assert.match(
        await row.locator('span[title="online"], span[role="img"]').getAttribute('class'),
        /bg-emerald-500/,
      );
      await capture('arrived-healthy');
      await page.evaluate(() => window.__healthFilters.withdraw());
      await counts(binding.mode === 'parent' ? 1 : 0, binding.mode === 'parent' ? 0 : 1);
      assert.equal(await row.count(), binding.mode === 'parent' ? 1 : 0);
      await button('Degraded').click();
      await page.waitForURL((url) => url.searchParams.get('status') === 'degraded');
      assert.equal(await row.count(), binding.mode === 'parent' ? 0 : 1);
      await capture('withdrawn-degraded');
      // Counts update on quiet clock ticks too: the selected logical button
      // must retain keyboard focus even when its row leaves the filter.
      await page.evaluate(() => window.__healthFilters.report());
      await button('Healthy').click();
      await page.waitForURL((url) => url.searchParams.get('status') === 'online');
      await page.clock.runFor(1000); // Settle ordinary source metadata refresh.
      await button('Healthy').focus();
      await button('Healthy').evaluate((el) => {
        window.__focusedHealthButton = el;
      });
      const focusReads = record.reads.length;
      await page.clock.runFor(299000);
      await counts(1, 0);
      await page.clock.runFor(30000);
      await counts(0, 1);
      assert.equal(await row.count(), 0);
      const focus = await page.evaluate(() => ({
        originalButtonConnected: window.__focusedHealthButton.isConnected,
        focusRetained: document.activeElement === window.__focusedHealthButton,
        currentFocusTag: document.activeElement.tagName,
      }));
      assert.equal(focus.originalButtonConnected, binding.focus_expected);
      assert.equal(focus.focusRetained, binding.focus_expected);
      assert.equal(record.reads.length, focusReads);
      assert.equal(new URL(page.url()).searchParams.get('status'), 'online');
      record.focusCheck = { ...focus, clockReads: 0, selectedHealthyEmpty: true };
      await capture('focused-health-clock');
      assert.equal(record.reads.filter((p) => p === '/api/resources').length, 1);
      record.sourceUpdateReads = record.reads.slice(reads);
      for (const p of record.sourceUpdateReads) assert.equal(p, '/api/agents/metadata');
      await page.waitForURL((url) => url.searchParams.get('q') === 'Silent');
      record.checks.push({
        exactBoundaryHealthy: true,
        heldAmber: true,
        heldFilter:
          binding.mode === 'parent'
            ? 'misleading Healthy 1 / Degraded 0; Healthy includes retained row; Degraded omits it'
            : 'Healthy 0 / Degraded 1; Healthy excludes retained row; Degraded includes it',
        canonicalSnapshotUnchangedByClock: true,
        retainedRowAndFocusBeforeFiltering: true,
        arrivalReclassifies: true,
        withdrawnReportReclassifies: true,
        searchAndSelectedURLPreserved: true,
        clockReads: 0,
      });
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1),
        true,
      );
      for (const key of ['writes', 'unexpectedReads', 'offOrigin', 'errors', 'consoleErrors'])
        assert.deepEqual(record[key], []);
      await page.close();
      page = null;
      await browser.close();
      browser = null;
    }
    result.completed = true;
  } finally {
    if (page) {
      result.unfinishedPage = await page
        .locator('body')
        .innerText()
        .catch(() => 'unavailable');
      await page.close();
    }
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    await server.close();
    result.cleanup.serverClosed = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2));
  }
  process.stdout.write(
    JSON.stringify({
      completed: result.completed,
      mode: binding.mode,
      cases: result.cases.length,
      captures: result.captures.length,
      cleanup: result.cleanup,
    }) + '\n',
  );
})().catch((err) => {
  process.stderr.write(err.stack + '\n');
  process.exitCode = 1;
});
