// Current production navigation, real action/Patrol/Docs consumers and safe GET
// fixtures. No automatic-axe result is substituted for native pixel inspection.
process.env.RAYON_NUM_THREADS = '2';
process.env.UV_THREADPOOL_SIZE = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const { default: AxeBuilder } = require(
  path.join(root, 'tests/integration/node_modules/@axe-core/playwright'),
);
const binding = JSON.parse(
  fs.readFileSync(path.join(root, 'tmp/navigation-name-proof/binding.json')),
);
const output = path.join(root, 'tmp/navigation-name-proof', binding.phase);
const hash = (value) => crypto.createHash('sha256').update(value).digest('hex');
const WCAG_TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];
const report = {
  binding,
  node: process.version,
  playwright: require('playwright/package.json').version,
  axe: require(path.join(root, 'tests/integration/node_modules/axe-core/package.json')).version,
  cases: [],
  captures: [],
  unexpected: [],
  cleanup: {},
};
let browser,
  server,
  current,
  counts = { patrol: 4, actions: 6 };
const summary = () => ({
  activeCount: counts.patrol,
  openCount: counts.patrol,
  acknowledgedCount: 0,
  suppressedCount: 0,
  uncertainCount: 0,
  resolvedCount: 0,
  calm: counts.patrol === 0,
  coverageState: 'current',
  evaluatedAt: '2026-10-20T00:45:00Z',
});
function api(req, res, next) {
  const url = new URL(req.url, 'http://fixture.invalid');
  if (!url.pathname.startsWith('/api/')) return next();
  current.requests.push(`${req.method} ${url.pathname}`);
  if (req.method !== 'GET') {
    report.unexpected.push(`${req.method} ${url.pathname}`);
    res.writeHead(405);
    res.end('{}');
    return;
  }
  let value;
  if (url.pathname === '/api/ai/patrol/attention/summary') value = summary();
  else if (url.pathname === '/api/ai/patrol/attention') value = { data: [], summary: summary() };
  else if (url.pathname === '/api/actions/pending') value = { actions: [], count: counts.actions };
  else if (url.pathname === '/api/actions')
    value = {
      view: url.searchParams.get('view') || 'pending',
      actions: [],
      count: 0,
      readOnly: false,
    };
  else if (url.pathname === '/api/security/status')
    value = {
      hasAuthentication: true,
      requiresAuth: true,
      settingsCapabilities: { authenticationWrite: false },
    };
  else {
    res.writeHead(503, { 'Content-Type': 'application/json' });
    res.end('{"error":"Synthetic observation unavailable"}');
    return;
  }
  res.writeHead(200, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(value));
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
    for (const [file, digest] of Object.entries(binding.files))
      assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, file);
    assert.equal(
      report.playwright,
      JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    const { createServer } = await import(
      path.join(frontend, 'node_modules/vite/dist/node/index.js')
    );
    server = await createServer({
      root: frontend,
      configFile: path.join(frontend, 'vite.config.ts'),
      cacheDir: path.join(output, 'vite-cache'),
      optimizeDeps: { noDiscovery: true, include: [] },
      plugins: [
        {
          name: 'read-only-navigation-fixture',
          configureServer(vite) {
            vite.middlewares.use(api);
          },
        },
      ],
      server: { host: '127.0.0.1', port: 5262, strictPort: true },
    });
    await server.listen();
    const origin = 'http://127.0.0.1:5262';
    for (const spec of [
      { name: 'chromium-desktop', engine: chromium, width: 1440, height: 900, touch: false },
      { name: 'webkit-phone', engine: webkit, width: 390, height: 844, touch: true },
    ]) {
      current = {
        name: spec.name,
        viewport: { width: spec.width, height: spec.height },
        requests: [],
        scans: [],
        names: [],
        checks: [],
        errors: [],
      };
      report.cases.push(current);
      counts = { patrol: 4, actions: 6 };
      browser = await spec.engine.launch(
        spec.touch
          ? { headless: true }
          : { headless: true, channel: 'chromium', args: ['--no-sandbox'] },
      );
      current.browser_version = browser.version();
      const context = await browser.newContext({
        viewport: current.viewport,
        hasTouch: spec.touch,
        isMobile: spec.touch,
        reducedMotion: 'reduce',
        colorScheme: spec.touch ? 'dark' : 'light',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      page.on('pageerror', (error) => current.errors.push(error.message));
      await page.route('**/*', (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== origin || !['GET', 'HEAD'].includes(request.method())) {
          report.unexpected.push(`${request.method()} ${url.origin}${url.pathname}`);
          return route.abort();
        }
        return route.continue();
      });
      if (spec.touch)
        await page.addInitScript(() =>
          document.addEventListener('DOMContentLoaded', () =>
            document.documentElement.classList.add('dark'),
          ),
        );
      const capture = async (state) => {
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
          false,
        );
        const file = `${spec.name}-${state}.png`;
        await page.screenshot({
          path: path.join(output, file),
          animations: 'disabled',
          timeout: 30000,
        });
        report.captures.push({
          file,
          state,
          sha256: hash(fs.readFileSync(path.join(output, file))),
        });
      };
      const scan = async (state) => {
        const result = await new AxeBuilder({ page }).withTags(WCAG_TAGS).analyze();
        current.scans.push({
          state,
          violations: result.violations.map((v) => ({
            id: v.id,
            impact: v.impact,
            help: v.help,
            nodes: v.nodes.map((n) => ({
              target: n.target,
              html: n.html,
              failureSummary: n.failureSummary,
            })),
          })),
        });
      };
      await page.goto(`${origin}/browser-tests/navigation-visible-names.html`, {
        waitUntil: 'domcontentloaded',
        timeout: 60000,
      });
      await page.evaluate(() => {
        history.replaceState({}, '', '/actions');
        dispatchEvent(new PopStateEvent('popstate'));
      });
      await expect(page.getByRole('heading', { name: 'Actions', exact: true })).toBeVisible();
      await page.waitForFunction(() => !!window.__navigationProof);
      const nav = page.getByRole('navigation', {
        name: spec.touch ? 'Mobile navigation' : 'Primary navigation',
        exact: true,
      });
      await expect(
        nav.getByRole('link', { name: 'Actions: 6 actions await approval', exact: true }),
      ).toBeVisible();
      for (const link of await nav.getByRole('link').all())
        current.names.push({
          text: await link.innerText(),
          aria: await link.getAttribute('aria-label'),
          snapshot: await link.ariaSnapshot(),
        });
      await scan('actions-with-counts');
      await capture('actions-with-counts');
      if (binding.phase === 'after') {
        const alerts = (name) => nav.getByRole('link', { name, exact: true });
        await expect(alerts('Alerts: 2 critical, 3 warning')).toBeVisible();
        await expect(alerts('Alerts: 2 critical, 3 warning')).toHaveAttribute('href', '/alerts');
        await expect(
          nav.getByRole('link', { name: 'Actions: 6 actions await approval', exact: true }),
        ).toHaveAttribute('aria-current', 'page');
        current.checks.push(
          'The actual shell renders the two critical and three warning badges with the same complete accessible name on desktop and phone.',
        );
        const setAlerts = async (items, name) => {
          await page.evaluate((value) => window.__navigationProof.setAlerts(value), items);
          await expect(alerts(name)).toBeVisible();
          current.checks.push(name);
        };
        await setAlerts(
          [
            { id: 'live-warning', level: 'warning', acknowledged: false },
            { id: 'acknowledged-critical', level: 'critical', acknowledged: true },
          ],
          'Alerts: 1 warning',
        );
        await scan('actions-warning-only');
        await setAlerts(
          [{ id: 'live-critical', level: 'critical', acknowledged: false }],
          'Alerts: 1 critical',
        );
        await scan('actions-critical-only');
        await setAlerts(
          [{ id: 'acknowledged-critical', level: 'critical', acknowledged: true }],
          'Alerts',
        );
        await scan('actions-zero-alerts');
        await capture('actions-zero-alerts');
        counts = { patrol: 0, actions: 0 };
        await page.evaluate(() => window.__navigationProof.refresh());
        await expect(nav.getByRole('link', { name: 'Actions', exact: true })).toBeVisible();
        await expect(nav.getByRole('link', { name: 'Patrol', exact: true })).toBeVisible();
        current.checks.push(
          'Actual GET producers update the Patrol/Actions counts to zero without stale count names; acknowledged alerts are omitted from both rail summaries.',
        );
        const activate = async (link) => {
          if (spec.touch) await link.tap();
          else {
            await link.focus();
            await link.press('Enter');
          }
        };
        await activate(nav.getByRole('link', { name: 'Patrol', exact: true }));
        await expect(page.getByRole('heading', { name: 'Patrol', exact: true })).toBeVisible();
        await expect(nav.getByRole('link', { name: 'Patrol', exact: true })).toHaveAttribute(
          'aria-current',
          'page',
        );
        await scan('patrol');
        await capture('patrol');
        await activate(nav.getByRole('link', { name: 'Actions', exact: true }));
        await expect(page.getByRole('heading', { name: 'Actions', exact: true })).toBeVisible();
        await expect(nav.getByRole('link', { name: 'Actions', exact: true })).toHaveAttribute(
          'aria-current',
          'page',
        );
        const skipLink = page.getByRole('link', { name: 'Skip to main content', exact: true });
        await page.evaluate(() => {
          document.body.tabIndex = -1;
          document.body.focus();
          document.body.removeAttribute('tabindex');
        });
        await page.keyboard.press('Tab');
        await expect(skipLink).toBeFocused();
        await skipLink.press('Enter');
        await expect(page.locator('#main')).toBeFocused();
        current.checks.push(
          'Keyboard/touch navigation reaches the production Patrol and Actions pages with the correct current destination; the skip link still focuses main.',
        );
        for (const result of current.scans) assert.deepEqual(result.violations, []);
      }
      assert.deepEqual(current.errors, []);
      await browser.close();
      browser = undefined;
    }
    assert.deepEqual(report.unexpected, []);
  } catch (error) {
    report.error = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    report.cleanup.browser_closed = true;
    if (server) await server.close();
    report.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(
      JSON.stringify({
        ...report,
        binding: {
          source: binding.source,
          phase: binding.phase,
          sha256: hash(JSON.stringify(binding)),
        },
      }),
    );
  }
})();
