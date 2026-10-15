// Exercise the shipped PMG symptom entry through the real Docs page/router/CSS.
// This fixture performs no API, credential, gateway or mail operation.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const preview = path.join(root, 'tmp/doc-command-help/static');
const output = path.join(root, 'tmp/pmg-help-entry');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const result = { source_sha: binding.source_sha, binding_sha256: hash(bindingBytes),
  result: 'incomplete', cases: [], captures: [], errors: [], failedResponses: [],
  failedRequests: [], unexpectedRequests: [], served: [] };
const checkpoint = () => fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');

(async () => {
  fs.mkdirSync(output, { recursive: true });
  let server, browser;
  try {
    for (const [file, expected] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(frontend, file))), expected, file);
    for (const [file, expected] of Object.entries(binding.artifacts))
      assert.equal(hash(fs.readFileSync(path.join(preview, file))), expected, file);
    result.verified_source_inputs = Object.keys(binding.sources).length;
    result.verified_artifacts = Object.keys(binding.artifacts).length;
    server = http.createServer((req, res) => {
      const route = new URL(req.url, 'http://fixture.invalid').pathname;
      const asset = route.startsWith('/docs/') && !route.endsWith('.md')
        ? 'browser-tests/doc-command-help.html' : route.slice(1);
      if (req.method !== 'GET' || !Object.hasOwn(binding.artifacts, asset)) {
        result.unexpectedRequests.push({ method: req.method, route });
        res.writeHead(404); res.end(); return;
      }
      const bytes = fs.readFileSync(path.join(preview, asset));
      assert.equal(hash(bytes), binding.artifacts[asset]);
      result.served.push({ route, asset, sha256: hash(bytes) });
      res.writeHead(200, { 'Content-Type': asset.endsWith('.md') ? 'text/markdown' :
        asset.endsWith('.css') ? 'text/css' : asset.endsWith('.js') ? 'text/javascript' : 'text/html' });
      res.end(bytes);
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const origin = `http://127.0.0.1:${server.address().port}`;
    result.playwright = require('playwright/package.json').version;
    assert.equal(result.playwright, '1.56.1');
    for (const state of [
      { name: 'desktop-light', engine: 'chromium', width: 1360, height: 960, dark: false },
      { name: 'phone-dark', engine: 'webkit', width: 320, height: 900, dark: true },
    ]) {
      browser = await { chromium, webkit }[state.engine].launch(state.engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width: state.width, height: state.height },
        isMobile: state.width < 500, hasTouch: state.width < 500 });
      const page = await context.newPage();
      page.on('pageerror', error => result.errors.push(error.message));
      page.on('console', message => { if (message.type() === 'error') result.errors.push(message.text()); });
      page.on('response', response => {
        if (response.status() >= 400) result.failedResponses.push({ url: response.url(), status: response.status() });
      });
      page.on('requestfailed', request => result.failedRequests.push({ url: request.url(), failure: request.failure() }));
      const caseResult = { ...state, browser: browser.version() };
      result.cases.push(caseResult);
      await page.addInitScript(dark => document.addEventListener('DOMContentLoaded', () =>
        document.documentElement.classList.toggle('dark', dark)), state.dark);
      const jump = locator => locator.evaluate(el => window.scrollBy(0, el.getBoundingClientRect().top - 48));
      const activate = async locator => {
        if (state.width < 500) await locator.tap();
        else { await locator.focus(); await page.keyboard.press('Enter'); }
      };
      const capture = async name => {
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), name);
        const file = `${state.name}-${name}.png`;
        await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))), state });
        checkpoint();
      };
      await page.goto(`${origin}/docs/TROUBLESHOOTING#choose-the-affected-workflow`);
      const row = page.locator('article li').filter({ hasText: 'Mail Gateway Test succeeds but collection fails:' });
      assert.equal(await row.count(), 1);
      assert.equal(await row.getByRole('link').count(), 2);
      assert.ok((await row.innerText()).includes('Missing queue data is not an empty queue'));
      await jump(row);
      await capture('entry');
      await activate(row.getByRole('link', { name: 'PMG read-failure checks', exact: true }));
      await page.waitForURL('**/docs/MAIL_GATEWAY#troubleshooting-without-changing-the-gateway');
      await page.getByRole('heading', { name: 'Troubleshooting without changing the gateway', exact: true }).waitFor();
      await jump(page.locator('#troubleshooting-without-changing-the-gateway'));
      await capture('read-failures');
      const table = page.locator('article table').filter({ hasText: 'First check' });
      assert.equal(await table.getByRole('row').count(), 6);
      assert.equal(await table.locator('th[scope="col"]').count(), 2);
      caseResult.tableScroll = await table.locator('..').evaluate(el => {
        el.scrollLeft = el.scrollWidth;
        return { extent: el.scrollWidth - el.clientWidth, reached: el.scrollLeft };
      });
      assert.ok(caseResult.tableScroll.extent - caseResult.tableScroll.reached <= 1);
      if (caseResult.tableScroll.extent > 1) {
        await jump(table);
        await capture('read-failures-right-edge');
      }
      await page.goBack();
      await page.waitForURL('**/docs/TROUBLESHOOTING#choose-the-affected-workflow');
      await row.waitFor();
      await activate(row.getByRole('link', { name: 'cluster-response guidance', exact: true }));
      await page.waitForURL('**/docs/MAIL_GATEWAY#cluster-response-or-backup-lookup-errors');
      await page.getByRole('heading', { name: 'Cluster response or backup lookup errors', exact: true }).waitFor();
      await jump(page.locator('#cluster-response-or-backup-lookup-errors'));
      await capture('cluster-errors');
      const text = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ['Pulse rejected a cluster ID returned as text',
        'Use PMG\'s own web interface', 'not proof of an empty queue or no backups',
        'Do not rename the connection or gateway, change DNS',
        'no new Test, backup, mail traffic or diagnostic run',
        'Keep passwords, tickets, tokens, mail addresses']) assert.ok(text.includes(phrase), phrase);
      await activate(page.getByRole('link', { name: 'collection limits', exact: true }));
      await page.waitForURL('**#read-the-mail-gateway-view');
      await page.getByRole('heading', { name: 'Read the Mail Gateway view', exact: true }).waitFor();
      assert.ok((await page.locator('article').innerText()).includes('Healthy is not complete collection'));
      Object.assign(caseResult, { entryLinks: state.width < 500 ? 'touch' : 'keyboard',
        historyBack: true, directErrorLink: true, collectionLimitsLink: true,
        sourceBoundGuidance: true, noDocumentOverflow: true });
      await page.waitForLoadState('networkidle');
      await browser.close(); browser = undefined;
    }
    checkpoint();
    assert.deepEqual(result.errors, []);
    assert.deepEqual(result.unexpectedRequests, []);
    assert.deepEqual(result.failedResponses, []);
    assert.deepEqual(result.failedRequests, []);
    result.result = 'passed';
    result.scope = 'source-bound PMG symptom help and navigation, not live gateway or collector recovery';
    checkpoint();
    console.log(JSON.stringify(result));
  } catch (error) {
    result.result = 'failed';
    result.failure = { name: error.name, message: error.message, stack: error.stack };
    throw error;
  } finally {
    try {
      if (browser) await browser.close();
      result.browser_closed = true;
    } finally {
      try {
        if (server) await new Promise(resolve => server.close(resolve));
        result.server_closed = true;
      } finally { checkpoint(); }
    }
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
