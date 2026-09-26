// Browser proof for #2263. Uses the production guest overview and resource hook
// in Vite, with only the network response replaced by bounded fixture data.
const assert = require('node:assert/strict');
const path = require('node:path');
const { chromium } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'browser-tests');
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5199, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    const requests = [];
    const pageErrors = [];
    const disk = {
      id: 'disk-guest-101', parentId: 'vm-resource-101', type: 'physical_disk',
      name: 'guest-sata', displayName: 'Guest SATA', platformId: 'lab',
      platformType: 'proxmox-pve', sourceType: 'agent', status: 'online',
      lastSeen: new Date().toISOString(),
      physicalDisk: { devPath: '/dev/sda', model: 'Guest SATA', diskType: 'sata',
        sizeBytes: 1_000_000_000, health: 'PASSED', temperature: 45,
        smart: { reallocatedSectors: 0, pendingSectors: 0, udmaCrcErrors: 2 } },
    };
    for (const width of [1440, 390]) {
      const page = await browser.newPage({ viewport: { width, height: width === 390 ? 844 : 900 } });
      page.on('pageerror', (error) => pageErrors.push(`${width}: ${error.message}`));
      page.on('requestfailed', (request) => console.error('requestfailed', request.url(), request.failure()));
      await page.routeWebSocket(/\/ws(?:\?|$)/, () => {});
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5199') return route.abort();
        if (url.pathname === '/api/resources') {
          const parent = url.searchParams.get('parent');
          requests.push({ width, type: url.searchParams.get('type'), parent,
            page: url.searchParams.get('page'), limit: url.searchParams.get('limit') });
          if (parent === 'vm-resource-error') return route.fulfill({ status: 503, json: { error: 'fixture' } });
          return route.fulfill({ json: { resources: [disk, { ...disk, id: 'host-disk',
            parentId: 'pve-host' }], meta: { totalPages: 1 } } });
        }
        if (url.pathname.startsWith('/api/')) return route.fulfill({ json: {} });
        return route.continue();
      });
      await page.goto('http://127.0.0.1:5199/browser-tests/guest-storage-2263.html', {
        waitUntil: 'domcontentloaded', timeout: 120_000,
      });
      await page.locator('[data-testid="guest-physical-disks"]').waitFor();
      assert.match(await page.locator('body').innerText(), /Physical Disks & SMART \(1\)/i);
      assert.equal(await page.locator('[data-testid="guest-physical-disk"]').count(), 1);
      assert.match(await page.locator('body').innerText(), /Guest RAID/i);
      assert.match(await page.locator('body').innerText(), /raid1/);
      await page.locator('[data-testid="guest-physical-disk"] summary').focus();
      await page.keyboard.press('Enter');
      await page.getByText('Reallocated Sectors').waitFor();
      await page.screenshot({ path: path.join(artifacts, `guest-storage-2263-${width}.png`), fullPage: true });
      const beforeNoAgent = requests.length;
      await page.getByRole('button', { name: 'No agent' }).click();
      assert.equal(await page.locator('[data-testid="guest-physical-disks"]').count(), 0);
      assert.equal(requests.length, beforeNoAgent, 'uninstrumented guest must not fetch physical disks');
      await page.getByRole('button', { name: 'Unavailable query' }).click();
      await page.getByText('Physical disk details are unavailable.').waitFor();
      assert.equal(await page.locator('[data-testid="guest-physical-disks"]').count(), 0);
      const dimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth,
        inner: window.innerWidth }));
      assert.ok(dimensions.scroll <= dimensions.inner + 1, `${width}px horizontal overflow: ${JSON.stringify(dimensions)}`);
      await page.close();
    }
    assert.equal(pageErrors.length, 0, `page errors: ${pageErrors.join('; ')}`);
    assert.deepEqual(requests.map(({ type, parent, page, limit }) => ({ type, parent, page, limit })), [
      { type: 'physical_disk', parent: 'vm-resource-101', page: '1', limit: '100' },
      { type: 'physical_disk', parent: 'vm-resource-error', page: '1', limit: '100' },
      { type: 'physical_disk', parent: 'vm-resource-101', page: '1', limit: '100' },
      { type: 'physical_disk', parent: 'vm-resource-error', page: '1', limit: '100' },
    ]);
    console.log(JSON.stringify({ result: 'passed', browser: browser.version(),
      viewports: [1440, 390], route: '/browser-tests/guest-storage-2263.html',
      requests, pageErrors }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
