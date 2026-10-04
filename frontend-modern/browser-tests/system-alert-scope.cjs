const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/system-alert-scope-proof';
const runtime = [
  'frontend-modern/src/utils/alertScope.ts',
  'frontend-modern/src/components/Alerts/alertAssistantHandoffModel.ts',
  'frontend-modern/src/components/Alerts/InvestigateAlertButton.tsx',
  'frontend-modern/src/features/alerts/AlertOverviewAlertCard.tsx',
  'frontend-modern/src/features/alerts/alertHistoryModel.ts',
  'frontend-modern/src/features/alerts/AlertHistoryItemActions.tsx',
];
const systemTypes = [
  'backup-evaluation',
  'notification-delivery',
  'deadman-delivery',
  'deadman-monitoring-stalled',
  'deadman-interruption',
  'deadman-state',
];
const legacy = {
  id: 'retained-legacy-system-id',
  type: 'notification-delivery',
  level: 'warning',
  resourceId: 'vm-wrong',
  resourceName: 'Pulse',
  node: 'wrong-node',
  metadata: { systemAlert: true, resourceType: 'vm' },
  message: 'Synthetic retained notification delivery outage.',
  startTime: '2026-10-04T14:00:00Z',
  lastSeen: '2026-10-04T14:30:00Z',
  acknowledged: true,
  value: 0,
  threshold: 0,
};
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const version = require('playwright/package.json').version;
  assert.equal(
    version,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const hashes = Object.fromEntries(
    runtime.map((file) => [file, hash(path.join('/workspace', file))]),
  );
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5297, strictPort: true, watch: null },
  });
  const origin = 'http://127.0.0.1:5297';
  const results = [],
    screenshots = [];
  let browser,
    phase = 'server';
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['chromium-desktop-light', chromium, 1365, false],
      ['chromium-desktop-dark', chromium, 1365, true],
      ['webkit-phone-dark', webkit, 390, true],
      ['webkit-narrow-light', webkit, 320, false],
    ]) {
      phase = name;
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: width < 500 ? 844 : 900 },
        isMobile: width < 500,
        hasTouch: width < 500,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(10000);
      page.setDefaultNavigationTimeout(60000);
      const errors = [],
        requests = [],
        offOrigin = [],
        writes = [],
        checks = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.addInitScript((dark) => {
        localStorage.clear();
        const apply = () => document.documentElement.classList.toggle('dark', dark);
        if (document.documentElement) apply();
        else document.addEventListener('DOMContentLoaded', apply, { once: true });
      }, dark);
      await page.routeWebSocket(/.*/, (socket) => socket.close());
      await page.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin) {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        requests.push({ path: url.pathname, query: url.search, method: req.method() });
        if (req.method() !== 'GET') {
          writes.push({ path: url.pathname, method: req.method(), body: req.postDataJSON() });
          assert.ok(
            ['/api/alerts/acknowledge', '/api/alerts/unacknowledge'].includes(url.pathname),
            'unexpected write',
          );
          assert.equal(req.method(), 'POST');
          assert.deepEqual(req.postDataJSON(), {
            alertIdentifier: 'pulse-system-backup-evaluation',
          });
          return route.fulfill({ json: { success: true } });
        }
        if (url.pathname === '/api/alerts/history') return route.fulfill({ json: [legacy] });
        if (url.pathname === '/api/alerts/incidents') return route.fulfill({ json: null });
        if (url.pathname === '/api/notifications/health')
          return route.fulfill({
            json: {
              overall_healthy: true,
              queue: {
                healthy: true,
                status: 'healthy',
                pending: 0,
                sending: 0,
                sent: 0,
                failed: 0,
                dlq: 0,
                attention_required: 0,
                reason_codes: [],
                completed_retention_days: 7,
                dead_letter_retention_days: 30,
                counts_are_retention_bounded: true,
                retry_attempts_affect_health: false,
                terminal_failures_affect_health: true,
                failure_classes_7d: {
                  authentication: 0,
                  rate_limited: 0,
                  connectivity: 0,
                  tls: 0,
                  configuration: 0,
                  rejected: 0,
                  server_error: 0,
                  unknown: 0,
                },
                failure_class_window_days: 7,
                failure_classes_available: true,
              },
            },
          });
        if (url.pathname.includes('license'))
          return route.fulfill({
            json: { capabilities: [], entitlements: [], limits: [], max_history_days: 90 },
          });
        if (url.pathname === '/api/security/status')
          return route.fulfill({
            json: {
              hasAuthentication: true,
              requiresAuth: false,
              sessionCapabilities: { assistantEnabled: true },
            },
          });
        return route.fulfill({ json: [] });
      });
      const check = async (label, action) => {
        phase = `${name}: ${label}`;
        await action();
        checks.push({ label, passed: true });
      };
      const capture = async (locator, state) => {
        const file = `${name}-${state}.png`;
        await locator.screenshot({ path: path.join(output, file) });
        screenshots.push({ file, state, width, dark, sha256: hash(path.join(output, file)) });
      };
      const assertSystemContext = async (id) => {
        const context = await page.evaluate(() => window.__systemAlertScope.context());
        assert.equal(context.targetId, undefined);
        assert.equal(context.targetType, undefined);
        assert.deepEqual(context.handoffResources, []);
        assert.equal(context.autonomousMode, false);
        assert.equal(context.context.guestName, undefined);
        assert.equal(context.context.node, undefined);
        assert.match(context.handoffContext, /Scope: Pulse itself \(not a monitored resource\)/);
        assert.ok(context.handoffContext.includes(`Alert Identifier: ${id}`));
        assert.ok(context.handoffContext.includes('Operator Boundary:'));
        assert.doesNotMatch(
          context.handoffContext,
          /Current Value:|Threshold:|wrong-node|vm-wrong/,
        );
        assert.doesNotMatch(context.briefing.detailLines.join('\n'), /Current value|threshold/);
        await page.evaluate(() => window.__systemAlertScope.closeExplanation());
      };
      try {
        await page.goto(`${origin}/browser-tests/system-alert-scope.html`);
        const overview = page.getByRole('region', { name: 'Existing alert overview' });
        const history = page.getByRole('region', { name: 'Existing alert history' });
        const backupCard = page.locator('#alert-pulse-system-backup-evaluation');
        await check(
          'canonical service warning has no false resource link or policy action',
          async () => {
            await backupCard.waitFor({ state: 'visible' });
            assert.equal(
              await overview
                .getByText('Notification delivery status is unavailable', { exact: true })
                .count(),
              0,
            );
            assert.equal(await backupCard.getByRole('link').count(), 0);
            assert.equal(
              await backupCard.getByRole('button', { name: 'Have Patrol investigate' }).count(),
              0,
            );
            assert.match(await backupCard.textContent(), /Backup-age alerts were not evaluated/);
            await page.waitForFunction(() => window.__systemAlertScope.historyRows().length === 3);
            const rows = await page.evaluate(() => window.__systemAlertScope.historyRows());
            for (const row of rows.filter((r) => r.systemAlert)) {
              assert.equal(row.resourceType, 'Pulse');
              assert.equal(row.resourceId, '');
            }
            assert.equal(rows.find((r) => r.id === 'resource-cpu').resourceType, 'vm');
          },
        );
        await capture(overview, 'backup-warning');
        await check('acknowledgement remains active, not recovery', async () => {
          await backupCard.getByRole('button', { name: 'Acknowledge', exact: true }).click();
          await backupCard.getByText('Acknowledged', { exact: true }).waitFor();
          assert.match(await backupCard.textContent(), /Backup-age alerts were not evaluated/);
          await backupCard.getByRole('button', { name: 'Unacknowledge', exact: true }).click();
          await backupCard.getByRole('button', { name: 'Acknowledge', exact: true }).waitFor();
        });
        await check('occurrence timeline keeps exact service identity', async () => {
          await backupCard.getByRole('button', { name: 'Timeline', exact: true }).click();
          await page.waitForFunction(() =>
            document
              .querySelector('#alert-pulse-system-backup-evaluation')
              ?.textContent.includes('No incident'),
          );
          const incident = requests.find((r) => r.path === '/api/alerts/incidents');
          assert.ok(incident);
          const query = new URLSearchParams(incident.query);
          assert.equal(query.get('alertIdentifier'), 'pulse-system-backup-evaluation');
          assert.equal(query.get('started_at'), '2026-10-04T15:00:00Z');
          await backupCard.getByRole('button', { name: 'Hide Timeline', exact: true }).click();
        });
        for (const type of systemTypes) {
          await check(`${type} explanation is service-scoped and model-only`, async () => {
            await page.evaluate((type) => window.__systemAlertScope.updateSystem(type), type);
            const card = page.locator(`#alert-pulse-system-${type}`);
            await card.waitFor({ state: 'visible' });
            assert.equal(await card.getByRole('link').count(), 0);
            const button = card.getByRole('button', { name: 'Ask Pulse Assistant', exact: true });
            if (width > 500) {
              await button.focus();
              await page.keyboard.press('Enter');
            } else await button.tap();
            await assertSystemContext(`pulse-system-${type}`);
          });
        }
        await check(
          'legacy system metadata wins over conflicting resource/metric hints',
          async () => {
            await page.evaluate(() =>
              window.__systemAlertScope.updateSystem('future-condition', true),
            );
            const card = page.locator('#alert-legacy-system-id');
            await card.waitFor({ state: 'visible' });
            assert.equal(await card.getByRole('link').count(), 0);
            assert.doesNotMatch(await card.textContent(), /limit:|wrong-node/);
            await card.getByRole('button', { name: 'Ask Pulse Assistant', exact: true }).click();
            await assertSystemContext('legacy-system-id');
          },
        );
        await check(
          'desktop and phone History reconstruct the retained service marker',
          async () => {
            const row =
              width < 500
                ? history
                    .locator('[data-alert-history-row-key]')
                    .filter({ hasText: legacy.message })
                : history.getByRole('row').filter({ hasText: legacy.message });
            assert.equal(await row.locator('[data-alert-history-action="resource"]').count(), 0);
            await row.getByRole('button', { name: 'Ask Pulse Assistant about this alert' }).click();
            await assertSystemContext(legacy.id);
          },
        );
        await capture(history, 'retained-history');
        await check(
          'monitored VM called Pulse retains its link and targeted affordances',
          async () => {
            const card = page.locator('#alert-resource-cpu');
            assert.equal(
              await card.getByRole('link', { name: 'Pulse', exact: true }).getAttribute('href'),
              '/proxmox/overview',
            );
            assert.match(await card.textContent(), /limit: 80%/);
            await card
              .getByRole('button', { name: 'Have Patrol investigate' })
              .waitFor({ state: 'visible' });
          },
        );
        await check(
          'authoritative recovery withdraws only the current service warning',
          async () => {
            await page.evaluate(() => window.__systemAlertScope.recover());
            assert.equal(await overview.locator('[id^="alert-legacy"]').count(), 0);
            assert.equal(await overview.locator('#alert-resource-cpu').count(), 1);
            assert.ok(
              (await page.evaluate(() => window.__systemAlertScope.historyRows())).some(
                (r) => r.id === legacy.id,
              ),
            );
          },
        );
        assert.deepEqual(errors, []);
        assert.deepEqual(offOrigin, []);
        assert.equal(writes.length, 2);
        assert.ok(
          !requests.some((r) => /patrol\/run|execute|diagnostic|ai\/chat|guest-agent/.test(r.path)),
        );
        results.push({
          name,
          width,
          dark,
          browser_version: browser.version(),
          checks,
          requests,
          writes,
          errors,
          offOrigin,
        });
      } finally {
        await browser.close();
        browser = null;
      }
    }
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        { passed: true, playwright_version: version, content_sha256: hashes, results, screenshots },
        null,
        2,
      ) + '\n',
    );
    console.log(
      JSON.stringify({
        passed: true,
        groups: results.length,
        checks: results.reduce((n, r) => n + r.checks.length, 0),
        screenshots: screenshots.length,
        playwright_version: version,
      }),
    );
  } catch (error) {
    fs.writeFileSync(
      path.join(output, 'failed.json'),
      JSON.stringify(
        { passed: false, phase, error: error.message, results, screenshots },
        null,
        2,
      ) + '\n',
    );
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
