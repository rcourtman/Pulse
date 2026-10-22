// Existing reading flow only: no portal, billing, provisioning or recovery action.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/cloud-availability');
const preview = path.join(root, 'tmp/doc-command-help/static');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const result = { result: 'incomplete', source_sha: binding.source_sha,
  binding_sha256: hash(bindingBytes), cases: [], captures: [], requests: [], errors: [], cleanup: {} };
const fragment = 'what-do-pro-and-cloud-unlock';
const changed = ['CLOUD', 'FAQ', 'PULSE_PRO'];
const baseline = Object.fromEntries(changed.map(name => [name,
  fs.readFileSync(path.join(output, `parent-${name}.md`))]));
result.parent_docs = Object.fromEntries(changed.map(name => [name, hash(baseline[name])]));
const slugSection = async (page, id) => page.locator(`[id="${id}"]`).evaluate(heading => {
  let text = '';
  for (let node = heading.nextElementSibling; node && !/^H[1-3]$/.test(node.tagName);
    node = node.nextElementSibling) text += ' ' + node.textContent;
  return text.replace(/\s+/g, ' ');
});
(async () => {
  let server, browser;
  try {
    result.playwright = require('playwright/package.json').version;
    assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.join(root,
      'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
    for (const [file, expected] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(frontend, file))), expected, file);
    for (const [file, expected] of Object.entries(binding.artifacts))
      assert.equal(hash(fs.readFileSync(path.join(preview, file))), expected, file);
    result.source_inputs = Object.keys(binding.sources).length;
    result.artifacts = Object.keys(binding.artifacts).length;
    server = http.createServer((req, res) => {
      if (req.method !== 'GET') { res.writeHead(405); res.end(); return; }
      const route = new URL(req.url, 'http://fixture.invalid').pathname;
      let asset = route.slice(1);
      if (route.startsWith('/docs/') && !route.endsWith('.md'))
        asset = 'browser-tests/doc-command-help.html';
      if (!Object.hasOwn(binding.artifacts, asset)) { res.writeHead(404); res.end(); return; }
      const bytes = fs.readFileSync(path.join(preview, asset));
      assert.equal(hash(bytes), binding.artifacts[asset]);
      const type = {'.md':'text/markdown', '.css':'text/css', '.js':'text/javascript', '.html':'text/html'}[path.extname(asset)];
      res.writeHead(200, {'Content-Type': `${type}; charset=utf-8`, 'Cache-Control':'no-store'});
      res.end(bytes);
    });
    await new Promise((resolve, reject) => {
      server.once('error', reject); server.listen(0, '127.0.0.1', resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const state of [
      { name: 'desktop', width: 1360, height: 960, engine: 'chromium', touch: false },
      { name: 'narrow', width: 320, height: 920, engine: 'webkit', touch: true },
    ]) {
      browser = await (state.touch ? webkit.launch({headless:true}) :
        chromium.launch({headless:true, channel:'chromium', args:['--no-sandbox']}));
      const record = { ...state, browser: browser.version(), baseline: {}, checks: [] };
      result.cases.push(record);
      const context = await browser.newContext({viewport:{width:state.width, height:state.height},
        isMobile:state.touch, hasTouch:state.touch, locale:'en-GB', colorScheme:'light'});
      let parent = true;
      await context.route('**/*', route => {
        const request = route.request(); const url = new URL(request.url());
        result.requests.push({ path: url.pathname, method: request.method(), local: url.origin === origin });
        if (url.origin !== origin || request.method() !== 'GET' || url.pathname.startsWith('/api/')) {
          result.errors.push('Unexpected request rejected'); return route.abort();
        }
        const name = /^\/docs\/(CLOUD|FAQ|PULSE_PRO)\.md$/.exec(url.pathname)?.[1];
        if (parent && name) return route.fulfill({status:200, contentType:'text/markdown', body:baseline[name]});
        return route.continue();
      });
      let page = await context.newPage();
      page.setDefaultTimeout(8000);
      page.on('pageerror', error => result.errors.push(error.message));
      await page.goto(`${origin}/docs/FAQ#${fragment}`);
      await page.locator(`#${fragment}`).waitFor();
      record.baseline.faq_boundary_missing = !(await slugSection(page, fragment)).includes('not generally available');
      assert(record.baseline.faq_boundary_missing);
      await page.goto(`${origin}/docs/CLOUD`);
      await page.locator('#how-it-works').waitFor();
      record.baseline.signup_instruction_present = (await page.locator('article').innerText()).includes('Sign up via the Pulse Cloud portal');
      assert(record.baseline.signup_instruction_present);
      await page.close();
      parent = false;
      page = await context.newPage();
      page.setDefaultTimeout(8000);
      page.on('pageerror', error => result.errors.push(error.message));
      await page.goto(`${origin}/docs/FAQ#${fragment}`);
      await page.locator(`#${fragment}`).waitFor();
      const activate = async (link) => {
        await link.scrollIntoViewIfNeeded();
        if (state.touch) await link.tap(); else { await link.focus(); await page.keyboard.press('Enter'); }
      };
      const capture = async (suffix, target) => {
        await page.locator(target).evaluate(el => window.scrollBy(0, el.getBoundingClientRect().top - 32));
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'horizontal overflow');
        const file = `${state.name}-${suffix}.png`;
        await page.screenshot({path:path.join(output, file)});
        result.captures.push({file, sha256:hash(fs.readFileSync(path.join(output,file)))});
      };
      const text = await slugSection(page, fragment);
      assert(text.includes('Pulse Cloud is not generally available.'));
      assert(text.indexOf('not generally available') < text.indexOf('hands-on Patrol'));
      await capture('faq', `#${fragment}`);
      await page.evaluate(() => window.__cloud_help_shell = 'preserved');
      await activate(page.locator('article a[href="/docs/CLOUD#1-confirm-existing-hosted-access"]'));
      await page.waitForURL('**/docs/CLOUD#1-confirm-existing-hosted-access');
      await page.locator('#1-confirm-existing-hosted-access').waitFor();
      assert.equal(await page.evaluate(() => window.__cloud_help_shell), 'preserved');
      assert((await slugSection(page, '1-confirm-existing-hosted-access')).includes('rather than purchasing again'));
      await capture('existing-access', '#1-confirm-existing-hosted-access');
      record.checks.push('FAQ existing fragment, availability first, keyboard/touch same-shell link');
      await activate(page.locator('article a[href="/docs/PULSE_PRO"]').first());
      await page.waitForURL('**/docs/PULSE_PRO');
      await page.locator('#cloud').waitFor();
      assert((await slugSection(page, 'cloud')).includes('not generally available'));
      await activate(page.locator('article a[href="/docs/CLOUD"]').first());
      await page.waitForURL('**/docs/CLOUD');
      await page.locator('#how-it-works').waitFor();
      const cloud = await page.locator('article').innerText();
      assert(cloud.includes('Pulse Cloud is not generally available.'));
      assert(!cloud.includes('Sign up via the Pulse Cloud portal'));
      assert(!cloud.includes('provisioned automatically after checkout'));
      await capture('cloud', 'article');
      record.checks.push('Plans Cloud boundary and existing guide link, no signup/provisioning instruction');
      const legacy = await context.newPage();
      await legacy.goto(`${origin}/docs/CLOUD#1-create-your-account`);
      await legacy.waitForFunction(() => document.activeElement?.id === '1-create-your-account');
      assert(await legacy.locator('#1-confirm-existing-hosted-access').evaluate(el =>
        el.getBoundingClientRect().top >= 0 && el.getBoundingClientRect().top < innerHeight));
      await legacy.close();
      record.checks.push('old account fragment still focuses and reveals replacement access section');
      await activate(page.locator('article a[href="/docs/INSTALL"]').first());
      await page.waitForURL('**/docs/INSTALL');
      await page.locator('h1').filter({hasText:'Installation Guide'}).waitFor();
      record.checks.push('new-install reader reaches actual Installation Guide without external request');
      for (const name of changed) {
        const response = await page.request.get(`${origin}/docs/${name}.md`);
        assert.equal(response.status(),200);
        assert.equal(hash(await response.body()),binding.sources[`public/docs/${name}.md`]);
      }
      record.checks.push('all three served producer files match the compiled binding');
      await context.close(); await browser.close(); browser = undefined;
    }
    assert.deepEqual(result.errors,[]);
    result.result = 'passed';
  } catch(error) { result.result='failed'; result.error=String(error.stack || error); process.exitCode=1; }
  finally {
    if (browser) await browser.close();
    result.cleanup.browsers_closed = true;
    if (server) await new Promise(resolve=>server.close(resolve));
    result.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output,'browser-result.json'),JSON.stringify(result,null,2)+'\n');
    console.log(JSON.stringify({source:result.source_sha,result:result.result,cases:result.cases.length,
      captures:result.captures.length,errors:result.errors,cleanup:result.cleanup,error:result.error}));
  }
})();
