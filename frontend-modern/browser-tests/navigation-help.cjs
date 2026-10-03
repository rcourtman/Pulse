// Production Docs/router/styles with the actual shipped Markdown; no backend.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules', 'navigation-help-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-navigation-help'),
    server: { host: '127.0.0.1', port: 5247, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [engine, width, tone] of [['chromium', 1280, 'light'], ['webkit', 390, 'dark']]) {
      browser = await { chromium, webkit }[engine].launch(
        engine === 'chromium'
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: 900 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}),
      });
      const errors = [];
      page.on('pageerror', (e) => errors.push(e.message));
      page.on('console', (m) => { if (m.type() === 'error') errors.push(m.text()); });
      await page.goto('http://127.0.0.1:5247/browser-tests/docs-fragment-navigation.html?scenario=troubleshooting',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      await page.getByRole('heading', { name: "Old bookmarks don't work", exact: true }).waitFor();
      if (tone === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
      const routes = page.locator('article table').filter({ hasText: '/standalone/machines' });
      assert.equal(await routes.count(), 1);
      const entryRoutes = await routes.locator('tbody tr').evaluateAll((rows) =>
        rows.map((row) => Array.from(row.cells, (cell) => cell.textContent.trim())),
      );
      assert.deepEqual(entryRoutes, [
        ['Proxmox', '/proxmox/overview'], ['Docker', '/docker/overview'],
        ['Kubernetes', '/kubernetes/overview'], ['TrueNAS', '/truenas/overview'],
        ['vSphere', '/vmware/overview'], ['Machines', '/standalone/machines'],
      ]);
      const article = await page.locator('article').innerText();
      assert.ok(article.includes('are supported; do not replace them with the retired task-based routes.'));
      assert.ok(article.includes('Do not delete connections or re-enrol agents'));
      await routes.scrollIntoViewIfNeeded();
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-routes.png`) });

      await page.getByRole('link', { name: 'FAQ', exact: true }).click();
      const faq = page.getByRole('heading', { name: 'How is navigation organised in Pulse v6?', exact: true });
      await faq.waitFor();
      await page.waitForFunction(() => {
        const heading = document.getElementById('how-is-navigation-organised-in-pulse-v6');
        return heading && heading.getBoundingClientRect().top >= 0 && heading.getBoundingClientRect().top < innerHeight;
      });
      assert.ok(page.url().endsWith('/docs/FAQ#how-is-navigation-organised-in-pulse-v6'));

      await page.getByRole('link', { name: '← All documentation' }).click();
      await page.getByRole('link', { name: 'TrueNAS SCALE and CORE', exact: true }).click();
      const inventory = page.locator('article table').filter({ hasText: 'Pulse page / tab' });
      await inventory.waitFor();
      const destinations = await inventory.locator('tbody tr td:nth-child(2)').allTextContents();
      assert.deepEqual(destinations, ['TrueNAS → Overview', 'TrueNAS → VMs', 'TrueNAS → Apps',
        'TrueNAS → Storage', 'TrueNAS → Storage', 'TrueNAS → Storage',
        'TrueNAS → Protection', 'TrueNAS → Protection', 'Alerts']);
      const mapping = await page.locator('article').innerText();
      for (const route of ['/truenas/overview', '/truenas/vms', '/truenas/apps', '/truenas/storage', '/truenas/protection'])
        assert.ok(mapping.includes(route));
      await inventory.scrollIntoViewIfNeeded();
      const geometry = await inventory.evaluate((table) => ({
        localScroll: table.parentElement.dataset.docTableScroll !== undefined,
        viewport: innerWidth,
        outer: document.documentElement.scrollWidth,
        table: table.parentElement.scrollWidth,
        wrapper: table.parentElement.clientWidth,
      }));
      assert.ok(geometry.localScroll);
      assert.ok(geometry.outer <= geometry.viewport + 1);
      if (width === 390) assert.ok(geometry.table > geometry.wrapper);
      await page.screenshot({ path: path.join(artifacts, `${engine}-truenas.png`) });

      await page.getByRole('link', { name: '← All documentation' }).click();
      await page.getByRole('link', { name: 'Proxmox Backup Server', exact: true }).click();
      const pbs = page.getByRole('heading', { name: 'Data Source Indicator', exact: true });
      await pbs.waitFor();
      const pbsText = await page.locator('article').innerText();
      assert.ok(pbsText.includes('Proxmox → Backups (/proxmox/backups)'));
      assert.ok(pbsText.includes('There is no current top-level Recovery page'));
      await pbs.scrollIntoViewIfNeeded();
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-pbs.png`) });
      assert.deepEqual(errors, []);
      results.push({ engine, version: browser.version(), width, tone, routes: 6, destinations,
        geometry, faqFragment: true, pbsPlacement: true, errors });
      await browser.close();
      browser = undefined;
    }
    const result = { result: 'passed', playwright: require('playwright/package.json').version,
      scope: 'shipped help rendering and navigation, not appliance collection', results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
