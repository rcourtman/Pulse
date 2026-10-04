// #2257: the real Patrol page must retain its setup task and independent
// attention inbox when Patrol is off. Only API responses are synthetic.
const assert = require('node:assert/strict');
const path = require('node:path');
const { chromium } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5207, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    const seen = [];
    const item = {
      id: 'record-1', operationalRecordId: 'record-1', subjectResourceId: 'pve:vm:101',
      subjectResourceName: 'Database VM', subjectResourceType: 'vm', kind: 'disk',
      title: 'Disk pressure on Database VM',
      plainLanguageSummary: 'The database disk is nearly full.', severity: 'critical',
      state: 'open', firstObservedAt: '2026-09-26T18:00:00Z',
      lastObservedAt: '2026-09-26T18:00:00Z', evidenceFreshness: 'fresh',
      evidenceCompleteness: 'complete', impact: 'Writes may fail.',
      relatedResources: [], recommendedNextStep: 'Free disk space or expand the volume.',
      availableActions: [], verificationState: 'not_available',
    };
    const summary = { activeCount: 1, openCount: 1, acknowledgedCount: 0,
      suppressedCount: 0, uncertainCount: 0, resolvedCount: 0,
      calm: false, coverageState: 'current', evaluatedAt: '2026-09-26T18:00:00Z' };
    for (const width of [1440, 390]) {
      const pageErrors = [];
      const page = await browser.newPage({ viewport: { width, height: width === 390 ? 844 : 900 } });
      page.on('pageerror', (error) => pageErrors.push(error.message));
      await page.routeWebSocket(/\/ws(?:\?|$)/, () => {});
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5207') return route.abort();
        const p = url.pathname;
        if (!p.startsWith('/api/')) return route.continue();
        seen.push(p);
        let json = {};
        if (p === '/api/settings/ai') json = { patrol_enabled: false,
          patrol_readiness: { status: 'not_ready', ready: false,
            summary: 'The selected Patrol model cannot run tools.', cause: 'model_tool_check' } };
        else if (p === '/api/ai/patrol/status') json = { enabled: false,
          runtime_state: 'disabled', running: false, readiness: { status: 'not_ready', ready: false,
            summary: 'The selected Patrol model cannot run tools.', cause: 'model_tool_check' },
          summary: { active: 0, critical: 0, warning: 0, info: 0 } };
        else if (p === '/api/ai/patrol/attention') json = { data: [item], summary,
          meta: { page: 1, limit: 50, total: 1, totalPages: 1 } };
        else if (p === '/api/ai/patrol/attention/summary') json = summary;
        else if (p === '/api/ai/patrol/attention/record-1') json = {
          item, operationalRecord: { id: item.id, kind: item.kind, state: item.state,
            severity: item.severity, title: item.title, subjectResourceId: item.subjectResourceId,
            subjectResourceName: item.subjectResourceName, createdAt: item.firstObservedAt,
            updatedAt: item.lastObservedAt }, timeline: [], evidence: [] };
        else if (p === '/api/ai/patrol/autonomy') json = {
          autonomy_level: 'monitor', full_mode_unlocked: false };
        else if (p === '/api/ai/patrol/findings' || p === '/api/ai/patrol/runs') json = [];
        else if (p === '/api/ai/unified/findings') json = { findings: [] };
        else if (p === '/api/ai/approvals') json = { approvals: [] };
        else if (p === '/api/ai/models') json = { models: [] };
        else if (p === '/api/license/runtime-capabilities') json = { capabilities: ['ai_patrol'],
          limits: [], hosted_mode: false, max_history_days: 7,
          runtime: { build: 'community', label: 'Pulse Community runtime' }, blocked_capabilities: [] };
        return route.fulfill({ json });
      });
      await page.goto('http://127.0.0.1:5207/browser-tests/patrol-off-attention-2257.html', {
        waitUntil: 'domcontentloaded', timeout: 120_000,
      });
      await page.getByRole('heading', { name: 'Patrol needs setup' }).waitFor();
      await page.getByRole('region', { name: 'Patrol decision inbox' }).waitFor();
      await page.getByText('Database VM · Disk pressure').waitFor();
      assert.equal(await page.getByRole('button', { name: 'Toggle Patrol' }).getAttribute('aria-pressed'), 'false');
      await page.waitForTimeout(350);
      await page.screenshot({ path: path.join(root, 'browser-tests', `patrol-off-attention-2257-list-${width}.png`),
        fullPage: true });
      await page.getByRole('button', { name: 'Start review' }).click();
      await page.getByRole('heading', { name: 'Database VM · Disk pressure' }).waitFor();
      await page.getByText('Writes may fail.').waitFor();
      await page.screenshot({ path: path.join(root, 'browser-tests', `patrol-off-attention-2257-${width}.png`),
        fullPage: true });
      const dimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth,
        inner: window.innerWidth }));
      assert.ok(dimensions.scroll <= dimensions.inner + 1,
        `${width}px horizontal overflow: ${JSON.stringify(dimensions)}`);
      assert.deepEqual(pageErrors, [], `browser errors at ${width}px`);
      await page.getByRole('button', {
        name: width === 390 ? 'Back to attention list' : 'Close attention detail',
      }).click();
      await page.getByRole('button', { name: 'Start review' }).waitFor();
      await page.goto('http://127.0.0.1:5207/browser-tests/patrol-off-attention-2257.html', {
        waitUntil: 'domcontentloaded',
      });
      await page.getByRole('heading', { name: 'Patrol needs setup' }).waitFor();
      await page.getByText('Database VM · Disk pressure').waitFor();
      await page.close();
    }
    assert.ok(seen.includes('/api/ai/patrol/status'));
    assert.ok(seen.includes('/api/ai/patrol/attention'));
    console.log(JSON.stringify({ result: 'passed', browser: browser.version(), viewports: [1440, 390] }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
