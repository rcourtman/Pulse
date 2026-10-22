// Read the real shipped guides in the existing Docs/router consumer. This is
// browser guidance proof, not a Docker install, public TLS or SSO lifecycle.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const root = process.cwd();
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/bootstrap-transport');
const digest = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const result = { result: 'failed', checks: [], captures: [], errors: [], requests: [], sources: {} };
const check = (ok, name) => { result.checks.push({ name, passed: ok }); assert.ok(ok, name); };
const guides = ['INSTALL', 'DOCKER', 'i18n/de/README', 'i18n/es/README'];
const anchor = 'open-a-protected-setup-connection';
(async () => {
  fs.mkdirSync(output, { recursive: true });
  result.source = JSON.parse(fs.readFileSync(path.join(output, 'source.json'), 'utf8'));
  for (const name of guides) {
    const file = `frontend-modern/public/docs/${name}.md`;
    result.sources[file] = digest(fs.readFileSync(path.join(root, file)));
  }
  for (const file of ['frontend-modern/package.json', 'frontend-modern/package-lock.json',
    'frontend-modern/vite.config.ts', 'frontend-modern/src/pages/Docs.tsx',
    'frontend-modern/src/features/docs/docMarkdown.ts', 'frontend-modern/src/index.css',
    'frontend-modern/browser-tests/docs-fragment-navigation.tsx', 'scripts/check-bootstrap-transport.cjs'])
    result.sources[file] = digest(fs.readFileSync(path.join(root, file)));
  const { createServer } = await import(path.join(frontend, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({ root: frontend, configFile: path.join(frontend, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5199, strictPort: true } });
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    result.playwright = require('playwright/package.json').version;
    result.chromium = browser.version();
    assert.equal(result.playwright, '1.56.1');
    for (const [name, width] of [['desktop', 1440], ['phone', 320]]) {
      const context = await browser.newContext({ viewport: { width, height: 900 }, isMobile: width < 500, hasTouch: width < 500 });
      const page = await context.newPage();
      page.on('pageerror', error => result.errors.push(error.message));
      page.on('response', response => { if (response.status() >= 400) result.errors.push(`${response.status()} ${response.url()}`); });
      await page.route('**/*', route => {
        const request = route.request(); const url = new URL(request.url());
        result.requests.push({ method: request.method(), pathname: url.pathname });
        if (url.origin !== 'http://127.0.0.1:5199') { result.errors.push('off-origin request'); return route.abort(); }
        if (url.pathname.startsWith('/api/')) return route.fulfill({ status: 200, contentType: 'application/json', body: '{"hasAuthentication":false}' });
        return route.fallback();
      });
      const move = async locator => {
        await locator.evaluate(el => window.scrollBy(0, el.getBoundingClientRect().top - 64));
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      };
      const capture = async (label, locator) => {
        await move(locator);
        check(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `${name}/${label}: no page overflow`);
        const file = `${name}-${label}.png`;
        await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: digest(fs.readFileSync(path.join(output, file))), width });
      };
      const activate = async locator => {
        if (width < 500) await locator.tap(); else { await locator.focus(); await page.keyboard.press('Enter'); }
      };
      await page.goto('http://127.0.0.1:5199/browser-tests/docs-fragment-navigation.html?scenario=installation');
      await page.locator(`article #${anchor}`).waitFor();
      check((await page.locator('article').innerText()).replace(/\s+/g, ' ').includes('The token controls setup access; it does not encrypt the connection'), `${name}: token versus transport distinction`);
      await capture('protected-setup', page.locator(`#${anchor}`));
      const tunnel = page.locator('article pre').filter({ hasText: 'ssh -N -T' });
      check((await tunnel.innerText()).includes('-L 127.0.0.1:17655:127.0.0.1:7655'), `${name}: loopback tunnel copied intact`);
      check((await tunnel.innerText()).includes('StrictHostKeyChecking=yes'), `${name}: SSH key verification visible`);
      await capture('setup-tunnel', tunnel);
      check(await tunnel.evaluate(el => el.scrollWidth <= el.clientWidth || getComputedStyle(el).overflowX === 'auto'), `${name}: long command stays in its scroll area`);
      const reverse = page.locator(`#${anchor} + p + ul a`).filter({ hasText: 'reverse-proxy guide' });
      await activate(reverse);
      await page.waitForURL('**/docs/REVERSE_PROXY#before-configuring-the-proxy');
      await page.waitForFunction(() => document.activeElement?.id === 'before-configuring-the-proxy');
      check(true, `${name}: HTTPS configuration target reached and focused`);
      for (const [doc, phrase, slug] of [
        ['DOCKER', 'This new-install example binds the published port', 'docker-start'],
        ['i18n/de/README', 'Dieses Beispiel für Neuinstallationen', 'german-start'],
        ['i18n/es/README', 'Este ejemplo para instalaciones nuevas', 'spanish-start'],
      ]) {
        // Reach each guide through actual index links rather than injecting
        // a history event. The reader and fragment focus remain unmodified.
        await activate(page.getByRole('link', { name: 'All documentation', exact: false }).first());
        await page.waitForURL('**/docs/README');
        await activate(page.locator(`article a[href="/docs/${doc}"]`).first());
        await page.waitForURL(`**/docs/${doc}`);
        const note = page.locator('article p').filter({ hasText: phrase }).first();
        await note.waitFor();
        const code = page.locator('article pre').filter({ hasText: 'docker run -d' }).first();
        const box = await code.boundingBox();
        result.codePanels ??= [];
        result.codePanels.push({ doc, name, box, styles: await code.evaluate(el => ({ display: getComputedStyle(el).display, visibility: getComputedStyle(el).visibility, parent: el.parentElement.tagName, html: el.outerHTML.slice(0, 600) })) });
        check(box && box.width > 0 && box.height > 0, `${name}/${slug}: code panel has visible geometry`);
        check((await code.innerText()).includes('-p 127.0.0.1:7655:7655'), `${name}/${slug}: loopback Docker publication`);
        await capture(slug, code);
        await activate(page.locator(`article a[href="/docs/INSTALL#${anchor}"]`).first());
        await page.waitForURL(`**/docs/INSTALL#${anchor}`);
        await page.waitForFunction(id => document.activeElement?.id === id, anchor);
        check(true, `${name}/${slug}: protected setup link lands with focus`);
      }
      for (const doc of guides) {
        const response = await page.request.get(`http://127.0.0.1:5199/docs/${doc}.md`);
        check(response.status() === 200 && digest(await response.body()) === result.sources[`frontend-modern/public/docs/${doc}.md`], `${name}/${doc}: served bytes match final source`);
      }
      await context.close();
    }
    check(result.errors.length === 0, 'no browser, response or off-origin errors');
    check(result.requests.every(r => r.method === 'GET'), 'guide reading makes no mutation request');
    result.result = 'passed';
  } catch (error) {
    result.error = String(error.stack || error); process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    await server.close();
    result.cleanup = { browserClosed: !browser?.isConnected(), serverClosed: !server.httpServer?.listening };
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify({ result: result.result, checks: result.checks.length, captures: result.captures.length, cleanup: result.cleanup, error: result.error }));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
