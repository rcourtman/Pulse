// Go-emitted connection-test responses through the unchanged production editor.
// Owned synthetic TLS fixtures only; no external endpoint, credential or save.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const out = path.join(root, 'tmp/endpoint-authority');
const compiled = path.join(root, 'tmp/node-connection-test/static');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const producerBytes = fs.readFileSync(path.join(out, 'producer.json'));
const producer = JSON.parse(producerBytes);
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = { producerSource: producer.source, producerSHA256: hash(producerBytes), compilerSource: binding.source_sha, protocolSHA256: hash(fs.readFileSync(__filename)), playwright: require('playwright/package.json').version, cases: [], requests: [], captures: [], errors: [], unexpected: [], cleanup: {} };
let server, browser, selected, consumed;
(async () => {
  try {
    fs.mkdirSync(out, { recursive: true });
    assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
    for (const [file, digest] of Object.entries(producer.files)) assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, file);
    for (const [file, digest] of Object.entries(binding.sources)) assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
    const assets = new Map();
    for (const [file, digest] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(compiled, file));
      assert.equal(hash(bytes), digest, file); assets.set('/' + file, bytes);
    }
    result.compilerInputs = Object.keys(binding.sources).length;
    result.compilerArtifacts = assets.size;
    server = http.createServer((req, res) => {
      const pathname = new URL(req.url, 'http://127.0.0.1').pathname;
      const chunks = [];
      req.on('data', (chunk) => chunks.push(chunk));
      req.on('end', () => {
        try {
          if (pathname.startsWith('/api/')) {
            assert(selected && !consumed, 'Only one explicit test consumes a producer response');
            assert.equal(pathname, selected.path);
            assert.equal(req.method, 'POST');
            assert.equal(req.headers['x-csrf-token'], 'fixture-csrf');
            const data = JSON.parse(Buffer.concat(chunks).toString());
            const { name, ...payload } = data;
            assert(name, 'Editor retains a connection name');
            assert.deepEqual(payload, selected.request, 'Actual form credentials, endpoint and TLS must match the Go producer');
            consumed = true;
            result.requests.push({ type: selected.type, network: selected.network, phase: selected.phase, path: pathname, method: req.method, bodySHA256: hash(Buffer.from(JSON.stringify(data))), responseSHA256: hash(Buffer.from(JSON.stringify(selected.body))), status: selected.status });
            res.writeHead(selected.status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
            res.end(JSON.stringify(selected.body)); return;
          }
          assert.equal(req.method, 'GET');
          const file = pathname === '/settings/infrastructure' ? '/browser-tests/node-connection-test.html' : pathname;
          const bytes = assets.get(file);
          assert(bytes, `Unexpected resource ${pathname}`);
          res.writeHead(200, { 'Content-Type': { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' }[path.extname(file)], 'Cache-Control': 'no-store' });
          res.end(bytes);
        } catch (error) {
          result.unexpected.push(String(error.stack || error));
          res.writeHead(500); res.end('Fixture protocol rejected');
        }
      });
    });
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [engine, client, width, network] of [['chromium', chromium, 1440, 'tcp4'], ['webkit', webkit, 320, 'tcp6']]) {
      browser = await client.launch(client === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const viewport = { width, height: 1000 }, narrow = width === 320;
      for (const type of ['pve', 'pbs', 'pmg']) {
        const context = await browser.newContext({ viewport, isMobile: narrow, hasTouch: narrow, locale: 'en-GB' });
        await context.addCookies([{ name: 'pulse_csrf', value: 'fixture-csrf', url: origin }]);
        const page = await context.newPage(); page.setDefaultTimeout(10000);
        page.on('pageerror', (error) => result.errors.push(String(error)));
        await context.route('**/*', (route) => {
          if (new URL(route.request().url()).origin === origin) return route.continue();
          result.unexpected.push('off-origin'); return route.abort();
        });
        await page.goto(`${origin}/settings/infrastructure?fixtureType=${type}`);
        if (narrow) await page.evaluate(() => document.documentElement.classList.add('dark'));
        if (type !== 'pmg') await page.getByRole('radio', { name: 'Username & Password' }).check();
        await page.getByRole('textbox', { name: /^Username/ }).fill('fixture@pam');
        await page.getByLabel('Password', { exact: true }).fill('synthetic-password');
        const endpoint = page.getByRole('textbox', { name: /^Endpoint URL/ });
        const pin = page.getByRole('textbox', { name: 'SSL Fingerprint (optional)' });
        const verify = page.getByRole('checkbox', { name: 'Verify SSL certificate' });
        const button = page.getByRole('button', { name: 'Test entered settings', exact: true });
        const panel = page.getByRole('status', { name: 'Connection test result' });
        const checks = [];
        for (const phase of ['matching-uppercase', 'wrong-pin', 'strict-untrusted', 'unsupported-scheme', 'recovered']) {
          selected = producer.frames.find((frame) => frame.network === network && frame.type === type && frame.mode === 'draft' && frame.phase === phase);
          assert(selected, 'Missing actual handler response'); consumed = false;
          await endpoint.fill(selected.request.host);
          await pin.fill(selected.request.fingerprint);
          await expect(verify).toBeChecked();
          await expect(panel).toHaveCount(0, 'Editing retires the previous verdict');
          const responsePromise = page.waitForResponse((r) => new URL(r.url()).pathname === selected.path);
          if (narrow) await button.tap(); else { await button.focus(); await button.press('Enter'); }
          const response = await responsePromise;
          assert.equal(response.status(), selected.status);
          assert.deepEqual(await response.json(), selected.body);
          await expect(panel).toContainText(selected.body.message);
          await expect(panel).toContainText('Entered settings test');
          await expect(panel).toContainText('No settings are saved');
          await expect(verify).toBeChecked();
          await expect(endpoint).toHaveValue(selected.request.host);
          await expect(pin).toHaveValue(selected.request.fingerprint);
          await expect(button).toBeEnabled();
          assert(consumed);
          assert.equal(selected.configurationUnchanged, true);
          checks.push({ phase, status: selected.status, reads: selected.reads, logins: selected.logins, exactProducerResponse: true, verificationStillEnabled: true, noSavedConfigMutation: true });
          if (['wrong-pin', 'recovered'].includes(phase)) {
            await panel.evaluate((element) => element.scrollIntoView({ block: 'center' }));
            await page.evaluate(() => document.fonts.ready);
            await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
            assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'No horizontal overflow');
            await expect(button).toBeInViewport({ ratio: 1 });
            const file = `${engine}-${type}-${phase}.png`;
            await page.screenshot({ path: path.join(out, file), animations: 'disabled' });
            result.captures.push({ file, engine, type, phase, viewport, sha256: hash(fs.readFileSync(path.join(out, file))) });
          }
        }
        result.cases.push({ engine, version: browser.version(), viewport, network, type, checks });
        await context.close();
      }
      await browser.close(); browser = null;
    }
    assert.deepEqual(result.errors, []); assert.deepEqual(result.unexpected, []);
    assert.equal(result.requests.length, 30); assert.equal(result.captures.length, 12);
    result.passed = true;
  } catch (error) {
    result.passed = false; result.failure = String(error.stack || error); process.exitCode = 1;
  } finally {
    if (browser) await browser.close(); result.cleanup.browserClosed = true;
    if (server) { await new Promise((resolve) => { server.close(resolve); server.closeAllConnections(); }); result.cleanup.serverClosed = true; }
    fs.writeFileSync(path.join(out, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  }
})();
