// Production transfer flow/dialogs/API client/toasts; synthetic archives and
// localhost replies only. This is preflight proof, not encryption or recovery.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const staticRoot = path.join(root, 'tmp/configuration-transfer-response/static');
const output = path.join(root, 'tmp/configuration-import-limits/browser');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const limit = 1024 * 1024;
const password = 'synthetic-private-passphrase';
const wire = (data, passphrase = password) => JSON.stringify({ passphrase, data });
const result = { result: 'failed', observations: [], requests: [], captures: [], cleanup: {} };
let server, browser;

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const binding = JSON.parse(fs.readFileSync(path.join(staticRoot, 'binding.json')));
  assert.match(binding.source_sha, /^[a-f0-9]{40}$/);
  assert.ok(binding.sources['browser-tests/configuration-import-limits.cjs']);
  for (const [file, expected] of Object.entries(binding.sources))
    assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), expected, file);
  for (const [file, expected] of Object.entries(binding.artifacts))
    assert.equal(hash(fs.readFileSync(path.join(staticRoot, file))), expected, file);
  assert.ok(
    fs
      .readFileSync(
        path.join(root, 'internal/api/configapi/config_export_import_handlers.go'),
        'utf8',
      )
      .includes('http.MaxBytesReader(w, r.Body, 1024*1024)'),
    'existing server budget has not changed',
  );
  result.source_sha = binding.source_sha;
  result.binding_sha256 = hash(fs.readFileSync(path.join(staticRoot, 'binding.json')));
  result.playwright = require('playwright/package.json').version;
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  let expectedBody = '',
    reject = false;
  server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://127.0.0.1');
    if (url.pathname.startsWith('/api/')) {
      if (url.pathname === '/api/config/import') {
        const parts = [];
        req.on('data', (chunk) => parts.push(chunk));
        req.on('end', () => {
          const received = Buffer.concat(parts);
          assert.equal(req.method, 'POST');
          assert.equal(received.toString(), expectedBody);
          assert.ok(received.length <= limit, 'no oversized import reaches HTTP');
          result.requests.push({
            method: req.method,
            path: url.pathname,
            bytes: received.length,
            sha256: hash(received),
            rejected: reject,
          });
          res.setHeader('Content-Type', 'application/json');
          res.statusCode = reject ? 400 : 200;
          res.end(
            JSON.stringify(
              reject
                ? { error: 'synthetic-private-rejection' }
                : { status: 'success', message: 'Configuration imported successfully' },
            ),
          );
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
    });
    const errors = [],
      offOrigin = [],
      failures = [];
    await context.route('**/*', async (route) => {
      if (
        !route
          .request()
          .url()
          .startsWith(origin + '/')
      ) {
        offOrigin.push(route.request().url());
        await route.abort();
      } else await route.continue();
    });
    await context.addInitScript(() => {
      window.fixtureDocumentIdentity = crypto.randomUUID();
      window.fixtureReads = [];
      const read = File.prototype.text;
      File.prototype.text = function () {
        window.fixtureReads.push(this.size);
        return read.call(this);
      };
      if (innerWidth === 320)
        document.addEventListener(
          'DOMContentLoaded',
          () => document.documentElement.classList.add('dark'),
          { once: true },
        );
    });
    const page = await context.newPage();
    page.setDefaultTimeout(12000);
    page.on('pageerror', (error) => errors.push(String(error)));
    const activate = async (button) => {
      if (name === 'narrow') await button.tap();
      else {
        await button.focus();
        await page.keyboard.press('Enter');
      }
    };
    const capture = async (suffix) => {
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      const file = path.join(output, `${name}-${suffix}.png`);
      await page.screenshot({ path: file, animations: 'disabled' });
      result.captures.push({
        file: path.relative(root, file),
        sha256: hash(fs.readFileSync(file)),
      });
    };
    const open = async (content, passphrase = password) => {
      await activate(page.getByRole('button', { name: 'Restore Configuration', exact: true }));
      const dialog = page.getByRole('dialog', { name: 'Import configuration', exact: true });
      await dialog.waitFor();
      await dialog.evaluate((el) =>
        Promise.all(el.getAnimations({ subtree: true }).map((a) => a.finished)),
      );
      await dialog.getByLabel('Configuration File', { exact: true }).setInputFiles({
        name: 'synthetic-backup.json',
        mimeType: 'application/json',
        buffer: Buffer.from(content),
      });
      await dialog.getByLabel('Backup Password', { exact: true }).fill(passphrase);
      return dialog;
    };
    const rejectLocally = async (
      content,
      passphrase,
      label,
      expectedReads,
      takeCapture = false,
    ) => {
      const before = result.requests.length;
      const reads = await page.evaluate(() => window.fixtureReads.length);
      const identity = await page.evaluate(() => window.fixtureDocumentIdentity);
      const dialog = await open(content, passphrase);
      const file = dialog.getByLabel('Configuration File', { exact: true });
      assert.equal(await file.getAttribute('aria-describedby'), 'backup-import-file-help');
      assert.ok(
        (await dialog.locator('#backup-import-file-help').innerText()).includes(
          'Maximum file size: 1 MiB',
        ),
      );
      if (label === 'oversized-file') {
        await file.scrollIntoViewIfNeeded();
        await capture('file-guidance');
      }
      await activate(dialog.getByRole('button', { name: 'Import', exact: true }));
      const alert = dialog.getByRole('alert');
      await alert.waitFor();
      const feedback = await alert.innerText();
      assert.ok(feedback.includes('No import request was sent'));
      assert.ok(feedback.includes('Keep the original backup intact'));
      assert.ok(!feedback.includes('may already have changed'));
      assert.ok(!feedback.includes(passphrase));
      assert.equal(result.requests.length, before);
      assert.equal(await page.evaluate(() => window.fixtureReads.length), reads + expectedReads);
      assert.equal(
        await dialog.getByLabel('Backup Password', { exact: true }).inputValue(),
        passphrase,
      );
      assert.equal(await file.evaluate((el) => el.files[0].name), 'synthetic-backup.json');
      assert.equal(
        await page.getByRole('dialog', { name: 'API token required', exact: true }).count(),
        0,
      );
      assert.equal(
        await page.getByRole('region', { name: 'Notifications', includeHidden: true }).innerText(),
        '',
      );
      assert.equal(await page.evaluate(() => window.fixtureDocumentIdentity), identity);
      assert.ok(await dialog.getByRole('button', { name: 'Cancel', exact: true }).isEnabled());
      if (takeCapture) {
        await alert.scrollIntoViewIfNeeded();
        await capture(label);
        await page.waitForTimeout(2250);
        assert.equal(result.requests.length, before);
        assert.equal(await page.evaluate(() => window.fixtureDocumentIdentity), identity);
      }
      failures.push({
        label,
        fileBytes: Buffer.byteLength(content),
        reads: expectedReads,
        noRequest: true,
        inputsRetained: true,
        noSuccessOrReload: true,
      });
      return dialog;
    };
    const cancel = async (dialog) => {
      await activate(dialog.getByRole('button', { name: 'Cancel', exact: true }));
      await dialog.waitFor({ state: 'hidden' });
    };

    await page.goto(entry + '?require-token=1');
    await cancel(await rejectLocally('A'.repeat(limit + 1), password, 'oversized-file', 0, true));
    await page.goto(entry);
    for (const [label, content, passphrase] of [
      ['file-at-limit-request-too-large', 'A'.repeat(limit), password],
      ['request-one-byte-over', 'A'.repeat(limit - Buffer.byteLength(wire('')) + 1), password],
      [
        'multibyte-password',
        'A'.repeat(limit - Buffer.byteLength(wire('', '🔒'.repeat(12))) + 1),
        '🔒'.repeat(12),
      ],
      [
        'escaped-password',
        'A'.repeat(limit - Buffer.byteLength(wire('', '"\\'.repeat(12))) + 1),
        '"\\'.repeat(12),
      ],
      ['escaped-raw-archive', 'A\\'.repeat(Math.floor(limit / 3)), password],
    ]) {
      const dialog = await rejectLocally(
        content,
        passphrase,
        label,
        1,
        label === 'request-one-byte-over',
      );
      if (label === 'request-one-byte-over') {
        // Correcting a local rejection does not resubmit it implicitly. A later
        // rejected server reply is still uncertain, never called a local refusal.
        expectedBody = wire('synthetic-small-archive');
        reject = true;
        const before = result.requests.length;
        await dialog.getByLabel('Configuration File', { exact: true }).setInputFiles({
          name: 'corrected.json',
          mimeType: 'application/json',
          buffer: Buffer.from('{"data":"synthetic-small-archive"}'),
        });
        assert.equal(result.requests.length, before);
        await activate(dialog.getByRole('button', { name: 'Import', exact: true }));
        await dialog
          .getByRole('alert')
          .filter({ hasText: 'Settings may already have changed' })
          .waitFor();
        assert.equal(result.requests.length, before + 1);
        assert.ok(
          !(await dialog.getByRole('alert').innerText()).includes('synthetic-private-rejection'),
        );
        reject = false;
      }
      await cancel(dialog);
    }
    const accepted = [];
    for (const size of [limit - 1, limit]) {
      const data = 'A'.repeat(size - Buffer.byteLength(wire('')));
      expectedBody = wire(data);
      const dialog = await open(JSON.stringify({ status: 'success', data }));
      const before = result.requests.length;
      const identity = await page.evaluate(() => window.fixtureDocumentIdentity);
      await activate(dialog.getByRole('button', { name: 'Import', exact: true }));
      await dialog
        .getByText('Configuration imported. Reloading this page…', { exact: true })
        .waitFor();
      assert.equal(result.requests.length, before + 1);
      assert.equal(result.requests.at(-1).bytes, size);
      assert.ok(await dialog.getByRole('button', { name: 'Importing…', exact: true }).isDisabled());
      if (size === limit) await capture('exact-limit-acknowledged');
      await page.waitForFunction(
        (previous) => window.fixtureDocumentIdentity !== previous,
        identity,
      );
      await page.getByRole('button', { name: 'Restore Configuration', exact: true }).waitFor();
      accepted.push({ bytes: size, oneRequest: true, acknowledgedReload: true });
    }
    assert.deepEqual(errors, []);
    assert.deepEqual(offOrigin, []);
    result.observations.push({
      name,
      width,
      height,
      engine: browser.version(),
      failures,
      accepted,
      correctedExplicitSubmission: true,
      serverRejectionStillUncertain: true,
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
