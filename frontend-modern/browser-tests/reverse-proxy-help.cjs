// Real shipped Docs/router/CSS, with no backend, proxy or real credential.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const workspace = process.cwd();
  const root = path.join(workspace, 'frontend-modern');
  const output = path.join(workspace, 'tmp', 'reverse-proxy-help');
  fs.mkdirSync(output, { recursive: true });
  const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json'), 'utf8'));
  assert.match(binding.sourceSha, /^[0-9a-f]{40}$/);
  for (const [file, digest] of Object.entries(binding.contentSha256)) {
    assert.equal(
      createHash('sha256').update(fs.readFileSync(path.join(workspace, file))).digest('hex'),
      digest,
      file,
    );
  }
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-reverse-proxy-help'),
    server: { host: '127.0.0.1', port: 5268, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  const observations = [];
  let browser;
  const inViewport = (id) => {
    const heading = document.getElementById(id);
    return heading && heading.getBoundingClientRect().top >= 0 &&
      heading.getBoundingClientRect().top < innerHeight;
  };
  try {
    await server.listen();
    for (const [engine, width, height, tone] of [
      ['chromium', 1280, 1000, 'light'], ['webkit', 390, 844, 'dark'],
    ]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const page = await browser.newPage({
        viewport: { width, height },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}),
      });
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      await page.goto('http://127.0.0.1:5268/browser-tests/docs-fragment-navigation.html');
      if (tone === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
      await page.getByRole('link', { name: '← All documentation' }).click();
      const entry = page.locator('a[data-doc-link][href="/docs/REVERSE_PROXY"]').first();
      await entry.focus();
      await page.keyboard.press('Enter');
      const heading = page.locator('#before-configuring-the-proxy');
      await heading.waitFor();
      await heading.scrollIntoViewIfNeeded();
      let text = await page.locator('article').innerText();
      for (const phrase of [
        'configured immediate peer', 'PULSE_TRUSTED_PROXY_CIDRS=127.0.0.1/32',
        'Wildcard ranges', 'rejected at startup', 'not an authentication bypass',
        'Adding the header alone is not sufficient',
      ]) assert.ok(text.includes(phrase), phrase);
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(output, `${engine}-trust.png`) });
      const nginx = page.locator('pre').filter({ hasText: 'proxy_set_header X-Forwarded-For $remote_addr;' });
      await nginx.scrollIntoViewIfNeeded();
      const copied = await nginx.innerText();
      assert.ok(copied.includes('proxy_set_header X-Forwarded-Proto $scheme;'));
      assert.ok(copied.includes('proxy_set_header Forwarded "";'));
      assert.ok(!copied.includes('$proxy_add_x_forwarded_for'));
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(output, `${engine}-nginx.png`) });
      const configuration = page.getByRole('link', { name: 'configuration overrides', exact: true });
      await configuration.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(inViewport, 'common-overrides-environment-variables');
      text = await page.locator('article').innerText();
      assert.ok(text.includes('forwarded client IP, scheme, host and port'));
      const back = page.getByRole('link', { name: 'Reverse Proxy', exact: true });
      await back.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(inViewport, 'before-configuring-the-proxy');
      assert.ok(page.url().endsWith('/docs/REVERSE_PROXY#before-configuring-the-proxy'));
      const authentication = page.getByRole('link', { name: 'proxy authentication guide', exact: true });
      await authentication.focus();
      await page.keyboard.press('Enter');
      const boundary = page.getByRole('heading', { name: '⚠️ Header Trust Boundary', exact: true });
      await boundary.waitFor();
      assert.ok(page.url().endsWith('/docs/PROXY_AUTH'));
      await boundary.scrollIntoViewIfNeeded();
      assert.ok((await page.locator('article').innerText()).includes('never append to them'));
      assert.deepEqual(errors, []);
      observations.push({
        engine, width, height, tone, browserVersion: browser.version(),
        trustWarningRendered: true, safeNginxBlockRendered: true,
        keyboardConfigurationRoundTrip: true, keyboardAuthenticationBoundary: true,
        noPageOverflow: true, errors,
      });
      await browser.close();
      browser = undefined;
    }
    const result = {
      result: 'passed', playwrightVersion: require('playwright/package.json').version,
      sourceSha: binding.sourceSha, verifiedAt: new Date().toISOString(),
      contentSha256: binding.contentSha256, dependencySnapshot: binding.dependencySnapshot,
      scope: 'Production Docs rendering/navigation; not native proxy configuration, TLS or SSO acceptance',
      observations,
    };
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
