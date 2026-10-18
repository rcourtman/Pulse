// Shipped report guidance under the production Docs page/router/renderer/CSS.
// This is help acceptance, not a provider call, report generation or deployment.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules', 'report-narration-scope-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-report-narration-scope'),
    server: { host: '127.0.0.1', port: 5251, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  let browser;
  const results = [];
  try {
    await server.listen();
    for (const [engine, width, dark] of [['chromium', 1280, false], ['webkit', 390, true]]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const page = await browser.newPage({ viewport: { width, height: 900 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [];
      const backendRequests = [];
      const mutations = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      page.on('request', (request) => {
        if (!['GET', 'HEAD'].includes(request.method())) mutations.push(request.method());
      });
      await page.route('**/api/**', async (route) => {
        backendRequests.push({ method: route.request().method(), pathname: new URL(route.request().url()).pathname });
        await route.abort();
      });
      await page.goto('http://127.0.0.1:5251/browser-tests/docs-fragment-navigation.html', {
        waitUntil: 'domcontentloaded', timeout: 60000,
      });
      if (dark) await page.evaluate(() => document.documentElement.classList.add('dark'));
      for (const [guide, heading] of [['API', 'advanced-reporting-pro'], ['MSP', 'per-client-reports']]) {
        await page.evaluate((target) => {
          history.pushState({}, '', target);
          dispatchEvent(new PopStateEvent('popstate'));
        }, `/docs/${guide}#${heading}`);
        await page.waitForFunction((id) => {
          const target = document.getElementById(id);
          const rect = target?.getBoundingClientRect();
          return target && document.activeElement === target && rect.top >= 0 && rect.top < innerHeight;
        }, heading);
        const section = await page.evaluate((id) => {
          const target = document.getElementById(id);
          const paragraphs = [];
          for (let node = target.nextElementSibling; node && !/^H[1-3]$/.test(node.tagName); node = node.nextElementSibling) {
            paragraphs.push(node.textContent);
          }
          return paragraphs.join(' ').replace(/\s+/g, ' ');
        }, heading);
        for (const phrase of [
          'Report generation uses settings:read.',
          'single-resource and fleet PDF narration, an API token also needs ai:execute (or full access).',
          'the report still succeeds with a deterministic summary and no AI provider call',
          'a single-resource PDF also explains the missing scope.',
          'Browser sessions and server-run schedules keep their existing narration behaviour.',
          'AI budgets still apply. GET and CSV never use AI narration',
        ]) assert.ok(section.includes(phrase), `${guide}: ${phrase}`);
        assert.ok(section.includes('POST /api/admin/reports/generate'));
        assert.ok(section.includes('POST /api/admin/reports/generate-multi'));
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        await page.waitForFunction(() => document.getAnimations().length === 0);
        await page.screenshot({ path: path.join(artifacts, `${engine}-${guide}.png`) });
        results.push({ guide, engine, browserVersion: browser.version(), width, dark,
          route: new URL(page.url()).pathname + new URL(page.url()).hash,
          focusedVisibleHeading: true, narrationPermissionAndFallbackVisible: true,
          unchangedSessionScheduleBudgetAndGETCSVGuidance: true, noHorizontalPageOverflow: true });
      }
      assert.deepEqual(backendRequests, []);
      assert.deepEqual(mutations, []);
      assert.deepEqual(errors, []);
      results.push({ engine, backendRequests, mutations, errors });
      await browser.close();
      browser = undefined;
    }
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
  const documents = Object.fromEntries(['API', 'MSP'].map((guide) => [guide,
    crypto.createHash('sha256').update(fs.readFileSync(path.join(root, 'public/docs', `${guide}.md`))).digest('hex')]));
  const result = { result: 'passed', playwright: require('playwright/package.json').version,
    documents, results, browserAndServerClosed: true,
    scope: 'production documentation renderer and shipped assets; not report/provider/native/release acceptance' };
  fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  console.log(JSON.stringify(result));
})().catch((error) => { console.error(error); process.exitCode = 1; });
