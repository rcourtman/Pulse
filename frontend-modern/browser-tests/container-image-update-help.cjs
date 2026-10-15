// Consumes the existing source-bound doc-command-help build of the real Docs
// page, Markdown processor, router and CSS. No runtime or update is contacted.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const preview = path.join(root, 'tmp/doc-command-help/static');
const output = path.join(root, 'tmp/container-image-update-help');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const result = { source_sha: binding.source_sha, binding_sha256: hash(bindingBytes),
  cases: [], captures: [], errors: [], unexpectedRequests: [], served: [] };
const fragment = 'image-update-disagrees-with-the-runtime';

(async () => {
  fs.mkdirSync(output, { recursive: true });
  let server, browser;
  try {
    for (const [file, expected] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(frontend, file))), expected, file);
    for (const [file, expected] of Object.entries(binding.artifacts))
      assert.equal(hash(fs.readFileSync(path.join(preview, file))), expected, file);
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
      await page.addInitScript(dark => document.addEventListener('DOMContentLoaded', () =>
        document.documentElement.classList.toggle('dark', dark)), state.dark);
      const jump = locator => locator.evaluate(el => window.scrollBy(0, el.getBoundingClientRect().top - 48));
      const capture = async name => {
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), name);
        const file = `${state.name}-${name}.png`;
        await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))), state });
      };
      await page.goto(`${origin}/docs/TROUBLESHOOTING#container-${fragment}`);
      await page.getByRole('heading', { name: 'Container image update disagrees with the runtime', exact: true }).waitFor();
      await jump(page.locator(`#container-${fragment}`));
      await capture('entry');
      const link = page.getByRole('link', { name: 'image-update comparison guide', exact: true });
      if (state.width < 500) await link.tap();
      else { await link.focus(); await page.keyboard.press('Enter'); }
      await page.waitForURL(`**/docs/DOCKER#${fragment}`);
      await page.locator(`#${fragment}`).waitFor();
      await jump(page.locator(`#${fragment}`));
      await capture('comparison');
      const table = page.locator('article table').filter({ hasText: 'Existing observation' });
      assert.equal(await table.getByRole('row').count(), 5);
      assert.equal(await table.locator('th[scope="col"]').count(), 2);
      const scroll = table.locator('..');
      const scrollResult = await scroll.evaluate(el => {
        el.scrollLeft = el.scrollWidth;
        return { extent: el.scrollWidth - el.clientWidth, reached: el.scrollLeft };
      });
      assert.ok(scrollResult.extent <= 1 || scrollResult.reached > 0);
      await jump(table);
      await capture('table');
      const recipe = page.locator('article pre').filter({ hasText: 'image_id=$(docker inspect' });
      assert.equal(await recipe.count(), 1);
      assert.ok((await recipe.innerText()).includes('"$image_id"'));
      await jump(recipe);
      await capture('recipe');
      const text = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ['does not contact a registry or pull an image',
        'do not switch to root\'s separate image store', 'If inspection fails, retain the error and stop',
        'Never share registry credentials', 'clear alerts or retry an update']) assert.ok(text.includes(phrase), phrase);
      const receipts = page.getByRole('link', { name: 'the existing receipt and workload-state checks', exact: true });
      if (state.width < 500) await receipts.tap();
      else { await receipts.focus(); await page.keyboard.press('Enter'); }
      await page.waitForURL('**#check-a-failed-or-pending-workload-update');
      await page.getByRole('heading', { name: 'Check a failed or pending workload update', exact: true }).waitFor();
      await page.goto(`${origin}/docs/ISSUE_TRIAGE#intake-contract`);
      await page.getByRole('heading', { name: 'Intake contract', exact: true }).waitFor();
      await jump(page.locator('#intake-contract'));
      const intake = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ['Materially editing an already populated field also surfaces it for review',
        'Case, whitespace and hidden-comment-only changes do not restart that review'])
        assert.ok(intake.includes(phrase), phrase);
      await capture('topic-intake');
      result.cases.push({ ...state, browser: browser.version(), entryLink: state.width < 500 ? 'touch' : 'keyboard',
        receiptLink: true, fourComparisonRows: true, scrollResult, topicIntakeMirror: true, noDocumentOverflow: true });
      await browser.close(); browser = undefined;
    }
    assert.deepEqual(result.errors, []);
    assert.deepEqual(result.unexpectedRequests, []);
    result.result = 'passed';
    result.scope = 'source-bound rendered help, safe comparison and navigation; not runtime or collector recovery';
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    if (server) await new Promise(resolve => server.close(resolve));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
