const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/linked-guest-history/browser-final';
const origin = 'http://127.0.0.1:5345';
const runtime = [
  'frontend-modern/src/components/Infrastructure/resourceDetailDrawerMetricsHistoryModel.ts',
  'frontend-modern/src/components/Infrastructure/ResourceDetailDrawer.tsx',
  'frontend-modern/src/components/Workloads/GuestDrawer.tsx',
  'frontend-modern/src/components/Workloads/GuestDrawerHistory.tsx',
  'frontend-modern/src/hooks/useWorkloads.ts',
  'frontend-modern/src/utils/workloadGuestPresentation.ts',
];
const fixture = ['cjs', 'tsx', 'html'].map(
  (ext) => `frontend-modern/browser-tests/linked-guest-history.${ext}`,
);
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    playwright: require('playwright/package.json').version,
    runtime_hashes: Object.fromEntries(runtime.map((p) => [p, hash('/workspace/' + p)])),
    fixture_hashes: Object.fromEntries(fixture.map((p) => [p, hash('/workspace/' + p)])),
    cases: [],
    screenshots: [],
    cleanup: { browsers: false, server: false },
    limits:
      'Production canonical drawer and Workloads snapshot adapter/drawer, History, clients and CSS with synthetic snapshots/GET responses. No native filesystem, thaw, restart, guest action, installed or release acceptance.',
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: output + '/cache',
    optimizeDeps: { entries: ['browser-tests/linked-guest-history.html'] },
    server: { host: '127.0.0.1', port: 5345, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    for (const [engineName, engine, width, height, dark] of [
      ['desktop-light', chromium, 1280, 900, false],
      ['phone-dark', webkit, 390, 844, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      for (const lxc of [false, true])
        for (const guest of [false, true]) {
          const name = `${guest ? 'workloads' : 'canonical'}-${lxc ? 'lxc' : 'vm'}-${engineName}`;
          const page = await browser.newPage({
            viewport: { width, height },
            isMobile: width < 768,
            hasTouch: width < 768,
            locale: 'en-GB',
            timezoneId: 'UTC',
          });
          page.setDefaultTimeout(12000);
          page.setDefaultNavigationTimeout(45000);
          const record = {
            name,
            width,
            height,
            dark,
            lxc,
            guest,
            browser: browser.version(),
            checks: [],
            errors: [],
            requests: [],
          };
          result.cases.push(record);
          let stored = false;
          page.on('pageerror', (e) => record.errors.push(e.message));
          await page.route('**/*', (route) => {
            const request = route.request(),
              url = new URL(request.url());
            if (url.origin !== origin) {
              record.requests.push({ blocked: true, origin: url.origin });
              return route.abort();
            }
            if (!url.pathname.startsWith('/api/')) return route.continue();
            assert.equal(request.method(), 'GET');
            record.requests.push({ path: url.pathname, method: request.method(), stored });
            if (url.pathname === '/api/metrics-store/history') {
              assert.equal(url.searchParams.get('resourceType'), lxc ? 'system-container' : 'vm');
              assert.equal(url.searchParams.get('resourceId'), 'site-a:pve1:105');
              const start = Date.parse('2026-10-12T00:00:00Z'),
                end = Date.parse('2026-10-12T01:00:00Z');
              return route.fulfill({
                json: {
                  resourceType: lxc ? 'system-container' : 'vm',
                  resourceId: 'site-a:pve1:105',
                  range: url.searchParams.get('range'),
                  start,
                  end,
                  source: 'store',
                  metrics: stored
                    ? {
                        disk: [
                          { timestamp: start, value: 12, min: 12, max: 12 },
                          { timestamp: end, value: 30, min: 30, max: 30 },
                        ],
                      }
                    : {},
                },
              });
            }
            return route.fulfill({
              json:
                url.pathname === '/api/license/runtime-capabilities'
                  ? {
                      capabilities: [],
                      limits: [],
                      max_history_days: 90,
                      hosted_mode: false,
                      runtime: { build: 'community' },
                      blocked_capabilities: [],
                    }
                  : {
                      success: true,
                      data: [],
                      alerts: [],
                      anomalies: [],
                      config: null,
                      capabilities: [],
                      relationships: [],
                      recentChanges: [],
                      available: false,
                      audits: [],
                      count: 0,
                    },
            });
          });
          await page.addInitScript((isDark) => {
            document.addEventListener('DOMContentLoaded', () =>
              document.documentElement.classList.toggle('dark', isDark),
            );
          }, dark);
          const suffix =
            '?' +
            new URLSearchParams({
              ...(guest ? { guest: '1' } : {}),
              ...(lxc ? { lxc: '1' } : {}),
            }).toString();
          await page.goto(origin + '/browser-tests/linked-guest-history.html' + suffix);
          const activate = async (locator) => {
            if (width < 768) await locator.tap();
            else {
              await locator.focus();
              await locator.press('Enter');
            }
          };
          await activate(page.getByRole('tab', { name: 'History', exact: true }));
          const chart = page.locator('[data-history-group="utilization"]');
          const current = () => chart.locator('[data-history-current="disk"]');
          const prior = () => chart.locator('[data-history-last-known="disk"]');
          const deferred = () => chart.locator('[data-history-deferred="disk"]');
          const set = (label) => activate(page.getByRole('button', { name: label, exact: true }));
          const check = async (label, fn) => {
            await fn();
            record.checks.push(label);
          };
          const text = async (locator, expression) => {
            await locator.waitFor();
            assert.match(await locator.innerText(), expression);
          };
          const missing = async (locator) => assert.equal(await locator.count(), 0);
          const capture = async (state) => {
            await page.evaluate(async () => {
              await document.fonts.ready;
              await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
            });
            const file = `${name}-${state}.png`;
            await page.screenshot({ path: path.join(output, file), fullPage: true });
            result.screenshots.push({ file, name, state, sha256: hash(path.join(output, file)) });
            const geometry = await page.evaluate(() => ({
              width: document.documentElement.clientWidth,
              scroll: document.documentElement.scrollWidth,
            }));
            assert.ok(geometry.scroll <= geometry.width + 1, JSON.stringify(geometry));
          };
          await check('Live linked reading survives QGA failure', async () => {
            await text(current(), /62.0%\s*current/);
            await missing(deferred());
          });
          await page.evaluate(() => {
            window.__chart = document.querySelector('[data-history-group="utilization"]');
          });
          await check('Silent agent is last known despite online platform', async () => {
            await set('Silent agent');
            await text(prior(), /62.0%\s*last known/);
            await missing(current());
            await text(deferred(), /The Pulse Agent in this guest stopped reporting/);
          });
          await capture('silent');
          await check('QGA recovery and Last seen cannot renew the agent report', async () => {
            await set('QGA recovered');
            await set('Advance snapshot');
            await text(prior(), /62.0%\s*last known/);
            await text(deferred(), /Pulse Agent/);
          });
          await check('Stopped in-guest reading is unavailable, not last known', async () => {
            await set('Stopped guest');
            await missing(current());
            await missing(prior());
            await text(deferred(), /unavailable/);
          });
          await capture('stopped');
          await check('Fresh linked-agent zero is current', async () => {
            await set('Fresh zero');
            await text(current(), /0.0%\s*current/);
            await missing(deferred());
          });
          await check('Missing aggregate is not fabricated from filesystem rows', async () => {
            await set('Missing aggregate');
            await missing(current());
            await missing(prior());
          });
          await check('Platform filesystem precedence uses its own read state', async () => {
            await set('Platform filesystems');
            if (lxc) {
              await text(current(), /62.0%\s*current/);
              await missing(deferred());
            } else {
              await text(prior(), /62.0%\s*last known/);
              await text(deferred(), /operation lock/);
            }
          });
          await check('Explicit empty platform list does not borrow agent freshness', async () => {
            await set('Empty platform list');
            if (lxc) {
              await text(current(), /62.0%\s*current/);
            } else {
              await text(prior(), /62.0%\s*last known/);
              await text(deferred(), /operation lock/);
            }
          });
          await check('Stopped LXC platform storage stays independent', async () => {
            await set('Stopped platform');
            if (lxc) {
              await text(current(), /62.0%\s*current/);
              await missing(deferred());
            } else {
              await missing(current());
              await missing(prior());
              await text(deferred(), /VM is stopped/);
            }
          });
          await check(
            'Stored observations remain intact through source/power changes',
            async () => {
              await set('Live agent');
              stored = true;
              await activate(page.getByRole('button', { name: 'Refresh history', exact: true }));
              await chart.locator('path').waitFor();
              const before = await chart.locator('path').getAttribute('d');
              assert.ok(before);
              await set('Silent agent');
              await text(deferred(), /Pulse Agent/);
              assert.equal(await chart.locator('path').getAttribute('d'), before);
              await missing(prior());
              await set('Stopped guest');
              await text(deferred(), /unavailable/);
              assert.equal(await chart.locator('path').getAttribute('d'), before);
              await missing(current());
              assert.match(await chart.innerText(), /30.0%/);
            },
          );
          await check(
            'Same mounted chart, usable independent metrics and no fabricated samples',
            async () => {
              assert.equal(
                await page.evaluate(
                  () =>
                    window.__chart === document.querySelector('[data-history-group="utilization"]'),
                ),
                true,
              );
              await text(chart.locator('[data-history-current="cpu"]'), /10.0%\s*current/);
              await text(chart.locator('[data-history-current="memory"]'), /0.0%\s*current/);
              assert.equal(await chart.locator('path').count(), 1);
              assert.deepEqual(record.errors, []);
              assert.ok(record.requests.every((r) => !r.blocked));
            },
          );
          await capture('stored-stopped');
          await page.close();
        }
      await browser.close();
      browser = null;
    }
    result.result = 'passed';
  } catch (error) {
    result.error = String(error);
    throw error;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browsers = true;
    await server.close();
    result.cleanup.server = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
