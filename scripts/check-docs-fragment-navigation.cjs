// Offline real-browser verification for #2067's shipped-documentation
// navigation. Mounts the production Docs page under the production router and
// serves the real frontend-modern/public/docs/API.md asset, then checks
// GitHub-compatible heading IDs, direct/reload fragment focus and scrolling,
// malformed-fragment safety and keyboard activation of intra-document links.
//
// Run from the assigned workspace root:
//   pulse-worker-browser scripts/check-docs-fragment-navigation.cjs
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright');

const ROOT = path.resolve(process.cwd(), 'frontend-modern');
const PORT = 5197;
const FIXTURE = '/browser-tests/docs-fragment-navigation.html';
const TARGET_ID = 'resource-maintenance-and-operator-state';
const launchOptions = { headless: true, channel: 'chromium', args: ['--no-sandbox'] };

const failures = [];
const observations = [];
const pageErrors = [];
const httpErrors = [];

function check(condition, message) {
  if (!condition) failures.push(message);
  observations.push(`${condition ? 'PASS' : 'FAIL'}: ${message}`);
}

(async () => {
  process.chdir(ROOT);
  const { createServer } = await import(
    path.join(ROOT, 'node_modules', 'vite', 'dist', 'node', 'index.js')
  );
  const server = await createServer({
    root: ROOT,
    configFile: path.join(ROOT, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: PORT, strictPort: true },
  });
  let browser;
  let result = { result: 'failed', failures, observations };
  try {
    await server.listen();
    browser = await chromium.launch(launchOptions);
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const page = await context.newPage();
    page.on('pageerror', (error) => pageErrors.push(error.message));
    page.on('console', (message) => {
      if (message.type() !== 'error') return;
      if (message.text().includes('Failed to load resource')) return;
      pageErrors.push(`console: ${message.text()}`);
    });
    page.on('response', (response) => {
      if (response.status() >= 400) httpErrors.push(`${response.status()} ${response.url()}`);
    });
    // The Docs fixture has no backend. Answer the app-runtime auth probe so it
    // settles instead of logging an uncaught network failure. Anchor to the
    // server root so `src/api/...` modules are not intercepted.
    const backendApi = new RegExp(`^http://127\\.0\\.0\\.1:${PORT}/api/`);
    await page.route(backendApi, (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '{}' }),
    );
    await page.route(new RegExp(`^http://127\\.0\\.0\\.1:${PORT}/api/security/status$`), (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ hasAuthentication: false }),
      }),
    );

    const url = (scenario) =>
      `http://127.0.0.1:${PORT}${FIXTURE}?scenario=${scenario}`;

    const waitForTargetFocus = async (timeout = 20000) => {
      await page.waitForSelector(`#${TARGET_ID}`, { timeout });
      await page.waitForFunction(
        (id) => document.activeElement && document.activeElement.id === id,
        TARGET_ID,
        { timeout },
      );
    };

    // --- plain: heading IDs, anchor resolution, layout ----------------------
    await page.goto(url('plain'));
    await page.waitForSelector('article h1, article h2', { timeout: 20000 });
    const plain = await page.evaluate(() => {
      const headings = Array.from(document.querySelectorAll('article h1, article h2, article h3, article h4, article h5, article h6'));
      const ids = headings.map((heading) => heading.id);
      const docLinks = Array.from(document.querySelectorAll('article a[href]'));
      return {
        count: headings.length,
        missingIds: ids.filter((id) => !id).length,
        uniqueIds: new Set(ids).size === ids.length,
        targetExists: ids.includes('resource-maintenance-and-operator-state'),
        fragmentLinks: docLinks
          .map((anchor) => anchor.getAttribute('href') || '')
          .filter((href) => href.includes('#')),
        overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      };
    });
    check(plain.count > 10, `renders the API reference headings (${plain.count})`);
    check(plain.missingIds === 0, 'every rendered heading carries a generated id');
    check(plain.uniqueIds, 'generated heading ids are unique within the document');
    check(plain.targetExists, `generated id "${TARGET_ID}" exists for the maintenance section`);
    check(
      plain.fragmentLinks.some((href) => href.endsWith(`#${TARGET_ID}`)),
      'the in-document maintenance fragment link resolves to the generated id',
    );
    check(!plain.overflow, 'desktop viewport has no horizontal page overflow');

    // --- direct: fragment focus after async fetch ---------------------------
    await page.goto(url('direct'));
    await waitForTargetFocus();
    const direct = await page.evaluate((id) => {
      const target = document.getElementById(id);
      const rect = target.getBoundingClientRect();
      return {
        active: document.activeElement ? document.activeElement.id : null,
        tabIndex: target.tabIndex,
        top: Math.round(rect.top),
        hash: window.location.hash,
      };
    }, TARGET_ID);
    check(direct.active === TARGET_ID, 'direct fragment link moves keyboard focus to the heading');
    check(direct.tabIndex === -1, 'fragment target receives programmatic focus without joining the tab order');
    check(direct.top >= -4 && direct.top <= 200, `direct fragment scrolled target into view (top ${direct.top})`);
    check(direct.hash === `#${TARGET_ID}`, 'direct fragment hash is preserved');

    // --- reload: same behaviour after a full reload -------------------------
    await page.reload();
    await waitForTargetFocus();
    const reloaded = await page.evaluate((id) => ({
      active: document.activeElement ? document.activeElement.id : null,
      exists: Boolean(document.getElementById(id)),
    }), TARGET_ID);
    check(reloaded.exists && reloaded.active === TARGET_ID, 'reload on a direct fragment restores focus and target');

    // --- malformed and missing fragments ------------------------------------
    for (const scenario of ['malformed', 'missing']) {
      await page.goto(url(scenario));
      await page.waitForSelector('article h1, article h2', { timeout: 20000 });
      await page.waitForTimeout(500);
      const state = await page.evaluate(() => ({
        activeTag: document.activeElement ? document.activeElement.tagName : null,
        activeIsHeading: Boolean(document.activeElement && /^H[1-6]$/.test(document.activeElement.tagName)),
        hasContent: Boolean(document.querySelector('article h1, article h2')),
      }));
      check(state.hasContent, `${scenario} fragment leaves the document rendered`);
      check(!state.activeIsHeading, `${scenario} fragment does not move focus onto a heading`);
    }

    // --- keyboard activation of an intra-document link ----------------------
    await page.goto(url('plain'));
    await page.waitForSelector('article h1, article h2', { timeout: 20000 });
    await page.evaluate(() => {
      window.__spaMarker = 'alive';
    });
    const backLink = page.locator('a[data-doc-link]').first();
    await backLink.focus();
    await backLink.press('Enter');
    await page.waitForFunction(
      () => window.location.pathname === '/docs/README',
      undefined,
      { timeout: 20000 },
    );
    await page.waitForFunction(
      () => document.body.innerText.includes('Pulse documentation'),
      undefined,
      { timeout: 20000 },
    );
    const keyboard = await page.evaluate(() => ({
      path: window.location.pathname,
      marker: window.__spaMarker,
      heading: Boolean(document.querySelector('article h1, article h2')),
    }));
    check(keyboard.path === '/docs/README', 'keyboard activation of a documentation link navigates the router');
    check(keyboard.marker === 'alive', 'keyboard navigation stays inside the SPA shell (no full reload)');
    check(keyboard.heading, 'the destination document renders after keyboard navigation');

    // --- narrow viewport ----------------------------------------------------
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(url('plain'));
    await page.waitForSelector('article h1, article h2', { timeout: 20000 });
    const narrow = await page.evaluate(() => ({
      overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      tableScrollers: Array.from(document.querySelectorAll('article [data-doc-table-scroll]')).length,
    }));
    check(!narrow.overflow, 'narrow viewport has no horizontal page overflow');
    check(narrow.tableScrollers > 0, 'wide API tables are wrapped in their own scroll container');

    check(pageErrors.length === 0, `no uncaught page errors (${pageErrors.join('; ') || 'none'})`);
    check(httpErrors.length === 0, `no failed HTTP responses (${httpErrors.join('; ') || 'none'})`);

    result = {
      result: failures.length === 0 ? 'passed' : 'failed',
      failures,
      observations,
      pageErrors,
      httpErrors,
      chromium: browser.version(),
    };
  } catch (error) {
    failures.push(`runner error: ${error.message}`);
    result = { result: 'failed', failures, observations, pageErrors, httpErrors };
  } finally {
    if (browser) await browser.close().catch(() => undefined);
    await server.close().catch(() => undefined);
  }

  const out = path.join(process.cwd(), 'docs-fragment-verification-result.json');
  fs.writeFileSync(out, JSON.stringify(result, null, 2));
  console.log(JSON.stringify(result, null, 2));
  process.exit(result.result === 'passed' ? 0 : 1);
})();
