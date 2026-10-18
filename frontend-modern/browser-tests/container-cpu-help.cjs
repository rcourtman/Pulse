// The real Docs renderer, router and shipped help. No runtime socket, workload,
// credentials, alert test or diagnostics collection is exercised.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const artifacts = path.join(root, 'node_modules', 'container-cpu-help-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-container-cpu-help'),
    server: { host: '127.0.0.1', port: 5265, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [engine, width, tone] of [
      ['chromium', 1280, 'light'], ['chromium', 1280, 'dark'],
      ['webkit', 390, 'light'], ['webkit', 390, 'dark'],
    ]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const page = await browser.newPage({ viewport: { width, height: 900 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [];
      const operations = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      page.on('request', (request) => {
        if (request.url().includes('/api/') || request.method() !== 'GET')
          operations.push({ method: request.method(), url: request.url() });
      });
      await page.goto('http://127.0.0.1:5265/browser-tests/docs-fragment-navigation.html?scenario=troubleshooting',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      if (tone === 'dark') await page.evaluate(async () => {
        document.documentElement.classList.add('dark');
        await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
      });
      const entry = page.getByRole('heading', {
        name: 'Container CPU differs from Docker or Podman stats', exact: true,
      });
      await entry.waitFor();
      await entry.evaluate((heading) => heading.scrollIntoView({ block: 'start' }));
      await page.screenshot({ path: path.join(artifacts, `${engine}-${tone}-entry.png`) });
      const guideLink = page.getByRole('link', { name: 'container CPU readings', exact: true });
      if (engine === 'webkit') await guideLink.tap();
      else {
        await guideLink.focus();
        await page.keyboard.press('Enter');
      }
      const heading = page.getByRole('heading', { name: 'Container CPU readings', exact: true });
      await heading.waitFor();
      await page.waitForFunction(() => {
        const h = document.getElementById('container-cpu-readings');
        return h && h.getBoundingClientRect().top >= 0 && h.getBoundingClientRect().top < innerHeight;
      });
      assert.ok(page.url().endsWith('/docs/DOCKER#container-cpu-readings'));
      const article = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ["Docker/Podman host's total CPU capacity", '100% for one logical CPU',
        'not an assumption about your host', 'v6.5.0-rc.1', 'unavailable data, not a measured zero',
        'Do not divide Pulse History again', 'Active does not prove fresh CPU samples',
        'not that an alert should fire or that its notification was delivered',
        'Do not restart workloads, create CPU load, lower alert thresholds'])
        assert.ok(article.includes(phrase), phrase);
      const table = page.locator('article table').filter({ hasText: 'Example host logical CPUs' });
      assert.equal(await table.getByRole('row').count(), 4);
      assert.equal(await table.getByRole('columnheader').count(), 3);
      const rows = await table.getByRole('row').allTextContents();
      assert.ok(rows[1].includes('240%') && rows[1].includes('60%'));
      assert.ok(rows[2].includes('16.52%') && rows[2].includes('2.75%'));
      assert.ok(rows[3].includes('1.04%') && rows[3].includes('0.17%'));
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await table.scrollIntoViewIfNeeded();
      await page.screenshot({ path: path.join(artifacts, `${engine}-${tone}-examples.png`) });
      const performance = page.getByRole('link', { name: 'safe performance measurements', exact: true });
      if (engine === 'webkit') await performance.tap();
      else {
        await performance.focus();
        await page.keyboard.press('Enter');
      }
      await page.getByRole('heading', { name: 'Excessive CPU, writes or database growth', exact: true }).waitFor();
      await page.waitForFunction(() => {
        const h = document.getElementById('excessive-cpu-writes-or-database-growth');
        return h && h.getBoundingClientRect().top >= 0 && h.getBoundingClientRect().top < innerHeight;
      });
      assert.ok(page.url().endsWith('/docs/TROUBLESHOOTING#excessive-cpu-writes-or-database-growth'));
      assert.deepEqual(errors, []);
      assert.deepEqual(operations, []);
      results.push({ engine, version: browser.version(), width, tone, exampleRows: rows,
        entryLink: engine === 'webkit' ? 'touch' : 'keyboard', performanceLink: true,
        noDocumentOverflow: true, errors, operations });
      await browser.close();
      browser = undefined;
    }
    const docs = ['DOCKER.md', 'TROUBLESHOOTING.md'].map((name) => ({ name,
      sha256: crypto.createHash('sha256').update(fs.readFileSync(path.join(root, 'public', 'docs', name))).digest('hex'),
    }));
    const result = { result: 'passed', playwright: require('playwright/package.json').version,
      scope: 'shipped CPU help rendering and navigation, not collector accuracy or alert delivery', docs, results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
