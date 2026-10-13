// Production Docs renderer/router/CSS and exact shipped Markdown, no PBS call.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');

const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const evidence = path.join(root, 'tmp/pbs-missing-help');
const preview = path.join(root, 'tmp/doc-command-help/static');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const fragment = 'pbs-is-active-but-missing-from-monitoring';
const result = { source: binding.source_sha, binding_sha256: hash(bindingBytes),
  cases: [], captures: [], errors: [], backendRequests: [], mutations: [], served: [] };

(async () => {
  fs.mkdirSync(evidence, { recursive: true });
  let server, browser;
  try {
    for (const [file, expected] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(frontend, file))), expected, file);
    for (const [file, expected] of Object.entries(binding.artifacts))
      assert.equal(hash(fs.readFileSync(path.join(preview, file))), expected, file);
    for (const name of ['PBS', 'TROUBLESHOOTING'])
      assert.deepEqual(fs.readFileSync(path.join(preview, 'docs', `${name}.md`)),
        fs.readFileSync(path.join(root, 'docs', `${name}.md`)));
    result.source_inputs = Object.keys(binding.sources).length;
    server = http.createServer((request, response) => {
      const route = new URL(request.url, 'http://fixture.invalid').pathname;
      let asset = route.slice(1);
      if (route.startsWith('/docs/') && !route.endsWith('.md'))
        asset = 'browser-tests/doc-command-help.html';
      if (!Object.hasOwn(binding.artifacts, asset)) {
        response.writeHead(404); response.end(); return;
      }
      const bytes = fs.readFileSync(path.join(preview, asset));
      assert.equal(hash(bytes), binding.artifacts[asset]);
      result.served.push({ route, asset, sha256: hash(bytes) });
      const type = asset.endsWith('.md') ? 'text/markdown; charset=utf-8' :
        asset.endsWith('.js') ? 'text/javascript' : asset.endsWith('.css') ? 'text/css' : 'text/html';
      response.writeHead(200, { 'Content-Type': type, 'Cache-Control': 'no-store' });
      response.end(bytes);
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const state of [
      { engine: 'chromium', name: 'desktop-light', width: 1280, height: 900, dark: false },
      { engine: 'webkit', name: 'phone-dark', width: 390, height: 900, dark: true },
    ]) {
      browser = await { chromium, webkit }[state.engine].launch(state.engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width: state.width, height: state.height },
        isMobile: state.width < 500, hasTouch: state.width < 500 });
      const page = await context.newPage();
      page.on('pageerror', error => result.errors.push(error.message));
      page.on('request', request => {
        if (!['GET', 'HEAD'].includes(request.method())) result.mutations.push(request.method());
      });
      await page.route('**/api/**', async route => {
        result.backendRequests.push(new URL(route.request().url()).pathname); await route.abort();
      });
      await page.addInitScript(dark => {
        document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.toggle('dark', dark));
        const fetch = window.fetch.bind(window);
        window.__helpDocuments = [];
        window.fetch = async (...args) => {
          const response = await fetch(...args);
          if (/\/docs\/(PBS|TROUBLESHOOTING)\.md$/.test(new URL(response.url).pathname))
            window.__helpDocuments.push({ path: new URL(response.url).pathname, status: response.status,
              bytes: Array.from(new Uint8Array(await response.clone().arrayBuffer())) });
          return response;
        };
      }, state.dark);
      const activate = async link => {
        if (state.width < 500) await link.tap();
        else { await link.focus(); await page.keyboard.press('Enter'); }
      };
      const capture = async (label, locator) => {
        await locator.evaluate(element => window.scrollBy(0, element.getBoundingClientRect().top - 64));
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${state.name}-${label}.png`;
        await page.screenshot({ path: path.join(evidence, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(evidence, file))), state });
      };
      await page.goto(`${origin}/docs/TROUBLESHOOTING#choose-the-affected-workflow`);
      const entry = page.locator(`article a[href="/docs/PBS#${fragment}"]`);
      await entry.waitFor();
      await capture('entry', entry.locator('..'));
      await activate(entry);
      await page.waitForURL(`**/docs/PBS#${fragment}`);
      const heading = page.locator(`#${fragment}`);
      await page.waitForFunction(id => document.activeElement?.id === id, fragment);
      await capture('missing-server', heading);
      const section = await heading.evaluate(element => {
        const text = [];
        for (let next = element.nextElementSibling; next && !/^H[1-3]$/.test(next.tagName); next = next.nextElementSibling)
          text.push(next.textContent);
        return text.join(' ').replace(/\s+/g, ' ');
      });
      for (const phrase of ["connection's polling health", 'does not require a Pulse Agent',
        'The PBS VM/LXC in Overview is a different resource', 'If Proxmox navigation is absent too',
        'PVE passthrough backups do not establish', 'unknown is valid',
        'Do not delete and re-add the connection, replace tokens, install an agent',
        'Keep certificate verification and existing access restrictions intact'])
        assert.ok(section.includes(phrase), `${state.name}: ${phrase}`);
      const safety = page.locator('article p').filter({ hasText: 'Do not delete and re-add the connection, replace tokens, install an agent' });
      await capture('preserve-state', safety);
      await activate(page.getByRole('link', { name: 'PBS History checks', exact: true }));
      await page.waitForURL('**#pbs-is-connected-but-history-stays-empty');
      await page.waitForFunction(() => document.activeElement?.id === 'pbs-is-connected-but-history-stays-empty');
      const consumed = await page.evaluate(() => window.__helpDocuments);
      assert.ok(consumed.some(item => item.path === '/docs/PBS.md'));
      assert.ok(consumed.some(item => item.path === '/docs/TROUBLESHOOTING.md'));
      for (const item of consumed) {
        assert.equal(item.status, 200);
        assert.equal(hash(Buffer.from(item.bytes)), binding.artifacts[item.path.slice(1)]);
      }
      result.cases.push({ ...state, browser: browser.version(), focusedFragment: true,
        historyLink: true, noHorizontalPageOverflow: true,
        consumed: consumed.map(item => ({ path: item.path, sha256: hash(Buffer.from(item.bytes)) })) });
      await browser.close(); browser = undefined;
    }
    assert.deepEqual(result.errors, []);
    assert.deepEqual(result.backendRequests, []);
    assert.deepEqual(result.mutations, []);
    result.playwright = require('playwright/package.json').version;
    assert.equal(result.playwright, '1.56.1');
    result.result = 'passed';
  } finally {
    if (browser) await browser.close();
    if (server) await new Promise(resolve => server.close(resolve));
    result.browserAndServerClosed = true;
    fs.writeFileSync(path.join(evidence, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  }
  console.log(JSON.stringify(result));
})().catch(error => { console.error(error); process.exitCode = 1; });
