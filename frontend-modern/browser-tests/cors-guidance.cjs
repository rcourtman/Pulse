const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const artifacts = path.join(root, 'node_modules', 'cors-guidance-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-cors-guidance'),
    server: { host: '127.0.0.1', port: 5258, strictPort: true },
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
      const page = await browser.newPage({ viewport: { width, height: 1000 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => { if (message.type() === 'error') errors.push(message.text()); });
      for (const override of [false, true]) {
        await page.goto(`http://127.0.0.1:5258/browser-tests/cors-guidance.html${override ? '?override=1' : ''}`,
          { waitUntil: 'domcontentloaded', timeout: 60_000 });
        if (tone === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
        const input = page.getByRole('textbox', { name: 'CORS Allowed Origins', exact: true });
        await input.waitFor();
        assert.equal(await input.getAttribute('placeholder'), 'https://app.example.com:8443');
        assert.equal(await input.getAttribute('aria-describedby'), 'cors-origin-help cors-origin-limits');
        assert.ok((await page.locator('#cors-origin-help').innerText()).includes('no CORS exception'));
        assert.ok((await page.locator('#cors-origin-limits').innerText()).includes('without credentialed browser access'));
        assert.equal(await input.isDisabled(), override);
        assert.equal(await input.inputValue(), override ? 'https://managed.example.com' : '');
        assert.equal(await page.getByRole('status', { name: 'Fixture changed' }).innerText(), 'unchanged');
        if (!override) {
          await input.focus();
          await page.keyboard.type('https://one.example.com,https://two.example.com:8443');
          assert.equal(await input.inputValue(), 'https://one.example.com,https://two.example.com:8443');
          assert.equal(await page.getByRole('status', { name: 'Fixture changed' }).innerText(), 'changed');
          assert.ok(await input.evaluate((element) => document.activeElement === element));
        }
        await page.locator('#cors-origin-help').scrollIntoViewIfNeeded();
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        await page.screenshot({ path: path.join(artifacts, `${engine}-${override ? 'override' : 'editable'}.png`) });
      }
      const faq = page.getByRole('link', { name: 'Open FAQ', exact: true });
      await faq.focus();
      await page.keyboard.press('Enter');
      await page.locator('#cors-errors').waitFor();
      const help = page.getByRole('link', { name: 'CORS checks', exact: true });
      assert.equal(await help.getAttribute('href'), '/docs/TROUBLESHOOTING#cors-errors');
      await help.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(() => {
        const heading = document.getElementById('cors-errors');
        return heading && heading.getBoundingClientRect().top >= 0 && heading.getBoundingClientRect().top < innerHeight;
      });
      assert.ok(page.url().endsWith('/docs/TROUBLESHOOTING#cors-errors'));
      const text = await page.locator('article').innerText();
      for (const phrase of ['without credentialed browser access', 'Multiple exact origins are comma-separated',
        'verify the effective value', 'Keep cookies, authorization headers']) assert.ok(text.includes(phrase), phrase);
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-help.png`) });
      assert.deepEqual(errors, []);
      results.push({ engine, browserVersion: browser.version(), width, tone, editableAndOverride: true,
        keyboardInputAndHelpLink: true, descriptionAssociation: true, noBodyOverflow: true, errors });
      await browser.close();
      browser = undefined;
    }
    const files = ['src/components/Settings/NetworkBoundarySettingsSection.tsx',
      ...['FAQ', 'TROUBLESHOOTING', 'CONFIGURATION', 'REVERSE_PROXY'].map((name) => `public/docs/${name}.md`)];
    const content_sha256 = Object.fromEntries(files.map((file) => [`frontend-modern/${file}`,
      createHash('sha256').update(fs.readFileSync(path.join(root, file))).digest('hex')]));
    const result = { result: 'passed', playwrightVersion: require('playwright/package.json').version,
      scope: 'real component/Docs/router/CSS fixtures; not native CORS, settings persistence, authentication or proxy acceptance',
      content_sha256, results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
