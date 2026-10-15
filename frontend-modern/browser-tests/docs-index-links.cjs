// Real Docs page, router, renderer and styles; remote destinations are inert
// intercepted fixtures, never external reads or a claim about GitHub's pixels.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const root = '/workspace/frontend-modern';
const preview = '/workspace/tmp/doc-command-help/static';
const output = '/workspace/tmp/docs-index-proof/browser';
const digest = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const destinations = [
  ['Product screenshots on GitHub', 'https://github.com/rcourtman/Pulse/blob/main/docs/SCREENSHOTS.md'],
  ['Published release notes on GitHub', 'https://github.com/rcourtman/Pulse/releases'],
];
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const binding = JSON.parse(fs.readFileSync(path.join(preview, 'binding.json')));
  const baseline = fs.readFileSync('/workspace/tmp/docs-index-proof/parent-README.md');
  assert.equal(digest(baseline), 'd1a9c79ac11d6d7688c3ec62ab8028d1b4372bc7dc4ff939d4a2a82719cb7ca7');
  const result = { result: 'incomplete', source_sha: binding.source_sha,
    parent_sha: '94c345825934731059d9a65c3da405ace00d0970', parent_index_sha256: digest(baseline),
    playwright: require('playwright/package.json').version, cases: [], captures: [],
    external_reads: 0, errors: [], cleanup: {} };
  let browser, server;
  try {
    assert.equal(result.playwright, JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json'))
      .packages['node_modules/@playwright/test'].version);
    for (const [file, hash] of Object.entries(binding.sources))
      assert.equal(digest(fs.readFileSync(path.join(root, file))), hash, file);
    const assets = new Map();
    for (const [file, hash] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(preview, file));
      assert.equal(digest(bytes), hash, file); assets.set(file, bytes);
    }
    assert.equal(binding.artifacts['docs/README.md'], digest(fs.readFileSync('/workspace/docs/README.md')));
    // Missing Markdown gets the SPA HTML fallback, as in the real static handler.
    server = http.createServer((req, res) => {
      assert.equal(req.method, 'GET');
      const url = new URL(req.url, 'http://fixture.invalid');
      let file = url.pathname.slice(1);
      if (!assets.has(file)) file = 'browser-tests/doc-command-help.html';
      const type = ({ '.md': 'text/markdown', '.css': 'text/css', '.js': 'text/javascript', '.html': 'text/html' })[path.extname(file)];
      res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' });
      res.end(assets.get(file));
    });
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, width, dark] of [['desktop', 1360, false], ['phone', 390, true]]) {
      browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
      const context = await browser.newContext({ viewport: { width, height: 900 },
        isMobile: width < 500, hasTouch: width < 500, locale: 'en-GB' });
      let oldIndex = true;
      const record = { name, width, height: 900, dark, browser: browser.version(),
        baseline_dead_ends: [], intercepted_destinations: [], checks: [] }; result.cases.push(record);
      await context.route('**/*', route => {
        const url = new URL(route.request().url());
        assert.equal(route.request().method(), 'GET');
        if (destinations.some(([, target]) => target === url.href)) {
          record.intercepted_destinations.push(url.href);
          return route.fulfill({ status: 200, contentType: 'text/html', body:
            '<!doctype html><title>Offline destination fixture</title><p>Intercepted link only; no external read.</p>' });
        }
        assert.equal(url.origin, origin, 'unexpected external destination');
        assert(!url.pathname.startsWith('/api/'));
        if (oldIndex && url.pathname === '/docs/README.md')
          return route.fulfill({ status: 200, contentType: 'text/markdown', body: baseline });
        return route.continue();
      });
      const page = await context.newPage(); page.setDefaultTimeout(10000);
      page.on('pageerror', error => result.errors.push(error.message));
      page.on('console', message => { if (message.type() === 'error') result.errors.push(message.text().slice(0, 500)); });
      await page.addInitScript(dark => document.addEventListener('DOMContentLoaded', () =>
        document.documentElement.classList.toggle('dark', dark)), dark);
      const activate = async link => {
        await link.scrollIntoViewIfNeeded();
        if (width < 500) await link.tap(); else { await link.focus(); await page.keyboard.press('Enter'); }
      };
      const atFragment = id => page.waitForFunction(id => {
        const heading = document.getElementById(id);
        return heading && document.activeElement === heading && heading.getBoundingClientRect().top >= 0
          && heading.getBoundingClientRect().top < innerHeight;
      }, id);
      const capture = async suffix => {
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'page overflow');
        const file = `${name}-${suffix}.png`; await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: digest(fs.readFileSync(path.join(output, file))) });
      };
      for (const [label, target] of [['Current product screenshots', 'SCREENSHOTS'], ['Release notes index', 'RELEASE_NOTES']]) {
        await page.goto(`${origin}/docs/README`, { waitUntil: 'domcontentloaded' });
        await activate(page.getByRole('link', { name: label, exact: true }));
        await page.waitForURL(`**/docs/${target}`);
        await page.getByText("That document is not part of this installation's shipped documentation set.", { exact: true }).waitFor();
        record.baseline_dead_ends.push(target);
      }
      await capture('old-notes-dead-end');
      oldIndex = false;
      await page.goto(`${origin}/docs/README#monitoring-and-operations`, { waitUntil: 'domcontentloaded' });
      await atFragment('monitoring-and-operations');
      await page.evaluate(() => window.__indexProofShell = 'same-shell');
      for (const [position, [label, target]] of destinations.entries()) {
        const link = page.getByRole('link', { name: label, exact: true });
        assert.equal(await link.getAttribute('href'), target);
        assert.equal(await link.getAttribute('data-doc-link'), null);
        assert.equal(await link.getAttribute('target'), '_blank');
        const rel = (await link.getAttribute('rel')).split(' ');
        assert(rel.includes('noopener') && rel.includes('noreferrer'));
        await link.scrollIntoViewIfNeeded(); await capture(position === 0 ? 'screenshots-link' : 'published-notes-link');
        const popupPromise = context.waitForEvent('page');
        await activate(link); const popup = await popupPromise;
        await popup.waitForLoadState('domcontentloaded');
        assert.equal(popup.url(), target);
        assert.equal(await popup.title(), 'Offline destination fixture');
        assert.equal(await popup.evaluate(() => window.opener), null);
        await popup.close();
        assert.equal(await page.evaluate(() => window.__indexProofShell), 'same-shell');
      }
      assert((await page.locator('article').innerText()).includes('unpublished draft'));
      await activate(page.getByRole('link', { name: 'Releases and update channels', exact: true }));
      await page.waitForURL('**/docs/RELEASE_PROCESS');
      await page.getByRole('heading', { name: 'Releases and update channels', exact: true }).waitFor();
      assert.equal(await page.evaluate(() => window.__indexProofShell), 'same-shell');
      await page.goBack(); await page.getByRole('link', { name: destinations[1][0], exact: true }).waitFor();
      await page.reload({ waitUntil: 'domcontentloaded' });
      await page.getByRole('link', { name: destinations[1][0], exact: true }).waitFor();
      record.checks.push('both exact-parent dead ends reproduced', 'published notes and screenshots open isolated new tabs',
        'destination GETs intercepted with inert fixtures; no GitHub content fetched',
        'keyboard/touch, internal navigation, retained shell, Back/reload and no horizontal overflow');
      await context.close(); await browser.close(); browser = null;
    }
    assert.deepEqual(result.errors, []); result.result = 'passed';
  } catch (error) { result.failure = String(error.stack).slice(0, 3000); throw error; }
  finally {
    if (browser) await browser.close(); result.cleanup.browser_closed = true;
    if (server) await new Promise((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve()); server.closeAllConnections();
    }); result.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  }
})().catch(error => { console.error(String(error.stack).slice(0, 3000)); process.exitCode = 1; });
