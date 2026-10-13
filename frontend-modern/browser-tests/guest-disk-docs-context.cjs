// Exercise the shipped guidance through the production row, drawer and Docs
// renderer. Synthetic observations only; no guest request or setup action.
const fs = require('node:fs');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const {
  createPublicationFixtureServer,
} = require('/workspace/frontend-modern/browser-tests/publication-fixture-server.cjs');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-disk-docs-context';
const origin = 'http://127.0.0.1:5331';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    playwright: require('playwright/package.json').version,
    driver_sha256: hash(fs.readFileSync(__filename)),
    cases: [],
    captures: [],
    cleanup: { browser: false, server: false },
    limits:
      'Current-source presentation and guidance with safe fixtures; not PR #2893, native recovery, installed use or release acceptance.',
  };
  let server, browser;
  try {
    assert.equal(
      result.playwright,
      JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    server = createPublicationFixtureServer(root, 5331, 'browser-tests/guest-reading-help.html');
    result.compiled = server.binding;
    const docHash = hash(fs.readFileSync(root + '/public/docs/VM_DISK_MONITORING.md'));
    assert.equal(server.binding.artifacts['docs/VM_DISK_MONITORING.md'], docHash);
    assert.equal(hash(fs.readFileSync('/workspace/docs/VM_DISK_MONITORING.md')), docHash);
    result.document_sha256 = docHash;
    await server.listen();
    for (const [name, engine, width] of [
      ['chromium-desktop', chromium, 1365],
      ['webkit-phone', webkit, 390],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const context = await browser.newContext({
        viewport: { width, height: 900 },
        isMobile: width <= 768,
        hasTouch: width <= 768,
        locale: 'en-GB',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(12000);
      const record = {
        name,
        browser: browser.version(),
        width,
        errors: [],
        requests: [],
        docs: [],
      };
      result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      page.on('response', (response) => {
        if (new URL(response.url()).pathname === '/docs/VM_DISK_MONITORING.md') {
          record.docs.push(
            response.body().then((body) => ({ status: response.status(), sha256: hash(body) })),
          );
        }
      });
      await page.route('**/*', (route) => {
        const request = route.request(),
          url = new URL(request.url());
        record.requests.push({
          method: request.method(),
          path: url.pathname,
          external: url.origin !== origin,
        });
        if (url.origin !== origin) return route.abort();
        if (url.pathname.startsWith('/api/'))
          return route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
              success: true,
              data: [],
              alerts: [],
              anomalies: [],
              config: null,
            }),
          });
        return route.continue();
      });
      await page.goto(origin, { waitUntil: 'domcontentloaded' });
      await page.waitForFunction(() => window.__readingHelp);
      const disclosure = page
        .locator('[data-guest-id="fixture-pve:pve-a:101"] [data-row-action]')
        .first();
      if (width <= 768) await page.getByText('reading-guest', { exact: true }).tap();
      else {
        await disclosure.focus();
        await page.keyboard.press('Enter');
      }
      const drawer = page.getByRole('region', { name: 'Guest details' });
      await drawer.waitFor({ state: 'visible' });
      const details = drawer.getByTestId('guest-technical-details');
      assert.match(await details.innerText(), /does not prove it is absent or stopped/);
      const guidance = details.getByRole('link', { name: 'Filesystem reading guidance' });
      if (width <= 768) await guidance.tap();
      else {
        await guidance.focus();
        await page.keyboard.press('Enter');
      }
      await page
        .locator('#a-missing-reading-is-not-an-installation-diagnosis')
        .waitFor({ state: 'visible' });
      const contextLink = page.getByRole('link', {
        name: "Check the reading's context",
        exact: true,
      });
      if (width <= 768) await contextLink.tap();
      else {
        await contextLink.focus();
        await page.keyboard.press('Enter');
      }
      const heading = page.locator('#current-retained-and-unavailable-disk-readings');
      await heading.waitFor({ state: 'visible' });
      await heading.scrollIntoViewIfNeeded();
      const text = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const expected of [
        'select the disk reading indicator',
        'phone or with a keyboard',
        'presence or absence of a caption does not establish freshness',
        "same guest's explanation before treating it as an agent setup problem",
        'unknown, not zero',
        'not current free space',
        'not a monitoring-coverage check',
        'does not prove that the QEMU-only path recovered',
        'Do not bypass a guest-read pause or widen token access',
      ])
        assert.ok(text.includes(expected), `Missing rendered guidance: ${expected}`);
      assert.doesNotMatch(text, /wider rows use Last known and Unavailable/);
      record.guide = { url: page.url(), heading: await heading.innerText(), checks: 9 };
      record.layout = await page.evaluate(() => ({
        width: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      }));
      assert.ok(
        record.layout.scrollWidth <= record.layout.width + 1,
        JSON.stringify(record.layout),
      );
      const file = name + '-reading-context.png';
      await page.screenshot({ path: output + '/' + file });
      result.captures.push({ file, sha256: hash(fs.readFileSync(output + '/' + file)) });
      record.docs = await Promise.all(record.docs);
      assert.ok(record.docs.length > 0, 'Docs did not consume the changed producer');
      for (const doc of record.docs) assert.deepEqual(doc, { status: 200, sha256: docHash });
      assert.deepEqual(record.errors, []);
      assert.equal(
        record.requests.filter((request) => !['GET', 'HEAD'].includes(request.method)).length,
        0,
      );
      assert.equal(record.requests.filter((request) => request.external).length, 0);
      await context.close();
      await browser.close();
      browser = null;
    }
    result.result = 'passed';
    result.verified_at = new Date().toISOString();
  } catch (error) {
    result.error = String(error.stack || error);
    throw error;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browser = true;
    if (server) await server.close();
    result.cleanup.server = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
