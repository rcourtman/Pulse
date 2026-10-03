// Run from the assigned root with pulse-worker-browser. This checks the real
// dialog/flow and shipped guides, not a native migration or release delivery.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const workspace = process.cwd();
  const root = path.join(workspace, 'frontend-modern');
  const output = path.join(workspace, 'tmp', 'configuration-migration');
  fs.mkdirSync(output, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    plugins: [{ name: 'migration-doc-harness', configureServer(server) {
      server.middlewares.use((req, _res, next) => {
        if (/^\/docs\/[^?]+$/.test(req.url || '') && !req.url.endsWith('.md'))
          req.url = '/browser-tests/configuration-migration.html';
        next();
      });
    } }],
    server: { host: '127.0.0.1', port: 5264, strictPort: true },
  });
  const observations = [];
  let browser;
  try {
    await server.listen();
    for (const [engine, width, height] of [['chromium', 1440, 1000], ['webkit', 390, 844]]) {
      browser = await (engine === 'chromium' ? chromium : webkit).launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const context = await browser.newContext({ viewport: { width, height }, acceptDownloads: true,
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [], operations = [];
      await context.route('**/api/**', async (route) => {
        const req = route.request();
        if (req.url().endsWith('/api/config/export')) {
          operations.push({ path: '/api/config/export', method: req.method(), body: req.postDataJSON() });
          await route.fulfill({ status: 200, contentType: 'application/json',
            body: JSON.stringify({ status: 'success', data: 'synthetic-archive-not-a-real-backup' }) });
        } else if (req.url().endsWith('/api/config/import')) {
          operations.push({ path: '/api/config/import', method: req.method(), body: req.postDataJSON() });
          // Deliberately reject this synthetic archive: warning, error and
          // cancel paths must remain available without pretending to migrate.
          await route.fulfill({ status: 400, contentType: 'text/plain', body: 'Synthetic rejected archive' });
        } else await route.fulfill({ status: 200, contentType: 'application/json',
          body: JSON.stringify({ hasAuthentication: false }) });
      });
      context.on('page', (p) => p.on('pageerror', (err) => errors.push(err.message)));
      const page = await context.newPage();
      await page.goto('http://127.0.0.1:5264/browser-tests/configuration-migration.html');
      if (engine === 'webkit') await page.evaluate(() => document.documentElement.classList.add('dark'));
      await page.getByRole('button', { name: 'Create Backup', exact: true }).click();
      const exporting = page.getByRole('dialog', { name: 'Export configuration', exact: true });
      await exporting.waitFor();
      await exporting.evaluate((el) => Promise.all(el.getAnimations({ subtree: true }).map((a) => a.finished))); 
      let text = await exporting.innerText();
      for (const phrase of ['Configuration only:', 'SSO settings', 'API-token records',
        'not history, TrueNAS/vSphere connections', 'Local login credentials and sessions are not included'])
        assert.ok(text.includes(phrase), phrase);
      assert.ok(await exporting.getByRole('button', { name: 'Export', exact: true }).isDisabled());
      const guide = exporting.getByRole('link', { name: 'migration guide', exact: true });
      await guide.scrollIntoViewIfNeeded();
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ animations: 'disabled', path: path.join(output, `${engine}-export.png`) });
      await guide.focus();
      const popupPromise = page.waitForEvent('popup');
      await page.keyboard.press('Enter');
      const docs = await popupPromise;
      await docs.locator('article').waitFor();
      const article = await docs.locator('article').innerText();
      for (const phrase of ['not a full backup', 'TrueNAS, vSphere and Machine Availability',
        'SSO configuration is included', 'Stop Pulse', 'does not merge', 'reload/apply failure'])
        assert.ok(article.includes(phrase), phrase);
      assert.ok(docs.url().endsWith('/docs/MIGRATION'));
      const scope = docs.locator('#configuration-transfer');
      await scope.scrollIntoViewIfNeeded();
      assert.ok(await docs.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await docs.screenshot({ animations: 'disabled', path: path.join(output, `${engine}-scope.png`) });
      const retarget = docs.getByRole('link', { name: 'agent retargeting guide', exact: true });
      await retarget.focus();
      await docs.keyboard.press('Enter');
      await docs.waitForFunction(() => document.activeElement?.id === 'moving-pulse-to-a-new-address');
      assert.ok((await docs.locator('article').innerText()).includes('restores API-token records, not the server-side'));
      assert.ok(docs.url().endsWith('/docs/UNIFIED_AGENT#moving-pulse-to-a-new-address'));
      await docs.close();
      await exporting.getByLabel('Encryption Passphrase', { exact: true }).fill('synthetic-migration-passphrase');
      const downloadPromise = page.waitForEvent('download');
      await exporting.getByRole('button', { name: 'Export', exact: true }).click();
      const download = await downloadPromise;
      assert.deepEqual(JSON.parse(fs.readFileSync(await download.path(), 'utf8')),
        { status: 'success', data: 'synthetic-archive-not-a-real-backup' });
      await page.getByRole('button', { name: 'Restore Configuration', exact: true }).click();
      const importing = page.getByRole('dialog', { name: 'Import configuration', exact: true });
      await importing.waitFor();
      await importing.evaluate((el) => Promise.all(el.getAnimations({ subtree: true }).map((a) => a.finished))); 
      text = await importing.innerText();
      for (const phrase of ['Back up the destination first', 'not the whole installation',
        'inventory, enrolment state, profiles and assignments are not', 'verify fresh admission'])
        assert.ok(text.includes(phrase), phrase);
      assert.ok(!text.includes('restores server-side agent records'));
      await importing.getByRole('link', { name: 'migration guide' }).scrollIntoViewIfNeeded();
      await page.screenshot({ animations: 'disabled', path: path.join(output, `${engine}-import.png`) });
      await importing.getByLabel('Backup Password', { exact: true }).fill('synthetic-migration-passphrase');
      await importing.getByLabel('Configuration File', { exact: true }).setInputFiles({ name: 'synthetic-backup.json',
        mimeType: 'application/json', buffer: Buffer.from(JSON.stringify({ data: 'synthetic-archive-not-a-real-backup' })) });
      const rejectedResponse = page.waitForResponse((r) => r.url().endsWith('/api/config/import'));
      await importing.getByRole('button', { name: 'Import', exact: true }).click();
      await rejectedResponse;
      assert.equal(operations.length, 2);
      assert.deepEqual(operations[1], { path: '/api/config/import', method: 'POST',
        body: { passphrase: 'synthetic-migration-passphrase', data: 'synthetic-archive-not-a-real-backup' } });
      assert.ok(await importing.isVisible(), 'failed import retains the dialog');
      await importing.getByRole('button', { name: 'Cancel', exact: true }).click();
      await importing.waitFor({ state: 'hidden' });
      assert.equal(operations.length, 2, 'copy/navigation/cancel never send another operation');
      assert.deepEqual(errors, []);
      observations.push({ engine, browserVersion: browser.version(), width, height,
        warningScope: true, keyboardGuideAndRetargetLinks: true, syntheticExportAndRejectedImport: true,
        noPageOverflow: true, errors });
      await browser.close();
      browser = undefined;
    }
    const files = ['frontend-modern/src/components/Settings/BackupTransferDialogs.tsx',
      'frontend-modern/public/docs/MIGRATION.md', 'frontend-modern/public/docs/UNIFIED_AGENT.md'];
    const result = { result: 'passed', playwrightVersion: require('playwright/package.json').version,
      baseSha: 'b62042798c7c9b326408dd5532d7fa330d6bb089',
      scope: 'Real dialog/flow/production Docs rendering with synthetic API; no native migration or shipment',
      contentSha256: Object.fromEntries(files.map((file) => [file,
        createHash('sha256').update(fs.readFileSync(path.join(workspace, file))).digest('hex')])), observations };
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((err) => { console.error(err); process.exitCode = 1; });
