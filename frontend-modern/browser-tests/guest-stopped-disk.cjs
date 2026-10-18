const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-stopped-disk/browser';
const origin = 'http://127.0.0.1:5338';
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const source = [
  'frontend-modern/src/utils/workloadGuestPresentation.ts',
  'frontend-modern/browser-tests/guest-disk-provenance.tsx',
  'frontend-modern/browser-tests/guest-stopped-disk.cjs',
];
const settle = (page) => page.evaluate(async () => {
  await document.fonts.ready;
  await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
});

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    playwright: require('playwright/package.json').version,
    source_sha256: Object.fromEntries(source.map((file) => [file, hash('/workspace/' + file)])),
    cases: [], screenshots: [], cleanup: { browser: false, server: false },
    limits: 'Production row, full drawer, History and CSS with synthetic local observations/API. No guest command, native QGA/thaw/writes, installed restart, physical-phone or release acceptance.',
  };
  assert.equal(result.playwright, JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({ root, configFile: path.join(root, 'vite.config.ts'), cacheDir: output + '/cache', server: { host: '127.0.0.1', port: 5338, strictPort: true } });
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['desktop-light', chromium, 1280, false], ['phone-dark', webkit, 390, true],
    ]) {
      browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width, height: width < 768 ? 844 : 900 }, isMobile: width < 768, hasTouch: width < 768, locale: 'en-GB', timezoneId: 'UTC' });
      const page = await context.newPage();
      page.setDefaultTimeout(12000);
      page.setDefaultNavigationTimeout(60000);
      const record = { name, browser: browser.version(), width, dark, errors: [], requests: [], checks: [] };
      let historyMode = 'stored';
      result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      await page.route('**/*', (route) => {
        const request = route.request(); const url = new URL(request.url());
        if (url.origin !== origin) { record.requests.push({ path: url.pathname, method: request.method(), blocked: true }); return route.abort(); }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        record.requests.push({ path: url.pathname, method: request.method() });
        if (url.pathname === '/api/metrics-store/history') {
          const end = Date.now(), start = end - 3600_000;
          const point = (timestamp, value) => ({ timestamp, value, min: value, max: value });
          return route.fulfill({ json: { resourceType: 'vm', resourceId: url.searchParams.get('resourceId'), range: url.searchParams.get('range'), start, end, metrics: historyMode === 'stored' ? { disk: [point(start, 25), point(end, 50)] } : {}, source: 'store' } });
        }
        return route.fulfill({ json: url.pathname === '/api/license/runtime-capabilities'
          ? { capabilities: [], limits: [], max_history_days: 7, hosted_mode: false, runtime: { build: 'community' }, blocked_capabilities: [] }
          : { success: true, data: [], alerts: [], anomalies: [], config: null } });
      });
      await page.addInitScript((isDark) => {
        document.addEventListener('DOMContentLoaded', () => { if (isDark) document.documentElement.classList.add('dark'); });
        window.__trustedInput = [];
        document.addEventListener('click', (event) => window.__trustedInput.push(event.isTrusted), true);
      }, dark);
      // Charts poll every 30 seconds; the proof drives that poll with the clock.
      await page.clock.install();
      await page.goto(origin + '/browser-tests/guest-disk-provenance.html');
      await page.waitForFunction(() => window.__guestDiskProvenance);
      const apply = async (value) => { await page.evaluate((next) => window.__guestDiskProvenance.apply(next), value); await settle(page); };
      const row = page.locator('[data-guest-id="fixture-pve:pve-a:101"]');
      const control = page.locator('[data-guest-id="fixture-pve:pve-a:102"]');
      const cell = row.locator('[data-workload-col="disk"]');
      const click = (target) => width < 768 ? target.tap() : target.click();
      await apply({ reason: '', lock: '', status: 'stopped', data: true, usage: 90 });
      for (const mode of ['bars', 'sparklines']) {
        await page.evaluate((value) => window.__guestDiskProvenance.mode(value), mode); await settle(page);
        assert.equal(await cell.locator('[data-workload-disk-read-status] [aria-hidden="true"]').count(), 0);
        assert.equal(await cell.getAttribute('title'), 'Guest filesystem stats unavailable while the VM is stopped.');
        assert.equal(await cell.getByRole('progressbar').count(), 0);
        for (const label of await cell.getByRole('img').evaluateAll((items) => items.map((item) => item.getAttribute('aria-label')))) assert.doesNotMatch(label, /current 90%/);
        assert.equal(await control.locator('[data-workload-disk-read-status]').count(), 0);
      }
      record.checks.push('Both row modes withdraw stopped-VM current usage; the independent running control is unchanged.');
      const disclosure = row.locator('[data-row-action]');
      if (width < 768) await click(cell);
      else { await disclosure.focus(); await page.keyboard.press('Enter'); }
      const drawer = page.getByRole('region', { name: 'Guest details' });
      await drawer.waitFor({ state: 'visible' });
      // Filesystems have their own Overview section; an unavailable one may be
      // lifted into Worth checking, so read the whole Overview.
      const technical = drawer.getByTestId('guest-drawer-overview');
      const filesystem = technical.getByText('/data', { exact: true }).locator('xpath=ancestor::tr');
      const capture = async (state, target) => {
        await target.scrollIntoViewIfNeeded(); await settle(page);
        const file = `${name}-${state}.png`; await target.screenshot({ path: output + '/' + file });
        result.screenshots.push({ file, sha256: hash(output + '/' + file), name, state });
      };
      assert.match(await filesystem.innerText(), /Usage unavailable/);
      assert.doesNotMatch(await filesystem.innerText(), /90%|9\.00 GB/);
      assert.equal(await filesystem.getByRole('progressbar').count(), 0);
      assert.equal(await technical.getByRole('link', { name: 'Filesystem reading guidance' }).getAttribute('href'), '/docs/VM_DISK_MONITORING#a-missing-reading-is-not-an-installation-diagnosis');
      await capture('stopped-filesystems', technical);
      // A linked agent's cached numbers cannot override the known VM power state.
      await apply({ disksFromAgent: true });
      assert.match(await filesystem.innerText(), /Usage unavailable/);
      record.checks.push('Platform and linked-agent carriers obey the same stopped-VM rule in filesystem detail.');
      // History charts sit on Overview beside the filesystem detail.
      const chart = drawer.locator('[data-history-group="utilization"]');
      await page.waitForFunction(() => document.querySelector('[data-history-group="utilization"] path[data-history-series]'));
      const stored = await chart.locator('path[data-history-series]').evaluateAll((items) => items.map((item) => item.getAttribute('d')));
      assert.ok(stored.length > 0);
      assert.equal(await chart.locator('[data-history-current="disk"]').count(), 0);
      assert.equal(await chart.locator('[data-history-last-known="disk"]').count(), 0);
      assert.match(await chart.locator('[data-history-deferred="disk"]').innerText(), /VM is stopped/);
      await capture('stopped-history', chart);
      await apply({ status: 'running', reason: 'prev-agent-busy', disksFromAgent: false });
      assert.match(await chart.locator('[data-history-deferred="disk"]').innerText(), /earlier guest request/);
      assert.equal(await chart.locator('[data-history-last-known="disk"]').count(), 0);
      assert.equal(await chart.locator('[data-history-current="disk"]').count(), 0);
      assert.deepEqual(await chart.locator('path[data-history-series]').evaluateAll((items) => items.map((item) => item.getAttribute('d'))), stored);
      record.checks.push('Stored historical values/paths stay separate from retained live numbers throughout stop and restart.');
      // A separate synthetic empty-store response exercises the live fallback
      // legend, not data removal or a guest probe.
      historyMode = 'empty';
      const read = page.waitForResponse((r) => r.url().includes('/api/metrics-store/history'));
      await page.clock.runFor(30_000);
      await read;
      await chart.getByText('No stored history in this range', { exact: true }).waitFor();
      assert.match(await chart.locator('[data-history-last-known="disk"]').innerText(), /90.0%.*last known/s);
      await apply({ status: 'stopped', reason: '' });
      assert.equal(await chart.locator('[data-history-current="disk"]').count(), 0);
      assert.equal(await chart.locator('[data-history-last-known="disk"]').count(), 0);
      assert.match(await chart.locator('[data-history-deferred="disk"]').innerText(), /VM is stopped/);
      await apply({ status: 'running', reason: 'prev-agent-busy' });
      assert.match(await chart.locator('[data-history-last-known="disk"]').innerText(), /90.0%.*last known/s);
      await capture('restart-retained', chart);
      await apply({ reason: '', usage: 0 });
      assert.match(await chart.locator('[data-history-current="disk"]').innerText(), /0.0%.*current/s);
      assert.equal(await chart.locator('[data-history-deferred="disk"]').count(), 0);
      assert.equal(await chart.locator('path[data-history-series]').count(), 0);
      record.checks.push('Empty-store History never fabricates points: restart deferral is last known and fresh measured zero returns as current.');
      assert.equal(await filesystem.getByRole('progressbar').getAttribute('aria-valuenow'), '0');
      await apply({ kind: 'lxc', status: 'stopped', reason: '', usage: 30 });
      assert.equal(await filesystem.getByRole('progressbar').getAttribute('aria-valuenow'), '30');
      await capture('stopped-lxc-control', technical);
      record.checks.push('Stopped LXC platform storage keeps its independent reading.');
      const geometry = await technical.evaluate((el) => ({ width: el.clientWidth, scroll: el.scrollWidth, documentWidth: document.documentElement.clientWidth, documentScroll: document.documentElement.scrollWidth }));
      assert.ok(geometry.scroll <= geometry.width + 1, JSON.stringify(geometry));
      assert.ok(geometry.documentScroll <= geometry.documentWidth + 1, JSON.stringify(geometry));
      record.geometry = geometry;
      await click(drawer.getByRole('button', { name: 'Collapse backup-guest details' }));
      assert.equal(await drawer.count(), 0);
      assert.ok((await page.evaluate(() => window.__trustedInput)).some(Boolean));
      assert.deepEqual(record.errors, []);
      // Opening details records one content-free usage milestone; nothing else may write.
      assert.ok(record.requests.every((item) => (['GET', 'HEAD'].includes(item.method) || (item.method === 'POST' && item.path === '/api/usage/workload-history')) && !item.blocked), JSON.stringify(record.requests));
      record.checks.push('Trusted keyboard/touch disclosure and close; no overflow, page error, off-origin or mutating request.');
      await context.close(); await browser.close(); browser = null;
    }
    result.result = 'passed';
  } catch (error) {
    result.error = String(error.stack || error); throw error;
  } finally {
    if (browser) await browser.close(); result.cleanup.browser = true;
    await server.close(); result.cleanup.server = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
