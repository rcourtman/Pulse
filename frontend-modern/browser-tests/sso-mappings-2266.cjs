// #2266: prove the production SSO form keeps IdP group names with spaces.
const assert = require('node:assert/strict');
const path = require('node:path');
const { chromium } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5206, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    const observations = [];
    for (const width of [1440, 390]) {
      let mappings = { 'Server Access': 'admin' };
      const saves = [];
      const errors = [];
      const page = await browser.newPage({ viewport: { width, height: width === 390 ? 844 : 900 } });
      page.on('pageerror', (error) => errors.push(error.message));
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5206') return route.abort();
        const method = route.request().method();
        if (url.pathname === '/api/security/sso/providers' && method === 'GET') {
          return route.fulfill({ json: { providers: [{ id: 'corp-oidc', name: 'Corporate OIDC',
            type: 'oidc', enabled: true, priority: 0 }], allowMultipleProviders: false } });
        }
        if (url.pathname === '/api/security/status') {
          return route.fulfill({ json: { publicUrl: 'https://pulse.example.com' } });
        }
        if (url.pathname === '/api/security/sso/providers/corp-oidc') {
          if (method === 'GET') return route.fulfill({ json: {
            id: 'corp-oidc', name: 'Corporate OIDC', type: 'oidc', enabled: true,
            oidc: { issuerUrl: 'https://idp.example.com', clientId: 'pulse',
              scopes: ['openid', 'profile', 'email'] }, groupsClaim: 'groups',
            groupRoleMappings: mappings,
          } });
          if (method === 'PUT') {
            const payload = JSON.parse(route.request().postData());
            mappings = payload.groupRoleMappings;
            saves.push(payload);
            return route.fulfill({ json: { id: 'corp-oidc' } });
          }
        }
        if (url.pathname.startsWith('/api/')) return route.fulfill({ status: 404, json: {} });
        return route.continue();
      });
      await page.goto('http://127.0.0.1:5206/browser-tests/sso-mappings-2266.html', {
        waitUntil: 'domcontentloaded', timeout: 120_000,
      });
      await page.getByRole('button', { name: 'Edit provider' }).click();
      await page.getByRole('button', { name: 'Show access restrictions & role mapping' }).click();
      const input = page.getByRole('textbox', { name: 'Group Role Mappings' });
      assert.equal(await input.inputValue(), 'Server Access=admin');
      await input.fill('Server Access=admin,\nEveryone=viewer');
      await page.getByRole('button', { name: 'Save Changes' }).click();
      await page.getByRole('button', { name: 'Edit provider' }).click();
      await page.getByRole('button', { name: 'Show access restrictions & role mapping' }).click();
      assert.equal(await input.inputValue(), 'Server Access=admin, Everyone=viewer');
      assert.deepEqual(mappings, { 'Server Access': 'admin', Everyone: 'viewer' });
      assert.equal(saves.length, 1);
      assert.deepEqual(saves[0].oidc.scopes, ['openid', 'profile', 'email']);
      await page.getByRole('dialog').waitFor({ state: 'visible' });
      await input.scrollIntoViewIfNeeded();
      await input.focus();
      await page.waitForTimeout(350);
      const dialogBox = await page.getByRole('dialog').boundingBox();
      const mappingBox = await input.boundingBox();
      assert.ok(dialogBox && mappingBox && mappingBox.y >= dialogBox.y &&
        mappingBox.y + mappingBox.height <= dialogBox.y + dialogBox.height,
      `mapping input clipped outside dialog at ${width}px: ${JSON.stringify({ dialogBox, mappingBox })}`);
      await page.screenshot({ path: path.join(root, 'browser-tests', `sso-mappings-2266-${width}.png`),
        fullPage: true });
      const dimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth,
        inner: window.innerWidth }));
      assert.ok(dimensions.scroll <= dimensions.inner + 1,
        `${width}px horizontal overflow: ${JSON.stringify(dimensions)}`);
      await page.keyboard.press('Escape');
      assert.equal(await page.getByRole('textbox', { name: 'Group Role Mappings' }).count(), 0);
      assert.deepEqual(errors, []);
      observations.push({ width, dimensions, saves: saves.length, mapping: mappings });
      await page.close();
    }
    console.log(JSON.stringify({ result: 'passed', browser: browser.version(), observations }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
