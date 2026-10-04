// Inspect the existing shipped help using the production Docs renderer.
// This creates no token, authenticates no client and performs no rotation.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'node_modules', 'token-guidance-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-token-guidance'),
    server: { host: '127.0.0.1', port: 5253, strictPort: true },
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
      await page.goto('http://127.0.0.1:5253/browser-tests/docs-fragment-navigation.html?scenario=configuration',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      const tokens = page.getByRole('heading', { name: '🔑 API Tokens', exact: true });
      await tokens.waitFor();
      await page.waitForFunction(() => document.activeElement?.id === '-api-tokens');
      if (engine === 'webkit') {
        await page.evaluate(async () => {
          document.documentElement.classList.add('dark');
          await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
          await Promise.all(document.getAnimations().map((animation) => animation.finished.catch(() => {})));
        });
      }
      const presetTable = page.getByRole('table').filter({ has: page.getByRole('columnheader',
        { name: 'Preset', exact: true }) });
      const agent = presetTable.getByRole('row').filter({ has: page.getByRole('cell',
        { name: 'Agent', exact: true }) });
      assert.equal(await agent.getByRole('cell').nth(1).innerText(), 'agent:report, agent:config:read');
      assert.equal(await presetTable.getByRole('row').count(), 8);
      const scopesTable = page.getByRole('table').filter({ has: page.getByRole('columnheader',
        { name: 'Scope', exact: true }) });
      assert.equal(await scopesTable.getByRole('row').count(), 17);
      for (const table of [presetTable, scopesTable]) {
        assert.ok(await table.evaluate((element) => {
          const parent = element.parentElement;
          return parent.dataset.docTableScroll !== undefined &&
            getComputedStyle(parent).overflowX === 'auto' &&
            [...element.querySelectorAll('thead th')].every((header) => header.scope === 'col');
        }));
      }
      const body = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const text of ['Settings → API Access', 'Reporting is not remote execution',
        'not to fix a missing reading', 'revoke it promptly', 'even if that interrupts monitoring',
        'Magic Kiosk Link', 'Copy Link', 'no administrator session', 'bearer credential',
        'initial request already carried the token', 'proxy/server logs',
        'Do not rely on this session surviving']) assert.ok(body.includes(text), text);
      assert.ok(!body.includes('?token=YOUR_TOKEN_HERE'));
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      for (const [name, heading] of [
        ['rotation', tokens],
        ['presets', page.getByRole('heading', { name: 'Presets', exact: true })],
        ['kiosk', page.getByRole('heading', { name: 'Kiosk Mode', exact: true })],
      ]) {
        await heading.evaluate((element) => element.scrollIntoView({ block: 'start' }));
        await page.screenshot({ path: path.join(artifacts, `${engine}-${name}.png`) });
      }
      for (const [name, href, heading, id] of [
        ['private header-file procedure', '/docs/API#api-token-recommended',
          'API Token (Recommended)', 'api-token-recommended'],
        ['agent retargeting', '/docs/UNIFIED_AGENT#moving-pulse-to-a-new-address',
          'Moving Pulse to a new address', 'moving-pulse-to-a-new-address'],
      ]) {
        const link = page.getByRole('link', { name, exact: true });
        assert.equal(await link.getAttribute('href'), href);
        await link.focus();
        await page.keyboard.press('Enter');
        await page.waitForURL(`**${href}`);
        await page.getByRole('heading', { name: heading, exact: true }).waitFor();
        assert.equal(await page.evaluate(() => document.activeElement?.id), id);
        await page.goBack();
        await tokens.waitFor();
      }
      assert.deepEqual(errors, []);
      results.push({ engine, version: browser.version(), width,
        tokenAnchorFocus: true, leastPrivilegeAgentRow: true, actualPresetCount: 7,
        scopeCount: 16, semanticContainedTables: true, noDocumentOverflow: true,
        kioskCredentialWarning: true, keyboardApiHelp: true, keyboardAgentRetargeting: true, errors });
      await browser.close();
      browser = undefined;
    }
    const files = ['public/docs/CONFIGURATION.md', 'browser-tests/token-guidance.cjs',
      'browser-tests/docs-fragment-navigation.tsx', 'src/features/docs/docMarkdown.ts',
      'src/pages/Docs.tsx'].map((file) => ({ file,
        sha256: crypto.createHash('sha256').update(fs.readFileSync(path.join(root, file))).digest('hex') }));
    const result = { result: 'passed', playwright: require('playwright/package.json').version,
      scope: 'shipped token guidance rendering and keyboard help links; no token/native operation performed',
      dompurify: JSON.parse(fs.readFileSync(path.join(root, 'node_modules/dompurify/package.json'))).version,
      files, results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
