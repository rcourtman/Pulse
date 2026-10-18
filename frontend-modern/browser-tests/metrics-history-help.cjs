// Exercise only existing help: real Docs fetch, renderer, routes and CSS.
// No monitored system, diagnostics, licence change or History collector runs.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const sha256 = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'node_modules', 'metrics-history-help-browser');
  const sourceDocs = Object.fromEntries(
    ['METRICS_HISTORY', 'TROUBLESHOOTING', 'PULSE_PRO', 'VM_DISK_MONITORING'].map((name) => {
      const bytes = fs.readFileSync(path.join(root, `public/docs/${name}.md`));
      assert.deepEqual(bytes, fs.readFileSync(`/workspace/docs/${name}.md`));
      return [name, { bytes: bytes.length, sha256: sha256(bytes) }];
    }),
  );
  process.chdir(root);
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-metrics-history-help'),
    server: { host: '127.0.0.1', port: 5257, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [engine, width, dark] of [['chromium', 1280, false], ['webkit', 390, true]]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const page = await browser.newPage({ viewport: { width, height: 900 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [], backendRequests = [], mutations = [], requests = [], captures = [];
      await page.addInitScript(() => {
        const fetch = window.fetch.bind(window);
        window.__historyHelpDocuments = [];
        window.fetch = async (...args) => {
          const response = await fetch(...args);
          const pathname = new URL(response.url).pathname;
          if (/^\/docs\/.+\.md$/.test(pathname)) {
            const bytes = Array.from(new Uint8Array(await response.clone().arrayBuffer()));
            window.__historyHelpDocuments.push({ pathname, status: response.status, bytes });
          }
          return response;
        };
      });
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      page.on('request', (request) => {
        requests.push(new URL(request.url()).pathname);
        if (!['GET', 'HEAD'].includes(request.method())) mutations.push(request.method());
      });
      await page.route('**/api/**', async (route) => {
        backendRequests.push(new URL(route.request().url()).pathname);
        await route.abort();
      });
      const navigate = async (route, fragment) => {
        await page.evaluate((href) => {
          history.pushState({}, '', href);
          dispatchEvent(new PopStateEvent('popstate'));
        }, route);
        await focused(fragment);
      };
      const focused = async (id) => {
        await page.waitForFunction((fragment) => {
          const target = document.getElementById(fragment);
          const rect = target?.getBoundingClientRect();
          return target && document.activeElement === target && rect.top >= 0 && rect.top < innerHeight;
        }, id);
      };
      const activate = async (name, id) => {
        const link = page.getByRole('link', { name, exact: true });
        await link.focus();
        await page.keyboard.press('Enter');
        await focused(id);
      };
      const capture = async (name) => {
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        await page.waitForFunction(() => document.getAnimations().length === 0);
        const filename = `${engine}-${name}.png`;
        const filepath = path.join(artifacts, filename);
        await page.screenshot({ path: filepath });
        captures.push({ filename, bytes: fs.statSync(filepath).size, sha256: sha256(fs.readFileSync(filepath)) });
      };
      await page.goto('http://127.0.0.1:5257/browser-tests/docs-fragment-navigation.html', {
        waitUntil: 'domcontentloaded', timeout: 60000,
      });
      if (dark) await page.evaluate(() => document.documentElement.classList.add('dark'));
      await navigate('/docs/TROUBLESHOOTING#guest-memory-is-high-but-available-memory-is-plentiful',
        'guest-memory-is-high-but-available-memory-is-plentiful');
      assert.ok((await page.locator('article').innerText()).includes("today's live source does not date a past reading"));
      await capture('entry');
      await activate('History panel interpretation', 'read-a-history-panel');
      const section = await page.evaluate(() => {
        const nodes = [];
        for (let node = document.getElementById('read-a-history-panel').nextElementSibling;
          node && node.tagName !== 'H2'; node = node.nextElementSibling) nodes.push(node.textContent);
        return nodes.join(' ').replace(/\s+/g, ' ');
      });
      for (const phrase of [
        'Latest stored', 'Single observation. No trend yet.', 'Missing is not measured zero',
        'CPU 22% at 12:02 and memory 95% at 12:01 are not one observation',
        'arrow keys, Home or End, or move it by touch',
        "Today's source cannot identify the source of a past warning",
        'do not run Diagnostics or guest-agent probes', 'do not share raw responses, credentials',
      ]) assert.ok(section.includes(phrase), `${engine}: ${phrase}`);
      await capture('observations');
      await page.getByText('For example, CPU', { exact: false }).scrollIntoViewIfNeeded();
      await capture('comparison');
      await navigate('/docs/METRICS_HISTORY#retention-and-accessible-range', 'retention-and-accessible-range');
      const rendered = await page.locator('article').innerText();
      for (const phrase of [
        'Community supports up to 7 days', 'Relay is no longer sold',
        '14-day compatibility limit', 'not a plan to buy', '402 Payment Required',
        'do not buy another licence, reset history or reinstall',
      ]) assert.ok(rendered.replace(/\s+/g, ' ').includes(phrase), `${engine}: ${phrase}`);
      assert.ok(!rendered.includes('Relay unlocks'));
      await capture('range');
      await activate('plans and legacy subscriptions', 'self-hosted-commercial-model');
      assert.ok((await page.locator('article').innerText()).includes('not the entitlement for continuing Relay subscribers'));
      await navigate('/docs/METRICS_HISTORY#read-a-history-panel', 'read-a-history-panel');
      await activate('Backup safety', 'backup-safety');
      assert.ok((await page.locator('article').innerText()).includes('every filesystem covered by the backup'));
      const documents = await page.evaluate(() => window.__historyHelpDocuments);
      const consumed = documents.filter((doc) => sourceDocs[doc.pathname.split('/').at(-1).replace('.md', '')])
        .map((doc) => ({ pathname: doc.pathname, status: doc.status,
          bytes: doc.bytes.length, sha256: sha256(Buffer.from(doc.bytes)) }));
      for (const [name, identity] of Object.entries(sourceDocs)) {
        const responses = consumed.filter((doc) => doc.pathname === `/docs/${name}.md`);
        assert.ok(responses.length >= 1, `${engine}: missing ${name} consumer`);
        for (const response of responses) {
          assert.equal(response.status, 200);
          assert.equal(response.bytes, identity.bytes);
          assert.equal(response.sha256, identity.sha256);
        }
      }
      assert.deepEqual(backendRequests, []);
      assert.deepEqual(mutations, []);
      assert.deepEqual(errors, []);
      results.push({ engine, browserVersion: browser.version(), width, dark, consumed, captures,
        memoryEntryKeyboardNavigation: true, plansKeyboardNavigation: true, safetyKeyboardNavigation: true,
        directRangeFragmentFocus: true, noHorizontalPageOverflow: true, backendRequests, mutations, errors,
        requestedSourcePaths: [...new Set(requests.filter((url) => url.startsWith('/src/')))].sort() });
      await browser.close();
      browser = undefined;
    }
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
  const result = { result: 'passed', playwright: require('playwright/package.json').version,
    sourceDocs, results, browserAndServerClosed: true,
    scope: 'Production Docs consumer and exact shipped help bytes; no native guest, collector, licence migration, full application or release acceptance.' };
  fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  console.log(JSON.stringify(result));
})().catch((error) => { console.error(error); process.exitCode = 1; });
