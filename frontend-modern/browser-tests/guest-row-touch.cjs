const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-row-touch';
const origin = 'http://127.0.0.1:5313';
const runtime = [
  'frontend-modern/src/components/Workloads/GuestRow.tsx',
  'frontend-modern/src/components/shared/Table.tsx',
];
const sha256 = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    playwright: require('playwright/package.json').version,
    runtime_hashes: Object.fromEntries(runtime.map((file) => [file, sha256('/workspace/' + file)])),
    results: [],
    screenshots: [],
    result: 'incomplete',
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: output + '/cache',
    server: { host: '127.0.0.1', port: 5313, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark, kinds] of [
      ['webkit-phone', webkit, 390, true, ['qemu', 'lxc']],
      ['chromium-desktop', chromium, 1365, false, ['qemu', 'lxc']],
      ['chromium-phone', chromium, 390, false, ['qemu']],
      ['webkit-narrow', webkit, 360, false, ['lxc']],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const context = await browser.newContext({
        viewport: { width, height: width <= 768 ? 844 : 900 },
        isMobile: width <= 768,
        hasTouch: width <= 768,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      for (const kind of kinds) {
        const page = await context.newPage();
        page.setDefaultTimeout(15000);
        page.setDefaultNavigationTimeout(60000);
        const record = {
          name,
          kind,
          browser: browser.version(),
          viewport: { width, height: width <= 768 ? 844 : 900 },
          dark,
          errors: [],
          requests: [],
          events: [],
          checks: [],
        };
        result.results.push(record);
        page.on('pageerror', (error) => record.errors.push(error.message));
        await page.route('**/*', (route) => {
          const request = route.request();
          const url = new URL(request.url());
          record.requests.push({
            method: request.method(),
            origin: url.origin,
            path: url.pathname,
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
        await page.addInitScript((isDark) => {
          window.__inputEvents = [];
          for (const type of ['pointerdown', 'pointerup', 'touchstart', 'touchend', 'click'])
            document.addEventListener(
              type,
              (event) =>
                window.__inputEvents.push({
                  type,
                  tag: event.target.tagName,
                  trusted: event.isTrusted,
                  pointerType: event.pointerType,
                }),
              true,
            );
          document.addEventListener('DOMContentLoaded', () => {
            if (isDark) document.documentElement.classList.add('dark');
          });
        }, dark);
        await page.goto(`${origin}/browser-tests/guest-row-touch.html?kind=${kind}`);
        await page.waitForFunction(() => window.__guestRowTouch);
        assert.equal(await page.evaluate(() => innerWidth), width);
        const row = page.locator('[data-guest-id]');
        const nameTarget = row.getByText('backup-guest', { exact: true });
        const drawer = page.getByRole('region', { name: 'Guest details' });
        const disclosure = row.locator('[data-row-action]');
        await page.evaluate(() => {
          window.__originalGuestRow = document.querySelector('[data-guest-id]');
        });
        const state = () =>
          page.evaluate(() => ({
            count: window.__guestRowTouch.actionCount(),
            expanded: window.__guestRowTouch.expanded(),
            closeCount: window.__guestRowTouch.closeCount(),
            disclosureExpanded: document
              .querySelector('[data-row-action]')
              ?.getAttribute('aria-expanded'),
            sameRow: document.querySelector('[data-guest-id]') === window.__originalGuestRow,
          }));
        const activate = async (target) => {
          if (width <= 768) await target.tap();
          else await target.click();
          await page.waitForTimeout(650);
        };
        const expectState = async (count, expanded) => {
          const value = await state();
          record.state = value;
          record.events = await page.evaluate(() => window.__inputEvents);
          assert.equal(value.count, count, `${name}/${kind}: single row activation`);
          assert.equal(value.expanded, expanded);
          assert.equal(value.disclosureExpanded, String(expanded));
          assert.equal(value.sameRow, true);
        };
        try {
          await activate(width === 360 ? row : nameTarget);
          await expectState(1, true);
          assert(record.events.some((event) => event.type === 'click' && event.trusted));
          if (width <= 768) {
            assert(record.events.some((event) => event.type === 'touchend' && event.trusted));
            assert.equal(await row.getAttribute('data-history-lens-active'), null);
          }
          await drawer.getByText('No completed backup found', { exact: true }).waitFor();
          await drawer.getByText('Running · not completed yet', { exact: true }).waitFor();
          if (kind === 'qemu')
            await drawer.getByText(/Using last known disk stats\. Guest reads paused/).waitFor();
          assert.equal(
            await disclosure.getAttribute('aria-controls'),
            await drawer.getAttribute('id'),
          );
          assert.equal(await row.getAttribute('tabindex'), null);
          assert.equal(await row.getAttribute('aria-expanded'), null);
          assert.equal(
            await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
            true,
          );
          record.checks.push(
            `first trusted ${width === 360 ? 'row-body' : 'name'} activation opens production full drawer with safe backup/read context`,
          );
          const screenshot = `${output}/${name}-${kind}-open.png`;
          await page.screenshot({ path: screenshot, fullPage: true });
          result.screenshots.push({
            path: path.relative('/workspace', screenshot),
            sha256: sha256(screenshot),
          });
          await activate(width === 360 ? row : nameTarget);
          await expectState(2, false);
          await activate(row);
          await expectState(3, true);
          await activate(page.getByRole('button', { name: 'Collapse backup-guest details' }));
          await expectState(3, false);
          assert.equal((await state()).closeCount, 1);
          record.checks.push(
            'repeated name and row-centre activation toggle once; nested full-drawer collapse stays independent',
          );

          await disclosure.focus();
          await page.keyboard.press('Enter');
          await expectState(4, true);
          await page.keyboard.press('Space');
          await expectState(5, false);
          await disclosure.locator('svg').click();
          await expectState(6, true);
          // Keyboard focus makes the compact disclosure button visible and
          // can consume the name's space; the row body stays the touch target.
          await activate(row);
          await expectState(7, false);
          record.checks.push(
            'native disclosure keyboard Enter/Space and nested SVG click do not double-activate',
          );

          if (width >= 1024) {
            // The adjacent link is intentionally hidden in condensed phone
            // identity. Exercise its real wide-layout action, preventing only
            // external navigation, not production propagation handling.
            const link = row.getByRole('link', { name: 'Open web interface for backup-guest' });
            await link.evaluate((anchor) =>
              anchor.addEventListener('click', (event) => event.preventDefault()),
            );
            await activate(link.locator('svg'));
            await expectState(7, false);
            record.checks.push(
              'real external-link SVG retains its separate action; navigation prevented in fixture only',
            );
          }

          await page.evaluate(() => window.__guestRowTouch.enabled(false));
          assert.equal(await disclosure.count(), 0);
          await activate(row);
          assert.equal((await state()).count, 7);
          assert.equal((await state()).expanded, false);
          await page.evaluate(() => window.__guestRowTouch.enabled(true));
          await activate(row);
          await expectState(8, true);
          record.checks.push(
            'disabling/re-enabling the row action preserves node identity and first-tap activation',
          );
          assert.deepEqual(record.errors, []);
          assert(
            record.requests.every(
              (request) => request.origin === origin && request.method === 'GET',
            ),
          );
          record.result = 'passed';
        } catch (error) {
          record.result = 'failed';
          record.error = error.message;
          const screenshot = `${output}/${name}-${kind}-failed.png`;
          await page.screenshot({ path: screenshot, fullPage: true });
          result.screenshots.push({
            path: path.relative('/workspace', screenshot),
            sha256: sha256(screenshot),
          });
          throw error;
        } finally {
          await page.close();
        }
      }
      // Exporting the compatibility marker must preserve the original shared
      // TableRow's own single-action and embedded-control ordering.
      const page = await context.newPage();
      await page.goto(`${origin}/browser-tests/table-row-touch.html`);
      const sharedState = () => page.getByRole('status', { name: 'Action state' }).textContent();
      const shared = page.locator('[data-testid="control-row"]');
      if (width <= 768) await shared.getByText('Shared resource name', { exact: true }).tap();
      else await shared.getByText('Shared resource name', { exact: true }).click();
      const sharedValue = JSON.parse(await sharedState());
      assert.equal(sharedValue.rowActions, 1);
      await shared.getByRole('button', { name: 'Child action' }).click();
      const childValue = JSON.parse(await sharedState());
      assert.equal(childValue.rowActions, 1);
      assert.equal(childValue.childActions, 1);
      result.results.push({
        name,
        control: 'shared-TableRow',
        single: sharedValue,
        child: childValue,
        result: 'passed',
      });
      await page.close();
      await context.close();
      await browser.close();
      browser = null;
    }
    result.result = 'passed';
    result.verified_at = new Date().toISOString();
    console.log(
      'PASS: trusted guest row/disclosure activation, full drawer safety context and child isolation',
    );
  } catch (error) {
    result.result = 'failed';
    result.error = error.message;
    throw error;
  } finally {
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error.stack);
  process.exitCode = 1;
});
