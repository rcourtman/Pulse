// Real PBS table, inline drawer and CSS on synthetic resources. No external navigation.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL } = require('node:url');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/pbs-web-interface-proof';
const origin = 'http://127.0.0.1:5337';
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const result = {
  result: 'incomplete',
  playwright: require('playwright/package.json').version,
  binding: JSON.parse(fs.readFileSync(output + '/binding.json')),
  sources: {},
  cases: [],
  captures: [],
  cleanup: {},
  limits:
    'Production PBS table, shared URL control, inline drawer and CSS in a synthetic fixture. Link activation is intercepted before navigation. No live PBS, guest, full application shell, installed release or reporter acceptance.',
};
assert.equal(
  result.playwright,
  JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
    'node_modules/@playwright/test'
  ].version,
);
for (const [file, digest] of Object.entries(result.binding.inputs)) {
  assert.equal(hash('/workspace/' + file), digest);
}

(async () => {
  let server;
  let browser;
  try {
    const { createServer } = await import(
      pathToFileURL(path.join(root, 'node_modules/vite/dist/node/index.js'))
    );
    const { default: solid } = await import(
      pathToFileURL(path.join(root, 'node_modules/vite-plugin-solid/dist/esm/index.mjs'))
    );
    const { default: tailwind } = await import(
      pathToFileURL(path.join(root, 'node_modules/@tailwindcss/vite/dist/index.mjs'))
    );
    server = await createServer({
      root,
      configFile: false,
      envFile: false,
      publicDir: false,
      plugins: [
        {
          name: 'pbs-proof-source-observation',
          enforce: 'pre',
          transform(_code, id) {
            const file = id.split('?')[0];
            if (file.startsWith(root + '/src/') && fs.existsSync(file)) {
              result.sources[path.relative('/workspace', file)] = hash(file);
            }
          },
        },
        solid(),
        tailwind(),
      ],
      resolve: { alias: { '@': root + '/src' } },
      optimizeDeps: { noDiscovery: true, esbuildOptions: { target: 'esnext' } },
      server: { host: '127.0.0.1', port: 5337, strictPort: true, hmr: false },
    });
    await server.listen();
    for (const [name, engine, width, height, dark] of [
      ['desktop', chromium, 1365, 900, false],
      ['phone', webkit, 390, 844, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const context = await browser.newContext({
        viewport: { width, height },
        isMobile: width < 768,
        hasTouch: width < 768,
        locale: 'en-GB',
        reducedMotion: 'reduce',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      const record = {
        name,
        width,
        height,
        dark,
        browser: browser.version(),
        reads: [],
        errors: [],
      };
      result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        assert.equal(url.origin, origin, 'no external request');
        if (!url.pathname.startsWith('/api/')) return route.continue();
        assert.equal(route.request().method(), 'GET', 'no mutation');
        assert(!/diagnostics|guest-agent|qemu|\/run/.test(url.pathname), 'no live probe');
        record.reads.push(url.pathname);
        return route.fulfill({
          json: { success: true, data: [], alerts: [], anomalies: [], config: null },
        });
      });
      await page.addInitScript((dark) => {
        window.__pbsActivations = [];
        document.addEventListener(
          'click',
          (event) => {
            const anchor = event.target.closest('a[target="_blank"]');
            if (anchor) {
              event.preventDefault();
              window.__pbsActivations.push(anchor.getAttribute('href'));
            }
          },
          true,
        );
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.toggle('dark', dark),
        );
      }, dark);
      await page.goto(origin + '/browser-tests/pbs-web-interface.html', {
        waitUntil: 'domcontentloaded',
        timeout: 45000,
      });
      const table = page.locator('[data-proxmox-backups-table="servers"]');
      const east = table.locator('a[href="https://east.example:8007/proxy/"]').first();
      await east.waitFor();
      assert.equal(await table.getByRole('link').count(), 6);
      assert.equal(await table.locator('a[href="http://[2001:db8::2]:8007/"]').count(), 2);
      assert.equal(await table.locator('a[href*="WRONG-HOST"]').count(), 0);
      assert.equal(
        await table
          .getByRole('img', { name: 'Web interface URL for unsafe-url is invalid' })
          .count(),
        2,
      );
      const row = east.locator('xpath=ancestor::tr');
      const identity = await row.elementHandle();
      const label = row.locator('span[title="pbs-main"]');
      const capture = async (state) => {
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: output + '/' + file, fullPage: true });
        result.captures.push({ file, sha256: hash(output + '/' + file) });
      };
      record.geometry = await label.evaluate((element) => ({
        visible: element.clientWidth,
        full: element.scrollWidth,
      }));
      assert(record.geometry.visible > 0 && record.geometry.full <= record.geometry.visible + 1);
      const box = await east.boundingBox();
      const cellBox = await row.locator('td').first().boundingBox();
      assert(box.width >= 24 && box.height >= 24, 'shared target remains usable');
      assert(box.x >= cellBox.x && box.x + box.width <= cellBox.x + cellBox.width + 1);
      assert.equal(await east.getAttribute('rel'), 'noopener noreferrer');
      assert.equal(await east.getAttribute('target'), '_blank');
      // Real Tab order and Enter, not a synthetic click standing in for keyboard activation.
      for (let step = 0; step < 30; step++) {
        await page.keyboard.press('Tab');
        if (await east.evaluate((element) => element === document.activeElement)) break;
      }
      assert.equal(
        await page.locator(':focus').getAttribute('href'),
        'https://east.example:8007/proxy/',
      );
      await capture('keyboard-link');
      await page.keyboard.press('Enter');
      if (width < 768) await east.tap();
      else await east.click();
      assert.deepEqual(await page.evaluate(() => window.__pbsActivations), [
        'https://east.example:8007/proxy/',
        'https://east.example:8007/proxy/',
      ]);
      assert.equal(await page.locator('[data-inline-platform-resource-detail-for]').count(), 0);
      if (width < 768) {
        // Shared platform rows use the full row touch target on phones.
        await label.tap();
      } else {
        const toggle = row.getByRole('button', { name: 'Expand details for pbs-main' });
        await toggle.focus();
        await page.keyboard.press('Space');
      }
      await page.locator('[data-inline-platform-resource-detail-for="pbs-east"]').waitFor();
      assert.equal(await page.locator('[data-inline-platform-resource-detail-for]').count(), 1);
      await capture('same-server-details');
      await label.click();
      await page.evaluate(() => {
        window.__pbsWeb.reverse();
        window.__pbsWeb.update('pbs-east', 'javascript:alert(1)');
      });
      await page.waitForFunction(
        () => !document.querySelector('a[href="https://east.example:8007/proxy/"]'),
      );
      assert.equal(
        await table.getByRole('img', { name: 'Web interface URL for pbs-main is invalid' }).count(),
        2,
      );
      assert.equal(await table.locator('a[href="http://[2001:db8::2]:8007/"]').count(), 2);
      assert(await identity.evaluate((element) => element.isConnected));
      await page.evaluate(() => window.__pbsWeb.update('pbs-east', ''));
      await page.waitForFunction(
        () =>
          ![...document.querySelectorAll('[role="img"]')].some(
            (element) =>
              element.getAttribute('aria-label') === 'Web interface URL for pbs-main is invalid',
          ),
      );
      assert.equal(await identity.evaluate((element) => element.querySelectorAll('a').length), 0);
      await page.evaluate(() =>
        window.__pbsWeb.update('pbs-east', ' https://edited.example:8443/pbs/ '),
      );
      await table.locator('a[href="https://edited.example:8443/pbs/"]').first().waitFor();
      assert.equal(await table.locator('a[href="https://edited.example:8443/pbs/"]').count(), 2);
      await capture('edited-url');
      if (width < 768) {
        await page.setViewportSize({ width: 320, height });
        await page.waitForTimeout(100);
        const narrow = await table
          .locator('a[href="https://edited.example:8443/pbs/"]')
          .first()
          .boundingBox();
        assert(narrow.width >= 24 && narrow.height >= 24);
        await capture('narrow-320');
      }
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      assert.deepEqual(record.errors, []);
      record.checks = [
        'service-owned targets',
        'invalid/unset/revoked targets',
        'Tab/Enter and pointer/touch',
        'separate real drawer disclosure',
        'same keyed rows after reordering',
        'no horizontal overflow',
      ];
      await context.close();
      await browser.close();
      browser = undefined;
      record.closed = true;
    }
    assert.equal(
      result.sources['frontend-modern/src/features/proxmox/ProxmoxBackupServersTable.tsx'],
      result.binding.inputs['frontend-modern/src/features/proxmox/ProxmoxBackupServersTable.tsx'],
    );
    result.result = 'passed';
  } catch (error) {
    result.error = error.message;
    throw error;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browser_closed = true;
    if (server) await server.close();
    result.cleanup.server_closed = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
