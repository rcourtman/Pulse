const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const parent = false;
(async () => {
  const root = parent ? '/workspace/tmp/resource-access-parent/frontend-modern' : '/workspace/frontend-modern';
  const artifacts = `/workspace/tmp/resource-evidence-${parent ? 'parent' : 'final'}-proof`;
  fs.mkdirSync(artifacts, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({ root, configFile: path.join(root, 'vite.config.ts'), cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-')), server: { host: '127.0.0.1', port: 5267, strictPort: true, fs: { allow: ['/workspace'] } } });
  const playwright = require('playwright/package.json').version;
  assert.equal(playwright, JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
  const observations = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, phone] of [['chromium-desktop', chromium, false], ['webkit-phone', webkit, true]]) {
      browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const page = await browser.newPage({ viewport: phone ? { width: 390, height: 844 } : { width: 1365, height: 900 }, hasTouch: phone, isMobile: phone });
      page.setDefaultTimeout(60000);
      let facetMode = 'success', actionMode = 'success', actor = 'Authorised response actor';
      const requests = [], errors = [], offOrigin = [], checks = [], held = [];
      page.on('pageerror', error => errors.push(error.message));
      await page.route('**/*', async route => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5267') { offOrigin.push(url.origin); return route.abort(); }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname.endsWith('/facets')) {
          const mode = facetMode;
          requests.push({ path: url.pathname, query: Object.fromEntries(url.searchParams), mode });
          if (mode === 'hold') await new Promise(resolve => held.push(resolve));
          if (mode === '403' || mode === '503') return route.fulfill({ status: Number(mode), json: { error: 'Fixture service unavailable' } });
          return route.fulfill({ json: { capabilities: [], relationships: [], counts: { recentChanges: 1 }, recentChanges: [{ id: 'remote-event', resourceId: 'pbs-a', observedAt: '2026-10-03T03:00:00Z', kind: 'restart', sourceType: 'platform_event', confidence: 'high', actor }] } });
        }
        if (url.pathname === '/api/audit/actions') {
          requests.push({ path: url.pathname, mode: actionMode });
          return actionMode === '503' ? route.fulfill({ status: 503, json: { error: 'Fixture actions unavailable' } }) : route.fulfill({ json: { audits: [], count: 0 } });
        }
        return route.fulfill({ json: url.pathname === '/api/license/runtime-capabilities' ? { capabilities: [], limits: [], max_history_days: 7, hosted_mode: false, runtime: { build: 'community' }, blocked_capabilities: [] } : { capabilities: [], relationships: [], recentChanges: [], audits: [], count: 0, data: [], enabled: false } });
      });
      const record = (name, pass) => { checks.push({ name, pass }); };
      const screenshot = state => page.screenshot({ path: path.join(artifacts, `${name}-${state}.png`), fullPage: true });
      const changes = page.getByTestId('resource-change-history-section');
      const actions = page.getByTestId('resource-action-history-section');
      await page.goto('http://127.0.0.1:5267/browser-tests/resource-evidence-access.html', { waitUntil: 'domcontentloaded', timeout: 120000 });
      if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
      await changes.getByText('Authorised response actor', { exact: true }).waitFor();
      record('successful remote evidence replaces snapshot', !(await changes.getByText('Embedded snapshot actor').count()));
      await screenshot('loaded');
      facetMode = '403'; actionMode = '503';
      await page.getByRole('button', { name: 'Remount details' }).click();
      await changes.getByRole('button', { name: 'Retry', exact: true }).waitFor();
      await page.waitForTimeout(500);
      record('denial withdraws embedded evidence', !(await changes.getByText('Embedded snapshot actor').count()) && !(await changes.getByText('Authorised response actor').count()));
      record('bounded denial guidance, not empty history', (await changes.getByText('Access denied. Check your permissions and license plan.', { exact: true }).count()) === 1 && !(await changes.getByText('No events yet.', { exact: true }).count()));
      record('denial is labelled unavailable', (await changes.getByText('Changes unavailable', { exact: true }).count()) === 1);
      await page.getByRole('tab', { name: 'Manage', exact: true }).click();
      record('failed first action read has visible retry, not empty history', (await actions.count()) === 1 && (await actions.getByRole('button', { name: 'Retry', exact: true }).count()) === 1 && !(await actions.getByText('No actions yet.', { exact: true }).count()));
      await screenshot('action-failed');
      await page.getByRole('tab', { name: 'Overview', exact: true }).click();
      await screenshot('denied');
      facetMode = 'hold';
      await changes.getByRole('button', { name: 'Retry', exact: true }).focus();
      await changes.getByRole('button', { name: 'Retry', exact: true }).press('Enter');
      await changes.getByText('Refreshing changes...', { exact: true }).waitFor();
      record('retry keeps denied evidence withdrawn', !(await changes.getByText('Embedded snapshot actor').count()) && !(await changes.getByText('Authorised response actor').count()));
      await screenshot('retry-pending');
      actor = 'Fresh response actor'; facetMode = 'success';
      held.splice(0).forEach(resolve => resolve());
      await changes.getByText('Fresh response actor', { exact: true }).waitFor();
      record('fresh successful retry restores evidence and clears guidance', !(await changes.getByText('Access denied. Check your permissions and license plan.').count()));
      await screenshot('recovered');
      await changes.getByRole('button', { name: 'Filter history', exact: true }).click();
      facetMode = '403';
      await changes.getByLabel('Change kind', { exact: true }).selectOption('restart');
      await changes.getByRole('button', { name: 'Retry', exact: true }).waitFor();
      record('filtered denial cannot substitute authorised unfiltered events', !(await changes.getByText('Fresh response actor').count()) && (await changes.getByText('Filtered changes unavailable', { exact: true }).count()) === 1);
      await screenshot('filtered-denied');
      await changes.getByRole('button', { name: 'Clear filters', exact: true }).click();
      await changes.getByText('Fresh response actor', { exact: true }).waitFor();
      record('clearing filter restores independently authorised base request', true);
      facetMode = '503';
      await page.getByRole('button', { name: 'Remount details' }).click();
      await changes.getByRole('button', { name: 'Retry', exact: true }).waitFor();
      record('transient failure preserves embedded snapshot evidence', (await changes.getByText('Embedded snapshot actor', { exact: true }).count()) === 1);
      await screenshot('transient');
      record('no page errors or off-origin requests', errors.length === 0 && offOrigin.length === 0);
      observations.push({ name, browser: browser.version(), viewport: page.viewportSize(), requests, errors, offOrigin, checks });
      await browser.close(); browser = null;
    }
  } finally {
    if (browser) await browser.close();
    await server.close();
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify({ parent, playwright, observations }, null, 2));
  }
  console.log(JSON.stringify({ parent, checks: observations.flatMap(o => o.checks).length, failures: observations.flatMap(o => o.checks).filter(c => !c.pass) }));
  if (!parent) assert.ok(observations.flatMap(o => o.checks).every(c => c.pass));
})().catch(error => { console.error(error.stack); process.exitCode = 1; });
