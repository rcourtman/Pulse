// Render actual shipped help through the production Docs/router/Markdown/CSS.
// No API, Docker, systemd or real credential is used here.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const artifacts = path.join(root, 'node_modules', 'password-recovery-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-password-recovery'),
    server: { host: '127.0.0.1', port: 5252, strictPort: true },
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
      await page.goto('http://127.0.0.1:5252/browser-tests/docs-fragment-navigation.html',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      if (tone === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
      for (const guide of ['DOCKER', 'FAQ']) {
        await page.getByRole('link', { name: '← All documentation' }).click();
        const entry = page.locator(`a[data-doc-link][href="/docs/${guide}"]`).first();
        await entry.focus();
        await page.keyboard.press('Enter');
        const link = page.getByRole('link', { name: 'password recovery guide', exact: true });
        await link.waitFor();
        await link.focus();
        await page.keyboard.press('Enter');
        await page.waitForFunction(() => {
          const h = document.getElementById('i-forgot-my-password');
          return h && h.getBoundingClientRect().top >= 0 && h.getBoundingClientRect().top < innerHeight;
        });
        assert.ok(page.url().endsWith('/docs/TROUBLESHOOTING#i-forgot-my-password'));
        const text = await page.locator('article').innerText();
        for (const phrase of ['Do not delete .env', 'setup replaces the primary API token',
          'pulse.service.d/override.conf', 'not just docker restart',
          'existing sessions and API tokens need separate review', 'If it still fails, stop'])
          assert.ok(text.includes(phrase), phrase);
        assert.ok(!text.includes('rm /data/.env') && !text.includes('rm /etc/pulse/.env'));
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        await page.screenshot({ path: path.join(artifacts, `${engine}-${guide.toLowerCase()}-recovery.png`) });
      }
      const code = page.locator('pre').filter({ hasText: 'htpasswd -nB -C 12' });
      await code.scrollIntoViewIfNeeded();
      assert.ok((await code.innerText()).includes('> "$recovery_dir/password-record"'));
      await page.screenshot({ path: path.join(artifacts, `${engine}-private-hash.png`) });
      const auth = page.getByRole('link', { name: 'authentication guide', exact: true });
      await auth.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(() => {
        const h = document.getElementById('private-docker-authentication-file');
        return h && h.getBoundingClientRect().top >= 0 && h.getBoundingClientRect().top < innerHeight;
      });
      assert.ok(page.url().endsWith('/docs/CONFIGURATION#private-docker-authentication-file'));
      assert.deepEqual(errors, []);
      results.push({ engine, browserVersion: browser.version(), width, tone,
        keyboardDockerAndFAQLinks: true, fragmentInView: true, sourcePrecedenceWarning: true,
        privateHashRecipeRendered: true, configurationLink: true, noBodyOverflow: true, errors });
      await browser.close();
      browser = undefined;
    }
    const docs = Object.fromEntries(['DOCKER', 'FAQ', 'TROUBLESHOOTING'].map((guide) => [
      `frontend-modern/public/docs/${guide}.md`,
      createHash('sha256').update(fs.readFileSync(path.join(root, 'public/docs', `${guide}.md`))).digest('hex'),
    ]));
    const result = { result: 'passed', playwrightVersion: require('playwright/package.json').version,
      scope: 'rendered recovery guidance/links; not native deployment/password/session recovery', docs, results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
