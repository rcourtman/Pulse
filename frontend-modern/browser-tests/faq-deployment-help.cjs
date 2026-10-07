// Verify shipped FAQ answers and their recovery links in the real Docs page.
// parent.md is the exact own-parent FAQ supplied locally before this run.
// No settings write, service/container change or external request is exercised.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const artifacts = path.join(root, 'node_modules/faq-deployment-browser');
  const parent = fs.readFileSync(path.join(artifacts, 'parent.md'), 'utf8');
  const final = fs.readFileSync(path.join(root, 'public/docs/FAQ.md'), 'utf8');
  const digest = (text) => createHash('sha256').update(text).digest('hex');
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(artifacts, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5259, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [engine, width, tone] of [['chromium', 1280, 'light'], ['webkit', 390, 'dark']]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const context = await browser.newContext({ viewport: { width, height: 1000 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const captures = [];
      for (const [label, content] of [['parent', parent], ['final', final]]) {
        const page = await context.newPage();
        const errors = [];
        page.on('pageerror', (error) => errors.push(error.message));
        await page.route('**/docs/FAQ.md', (route) => route.fulfill({
          status: 200, contentType: 'text/markdown; charset=utf-8', body: content,
        }));
        await page.goto('http://127.0.0.1:5259/browser-tests/docs-fragment-navigation.html',
          { waitUntil: 'domcontentloaded', timeout: 30_000 });
        if (tone === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
        await page.getByRole('link', { name: '← All documentation' }).click();
        const entry = page.locator('a[data-doc-link][href="/docs/FAQ"]').first();
        await entry.focus();
        await page.keyboard.press('Enter');
        const port = page.getByRole('heading', { name: 'How do I change the port?', exact: true });
        await port.waitFor();
        const text = await page.locator('article').innerText();
        if (label === 'parent') {
          assert.ok(text.includes('Use -p 8080:7655 in your run command.'));
          assert.ok(text.includes('Remove the env var to regain UI control.'));
          assert.ok(!text.includes('docker restart does not apply a new port mapping'));
        } else {
          for (const phrase of ['docker restart does not apply a new port mapping',
            'preserving the same image, data mount and other settings',
            'inside the Pulse container, not on the Proxmox host',
            'does not necessarily erase the saved value', 'check both the warning and effective value'])
            assert.ok(text.replace(/\s+/g, ' ').includes(phrase), phrase);
          assert.ok(!text.includes('Remove the env var to regain UI control.'));
        }
        for (const [part, heading] of [['ports', port], ['overrides', page.getByRole('heading', {
          name: "Why can't I change settings in the UI?", exact: true,
        })]]) {
          await heading.scrollIntoViewIfNeeded();
          assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
          const capture = `${engine}-${label}-${part}.jpg`;
          await page.screenshot({ path: path.join(artifacts, capture), type: 'jpeg', quality: 65 });
          captures.push(capture);
        }
        if (label === 'final') {
          for (const [name, target, heading] of [
            ['deployment-specific port checks', '/docs/TROUBLESHOOTING#port-change-didnt-take-effect', "Port change didn't take effect"],
            ['environment precedence', '/docs/CONFIGURATION#common-overrides-environment-variables', 'Common Overrides (Environment Variables)'],
            ['CORS checks', '/docs/TROUBLESHOOTING#cors-errors', 'CORS errors'],
          ]) {
            const link = page.getByRole('link', { name, exact: true });
            await link.focus();
            await page.keyboard.press('Enter');
            await page.getByRole('heading', { name: heading, exact: true }).waitFor();
            assert.ok(page.url().endsWith(target));
            await page.goBack();
            await page.getByRole('heading', { name: 'How do I change the port?', exact: true }).waitFor();
          }
        }
        assert.deepEqual(errors, []);
        await page.close();
      }
      results.push({ engine, version: browser.version(), width, tone, captures,
        parentGapReproduced: true, finalGuidanceRendered: true, keyboardLinks: 3, noBodyOverflow: true });
      await context.close();
      await browser.close();
      browser = undefined;
    }
    const result = { result: 'passed', playwrightVersion: require('playwright/package.json').version,
      parentSha256: digest(parent), finalSha256: digest(final), results,
      scope: 'FAQ rendering and recovery links, not installed port/override acceptance' };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
