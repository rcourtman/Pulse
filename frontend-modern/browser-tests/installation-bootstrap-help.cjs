// Exercise the Installation Guide in the production Docs/router/Markdown/CSS.
// This checks rendered instructions and navigation, not native Docker startup.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const artifacts = path.join(root, 'node_modules', 'installation-bootstrap-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-installation-bootstrap'),
    server: { host: '127.0.0.1', port: 5250, strictPort: true },
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
      await page.goto('http://127.0.0.1:5250/browser-tests/docs-fragment-navigation.html',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      if (tone === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
      await page.getByRole('link', { name: '← All documentation' }).click();
      const installLink = page.getByRole('link', { name: 'Install Pulse', exact: true });
      await installLink.focus();
      await page.keyboard.press('Enter');
      await page.getByRole('heading', { name: 'Docker Compose', exact: true }).waitFor();
      const text = await page.locator('article').innerText();
      for (const phrase of ['Leave authentication overrides unset for a new install',
        'do not reset authentication', 'deployment-supplied password takes precedence',
        'not a Compose interpolation recipe', 'Never share full docker inspect'])
        assert.ok(text.includes(phrase), phrase);
      assert.ok(!text.includes('secret123'));
      const compose = page.locator('pre').filter({ hasText: 'services:' }).first();
      const code = await compose.innerText();
      for (const phrase of ['${PULSE_IMAGE:-rcourtman/pulse:vX.Y.Z}',
        'pulse_data:/data', 'PULSE_DEPLOYMENT_METHOD=docker_compose'])
        assert.ok(code.includes(phrase), phrase);
      assert.ok(!code.includes('PULSE_AUTH_'));
      await compose.scrollIntoViewIfNeeded();
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-compose.png`) });
      const warning = page.getByText('For an existing installation, preserve its image', { exact: false });
      await warning.scrollIntoViewIfNeeded();
      await page.screenshot({ path: path.join(artifacts, `${engine}-existing-install.png`) });

      const bootstrap = page.getByRole('link', { name: 'bootstrap-token setup', exact: true }).last();
      await bootstrap.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(() => {
        const h = document.getElementById('step-1-get-the-token');
        return h && h.getBoundingClientRect().top >= 0 && h.getBoundingClientRect().top < innerHeight;
      });
      assert.ok(page.url().endsWith('/docs/INSTALL#step-1-get-the-token'));
      assert.ok((await page.locator('article').innerText()).includes('docker exec pulse /app/pulse bootstrap-token'));
      await page.screenshot({ path: path.join(artifacts, `${engine}-bootstrap.png`) });
      const auth = page.getByRole('link', { name: 'authentication guide', exact: true });
      await auth.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(() => {
        const h = document.getElementById('private-docker-authentication-file');
        return h && h.getBoundingClientRect().top >= 0 && h.getBoundingClientRect().top < innerHeight;
      });
      assert.ok(page.url().endsWith('/docs/CONFIGURATION#private-docker-authentication-file'));
      assert.ok((await page.locator('article').innerText()).includes('not out of Docker\'s container environment'));
      assert.deepEqual(errors, []);
      results.push({ engine, version: browser.version(), width, tone,
        noSharedComposeCredentials: true, paidImageAndDataPreserved: true,
        existingInstallWarning: true, keyboardBootstrapAndPrivateFileLinks: true, errors });
      await browser.close();
      browser = undefined;
    }
    const result = { result: 'passed', playwright: require('playwright/package.json').version,
      scope: 'rendered installation guidance and links; not native Docker or password changes', results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
