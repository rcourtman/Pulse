// Verify shipped setup guidance in the production Docs/router/Markdown/CSS.
// No backend, Docker daemon, credentials or container startup is exercised.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const artifacts = path.join(root, 'node_modules', 'private-auth-help-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  // Tailwind resolves its config/content from cwd, as the normal npm build
  // does. The tool is still invoked from the assigned repository root.
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-private-auth-help'),
    server: { host: '127.0.0.1', port: 5249, strictPort: true },
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
      const page = await browser.newPage({ viewport: { width, height: 900 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => { if (message.type() === 'error') errors.push(message.text()); });
      await page.goto('http://127.0.0.1:5249/browser-tests/docs-fragment-navigation.html',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      if (tone === 'dark') await page.evaluate(async () => {
        document.documentElement.classList.add('dark');
        await new Promise((resolve) => requestAnimationFrame(resolve));
      });
      await page.getByRole('link', { name: '← All documentation' }).click();
      await page.getByRole('link', { name: 'Configure Pulse', exact: true }).click();
      const privateHeading = page.getByRole('heading', { name: 'Private Docker authentication file', exact: true });
      await privateHeading.waitFor();
      let text = await page.locator('article').innerText();
      for (const phrase of ['complete bcrypt hash', 'no $$ substitution', 'Do not source the file',
        'Deployment-supplied environment values take precedence', 'not out of Docker\'s',
        'does not scrub its original value', 'this new-container example is not an upgrade procedure'])
        assert.ok(text.includes(phrase), phrase);
      assert.ok(!text.includes('secret123'));
      // The production Docs sanitizer removes Markdown language classes.
      const shell = await page.locator('pre code').allTextContents();
      assert.ok(shell.some((block) => block.includes('--env-file "$HOME/.config/pulse/docker-auth.env"')));
      assert.ok(shell.every((block) => !/-e\s+PULSE_AUTH_(PASS|USER)=/.test(block)));
      const firstLogin = page.getByRole('link', { name: 'First login', exact: true });
      await firstLogin.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(() => {
        const h = document.getElementById('step-1-get-the-token');
        return h && h.getBoundingClientRect().top >= 0 && h.getBoundingClientRect().top < innerHeight;
      });
      assert.ok(page.url().endsWith('/docs/INSTALL#step-1-get-the-token'));
      await page.goBack();
      await privateHeading.waitFor();
      await privateHeading.evaluate((h) => h.scrollIntoView({ block: 'start' }));
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-private-file.png`) });
      const launchCode = page.locator('pre').filter({ hasText: 'docker run -d' });
      await launchCode.scrollIntoViewIfNeeded();
      await page.screenshot({ path: path.join(artifacts, `${engine}-docker-credentials.png`) });

      await page.getByRole('link', { name: '← All documentation' }).click();
      await page.getByRole('link', { name: 'Docker and Podman', exact: true }).click();
      await page.getByRole('heading', { name: '📦 Docker Compose', exact: true }).waitFor();
      text = await page.locator('article').innerText();
      assert.ok(!text.includes('secret123'));
      assert.ok(text.includes('Leave authentication overrides unset'));
      assert.ok(text.includes('remain in the container environment and deployment file'));
      const privateLink = page.getByRole('link', { name: 'private Docker authentication file', exact: true });
      await privateLink.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(() => {
        const h = document.getElementById('private-docker-authentication-file');
        return h && h.getBoundingClientRect().top >= 0 && h.getBoundingClientRect().top < innerHeight;
      });
      assert.ok(page.url().endsWith('/docs/CONFIGURATION#private-docker-authentication-file'));

      for (const [locale, label, heading, safeguard] of [
        ['de', 'Deutsch', 'Erste Anmeldung', 'Verwende kein gemeinsames Beispielpasswort'],
        ['es', 'Español', 'Primer inicio de sesión', 'No uses una contraseña de ejemplo compartida'],
      ]) {
        await page.getByRole('link', { name: '← All documentation' }).click();
        const link = page.getByRole('link', { name: label, exact: true });
        await link.focus();
        await page.keyboard.press('Enter');
        await page.getByRole('heading', { name: heading, exact: true }).waitFor();
        text = await page.locator('article').innerText();
        assert.ok(!text.includes('secret123'));
        assert.ok(!text.includes('PULSE_AUTH_PASS='));
        assert.ok(text.includes('docker exec pulse /app/pulse bootstrap-token'));
        assert.ok(text.includes(safeguard));
        const compose = page.locator('pre').filter({ hasText: 'services:' });
        await compose.scrollIntoViewIfNeeded();
        assert.ok(!(await compose.innerText()).includes('PULSE_AUTH_'));
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        await page.screenshot({ path: path.join(artifacts, `${engine}-${locale}-bootstrap.png`) });
        const target = page.locator('a[href="/docs/CONFIGURATION#private-docker-authentication-file"]');
        await target.focus();
        await page.keyboard.press('Enter');
        await privateHeading.waitFor();
        assert.ok(page.url().endsWith('/docs/CONFIGURATION#private-docker-authentication-file'));
      }
      assert.deepEqual(errors, []);
      results.push({ engine, version: browser.version(), width, tone, firstLoginKeyboard: true,
        privateFileRendering: true, noCommandCredential: true, envVisibilityWarning: true,
        dockerFragmentKeyboard: true, translatedBootstrap: ['de', 'es'], errors });
      await browser.close();
      browser = undefined;
    }
    const result = { result: 'passed', playwright: require('playwright/package.json').version,
      scope: 'shipped setup help and links, not native Docker or password changes', results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
