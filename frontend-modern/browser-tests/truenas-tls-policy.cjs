// Synthetic config HTTP -> production normalizer/hook/editor/CSS. Draft edits
// only: never test, save, preview, send credentials or contact an appliance.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, expect } = require('playwright/test');
const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/connection-evidence/static');
const output = path.resolve('tmp/truenas-tls-policy/browser');
fs.mkdirSync(output, { recursive: true });
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
for (const [file, digest] of Object.entries(binding.sources))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Source changed: ${file}`);
const assets = new Map(Object.entries(binding.artifacts).map(([file, digest]) => {
  const bytes = fs.readFileSync(path.join(compiled, file));
  assert.equal(hash(bytes), digest, `Artifact changed: ${file}`);
  return ['/' + file, bytes];
}));
const result = {
  result: 'incomplete', source: binding.source_sha,
  playwright: require('playwright/package.json').version,
  checkedInputs: Object.keys(binding.sources).length, checkedArtifacts: assets.size,
  cases: [], captures: [], errors: [], requests: [], cleanup: {},
  limits: 'Draft-policy explanations with synthetic config. No TLS handshake, native appliance, test/save, installed renewal, monitoring or release acceptance.',
};
const server = http.createServer((req, res) => {
  const route = new URL(req.url, 'http://127.0.0.1').pathname;
  const bytes = assets.get(route);
  if (req.method !== 'GET' || !bytes) { res.writeHead(404); res.end(); return; }
  res.setHeader('Content-Type', route.endsWith('.js') ? 'text/javascript' : route.endsWith('.css') ? 'text/css' : 'text/html');
  res.setHeader('Cache-Control', 'no-store'); res.end(bytes);
});
let browser;
(async () => {
  try {
    assert.equal(result.playwright, JSON.parse(fs.readFileSync('tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
    const origin = `http://127.0.0.1:${server.address().port}`;
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    result.browser = browser.version();
    for (const width of [1360, 320]) {
      const viewport = { width, height: 980 };
      const context = await browser.newContext({ viewport, hasTouch: width === 320, isMobile: width === 320, colorScheme: width === 320 ? 'dark' : 'light' });
      const page = await context.newPage(); page.setDefaultTimeout(10000);
      page.on('pageerror', error => result.errors.push(error.message));
      page.on('console', message => { if (message.type() === 'error') result.errors.push(message.text()); });
      if (width === 320) await page.addInitScript(() => document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.add('dark')));
      await page.route('**/*', async route => {
        const req = route.request(), url = new URL(req.url());
        assert.equal(url.origin, origin, 'Off-origin request forbidden');
        if (!url.pathname.startsWith('/api/')) return route.continue();
        result.requests.push({ width, method: req.method(), path: url.pathname });
        assert.equal(req.method(), 'GET', 'No mutation/probe allowed');
        assert.equal(url.pathname, '/api/truenas/connections');
        return route.fulfill({ json: [{
          id: 'nas-fixture', name: 'NAS fixture', host: 'nas.example.invalid', port: 443,
          apiKey: '********', username: 'fixture', useHttps: true, insecureSkipVerify: false,
          enabled: true, monitorDatasets: true, monitorPools: true, monitorReplication: true,
        }] });
      });
      await page.goto(origin + '/browser-tests/connection-evidence.html?platform=truenas');
      const policy = page.getByRole('status', { name: 'TrueNAS form TLS policy' });
      const host = page.getByRole('textbox', { name: 'Host', exact: true });
      const port = page.getByRole('textbox', { name: 'Port', exact: true });
      const pin = page.getByRole('textbox', { name: 'TLS fingerprint', exact: true });
      const skip = page.getByRole('checkbox', { name: 'Skip TLS verification', exact: true });
      const https = page.getByRole('checkbox', { name: 'Use HTTPS', exact: true });
      const checks = [];
      const check = async title => { await expect(policy).toContainText(title); checks.push(title); };
      const act = async locator => {
        if (width === 320) await locator.tap();
        else { await locator.focus(); await page.keyboard.press('Space'); }
      };
      const capture = async name => {
        await policy.evaluate(el => el.scrollIntoView({ block: 'end', behavior: 'instant' }));
        await page.evaluate(async () => { await document.fonts.ready; await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))); });
        await expect(policy).toBeInViewport({ ratio: 1 });
        const file = `${width}-${name}.png`;
        await page.screenshot({ path: path.join(output, file) });
        const bytes = fs.readFileSync(path.join(output, file));
        result.captures.push({ file, viewport, name, bytes: bytes.length, sha256: hash(bytes) });
      };
      await check('Trusted certificate chain required');
      await expect(pin).toHaveAccessibleDescription(/Independently verify a replacement/);
      await pin.fill('AB:'.repeat(31) + 'AB');
      await check('Certificate chain and fingerprint required');
      await capture('chain-and-pin');
      await act(skip);
      await check('Certificate fingerprint required');
      await expect(policy).toContainText('hostname and expiry checks are skipped');
      assert((await skip.boundingBox()).height > 0);
      assert((await skip.locator('..').boundingBox()).height >= 44);
      if (width === 320) await capture('pin-only');
      await pin.fill(' SHA256: : ');
      await check('Certificate identity not verified');
      await pin.fill('invalid');
      await check('Certificate fingerprint is invalid');
      await pin.fill('ab'.repeat(32));
      await host.fill('http://nas.example.invalid:80');
      await check('HTTP does not protect credentials');
      await expect(https).toBeChecked();
      await expect(skip).toHaveAccessibleDescription(/Neither a fingerprint/);
      await capture('http-overrides-checkbox');
      await port.fill('');
      await check('Certificate fingerprint required');
      await expect(host).toHaveAccessibleDescription(/With no separate port, the client defaults to HTTPS/);
      await port.fill('80');
      await check('HTTP does not protect credentials');
      await host.fill('https://nas.example.invalid');
      await act(https);
      await expect(https).not.toBeChecked();
      await check('Certificate fingerprint required');
      await act(skip);
      await check('Certificate chain and fingerprint required');
      await pin.fill('');
      await check('Trusted certificate chain required');
      await expect(page.getByText(/Form settings only, not a connection-test result/)).toBeVisible();
      assert.equal(await page.getByText('TLS verification disabled.', { exact: true }).count(), 0);
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'Horizontal page overflow');
      result.cases.push({ viewport, checks, interactions: width === 320 ? 'touch and input' : 'keyboard and input' });
      await context.close();
    }
    assert.deepEqual(result.errors, []);
    assert.equal(result.requests.length, 2, 'Only one configuration read per context');
    result.result = 'passed';
  } finally {
    if (browser) { await browser.close(); result.cleanup.browser_closed = true; }
    await new Promise(resolve => server.close(resolve)); result.cleanup.server_closed = !server.listening;
    fs.writeFileSync(path.join(output, 'RESULT.json'), JSON.stringify(result, null, 2) + '\n');
  }
})().catch(error => { console.error(error.stack); process.exitCode = 1; });
