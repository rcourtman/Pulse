// Real transfer producer, dialogs, API client and toast consumer. Synthetic
// replies only: this proves response handling, not encryption/native recovery.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const dir = path.join(root, 'tmp/configuration-transfer-response');
const staticRoot = path.join(dir, 'static');
const output = path.join(dir, 'browser');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const passphrase = 'synthetic-transfer-passphrase';
const archive = 'synthetic-encrypted-archive-not-a-real-backup';
const privateSentinel = 'private-response-sentinel';
const result = { result: 'failed', observations: [], captures: [], cleanup: {} };
let server, browser;

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const binding = JSON.parse(fs.readFileSync(path.join(staticRoot, 'binding.json')));
  assert.match(binding.source_sha, /^[a-f0-9]{40}$/);
  for (const [file, expected] of Object.entries(binding.sources))
    assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), expected, file);
  for (const [file, expected] of Object.entries(binding.artifacts))
    assert.equal(hash(fs.readFileSync(path.join(staticRoot, file))), expected, file);
  result.source_sha = binding.source_sha;
  result.playwright = require('playwright/package.json').version;
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const requests = [];
  let reply;
  server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://127.0.0.1');
    if (url.pathname.startsWith('/api/')) {
      if (['/api/config/import', '/api/config/export'].includes(url.pathname)) {
        let body = '';
        req.on('data', (chunk) => {
          body += chunk;
        });
        req.on('end', () => {
          const data = JSON.parse(body);
          assert.equal(req.method, 'POST');
          assert.equal(data.passphrase, passphrase);
          if (url.pathname.endsWith('/import')) assert.equal(data.data, archive);
          requests.push({ path: url.pathname, method: req.method });
          res.setHeader('Content-Type', 'application/json');
          res.end(JSON.stringify(reply));
        });
      } else {
        res.setHeader('Content-Type', 'application/json');
        res.end(JSON.stringify({ hasAuthentication: false }));
      }
      return;
    }
    const file = path.join(staticRoot, decodeURIComponent(url.pathname));
    if (
      !file.startsWith(staticRoot + path.sep) ||
      !Object.hasOwn(binding.artifacts, path.relative(staticRoot, file))
    ) {
      res.statusCode = 404;
      return res.end();
    }
    res.setHeader(
      'Content-Type',
      file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html',
    );
    res.end(fs.readFileSync(file));
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${server.address().port}`;
  const entry = `${origin}/browser-tests/configuration-migration.html`;
  for (const [name, engine, width, height] of [
    ['desktop', chromium, 1365, 1000],
    ['narrow', webkit, 320, 1000],
  ]) {
    browser = await engine.launch(
      engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const context = await browser.newContext({
      viewport: { width, height },
      isMobile: name === 'narrow',
      hasTouch: name === 'narrow',
      acceptDownloads: true,
    });
    const offOrigin = [],
      errors = [],
      downloads = [];
    await context.route('**/*', async (route) => {
      const url = route.request().url();
      if (!url.startsWith(origin + '/') && !url.startsWith('blob:')) {
        offOrigin.push(url);
        await route.abort();
      } else await route.continue();
    });
    const page = await context.newPage();
    page.setDefaultTimeout(12000);
    page.on('pageerror', (error) => errors.push(String(error)));
    page.on('download', (download) => downloads.push(download));
    await page.goto(entry);
    if (name === 'narrow')
      await page.evaluate(() => document.documentElement.classList.add('dark'));
    const documentIdentity = await page.evaluate(() => {
      window.fixtureDocumentIdentity = crypto.randomUUID();
      return window.fixtureDocumentIdentity;
    });
    const capture = async (suffix) => {
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      const file = path.join(output, `${name}-${suffix}.png`);
      await page.screenshot({ path: file, animations: 'disabled' });
      result.captures.push({
        file: path.relative(root, file),
        sha256: hash(fs.readFileSync(file)),
      });
    };
    const open = async (kind) => {
      await page
        .getByRole('button', {
          name: kind === 'export' ? 'Create Backup' : 'Restore Configuration',
          exact: true,
        })
        .click();
      const dialog = page.getByRole('dialog', {
        name: kind === 'export' ? 'Export configuration' : 'Import configuration',
        exact: true,
      });
      await dialog.waitFor();
      await dialog.evaluate((el) =>
        Promise.all(el.getAnimations({ subtree: true }).map((a) => a.finished)),
      );
      await dialog
        .getByLabel(kind === 'export' ? 'Encryption Passphrase' : 'Backup Password', {
          exact: true,
        })
        .fill(passphrase);
      if (kind === 'import')
        await dialog.getByLabel('Configuration File', { exact: true }).setInputFiles({
          name: 'synthetic-backup.json',
          mimeType: 'application/json',
          buffer: Buffer.from(JSON.stringify({ data: archive })),
        });
      return dialog;
    };
    const failures = [];
    const malformed = [
      ['no acknowledgement', {}],
      ['null', null],
      ['array', []],
      ['error', { status: 'error', message: privateSentinel }],
      ['contradictory', { status: 'success', data: archive, error: privateSentinel }],
    ];
    for (const kind of ['export', 'import']) {
      const cases =
        kind === 'export'
          ? [
              ...malformed,
              ['missing archive', { status: 'success' }],
              ['blank archive', { status: 'success', data: ' \n\t' }],
              ['structured archive', { status: 'success', data: { secret: privateSentinel } }],
            ]
          : malformed;
      for (const [label, response] of cases) {
        reply = response;
        const dialog = await open(kind);
        const count = requests.length;
        await dialog
          .getByRole('button', { name: kind === 'export' ? 'Export' : 'Import', exact: true })
          .click();
        const alert = dialog.getByRole('alert');
        await alert.waitFor();
        assert.ok(
          (await alert.innerText()).includes(
            kind === 'export' ? 'Could not export' : 'Settings may already have changed',
          ),
        );
        assert.ok(!(await page.locator('body').innerText()).includes(privateSentinel));
        assert.equal(requests.length, count + 1);
        assert.equal(downloads.length, 0);
        assert.equal(
          await dialog
            .getByLabel(kind === 'export' ? 'Encryption Passphrase' : 'Backup Password', {
              exact: true,
            })
            .inputValue(),
          passphrase,
        );
        if (kind === 'import')
          assert.equal(
            await dialog
              .getByLabel('Configuration File', { exact: true })
              .evaluate((el) => el.files[0].name),
            'synthetic-backup.json',
          );
        // The dialog deliberately makes background toast siblings inert. Check
        // their contents too, rather than overlooking concealed success feedback.
        assert.equal(
          await page
            .getByRole('region', { name: 'Notifications', includeHidden: true })
            .innerText(),
          '',
        );
        assert.equal(await page.evaluate(() => window.fixtureDocumentIdentity), documentIdentity);
        assert.ok(await dialog.getByRole('button', { name: 'Cancel', exact: true }).isEnabled());
        failures.push({
          kind,
          reply: label,
          oneRequest: true,
          inputsPreserved: true,
          noSuccessOrDownload: true,
        });
        if (label === 'contradictory') {
          if (kind === 'import') {
            await page.waitForTimeout(2250);
            assert.equal(
              await page.evaluate(() => window.fixtureDocumentIdentity),
              documentIdentity,
            );
            assert.equal(requests.length, count + 1, 'no automatic replay');
          }
          await alert.scrollIntoViewIfNeeded();
          await capture(`${kind}-unconfirmed`);
        }
        // Native keyboard cancellation remains available after a bad response.
        await dialog.getByRole('button', { name: 'Cancel', exact: true }).focus();
        await page.keyboard.press('Enter');
        await dialog.waitFor({ state: 'hidden' });
      }
    }
    // An explicit later export can succeed; download only the archive contract.
    reply = { status: 'success', data: archive, privateExtra: privateSentinel };
    const exporting = await open('export');
    await exporting.getByRole('button', { name: 'Export', exact: true }).click();
    await exporting.waitFor({ state: 'hidden' });
    await page.getByText('Configuration exported successfully', { exact: true }).waitFor();
    assert.equal(downloads.length, 1);
    assert.deepEqual(JSON.parse(fs.readFileSync(await downloads[0].path(), 'utf8')), {
      status: 'success',
      data: archive,
    });
    assert.ok(!(await page.locator('body').innerText()).includes(privateSentinel));
    assert.equal(await page.evaluate(() => window.fixtureDocumentIdentity), documentIdentity);
    // A fresh, acknowledged import uses the ordinary owned reload once.
    reply = {
      status: 'success',
      message: 'Configuration imported successfully',
      remoteAgentEndpoints: { reconfigured: false, actionRequiredIfServerAddressChanged: true },
    };
    const importing = await open('import');
    await importing.getByRole('button', { name: 'Import', exact: true }).click();
    await importing
      .getByText('Configuration imported. Reloading this page…', { exact: true })
      .waitFor();
    assert.ok(
      await importing.getByRole('button', { name: 'Importing…', exact: true }).isDisabled(),
    );
    await capture('import-acknowledged');
    await page.waitForFunction(() => !window.fixtureDocumentIdentity);
    await page.getByRole('button', { name: 'Restore Configuration', exact: true }).waitFor();
    assert.equal(
      requests.length,
      (name === 'desktop' ? 1 : 2) * 15,
      '13 malformed + 2 acknowledged operations per viewport',
    );
    assert.deepEqual(errors, []);
    assert.deepEqual(offOrigin, []);
    result.observations.push({
      name,
      width,
      height,
      browser: browser.version(),
      failures,
      validExportDownload: true,
      extraFieldsExcluded: true,
      acknowledgedImportReload: true,
      noImplicitReplay: true,
      noPageErrors: true,
      noOffOriginRequests: true,
    });
    await context.close();
    await browser.close();
    browser = null;
  }
  result.result = 'passed';
})()
  .catch((error) => {
    result.failure = String(error.stack || error);
    process.exitCode = 1;
  })
  .finally(async () => {
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    if (server) await new Promise((resolve) => server.close(resolve));
    result.cleanup.serverClosed = true;
    result.verified_at = new Date().toISOString();
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  });
