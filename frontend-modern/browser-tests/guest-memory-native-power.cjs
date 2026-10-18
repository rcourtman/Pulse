// Native power-only deltas through the production merger, mapper and consumers.
// No provider, guest command, backup action or settings mutation is permitted.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const { chromium, webkit } = require('playwright');
const {
  createPublicationFixtureServer,
  hash,
} = require('/workspace/frontend-modern/browser-tests/publication-fixture-server.cjs');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-memory-native-power/browser';
const origin = 'http://127.0.0.1:5345';
const settle = (page) =>
  page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  });
const point = (timestamp, value) => ({ timestamp, value, min: value, max: value });

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    playwright: require('playwright/package.json').version,
    driver_sha256: hash(fs.readFileSync(root + '/browser-tests/guest-memory-native-power.cjs')),
    cases: [],
    captures: [],
    cleanup: {
      browsers_created: 0,
      browsers_closed: 0,
      server_created: false,
      server_listening: false,
      server_closed: false,
    },
    limits:
      'Synthetic power-only canonical deltas with real merger, Workloads mapper, row/bar/sparkline, memory disclosure, full guest and canonical drawers and History. No collector, guest operation, thaw, writes, installed recovery or release acceptance.',
  };
  let server, browser, lastPage;
  try {
    assert.equal(
      result.playwright,
      JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    server = createPublicationFixtureServer(
      root,
      5345,
      'browser-tests/guest-memory-agent-stale.html',
    );
    result.cleanup.server_created = true;
    result.preparation = {
      source_sha: server.binding.source_sha,
      sources: server.binding.sources,
      artifacts: server.binding.artifacts,
      graph: server.binding.graph,
    };
    await server.listen();
    result.cleanup.server_listening = true;
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1365, false],
      ['narrow', webkit, 320, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      result.cleanup.browsers_created++;
      const context = await browser.newContext({
        viewport: { width, height: 900 },
        isMobile: width < 768,
        hasTouch: width < 768,
        locale: 'en-GB',
        timezoneId: 'UTC',
        reducedMotion: 'reduce',
      });
      const page = await context.newPage();
      lastPage = page;
      page.setDefaultTimeout(10000);
      const record = {
        name,
        width,
        height: 900,
        browser: browser.version(),
        checks: [],
        requests: [],
        errors: [],
        failedLoads: [],
      };
      result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') record.errors.push(message.text());
      });
      page.on('requestfailed', (request) =>
        record.failedLoads.push({
          path: new URL(request.url()).pathname,
          failure: request.failure(),
        }),
      );
      await page.routeWebSocket('**/*', (socket) => socket.close());
      let stored = false;
      await page.route('**/*', (route) => {
        const request = route.request(),
          url = new URL(request.url());
        assert.equal(url.origin, origin, 'no external request');
        assert(['GET', 'HEAD'].includes(request.method()), 'read-only protocol');
        if (url.pathname === '/favicon.ico') return route.fulfill({ status: 204, body: '' });
        if (!url.pathname.startsWith('/api/')) return route.continue();
        record.requests.push({ path: url.pathname, method: request.method(), stored });
        assert(
          !/\/(diagnostics|run|run-command|guest-agent|qemu)(\/|$)/.test(url.pathname),
          'no guest operation',
        );
        let body = { success: true, data: [], alerts: [], anomalies: [], config: null };
        if (url.pathname === '/api/metrics-store/history')
          body = {
            resourceType: 'vm',
            resourceId: url.searchParams.get('resourceId'),
            range: url.searchParams.get('range') || '24h',
            start: 1791633600000,
            end: 1791720000000,
            source: 'store',
            metrics: stored ? { memory: [point(1791633600000, 10), point(1791720000000, 20)] } : {},
          };
        if (url.pathname === '/api/license/runtime-capabilities')
          body = {
            capabilities: [],
            limits: [],
            max_history_days: 7,
            hosted_mode: false,
            runtime: { build: 'community' },
            blocked_capabilities: [],
          };
        return route.fulfill({ json: body });
      });
      await page.addInitScript((dark) => {
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.toggle('dark', dark),
        );
        window.__nativeMemoryInputs = [];
        for (const type of ['click', 'keydown', 'change'])
          document.addEventListener(
            type,
            (event) => window.__nativeMemoryInputs.push({ type, trusted: event.isTrusted }),
            true,
          );
      }, dark);
      const apply = async (method, ...args) => {
        await page.evaluate(({ method, args }) => window.__staleAgentMemory[method](...args), {
          method,
          args,
        });
        await settle(page);
      };
      const evidence = () => page.evaluate(() => window.__staleAgentMemory.evidence());
      const capture = async (label, locator) => {
        await locator.scrollIntoViewIfNeeded();
        await settle(page);
        const file = name + '-' + label + '.png';
        await locator.screenshot({ path: output + '/' + file });
        result.captures.push({ file, sha256: hash(fs.readFileSync(output + '/' + file)) });
      };
      const activate = async (locator) => {
        await locator.scrollIntoViewIfNeeded();
        if (width < 768) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      await page.goto(origin, { waitUntil: 'domcontentloaded' });
      await page.waitForFunction(() => window.__staleAgentMemory?.evidence().projected);
      const row = page.getByRole('region', { name: 'Workload row', exact: true });
      const workload = page.getByRole('region', { name: 'Workload drawer', exact: true });
      const canonical = page.getByRole('region', { name: 'Canonical drawer', exact: true });
      const notice = row.locator('[data-workload-memory-read-status]');
      assert.equal(await notice.count(), 0);
      await activate(workload.getByTestId('guest-technical-details').locator('summary'));
      await workload
        .getByText(/Current · Pulse Agent/)
        .first()
        .waitFor();
      const original = (await evidence()).canonical.memory;
      await apply('power', 'paused');
      await notice.waitFor();
      assert.match(await notice.innerText(), width < 768 ? /Prior/ : /Last known/);
      const paused = await evidence();
      assert.equal(paused.canonical.status, 'online');
      assert.equal(paused.canonical.agent.stale, false);
      assert.equal(paused.projected.status, 'paused');
      assert.deepEqual(
        paused.canonical.memory,
        original,
        'power-only delta did not edit the wire carrier',
      );
      assert.equal(paused.filterHigh, false);
      assert.deepEqual(paused.sort, ['independent', 'fixture:pve1:101']);
      await workload
        .getByText(/Last known · Pulse Agent/)
        .first()
        .waitFor();
      assert.doesNotMatch(
        await canonical.getByTestId('resource-drawer-facts').innerText(),
        /of .* RAM/,
      );
      assert.doesNotMatch(
        await workload.getByTestId('guest-drawer-facts').innerText(),
        /of .* RAM/,
      );
      await capture('paused-overview', workload);
      await activate(notice.getByRole('button'));
      const dialog = page.getByRole('dialog', {
        name: 'Memory reading for linked-guest',
        exact: true,
      });
      await dialog.waitFor();
      assert.match(await dialog.innerText(), /Proxmox reports this guest paused/);
      assert.match(await dialog.innerText(), /2026-10-10 12:00:00 UTC/);
      await capture('paused-disclosure', dialog);
      await activate(dialog.getByRole('button', { name: 'Close reading details' }));
      await page.getByLabel('Metric display', { exact: true }).selectOption('sparklines');
      await row
        .getByRole('img', { name: 'linked-guest memory history, last known 95%', exact: true })
        .waitFor();
      await page.getByLabel('Memory basis', { exact: true }).selectOption('host');
      await row
        .getByRole('img', {
          name: 'linked-guest host memory share history, last known 48%',
          exact: true,
        })
        .waitFor();
      await capture('paused-host-share-row', row);
      record.checks.push(
        'Power-only native pause retains source/time/value and wire current state, but withdraws current memory in row/details, sort and numeric search; keyboard/touch disclosure explains why; both display and basis controls remain usable.',
      );
      const charts = [workload, canonical].map((surface) =>
        surface.locator('[data-history-group="utilization"]'),
      );
      const deferred = async (power, value = 95) => {
        for (const chart of charts) {
          await chart.locator('[data-history-deferred="memory"]').waitFor();
          assert.equal(await chart.locator('[data-history-current="memory"]').count(), 0);
          assert.match(
            await chart.locator('[data-history-deferred="memory"]').innerText(),
            new RegExp('guest ' + power),
          );
          const prior = chart.locator('[data-history-last-known="memory"]');
          if (value === null) assert.equal(await prior.count(), 0);
          else
            assert.match(
              await prior.innerText(),
              new RegExp(value.toFixed(1).replace('.', '\\.') + '%.*last known', 's'),
            );
        }
      };
      await deferred('paused');
      await capture('paused-canonical-history', canonical);
      for (const power of ['suspended', 'stopped', 'offline']) {
        await apply('power', power);
        await deferred(
          power === 'offline' ? 'stopped' : power,
          ['stopped', 'offline'].includes(power) ? null : 95,
        );
      }
      await row
        .getByRole('img', {
          name: 'linked-guest host memory share history, unavailable N/A',
          exact: true,
        })
        .waitFor();
      await capture('stopped-workload-history', workload);
      await apply('power', 'paused');
      for (const source of ['guest-agent-meminfo', 'guest-agent-meminfo-derived']) {
        await apply('reading', source, 10);
        await deferred('paused', 10);
      }
      await apply('reading', 'status-mem', 10);
      for (const chart of charts) await chart.locator('[data-history-current="memory"]').waitFor();
      await apply('reading', 'agent', 95);
      await apply('power', 'unknown');
      for (const chart of charts) await chart.locator('[data-history-current="memory"]').waitFor();
      await apply('power', 'paused');
      await apply('stale');
      await deferred('paused');
      await apply('power', 'running');
      for (const chart of charts)
        assert.match(
          await chart.locator('[data-history-deferred="memory"]').innerText(),
          /stopped reporting/,
        );
      await apply('recover');
      for (const chart of charts) {
        await chart.locator('[data-history-current="memory"]').waitFor();
        assert.match(
          await chart.locator('[data-history-current="memory"]').innerText(),
          /0\.0%.*current/s,
        );
        assert.equal(await chart.locator('[data-history-deferred="memory"]').count(), 0);
      }
      assert.equal(await notice.count(), 0);
      await capture('new-current-zero', canonical);
      record.checks.push(
        'Both empty-store History consumers keep paused/suspended reads last known and stopped reads unavailable; QGA source variants, host-source independence, unknown power, stale agent and new current zero controls pass.',
      );
      await apply('kind', 'system-container');
      await apply('power', 'stopped');
      await deferred('stopped', null);
      await apply('kind', 'vm');
      await apply('power', 'running');
      record.checks.push(
        'A linked Pulse agent does not supply current memory for a stopped LXC either.',
      );
      record.initial_inputs = await page.evaluate(() => window.__nativeMemoryInputs);
      // A new document supplies actual stored points; no failed journey is replayed.
      stored = true;
      await page.goto(origin + '?stored=1', { waitUntil: 'domcontentloaded' });
      await page.waitForFunction(() => window.__staleAgentMemory?.evidence().projected);
      const paths = [];
      for (const chart of charts) {
        await chart.locator('path[data-history-series]').first().waitFor();
        paths.push(
          await chart
            .locator('path[data-history-series]')
            .evaluateAll((items) => items.map((item) => item.getAttribute('d'))),
        );
      }
      for (const power of ['paused', 'stopped']) {
        await apply('power', power);
        for (let i = 0; i < charts.length; i++)
          assert.deepEqual(
            await charts[i]
              .locator('path[data-history-series]')
              .evaluateAll((items) => items.map((item) => item.getAttribute('d'))),
            paths[i],
          );
      }
      await capture('stored-history-preserved', canonical);
      record.checks.push(
        'Native power-only changes do not alter either drawer stored memory History path.',
      );
      record.original_carrier = original;
      record.paused_projection = paused.projected.memory;
      record.inputs = record.initial_inputs.concat(
        await page.evaluate(() => window.__nativeMemoryInputs),
      );
      assert(record.inputs.some((event) => event.trusted));
      assert.deepEqual(record.errors, []);
      assert.deepEqual(record.failedLoads, []);
      record.layout = await page.evaluate(() => ({
        client: document.documentElement.clientWidth,
        scroll: document.documentElement.scrollWidth,
      }));
      assert(record.layout.scroll <= record.layout.client + 1);
      await context.close();
      await browser.close();
      result.cleanup.browsers_closed++;
      browser = null;
    }
    result.result = 'passed';
    result.verified_at = new Date().toISOString();
  } catch (error) {
    result.error = String(error.stack || error);
    if (lastPage && !lastPage.isClosed()) {
      result.diagnostic = await lastPage
        .locator('body')
        .innerText()
        .catch(() => 'unavailable');
      await lastPage.screenshot({ path: output + '/failure.png', fullPage: true }).catch(() => {});
    }
    throw error;
  } finally {
    if (browser) {
      await browser.close();
      result.cleanup.browsers_closed++;
    }
    if (server) {
      await server.close();
      result.cleanup.server_closed = true;
    }
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
