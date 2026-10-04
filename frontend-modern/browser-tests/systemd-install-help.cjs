// Exercise the real Docs renderer/router/styles and shipped installation guide.
// This is help verification, not an installed service migration or upgrade.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'node_modules', 'systemd-install-help-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-systemd-install-help'),
    server: { host: '127.0.0.1', port: 5251, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [engine, width] of [['chromium', 1280], ['webkit', 390]]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const page = await browser.newPage({ viewport: { width, height: 1000 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      await page.goto('http://127.0.0.1:5251/browser-tests/docs-fragment-navigation.html?scenario=installation',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      const summary = page.locator('summary').filter({
        hasText: 'Manual or custom systemd services (advanced)',
      });
      await summary.waitFor();
      if (engine === 'webkit') {
        await page.evaluate(async () => {
          document.documentElement.classList.add('dark');
          await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
          await Promise.all(document.getAnimations().map((animation) => animation.finished.catch(() => {})));
        });
      }
      const details = summary.locator('..');
      assert.equal(await details.getAttribute('open'), null);
      await summary.focus();
      await page.keyboard.press('Enter');
      assert.notEqual(await details.getAttribute('open'), null);
      const content = await details.innerText();
      for (const text of ['use the signed installer above', 'User=pulse', 'Group=pulse',
        'NoNewPrivileges=true', 'ProtectSystem=strict', 'LoadState=loaded',
        'not that a root service is running', 'an empty User=',
        'the installer preserves existing service units during updates',
        'Do not overwrite a working unit, change only its service user',
        'preserve the data directory and its encryption key',
        'separate host agent has different privilege requirements'])
        assert.ok(content.includes(text), text);
      const command = await details.locator('pre').innerText();
      assert.ok(command.startsWith('systemctl show pulse.service'));
      assert.ok(command.includes('--property=LoadState'));
      assert.ok(Math.max(...command.split('\n').map((line) => line.length)) < 34);
      assert.ok(!/--property=(Environment|ExecStart)|sudo|tee|install -m/.test(command));
      assert.ok(!await page.locator('article').innerText().then((text) =>
        text.includes('sudo tee /etc/systemd/system/pulse.service')));
      await summary.evaluate((element) => element.scrollIntoView({ block: 'start' }));
      await page.screenshot({ path: path.join(artifacts, `${engine}-account.png`) });
      await details.locator('pre').evaluate((element) => element.scrollIntoView({ block: 'start' }));
      await page.screenshot({ path: path.join(artifacts, `${engine}-checks.png`) });
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      const recovery = details.getByRole('link', { name: 'the recovery guide', exact: true });
      assert.equal(await recovery.getAttribute('href'), '/docs/RECOVERY');
      await recovery.focus();
      await page.keyboard.press('Enter');
      await page.waitForURL('**/docs/RECOVERY');
      await page.getByRole('heading', { level: 1 }).filter({ hasText: 'Recovery' }).waitFor();
      assert.deepEqual(errors, []);
      results.push({ engine, version: browser.version(), width,
        keyboardDisclosure: true, nonRootServer: true, agentBoundary: true,
        readOnlyCredentialSafeCommand: true, loadedUnitRequired: true, phoneCommandFits: true,
        preservationWarning: true,
        noDocumentOverflow: true, keyboardRecoveryLink: true, errors });
      await browser.close();
      browser = undefined;
    }
    const files = ['public/docs/INSTALL.md', 'browser-tests/systemd-install-help.cjs',
      'browser-tests/docs-fragment-navigation.tsx', 'src/features/docs/docMarkdown.ts',
      'src/pages/Docs.tsx'].map((file) => ({ file,
        sha256: crypto.createHash('sha256').update(fs.readFileSync(path.join(root, file))).digest('hex') }));
    const result = { result: 'passed', playwright: require('playwright/package.json').version,
      scope: 'existing shipped installation-help rendering and navigation, not installed/systemd acceptance',
      dompurify: JSON.parse(fs.readFileSync(path.join(root, 'node_modules/dompurify/package.json'))).version,
      files, results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
