const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const {
  createPublicationFixtureServer,
} = require('/workspace/frontend-modern/browser-tests/publication-fixture-server.cjs');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/backup-completion-time/browser');
const origin = 'http://127.0.0.1:5298';
const start = Date.parse('2026-10-12T02:00:00Z');
const iso = '2026-10-12T00:30:00.000Z';
const label = '2026-10-12 00:30:00 UTC';
const runtime = [
  'frontend-modern/src/utils/format.ts',
  'frontend-modern/src/components/Workloads/GuestRowCells.tsx',
  'frontend-modern/src/components/Workloads/guestDrawerModel.ts',
];
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const result = {
  sourceHashes: Object.fromEntries(
    runtime.map((p) => [p, hash(fs.readFileSync(path.join(root, p)))]),
  ),
  playwright: require('playwright/package.json').version,
  scope:
    'Real AlertsAPI policy read, production VM/LXC row, table badge/indicator/hover, shared dialog and guest drawer with synthetic completion timestamps. Non-UTC desktop/phone contexts; no native backup, guest operation or availability claim.',
  checks: [],
  requests: [],
  loaded: {},
  captures: [],
  browsers: [],
  errors: [],
  consoleErrors: [],
  cleanup: {},
};
let server;
(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  server = createPublicationFixtureServer(
    frontend,
    5298,
    'browser-tests/backup-completion-time.html',
  );
  result.compiled = server.binding;
  result.source_sha = server.binding.source_sha;
  await server.listen();
  for (const spec of [
    {
      name: 'desktop-west',
      engine: chromium,
      launch: { headless: true, channel: 'chromium', args: ['--no-sandbox'] },
      viewport: { width: 1365, height: 900 },
      timezoneId: 'America/Los_Angeles',
      locale: 'en-US',
      colorScheme: 'light',
    },
    {
      name: 'phone-east',
      engine: webkit,
      launch: { headless: true },
      viewport: { width: 390, height: 844 },
      timezoneId: 'Asia/Tokyo',
      locale: 'ja-JP',
      colorScheme: 'dark',
      isMobile: true,
      hasTouch: true,
    },
  ]) {
    const browser = await spec.engine.launch(spec.launch);
    result.browsers.push({
      name: spec.name,
      version: browser.version(),
      viewport: spec.viewport,
      timezoneId: spec.timezoneId,
      locale: spec.locale,
    });
    let context;
    try {
      context = await browser.newContext({
        viewport: spec.viewport,
        timezoneId: spec.timezoneId,
        locale: spec.locale,
        colorScheme: spec.colorScheme,
        isMobile: !!spec.isMobile,
        hasTouch: !!spec.hasTouch,
      });
      const page = await context.newPage();
      page.setDefaultTimeout(8000);
      await page.clock.install({ time: start - 300_000 });
      await page.clock.pauseAt(start);
      page.on('pageerror', (e) => result.errors.push({ browser: spec.name, message: e.message }));
      page.on('console', (m) => {
        if (m.type() === 'error') result.consoleErrors.push({ browser: spec.name, text: m.text() });
      });
      await page.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin) {
          result.errors.push({ external: req.url() });
          return route.abort();
        }
        if (url.pathname.startsWith('/api/')) {
          const record = {
            browser: spec.name,
            method: req.method(),
            path: url.pathname,
            query: url.search,
          };
          result.requests.push(record);
          assert.equal(req.method(), 'GET', 'proof is read-only');
          let body,
            status = 200;
          if (url.pathname === '/api/alerts/config')
            body = {
              enabled: true,
              activationState: 'active',
              backupDefaults: { freshHours: 1, staleHours: 3 },
            };
          else if (url.pathname === '/api/license/runtime-capabilities')
            body = {
              capabilities: [],
              limits: [],
              max_history_days: 90,
              runtime: { build: 'community', label: 'Fixture community' },
            };
          else if (url.pathname === '/api/alerts/active') body = [];
          else if (url.pathname === '/api/resources')
            body = { data: [], pagination: { page: 1, limit: 100, total: 0, totalPages: 1 } };
          else if (url.pathname === '/api/ai/intelligence') body = null;
          else if (url.pathname === '/api/ai/intelligence/anomalies')
            body = { anomalies: [], count: 0 };
          else if (
            url.pathname.startsWith('/api/resources/') &&
            url.pathname.endsWith('/operator-state')
          )
            body = { configured: false, state: null };
          else if (url.pathname.startsWith('/api/guests/metadata/')) body = { customUrl: '' };
          else {
            status = 404;
            body = {};
            record.unmatched = true;
          }
          record.status = status;
          return route.fulfill({
            status,
            contentType: 'application/json',
            body: JSON.stringify(body),
          });
        }
        if (url.pathname.startsWith('/src/')) {
          const local = path.join(frontend, decodeURIComponent(url.pathname));
          if (fs.existsSync(local))
            result.loaded[path.relative(root, local)] = hash(fs.readFileSync(local));
        }
        return route.continue();
      });
      await page.routeWebSocket(/\/ws(?:\?|$)/, () => {});
      await page.goto(`${origin}/browser-tests/backup-completion-time.html`, {
        waitUntil: 'domcontentloaded',
        timeout: 30000,
      });
      await page.getByTestId('proof-policy').getByText('Policy read', { exact: true }).waitFor();
      if (spec.colorScheme === 'dark')
        await page.evaluate(() => document.documentElement.classList.add('dark'));
      const table = page.getByRole('region', { name: 'Backup table row' });
      const badge = () => table.getByRole('button', { name: /^Backup status:/ });
      // Running changes the existing indicator's accessible name; use its
      // native disclosure semantics, then separately assert its activity copy.
      const indicator = () => table.locator('button[aria-haspopup="dialog"]');
      const dialog = () => page.getByRole('dialog', { name: /^Backup status details for / });
      const drawerTime = () =>
        page
          .getByTestId('guest-technical-details')
          .getByText('Last completed backup', { exact: true })
          .locator('xpath=ancestor::tr')
          .locator('time');
      const record = async (name) => {
        const hasTime = await dialog().locator('time').count();
        result.checks.push({
          browser: spec.name,
          name,
          time: hasTime ? await dialog().locator('time').textContent() : null,
          datetime: hasTime ? await dialog().locator('time').getAttribute('datetime') : null,
        });
      };
      const capture = async (name) => {
        const p = path.join(output, `${spec.name}-${name}.png`);
        await page.screenshot({ path: p, fullPage: true });
        result.captures.push({
          path: path.relative(root, p),
          bytes: fs.statSync(p).size,
          sha256: hash(fs.readFileSync(p)),
        });
      };
      const checkTime = async (expectedLabel = label, expectedIso = iso) => {
        const time = dialog().locator('time');
        await time.waitFor();
        assert.equal(await time.textContent(), expectedLabel);
        assert.equal(await time.getAttribute('datetime'), expectedIso);
        assert.equal(await drawerTime().textContent(), expectedLabel);
        assert.equal(await drawerTime().getAttribute('datetime'), expectedIso);
        assert(
          await time.evaluate((e) => e.scrollWidth <= e.clientWidth),
          'completion text must not be clipped',
        );
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
      };
      const open = async (trigger) => {
        if (spec.hasTouch) await trigger.tap();
        else {
          await trigger.focus();
          await page.keyboard.press('Enter');
        }
        await dialog().waitFor();
        await dialog().evaluate(async (e) => {
          await Promise.all(
            e
              .getAnimations({ subtree: true })
              .filter((a) => Number.isFinite(a.effect?.getTiming().iterations))
              .map((a) => a.finished.catch(() => {})),
          );
        });
        assert.equal(await trigger.getAttribute('aria-expanded'), 'true');
        assert.equal(await page.getByTestId('proof-actions').innerText(), 'Row actions: 0');
        assert(
          await page
            .getByRole('button', { name: 'Close backup details' })
            .evaluate((e) => e === document.activeElement),
        );
      };
      const close = async (trigger) => {
        await page.keyboard.press('Escape');
        assert.equal(await dialog().count(), 0);
        assert(await trigger.evaluate((e) => e === document.activeElement));
      };
      const local = await page.evaluate(
        (iso) => ({
          date: new Date(iso).toLocaleDateString(),
          hour: new Date(iso).getHours(),
          zone: Intl.DateTimeFormat().resolvedOptions().timeZone,
        }),
        iso,
      );
      assert.equal(local.zone, spec.timezoneId);
      assert.notEqual(local.hour, 0);
      result.checks.push({
        browser: spec.name,
        name: 'local context genuinely differs from UTC',
        local,
      });
      for (const type of ['qemu', 'lxc']) {
        await page.selectOption('#proof-type', type);
        await page.selectOption('#proof-state', 'numeric');
        const beforeOpen = result.requests.length;
        await open(badge());
        await checkTime();
        assert.equal(result.requests.length, beforeOpen, 'opening evidence performs no API reads');
        await record(`${type}: native row badge and drawer agree across local calendar/time`);
        if (type === 'qemu') await capture('completed-dialog');
        for (const state of ['offset-west', 'offset-east']) {
          await page.selectOption('#proof-state', state);
          await checkTime();
          await record(`${type}: ${state} producer observation keeps the same absolute time`);
        }
        await page.selectOption('#proof-state', 'running');
        assert(await dialog().getByText('Backup running now…', { exact: true }).isVisible());
        const beforeTick = result.requests.length;
        await page.clock.runFor(60_000);
        await checkTime();
        assert(
          result.requests
            .slice(beforeTick)
            .every((r) => r.path === '/api/ai/intelligence/anomalies'),
          'passive tick adds no requests beyond the unchanged row anomaly poll',
        );
        await record(`${type}: running activity and passive age do not renew completion`);
        await page.selectOption('#proof-state', 'ended');
        await checkTime();
        await page.selectOption('#proof-state', 'new');
        await checkTime('2026-10-12 01:45:00 UTC', '2026-10-12T01:45:00.000Z');
        await record(`${type}: only replacement completion changes both surfaces`);
        await page.selectOption('#proof-state', 'invalid');
        assert.equal(await dialog().locator('time').count(), 0);
        assert.equal(await drawerTime().count(), 0);
        assert((await dialog().innerText()).includes('invalid timestamp'));
        await record(`${type}: invalid timestamp withdraws the time without calling it absence`);
        if (type === 'qemu' && spec.hasTouch) await capture('unknown-dialog');
        await page.selectOption('#proof-state', 'absent');
        assert.equal(await dialog().locator('time').count(), 0);
        assert((await dialog().innerText()).includes('No completed backup yet'));
        await record(`${type}: first running backup has no completed time`);
        await page.selectOption('#proof-state', 'future');
        assert.equal(await dialog().locator('time').count(), 0);
        assert((await dialog().innerText()).includes('timestamp is in the future'));
        const beforeFuture = result.requests.length;
        await page.clock.runFor(30_000);
        assert.equal(
          await dialog().locator('time').getAttribute('datetime'),
          await drawerTime().getAttribute('datetime'),
        );
        assert.equal(
          await dialog().locator('time').textContent(),
          await drawerTime().textContent(),
        );
        assert(
          result.requests
            .slice(beforeFuture)
            .every((r) => r.path === '/api/ai/intelligence/anomalies'),
        );
        await record(`${type}: now-valid supplied time reappears, no new observation manufactured`);
        await close(badge());
        await record(`${type}: Escape returns focus to the row disclosure`);
      }
      await page.selectOption('#proof-type', 'qemu');
      await page.selectOption('#proof-state', 'numeric');
      if (!spec.hasTouch) {
        await badge().hover();
        const tooltip = page
          .locator('[data-tooltip-portal="true"]')
          .filter({ hasText: 'Last completed backup' });
        await tooltip.waitFor();
        assert.equal(await tooltip.locator('time').textContent(), label);
        assert.equal(await tooltip.locator('time').getAttribute('datetime'), iso);
        await record('fine-pointer hover presents the same UTC evidence');
        await capture('completed-hover');
        await page.mouse.move(1200, 850);
      }
      await page.getByRole('button', { name: 'Hide Backup column', exact: true }).click();
      await page.selectOption('#proof-state', 'running');
      const beforeIndicator = result.requests.length;
      assert((await indicator().getAttribute('aria-label')).startsWith('Backup running now'));
      await open(indicator());
      await checkTime();
      assert.equal(result.requests.length, beforeIndicator);
      if (spec.hasTouch) {
        const target = await indicator().boundingBox();
        assert(target.width >= 24 && target.height >= 24);
      }
      await record(
        'name-cell indicator retains explicit completion time, activity and row-action isolation',
      );
      await capture('running-indicator');
      await close(indicator());
      await record('indicator Escape returns focus without opening the row');
    } finally {
      if (context) {
        await context.close();
        result.cleanup[`${spec.name}-context`] = 'closed';
      }
      await browser.close();
      result.cleanup[`${spec.name}-browser`] = 'closed';
    }
  }
  assert.equal(result.errors.length, 0);
  assert.equal(result.consoleErrors.length, 0);
  assert(!result.requests.some((r) => r.unmatched));
  for (const p of runtime)
    assert.equal(
      result.compiled.sources[p.replace('frontend-modern/', '')],
      result.sourceHashes[p],
    );
  result.result = 'passed';
})()
  .catch((e) => {
    result.result = 'failed';
    result.failure = String(e.stack || e);
    process.exitCode = 1;
  })
  .finally(async () => {
    if (server) {
      await server.close();
      result.cleanup.server = 'closed';
    }
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2));
    console.log(
      JSON.stringify({
        result: result.result,
        checks: result.checks.length,
        captures: result.captures.length,
        failure: result.failure,
        cleanup: result.cleanup,
      }),
    );
  });
