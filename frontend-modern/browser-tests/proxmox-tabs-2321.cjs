// #2321 browser proof: production Proxmox page and tab rail with a synthetic
// reactive resource-hook seam. No live API, credentials or reporter data.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'browser-tests');
  const width = JSON.parse(fs.readFileSync(path.join(artifacts, 'proxmox-tabs-2321-run.json'), 'utf8')).width;
  assert.ok(width === 1365 || width === 390);
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const { default: solid } = await import(path.join(root, 'node_modules/vite-plugin-solid/dist/esm/index.mjs'));
  const server = await createServer({
    root,
    configFile: false,
    plugins: [solid()],
    resolve: { alias: [
      { find: /^@\/hooks\/useUnifiedResources$/, replacement: path.join(artifacts, 'proxmox-tabs-2321-hook.ts') },
      { find: '@', replacement: path.join(root, 'src') },
    ] },
    optimizeDeps: { noDiscovery: true, include: [], esbuildOptions: { target: 'esnext' } },
    server: { host: '127.0.0.1', port: 5201, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    const page = await browser.newPage({ viewport: { width, height: width === 390 ? 844 : 900 } });
    const pageErrors = [];
    const apiRequests = [];
    page.on('pageerror', (error) => pageErrors.push(error.message));
    await page.routeWebSocket(/\/ws(?:\?|$)/, () => {});
    await page.route('**/*', (route) => {
      const url = new URL(route.request().url());
      if (url.origin !== 'http://127.0.0.1:5201') return route.abort();
      if (url.pathname.startsWith('/api/')) {
        apiRequests.push(url.pathname);
        if (url.pathname === '/api/replication/jobs') return route.fulfill({ json: { data: [] } });
        return route.fulfill({ json: { data: [] } });
      }
      return route.continue();
    });
    await page.goto('http://127.0.0.1:5201/browser-tests/proxmox-tabs-2321.html?tab=mail', {
      waitUntil: 'domcontentloaded', timeout: 120_000,
    });
    await page.getByTestId('proxmox-page').waitFor();
    const nav = page.getByRole('navigation', { name: 'Proxmox sections' });
    const labels = async () => (await nav.locator('a').allTextContents()).map((label) => label.trim());
    const setCounts = async (byType) => page.evaluate((value) => window.__proxmoxTabProof.setCounts({ total: 1, byType: value }), byType);
    const screenshot = async (state) => {
      await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      const rail = await page.evaluate(() => [...document.querySelectorAll('nav[aria-label="Proxmox sections"] a')].map((link) => ({
        label: link.textContent?.trim(), current: link.getAttribute('aria-current'),
        selectedClass: link.classList.contains('border-blue-500'),
      })));
      assert.ok(rail.every((link) => link.selectedClass === (link.current === 'page')), JSON.stringify({ state, rail }));
      await page.screenshot({ path: path.join(artifacts, `proxmox-tabs-2321-${state}-${width}.png`), fullPage: true });
    };

    assert.equal(new URL(page.url()).pathname, '/proxmox/mail');
    assert.equal(await nav.count(), 0, 'unknown counts must not show ghost tabs');
    await screenshot('unknown-direct-mail');

    await setCounts({ pbs: 1 });
    await nav.waitFor();
    assert.deepEqual(await labels(), ['Overview', 'Backups']);
    assert.equal(await nav.getByRole('link', { name: 'Overview' }).getAttribute('aria-current'), 'page');
    assert.equal(new URL(page.url()).pathname, '/proxmox/mail', 'fallback keeps the bookmarked URL');
    await screenshot('pbs-only-fallback');

    const backups = nav.getByRole('link', { name: 'Backups' });
    if (width === 390) {
      await backups.focus();
      await page.keyboard.press('Enter');
    } else {
      await backups.click();
    }
    try {
      await page.waitForURL('**/proxmox/backups/date', { timeout: 5_000 });
    } catch (error) {
      throw new Error(JSON.stringify({ href: await backups.getAttribute('href'), url: page.url(), body: await page.locator('body').innerText(), pageErrors }), { cause: error });
    }
    assert.equal(new URL(page.url()).pathname, '/proxmox/backups/date');
    assert.equal(await backups.getAttribute('aria-current'), 'page');
    await screenshot('pbs-only-backups');

    await setCounts({ pbs: 1, storage: 1, ceph: 1, pmg: 1 });
    assert.deepEqual(await labels(), ['Overview', 'Storage', 'Backups', 'Ceph', 'Mail Gateway']);
    await nav.getByRole('link', { name: 'Mail Gateway' }).click();
    await page.waitForURL('**/proxmox/mail');
    assert.equal(new URL(page.url()).pathname, '/proxmox/mail');
    assert.equal(await nav.getByRole('link', { name: 'Mail Gateway' }).getAttribute('aria-current'), 'page');
    await screenshot('available-mail');

    await setCounts({ pbs: 1 });
    assert.deepEqual(await labels(), ['Overview', 'Backups']);
    assert.equal(await nav.getByRole('link', { name: 'Overview' }).getAttribute('aria-current'), 'page');
    await screenshot('removed-mail');

    await setCounts({});
    assert.equal(await nav.count(), 0);
    await screenshot('no-optional-services');

    const dimensions = await page.evaluate(() => ({ scrollWidth: document.documentElement.scrollWidth, innerWidth: window.innerWidth }));
    assert.ok(dimensions.scrollWidth <= dimensions.innerWidth + 1, JSON.stringify(dimensions));
    assert.deepEqual(pageErrors, []);
    console.log(JSON.stringify({ result: 'passed', source: 'production ProxmoxPageSurface and PlatformSectionTabs; synthetic useUnifiedResources', width, browser: browser.version(), playwright: '1.56.1', apiRequests, pageErrors, dimensions }));
    await page.close();
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
