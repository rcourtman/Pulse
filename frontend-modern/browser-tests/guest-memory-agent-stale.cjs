const assert = require('node:assert/strict');
const fs = require('node:fs');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const {
  createPublicationFixtureServer,
} = require('/workspace/frontend-modern/browser-tests/publication-fixture-server.cjs');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-memory-agent-stale';
const origin = 'http://127.0.0.1:5344';
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const settle = (page) =>
  page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  });

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    playwright: require('playwright/package.json').version,
    cases: [],
    captures: [],
    cleanup: {},
    driver_sha256: hash(root + '/browser-tests/guest-memory-agent-stale.cjs'),
    limits:
      'Real canonical delta merger, Workloads projection, row/bar/sparkline, guest drawer and canonical History with synthetic source-facet updates and empty History API fixtures. No collector, native restart, thaw, guest operation, installed or release acceptance.',
  };
  let server, browser;
  try {
    assert.equal(
      result.playwright,
      JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    server = createPublicationFixtureServer(
      root,
      5344,
      'browser-tests/guest-memory-agent-stale.html',
    );
    result.compiled = server.binding;
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1365, false],
      ['phone', webkit, 390, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const context = await browser.newContext({
        viewport: { width, height: 900 },
        isMobile: width < 768,
        hasTouch: width < 768,
        locale: 'en-GB',
        timezoneId: 'UTC',
        reducedMotion: 'reduce',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10_000);
      const record = {
        name,
        width,
        browser: browser.version(),
        checks: [],
        requests: [],
        errors: [],
      };
      result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      await page.route('**/*', (route) => {
        const request = route.request(),
          url = new URL(request.url());
        assert.equal(url.origin, origin, 'external request refused');
        if (!url.pathname.startsWith('/api/')) return route.continue();
        record.requests.push({ path: url.pathname, method: request.method() });
        assert.equal(request.method(), 'GET', 'no mutations');
        assert(!/diagnostics|\/run|guest-agent|qemu/.test(url.pathname), 'no guest operations');
        let body = { success: true, data: [], alerts: [], anomalies: [], config: null };
        if (url.pathname.includes('/charts/'))
          body = {
            resourceType: 'vm',
            resourceId: url.searchParams.get('resourceId') || 'fixture:pve1:101',
            range: url.searchParams.get('range') || '24h',
            start: 1791633600000,
            end: 1791720000000,
            source: 'store',
            metrics: {},
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
      await page.addInitScript(
        (dark) =>
          document.addEventListener('DOMContentLoaded', () =>
            document.documentElement.classList.toggle('dark', dark),
          ),
        dark,
      );
      await page.goto(origin, { waitUntil: 'domcontentloaded' });
      await page.waitForFunction(() => window.__staleAgentMemory?.evidence().projected);
      const row = page.getByRole('region', { name: 'Workload row', exact: true });
      const workload = page.getByRole('region', { name: 'Workload drawer', exact: true });
      const canonical = page.getByRole('region', { name: 'Canonical drawer', exact: true });
      const notice = row.locator('[data-workload-memory-read-status]');
      assert.equal(await notice.count(), 0);
      await page.evaluate(() => window.__staleAgentMemory.stale());
      await notice.waitFor();
      assert.match(await notice.innerText(), width < 768 ? /Prior/ : /Last known/);
      assert.match(await notice.textContent(), /stopped reporting/);
      await workload
        .getByText(/Last known · Pulse Agent \(stopped reporting\)/)
        .first()
        .waitFor();
      const evidence = await page.evaluate(() => window.__staleAgentMemory.evidence());
      assert.equal(evidence.projected.agentStale, true);
      assert.equal(
        evidence.projected.memory.observation.state,
        'current',
        'wire carrier not rewritten',
      );
      assert.equal(evidence.projected.memory.observation.observedAt, '2026-10-10T12:00:00Z');
      assert.equal(evidence.filterHigh, false);
      assert.deepEqual(evidence.sort, ['independent', 'fixture:pve1:101']);
      record.source_facet = evidence;
      record.checks.push(
        'agent-only canonical delta invalidates its current carrier; original source/time and value survive; filter/sort exclude it',
      );
      const capture = async (label, locator) => {
        await settle(page);
        const file = name + '-' + label + '.png';
        await locator.screenshot({ path: output + '/' + file });
        result.captures.push({ file, sha256: hash(output + '/' + file) });
      };
      await capture('stale-overview', workload);
      await page.getByLabel('Metric display').selectOption('sparklines');
      await row
        .getByRole('img', { name: 'linked-guest memory history, last known 95%', exact: true })
        .waitFor();
      await page.getByLabel('Memory basis').selectOption('host');
      await row
        .getByRole('img', {
          name: 'linked-guest host memory share history, last known 48%',
          exact: true,
        })
        .waitFor();
      await capture('stale-row', row);
      record.checks.push(
        'real display/basis controls preserve retained bar/sparkline accessibility',
      );
      const historyTab = async (surface) => {
        const tab = surface.getByRole('tab', { name: 'History', exact: true });
        if (width < 768) await tab.tap();
        else {
          await tab.focus();
          await page.keyboard.press('Enter');
        }
        const chart = surface.locator('[data-history-group="utilization"]');
        await chart.locator('[data-history-last-known="memory"]').waitFor();
        assert.equal(await chart.locator('[data-history-current="memory"]').count(), 0);
        assert.match(
          await chart.locator('[data-history-last-known="memory"]').innerText(),
          /95\.0%.*last known/s,
        );
        assert.match(
          await chart.locator('[data-history-deferred="memory"]').innerText(),
          /stopped reporting/,
        );
        assert.match(
          await chart.locator('[data-history-deferred="memory"]').innerText(),
          /2026-10-10 12:00:00 UTC/,
        );
        return chart;
      };
      const guestChart = await historyTab(workload);
      const resourceChart = await historyTab(canonical);
      await capture('stale-history', canonical);
      record.checks.push(
        'keyboard/touch History in both real drawers retains the original value, not a current fallback or invented trend',
      );
      for (const source of ['guest-agent-meminfo', 'status-mem']) {
        await page.evaluate((source) => window.__staleAgentMemory.reading(source, 10), source);
        await guestChart.locator('[data-history-current="memory"]').waitFor();
        await resourceChart.locator('[data-history-current="memory"]').waitFor();
        assert.equal(await guestChart.locator('[data-history-last-known="memory"]').count(), 0);
        assert.equal(await resourceChart.locator('[data-history-last-known="memory"]').count(), 0);
        assert.equal(
          (await page.evaluate(() => window.__staleAgentMemory.evidence())).projected.agentStale,
          true,
        );
      }
      record.checks.push(
        'unrelated QGA/Proxmox readings stay current while linked agent remains stale',
      );
      await page.evaluate(() => window.__staleAgentMemory.reading('agent', 0));
      await guestChart.locator('[data-history-last-known="memory"]').waitFor();
      assert.match(
        await guestChart.locator('[data-history-last-known="memory"]').innerText(),
        /0\.0%.*last known/s,
      );
      await page.evaluate(() => window.__staleAgentMemory.recover());
      await guestChart.locator('[data-history-current="memory"]').waitFor();
      await resourceChart.locator('[data-history-current="memory"]').waitFor();
      assert.match(
        await guestChart.locator('[data-history-current="memory"]').innerText(),
        /0\.0%.*current/s,
      );
      assert.match(
        await resourceChart.locator('[data-history-current="memory"]').innerText(),
        /0\.0%.*current/s,
      );
      assert.equal(await notice.count(), 0);
      await capture('reporting-zero', canonical);
      record.checks.push(
        'same-ID reporting source restores measured zero in both drawers; stale zero was not missing or healthy current',
      );
      assert.deepEqual(record.errors, []);
      const layout = await page.evaluate(() => ({
        client: document.documentElement.clientWidth,
        scroll: document.documentElement.scrollWidth,
      }));
      assert(layout.scroll <= layout.client + 1, JSON.stringify(layout));
      record.layout = layout;
      await context.close();
      await browser.close();
      browser = null;
    }
    result.result = 'passed';
    result.verified_at = new Date().toISOString();
  } catch (error) {
    result.error = String(error.stack || error);
    throw error;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browser = true;
    if (server) await server.close();
    result.cleanup.server = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
  }
})();
