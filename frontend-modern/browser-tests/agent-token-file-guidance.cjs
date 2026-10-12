// Real Docs/router/Markdown/CSS, with no backend or credential access.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules', 'agent-token-file-guidance-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-agent-token-file-guidance'),
    server: { host: '127.0.0.1', port: 5251, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  let browser;
  const results = [];
  try {
    await server.listen();
    for (const [engine, width, dark] of [['chromium', 1280, false], ['webkit', 390, true]]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const page = await browser.newPage({ viewport: { width, height: 900 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [];
      const backendRequests = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      await page.route('**/api/**', async (route) => {
        backendRequests.push({ method: route.request().method(), pathname: new URL(route.request().url()).pathname });
        await route.abort();
      });
      await page.goto('http://127.0.0.1:5251/browser-tests/docs-fragment-navigation.html', {
        waitUntil: 'domcontentloaded', timeout: 60000,
      });
      await page.evaluate(() => {
        history.pushState({}, '', '/docs/UNIFIED_AGENT#private-file-installation-linux-macos-and-nas');
        dispatchEvent(new PopStateEvent('popstate'));
      });
      if (dark) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const waitForHeading = async (id) => {
        await page.waitForFunction((target) => {
          const heading = document.getElementById(target);
          const rect = heading?.getBoundingClientRect();
          return heading && document.activeElement === heading && rect.top >= 0 && rect.top < innerHeight;
        }, id);
      };
      await waitForHeading('private-file-installation-linux-macos-and-nas');
      const link = page.getByRole('link', { name: 'token-file format guidance', exact: true });
      await link.focus();
      await page.keyboard.press('Enter');
      await waitForHeading('token-file-format-is-not-shell-syntax');
      assert.ok(page.url().endsWith('/docs/UNIFIED_AGENT#token-file-format-is-not-shell-syntax'));
      await page.waitForFunction(() => document.getAnimations().length === 0);
      const table = page.getByRole('table').filter({ has: page.getByRole('columnheader', { name: 'File form', exact: true }) });
      assert.equal(await table.getByRole('row').count(), 5);
      assert.deepEqual(await table.getByRole('columnheader').allTextContents(), ['File form', 'What the agent reads']);
      for (const phrase of ['The quotes remain part', 'The prefix remains part', 'format and cause remain unknown']) {
        assert.ok((await table.innerText()).includes(phrase), phrase);
      }
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-file-form.png`) });
      const safeguard = page.getByText('Use how the file was saved or an existing private observation.', { exact: false });
      await safeguard.scrollIntoViewIfNeeded();
      const text = (await safeguard.innerText()).replace(/\s+/g, ' ');
      assert.ok(text.includes('not the value, length, hint, screenshot or file'));
      assert.ok(text.includes('Do not print, source, upload or edit'));
      assert.ok(text.includes('not proof of accepted reporting'));
      await page.screenshot({ path: path.join(artifacts, `${engine}-private-evidence.png`) });
      assert.deepEqual(backendRequests, []);
      assert.deepEqual(errors, []);
      results.push({ engine, version: browser.version(), width, dark,
        keyboardFormatLink: true, focusedVisibleHeading: true, fourFormsAndHeaders: true,
        privateUnknownEvidence: true, noHorizontalPageOverflow: true, backendRequests, errors });
      await browser.close();
      browser = undefined;
    }
    const result = { result: 'passed', playwright: require('playwright/package.json').version,
      document_sha256: crypto.createHash('sha256').update(fs.readFileSync(path.join(root, 'public/docs/UNIFIED_AGENT.md'))).digest('hex'),
      scope: 'shipped help and keyboard navigation; not credential hand-off, authentication or installed reporting', results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
