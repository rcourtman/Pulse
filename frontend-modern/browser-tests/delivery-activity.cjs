// Reuse the real Notifications-tab fixture and its bound build. All API data
// is synthetic; no test send, save, retry, dismissal or off-origin request.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/delivery-activity/browser';
const origin = 'http://127.0.0.1:5353';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const component = 'src/features/alerts/AlertDeliveryLogCard.tsx';

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    playwright: require('playwright/package.json').version,
    bindings: {},
    cases: [],
    captures: [],
    cleanup: { servers_created: 0, servers_closed: 0, browsers_created: 0, browsers_closed: 0 },
    limits:
      'Synthetic HTTP reads through production API readers and Notifications tab. Not recipient delivery, native recovery or installed availability. No Back/reload crash sequence.',
  };
  let server, browser;
  try {
    assert.equal(
      result.playwright,
      JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    const assets = new Map();
    for (const [version, directory] of [
      ['baseline', '/workspace/tmp/delivery-activity/baseline'],
      ['fixed', '/workspace/tmp/apprise-transport/static'],
    ]) {
      const binding = JSON.parse(fs.readFileSync(directory + '/binding.json'));
      assert(binding.sources[component]);
      assert(binding.sources['src/features/alerts/useNotificationDeliveryLog.ts']);
      assert(binding.sources['src/api/notifications.ts']);
      if (version === 'fixed') {
        for (const [file, digest] of Object.entries(binding.sources))
          assert.equal(
            hash(fs.readFileSync(path.join(root, file))),
            digest,
            'changed input: ' + file,
          );
      } else {
        assert.equal(binding.source_sha, 'dadc4b988832198bd75e4c1f43ee7d261de73fb8');
        assert.equal(
          binding.sources[component],
          hash(fs.readFileSync('/workspace/tmp/delivery-activity/baseline-component.tsx')),
        );
      }
      for (const [file, digest] of Object.entries(binding.artifacts)) {
        let bytes = fs.readFileSync(path.join(directory, file));
        assert.equal(hash(bytes), digest);
        // The HTML's root-relative assets get an isolated prefix. JS/CSS bytes
        // themselves are served unchanged from their retained build binding.
        if (file.endsWith('.html'))
          bytes = Buffer.from(bytes.toString().replaceAll('"/assets/', `"/${version}/assets/`));
        assets.set('/' + version + '/' + file, bytes);
      }
      result.bindings[version] = binding;
    }
    server = http.createServer((req, res) => {
      const pathname = new URL(req.url, origin).pathname;
      const bytes = assets.get(pathname);
      if (req.method !== 'GET' || !bytes) {
        res.writeHead(404);
        return res.end();
      }
      res.setHeader(
        'Content-Type',
        { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' }[
          path.extname(pathname)
        ] || 'application/octet-stream',
      );
      res.setHeader('Cache-Control', 'no-store');
      res.end(bytes);
    });
    result.cleanup.servers_created++;
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(5353, '127.0.0.1', resolve);
    });
    for (const [version, engine, width, dark] of [
      ['baseline', chromium, 1365, false],
      ['baseline', webkit, 320, true],
      ['fixed', chromium, 1365, false],
      ['fixed', webkit, 320, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      result.cleanup.browsers_created++;
      const page = await browser.newPage({
        viewport: { width, height: 900 },
        isMobile: width === 320,
        hasTouch: width === 320,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(10000);
      let mode = 'populated';
      const errors = [],
        writes = [],
        offOrigin = [],
        reads = [];
      const id = 'pve-cluster-production-node-west-' + 'a'.repeat(90) + ':qemu:101:memory';
      const ids = [
        id,
        'pve-cluster-production-node-east-' + 'b'.repeat(90) + ':qemu:101:SECOND-RESOURCE',
      ];
      const resource = 'guest-service-' + 'c'.repeat(95) + '-west';
      const message =
        'Delivery held by quiet hours until 08:00 UTC; monitoring continues. <not-markup>';
      const diagnostic =
        'TLS handshake failed for https://notify.example.invalid/' +
        'x'.repeat(200) +
        ': certificate is not trusted (fixture)';
      const timestamp = (seconds) => new Date(Date.now() - seconds * 1000).toISOString();
      const entry = (index) => ({
        notificationId: 'fixture-' + index,
        type: 'webhook',
        destinationId: 'webhook:wh-ops',
        outcome: index === 0 ? 'failed' : 'sent',
        alertIds: index === 0 ? ids : ['older-alert-' + index],
        alertCount: index === 0 ? 2 : 1,
        attempts: 1,
        success: index !== 0,
        errorMessage: index === 0 ? diagnostic : undefined,
        failureClass: index === 0 ? 'tls' : undefined,
        timestamp: timestamp(index + 3),
      });
      page.on('pageerror', (error) => errors.push(error.message));
      await page.addInitScript(
        (dark) =>
          document.addEventListener('DOMContentLoaded', () =>
            document.documentElement.classList.toggle('dark', dark),
          ),
        dark,
      );
      await page.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin) {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (req.method() !== 'GET') {
          writes.push({ method: req.method(), path: url.pathname });
          return route.abort();
        }
        reads.push(url.pathname + url.search);
        let json = [];
        if (
          mode === 'unavailable' &&
          ['/api/notifications/delivery-log', '/api/alerts/events'].includes(url.pathname)
        )
          return route.fulfill({ status: 503, json: { error: 'Synthetic evidence unavailable' } });
        if (url.pathname === '/api/notifications/webhooks')
          json = [
            {
              id: 'wh-ops',
              name: 'Operations destination ' + 'd'.repeat(60),
              url: 'https://notify.example.invalid',
              method: 'POST',
              headers: {},
              enabled: true,
            },
          ];
        if (url.pathname === '/api/notifications/apprise') json = { enabled: false };
        if (url.pathname === '/api/notifications/email') json = { enabled: false };
        if (url.pathname === '/api/alerts/deadman/config')
          json = { configured: false, pingUrl: '' };
        if (url.pathname === '/api/notifications/health')
          json = {
            overall_healthy: false,
            queue: {
              pending: 0,
              sending: 0,
              sent: 13,
              failed: 1,
              dlq: 0,
              healthy: false,
              status: 'degraded',
              attention_required: 1,
              reason_codes: ['retained_failed_deliveries'],
            },
          };
        if (url.pathname === '/api/notifications/delivery-log')
          json = {
            entries:
              mode === 'empty'
                ? []
                : mode === 'recovered'
                  ? [{ ...entry(0), alertIds: ['fresh-recovery-row'], alertCount: 1 }]
                  : Array.from({ length: 14 }, (_, i) => entry(i)),
            window_days: 30,
            completed_retention_days: 7,
            dead_letter_retention_days: 30,
          };
        if (url.pathname === '/api/alerts/events')
          json =
            mode === 'populated'
              ? [
                  {
                    id: 1,
                    occurredAt: timestamp(1),
                    type: 'notification_deferred',
                    alertId: id,
                    resourceName: resource,
                    alertType: 'memory',
                    reason: 'quiet_hours:performance',
                    message,
                  },
                ]
              : [];
        if (url.pathname === '/api/security/status')
          json = { hasAuthentication: true, requiresAuth: false };
        if (url.pathname === '/api/license/runtime-capabilities')
          json = {
            capabilities: [],
            limits: [],
            max_history_days: 7,
            hosted_mode: false,
            runtime: { build: 'community' },
            blocked_capabilities: [],
          };
        if (url.pathname === '/api/license/entitlements') json = { entitlements: [] };
        return route.fulfill({ json });
      });
      await page.goto(origin + '/' + version + '/browser-tests/apprise-transport.html');
      const card = page.locator('#notification-delivery-activity');
      await card.getByText('Deferred', { exact: true }).waitFor();
      const list = card.locator('ul');
      await page.evaluate(() => document.fonts.ready);
      const measure = () =>
        list.evaluate((e) => ({
          client: e.clientWidth,
          scroll: e.scrollWidth,
          height: e.clientHeight,
          contentHeight: e.scrollHeight,
          tabIndex: e.tabIndex,
          clippedIdentities: [...e.querySelectorAll('span.truncate')].filter(
            (s) => s.scrollWidth > s.clientWidth + 1,
          ).length,
        }));
      const before = await measure();
      const capture = async (state) => {
        const file = `${version}-${width}-${state}.png`;
        await card.screenshot({ path: output + '/' + file });
        result.captures.push({ file, sha256: hash(fs.readFileSync(output + '/' + file)) });
      };
      if (version === 'baseline') {
        assert.equal(before.tabIndex, -1);
        assert.equal(await card.getByText(message, { exact: true }).count(), 0);
        assert.equal(await list.locator('li').first().getAttribute('title'), message);
        assert(before.clippedIdentities > 0);
        assert(before.scroll > before.client + 1, JSON.stringify(before));
        await capture('clipped');
      } else {
        assert.equal(await page.getByRole('list', { name: 'Notification activity' }).count(), 1);
        assert.equal(before.tabIndex, 0);
        assert.equal(before.scroll, before.client);
        assert.equal(before.clippedIdentities, 0);
        assert.equal(await card.getByText(message, { exact: true }).isVisible(), true);
        assert.equal(await list.locator('not-markup').count(), 0);
        await card.getByRole('button').focus();
        await page.keyboard.press('Tab');
        assert.equal(await list.evaluate((e) => e === document.activeElement), true);
        await capture('keyboard-focus');
        await page.keyboard.press('PageDown');
        await page.waitForTimeout(150);
        assert((await list.evaluate((e) => e.scrollTop)) > 0, 'Keyboard must reach older rows');
        await page.keyboard.press('Home');
        await list.evaluate((e) => {
          e.scrollTop = 0;
        });
        const summary = list.locator('summary');
        if (width === 320) await summary.tap();
        else {
          await summary.focus();
          await page.keyboard.press('Enter');
        }
        assert.equal(await summary.locator('..').getAttribute('open'), '');
        assert.equal(await summary.locator('..').locator('p').innerText(), ids.join('\n'));
        assert.equal((await measure()).scroll, before.client);
        await capture('all-alert-ids');
        if (width === 320) {
          await summary
            .locator('..')
            .locator('p')
            .evaluate((e) => e.scrollIntoView({ block: 'end' }));
          await capture('all-alert-ids-end');
          const failure = list.locator('p').filter({ hasText: diagnostic });
          await failure.scrollIntoViewIfNeeded();
          assert.equal(await failure.evaluate((e) => e.scrollWidth <= e.clientWidth), true);
          await capture('full-diagnostic');
        }
        mode = 'unavailable';
        await card.getByRole('button').click();
        await card.getByRole('alert').first().waitFor();
        assert.equal(await list.count(), 0);
        assert.equal(await card.getByText(/No alert deliveries were attempted/).count(), 0);
        mode = 'empty';
        await card.getByRole('button').click();
        await card.getByText(/No alert deliveries were attempted/).waitFor();
        assert.equal(await card.getByRole('list').count(), 0);
        mode = 'recovered';
        await card.getByRole('button').click();
        await card.getByText('fresh-recovery-row', { exact: true }).waitFor();
        assert.equal(await card.getByText(message, { exact: true }).count(), 0);
        assert.equal(await card.getByRole('listitem').count(), 1);
        assert(reads.some((read) => read === '/api/notifications/delivery-log?limit=200'));
        assert(
          reads.some((read) => read.includes('/api/alerts/events?') && read.includes('limit=100')),
        );
      }
      assert.deepEqual(writes, []);
      assert.deepEqual(offOrigin, []);
      assert.deepEqual(errors, []);
      result.cases.push({
        version,
        width,
        dark,
        browser: browser.version(),
        passed: true,
        populatedList: before,
        reads,
        writes,
        offOrigin,
        errors,
      });
      await browser.close();
      browser = null;
      result.cleanup.browsers_closed++;
    }
    result.result = 'passed';
    result.verified_at = new Date().toISOString();
  } catch (error) {
    result.error = { name: error.name, message: error.message, stack: error.stack };
    process.exitCode = 1;
  } finally {
    if (browser) {
      await browser.close();
      result.cleanup.browsers_closed++;
    }
    if (server)
      await new Promise((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
        server.closeAllConnections();
      }).then(() => result.cleanup.servers_closed++);
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: result.result,
        cases: result.cases.length,
        captures: result.captures.length,
        cleanup: result.cleanup,
        error: result.error,
      }),
    );
  }
})();
