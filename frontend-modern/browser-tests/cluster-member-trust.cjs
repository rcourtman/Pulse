const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { pathToFileURL } = require('node:url');
const { chromium, webkit } = require('playwright');
const { expect } = require('playwright/test');

const root = path.resolve('frontend-modern');
const output = path.resolve('tmp/cluster-member-trust');
const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json')));
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
for (const [file, digest] of Object.entries(binding.files)) {
  assert.equal(hash(fs.readFileSync(file)), digest, `Source changed: ${file}`);
}
const pins = {
  primary: 'ef'.repeat(32),
  alpha: Array.from({ length: 32 }, (_, i) => i.toString(16).padStart(2, '0')).join(':'),
  beta: 'ab'.repeat(32),
  refreshed: 'cd'.repeat(32),
};
const result = {
  source: binding.source,
  playwright: require('playwright/package.json').version,
  boundInputs: Object.keys(binding.files).length,
  cases: [],
  captures: [],
  requests: [],
  errors: [],
  cleanup: {},
};
let server;
let browser;
let context;
const config = (refreshed = false) => [
  {
    id: 'fixture-cluster',
    type: 'pve',
    name: 'Synthetic cluster',
    host: 'https://primary.example.invalid:8006',
    user: 'fixture@pve',
    tokenName: 'fixture@pve!read',
    hasToken: true,
    verifySSL: true,
    fingerprint: pins.primary,
    isCluster: true,
    clusterEndpoints: ['alpha', 'beta', 'missing'].map((name, i) => ({
      nodeId: `node/${name}`,
      nodeIdentity: `identity-${name}`,
      nodeName: name,
      host: `https://${name}.example.invalid:8006`,
      ip: `192.0.2.${i + 10}`,
      online: true,
      lastSeen: '2026-10-15T16:00:00Z',
      pulseReachable: false,
      pulseError: i < 2 ? 'Certificate fingerprint mismatch' : 'Connection timed out',
      ...(i < 2
        ? { fingerprint: i === 0 ? (refreshed ? pins.refreshed : pins.alpha) : pins.beta }
        : {}),
    })),
  },
];

(async () => {
  try {
    const { createServer } = await import(
      pathToFileURL(path.join(root, 'node_modules/vite/dist/node/index.js')).href
    );
    server = await createServer({
      root,
      configFile: path.join(root, 'vite.config.ts'),
      logLevel: 'error',
      server: { host: '127.0.0.1', port: 0, strictPort: false },
      optimizeDeps: { noDiscovery: true, include: ['solid-js', 'solid-js/web', '@solidjs/router'] },
    });
    await server.listen();
    const origin = `http://127.0.0.1:${server.httpServer.address().port}`;
    for (const [engine, viewport] of [
      ['chromium', { width: 1440, height: 1000 }],
      ['webkit', { width: 390, height: 900 }],
      ['webkit', { width: 320, height: 900 }],
    ]) {
      const narrow = viewport.width < 768;
      browser = await (engine === 'chromium' ? chromium : webkit).launch(
        engine === 'chromium'
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      context = await browser.newContext({ viewport, hasTouch: narrow, isMobile: narrow });
      await context.addCookies([{ name: 'pulse_csrf', value: 'synthetic-csrf', url: origin }]);
      const page = await context.newPage();
      page.on('pageerror', (error) => result.errors.push(error.message));
      let refreshed = false;
      let saved;
      const requests = [];
      await page.route('**/*', async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        assert.equal(url.origin, origin, 'No off-origin request');
        if (!url.pathname.startsWith('/api/')) return route.continue();
        const record = { method: request.method(), path: url.pathname };
        requests.push(record);
        if (url.pathname === '/api/config/nodes' && request.method() === 'GET') {
          return route.fulfill({ json: config(refreshed) });
        }
        assert.equal(url.pathname, '/api/config/nodes/fixture-cluster');
        assert.equal(request.method(), 'PUT', 'No probe, test or certificate acquisition');
        saved = request.postDataJSON();
        record.body = saved;
        return route.fulfill({ json: { success: true } });
      });
      const capture = async (state, anchor) => {
        await anchor.scrollIntoViewIfNeeded();
        assert(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          'No page overflow',
        );
        const file = path.join(output, `${engine}-${viewport.width}-${state}.png`);
        await page.screenshot({ path: file });
        result.captures.push({
          file: path.relative(process.cwd(), file),
          sha256: hash(fs.readFileSync(file)),
          state,
          engine,
          viewport,
        });
      };
      const activate = async (locator) => {
        if (narrow) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      const disclosure = (member) => page.locator('summary').filter({ hasText: `for ${member}` });
      const trust = (member) =>
        page.getByRole('group', { name: `Saved certificate trust for ${member}` });
      await page.goto(`${origin}/browser-tests/cluster-member-trust.html`, {
        waitUntil: 'networkidle',
      });
      await expect(disclosure('alpha')).toBeVisible();
      await expect(page.getByText(pins.alpha, { exact: true })).toBeHidden();
      await activate(disclosure('alpha'));
      await expect(trust('alpha')).toBeVisible();
      await expect(trust('alpha').locator('code')).toHaveText(pins.alpha);
      await expect(trust('alpha')).toContainText(
        'Saved configuration, not a fresh certificate check',
      );
      await expect(trust('alpha')).toContainText('Do not clear a pin or disable verification');
      const geometry = await trust('alpha')
        .locator('code')
        .evaluate((element) => ({ width: element.clientWidth, scroll: element.scrollWidth }));
      assert(geometry.scroll <= geometry.width, 'Whole pin wraps inside the disclosure');
      const selected = await trust('alpha')
        .locator('code')
        .evaluate((element) => {
          const range = document.createRange();
          range.selectNodeContents(element);
          const selection = getSelection();
          selection.removeAllRanges();
          selection.addRange(range);
          const text = selection.toString();
          selection.removeAllRanges();
          return text;
        });
      assert.equal(selected, pins.alpha, 'Complete pin is selectable, not truncated');
      if (!narrow) {
        await disclosure('alpha').focus();
        await page.keyboard.press('Space');
        await expect(trust('alpha')).toBeHidden();
        await page.keyboard.press('Enter');
        await expect(trust('alpha')).toBeVisible();
      }
      assert((await disclosure('alpha').boundingBox()).height >= 44, 'Touch target');
      await capture('saved-pin', trust('alpha'));

      await activate(disclosure('beta'));
      await expect(trust('beta').locator('code')).toHaveText(pins.beta);
      await expect(trust('beta')).not.toContainText(pins.alpha);
      await activate(disclosure('missing'));
      await expect(trust('missing')).toContainText('No member certificate pin is recorded.');
      await expect(trust('missing').locator('code')).toHaveCount(0);
      await capture('missing-pin', trust('missing'));

      await page.getByLabel('Display name for alpha', { exact: true }).fill('Operator alpha');
      await page
        .getByLabel('Connection address for alpha', { exact: true })
        .fill('192.0.2.99:8006');
      refreshed = true;
      await page.evaluate(() => window.reloadClusterConfig());
      // Poll replaces member rows, while the state hook preserves dirty fields.
      await expect(page.getByText(pins.refreshed, { exact: true })).toBeAttached();
      if (!(await trust('alpha').isVisible())) await activate(disclosure('alpha'));
      await expect(trust('alpha').locator('code')).toHaveText(pins.refreshed);
      await expect(page.getByText(pins.alpha, { exact: true })).toHaveCount(0);
      await expect(page.getByLabel('Display name for alpha', { exact: true })).toHaveValue(
        'Operator alpha',
      );
      await expect(page.getByLabel('Connection address for alpha', { exact: true })).toHaveValue(
        '192.0.2.99:8006',
      );
      await expect(page.getByRole('checkbox', { name: 'Verify SSL certificate' })).toBeChecked();
      await capture('refreshed-pin', trust('alpha'));
      assert.equal(
        requests.filter((request) => request.method !== 'GET').length,
        0,
        'Inspection makes no mutation',
      );
      await activate(page.getByRole('button', { name: 'Save changes', exact: true }));
      await expect.poll(() => Boolean(saved)).toBe(true);
      assert.equal(saved.verifySSL, true);
      assert.equal(saved.fingerprint, pins.primary, 'Entry endpoint pin unchanged');
      assert.deepEqual(saved.clusterEndpointOverrides, [
        { nodeName: 'alpha', ipOverride: '192.0.2.99:8006' },
      ]);
      assert.deepEqual(saved.clusterNodeDisplayNameOverrides, [
        { nodeIdentity: 'identity-alpha', displayName: 'Operator alpha' },
      ]);
      for (const field of ['clusterEndpoints', 'tokenValue', 'password'])
        assert(!(field in saved), `${field} not submitted`);
      assert.equal(requests.filter((request) => request.method === 'PUT').length, 1);
      result.requests.push({ engine, viewport, requests });
      result.cases.push({
        engine,
        browser: browser.version(),
        viewport,
        states: [
          'native disclosure',
          'complete selectable wrapped pin',
          'separate member pins',
          'missing pin',
          'refreshed saved pin',
          'dirty edits preserved',
          'explicit unchanged-trust save',
        ],
      });
      await context.close();
      context = null;
      await browser.close();
      browser = null;
    }
    assert.deepEqual(result.errors, []);
  } catch (error) {
    result.errors.push(error.stack || String(error));
    process.exitCode = 1;
  } finally {
    if (context) await context.close();
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    if (server) await server.close();
    result.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        source: result.source,
        cases: result.cases.length,
        captures: result.captures.length,
        errors: result.errors,
        cleanup: result.cleanup,
      }),
    );
  }
})();
