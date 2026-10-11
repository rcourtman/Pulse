// Production drawer/state/CSS, synthetic local snapshots only. The parent
// mode records the held-age defect; the final mode requires its correction.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const root = '/workspace/frontend-modern';
const origin = 'http://127.0.0.1:5347';
const start = Date.parse('2026-10-11T10:00:00Z');
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');

(async () => {
  const binding = JSON.parse(fs.readFileSync('/workspace/tmp/source-evidence-age/binding.json'));
  assert.ok(['parent', 'final'].includes(binding.mode));
  for (const [file, expected] of Object.entries(binding.content_sha256)) {
    assert.equal(hash('/workspace/' + file), expected, file);
  }
  const output = '/workspace/tmp/source-evidence-age/' + binding.mode;
  fs.mkdirSync(output, { recursive: true });
  const result = {
    binding,
    playwright: require('playwright/package.json').version,
    cases: [],
    captures: [],
    completed: false,
    cleanup: {},
    limits:
      'Synthetic drawer/provider snapshots, not native QGA, thaw, filesystem writes, restart recovery, publication or a release. Parent observation is not a passing repair.',
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const { createServer } = await import(root + '/node_modules/vite/dist/node/index.js');
  const server = await createServer({
    root,
    configFile: root + '/vite.config.ts',
    cacheDir: output + '/vite-cache',
    server: { host: '127.0.0.1', port: 5347, strictPort: true, watch: null },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1280, false],
      ['phone', webkit, 390, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: 950 },
        isMobile: width < 500,
        hasTouch: width < 500,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(8000);
      const record = {
        name,
        browser: browser.version(),
        width,
        dark,
        checks: [],
        requests: [],
        writes: [],
        unexpectedReads: [],
        offOrigin: [],
        errors: [],
      };
      result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      await page.clock.install({ time: new Date(start - 1000) });
      await page.clock.pauseAt(new Date(start));
      await page.addInitScript((dark) => {
        localStorage.clear();
        localStorage.setItem('pulse_debug_mode', 'true');
        window.__sourceEvidenceClipboard = [];
        Object.defineProperty(navigator, 'clipboard', {
          configurable: true,
          value: {
            writeText: async (text) => window.__sourceEvidenceClipboard.push(text),
          },
        });
        document.addEventListener(
          'DOMContentLoaded',
          () => document.documentElement.classList.toggle('dark', dark),
          { once: true },
        );
      }, dark);
      await page.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin) {
          record.offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        record.requests.push({ path: url.pathname, method: req.method() });
        if (req.method() !== 'GET') {
          record.writes.push(url.pathname);
          return route.fulfill({
            status: 403,
            json: { error: 'Synthetic fixture refuses writes' },
          });
        }
        let data;
        if (/^\/api\/resources\/[^/]+\/operator-state$/.test(url.pathname))
          data = { configured: false };
        else if (/^\/api\/resources\/[^/]+\/maintenance-verifications$/.test(url.pathname))
          data = { reports: [] };
        else if (url.pathname === '/api/license/runtime-capabilities')
          data = {
            capabilities: [],
            limits: [],
            max_history_days: 7,
            hosted_mode: false,
            runtime: { build: 'community' },
            blocked_capabilities: [],
          };
        else if (url.pathname === '/api/security/status')
          data = {
            hasAuthentication: true,
            requiresAuth: false,
            sessionCapabilities: {},
          };
        else if (url.pathname === '/api/health') data = { status: 'ok' };
        else {
          record.unexpectedReads.push(url.pathname);
          return route.fulfill({ status: 404, json: { error: 'Unspecified synthetic read' } });
        }
        return route.fulfill({ json: data });
      });
      const activate = async (locator) => (width < 500 ? locator.tap() : locator.press('Enter'));
      const summary = (label) =>
        page.locator('summary').filter({ hasText: new RegExp('^' + label) });
      const capture = async (label) => {
        const file = path.join(output, name + '-' + label + '.png');
        await page.screenshot({ path: file, fullPage: true, animations: 'disabled' });
        result.captures.push({
          path: file.replace('/workspace/', ''),
          sha256: hash(file),
          bytes: fs.statSync(file).size,
        });
      };
      await page.goto(origin + '/browser-tests/source-evidence-age.html', {
        waitUntil: 'domcontentloaded',
        timeout: 60000,
      });
      await page.getByRole('tab', { name: 'Debug', exact: true }).waitFor();
      const notice = page.getByTestId('resource-guest-read-precaution');
      await notice.waitFor();
      const precaution = await notice.innerText();
      assert.match(precaution, /does not prove thaw/);
      await activate(page.getByRole('tab', { name: 'Debug', exact: true }));
      assert.match(await summary('Proxmox').innerText(), /deferred • 1 min ago/);
      assert.match(await summary('Agent').innerText(), /stale • 2 mins ago/);
      await activate(summary('Proxmox'));
      assert(await summary('Proxmox').evaluate((el) => el.parentElement.open));
      const heldElement = await summary('Proxmox').elementHandle();
      const heldPayload = await summary('Proxmox').locator('..').locator('pre').innerText();
      const heldResource = await page.evaluate(() =>
        JSON.stringify(window.__sourceEvidenceAge.snapshot()),
      );
      const copy = () => page.getByRole('button', { name: /^(Copy JSON|Copied)$/ });
      await activate(copy());
      await page.waitForFunction(() => window.__sourceEvidenceClipboard.length === 1);
      await summary('Proxmox').focus();
      await capture('initial');
      await page.clock.fastForward(2 * 3_600_000);
      record.sourceAgeAfterTwoHours = await summary('Proxmox').innerText();
      const expected =
        binding.mode === 'parent' ? /deferred • 1 min ago/ : /deferred • 2 hours ago/;
      assert.match(record.sourceAgeAfterTwoHours, expected);
      assert.match(
        await summary('Agent').innerText(),
        binding.mode === 'parent' ? /stale • 2 mins ago/ : /stale • 2 hours ago/,
      );
      assert(await summary('Proxmox').evaluate((el, held) => el === held, heldElement));
      assert(
        await summary('Proxmox').evaluate(
          (el) => el.parentElement.open && document.activeElement === el,
        ),
      );
      assert.equal(await summary('Proxmox').locator('..').locator('pre').innerText(), heldPayload);
      assert.equal(
        await page.evaluate(() => JSON.stringify(window.__sourceEvidenceAge.snapshot())),
        heldResource,
      );
      assert.equal(await notice.innerText(), precaution);
      await activate(copy());
      await page.waitForFunction(() => window.__sourceEvidenceClipboard.length === 2);
      const copies = await page.evaluate(() => window.__sourceEvidenceClipboard);
      assert.equal(copies[0], copies[1]);
      const bundle = JSON.parse(copies[1]);
      assert.equal(bundle.sources.sourceStatus.proxmox.lastSeen, start - 60_000);
      assert.equal(bundle.sources.proxmox.diskStatusReason, 'prev-agent-timeout');
      record.checks.push(
        'Held feed: source-age discrimination; same disclosure/focus/payload/clipboard timestamps/precaution, no new observation',
      );
      await capture('held-two-hours');
      await activate(page.getByRole('tab', { name: 'Overview', exact: true }));
      const lastSeenRow = page.getByText('Last seen', { exact: true }).locator('..');
      assert.match(await lastSeenRow.innerText(), /2 hours ago/);
      record.overviewAgeAfterTwoHours = await lastSeenRow.innerText();
      record.checks.push(
        'Real Overview already advances, discriminating the parent Debug-only defect',
      );
      await activate(page.getByRole('tab', { name: 'Debug', exact: true }));
      assert.match(await summary('Proxmox').innerText(), expected);
      await page.evaluate(() => window.__sourceEvidenceAge.replace());
      assert.match(await summary('Proxmox').innerText(), /available • 0s ago/);
      assert.equal(await page.getByText('Guest read deferred', { exact: true }).count(), 0);
      assert.equal(await summary('Agent').count(), 0);
      await page.clock.fastForward(60_000);
      assert.match(
        await summary('Proxmox').innerText(),
        binding.mode === 'parent' ? /available • 0s ago/ : /available • 1 min ago/,
      );
      record.checks.push(
        'Ordinary same-resource platformData replacement updates server status and removes the old error/source; subsequent age discrimination',
      );
      await page.evaluate(() => window.__sourceEvidenceAge.missingTimes());
      await page.clock.fastForward(60_000);
      assert.doesNotMatch(await summary('Proxmox').innerText(), /•/);
      assert.doesNotMatch(await summary('Agent').innerText(), /•/);
      assert.match(await summary('Docker').innerText(), /unknown/);
      record.checks.push(
        'Missing/invalid timestamps remain without invented ages; absent status remains unknown',
      );
      await capture('unknown-times');
      await page.evaluate(() => window.__sourceEvidenceAge.unmount());
      assert.equal(await page.getByRole('tab', { name: 'Debug' }).count(), 0);
      await page.clock.fastForward(60_000);
      await page.evaluate(() => window.__sourceEvidenceAge.remount());
      await activate(page.getByRole('tab', { name: 'Debug', exact: true }));
      assert.match(await summary('Proxmox').innerText(), /deferred/);
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      assert.deepEqual(record.writes, []);
      assert.deepEqual(record.unexpectedReads, []);
      assert.deepEqual(record.offOrigin, []);
      assert.deepEqual(record.errors, []);
      record.checks.push(
        'Unmount/remount works; no horizontal page overflow, API writes, unexpected reads, off-origin traffic or page errors',
      );
      record.completed = true;
      await browser.close();
      browser = undefined;
    }
    result.completed = true;
  } catch (error) {
    result.error = { name: error.name, message: error.message };
    process.exitCode = 1;
  } finally {
    try {
      if (browser) await browser.close();
      result.cleanup.browsersClosed = true;
    } catch (error) {
      result.cleanup.browserError = error.message;
      process.exitCode = 1;
    }
    try {
      await server.close();
      result.cleanup.serverClosed = true;
    } catch (error) {
      result.cleanup.serverError = error.message;
      process.exitCode = 1;
    }
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        completed: result.completed,
        output,
        error: result.error,
        cases: result.cases.length,
        cleanup: result.cleanup,
      }),
    );
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
