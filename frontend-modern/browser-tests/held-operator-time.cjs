const assert = require('node:assert/strict');
const fs = require('node:fs');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const root = '/workspace/frontend-modern';
const start = Date.parse('2026-10-11T04:00:00Z');
const iso = (offset) => new Date(start + offset).toISOString();
const origin = 'http://127.0.0.1:5343';
const version = {
  version: '6.5.0',
  isDevelopment: false,
  isDocker: false,
  isSourceBuild: false,
  deploymentType: 'systemd',
};
const update = {
  available: false,
  currentVersion: '6.5.0',
  latestVersion: '6.5.0',
  releaseNotes: '',
  downloadUrl: '',
  isPrerelease: false,
  isMajorUpgrade: false,
};
const operatorState = (surface) => ({
  canonicalId: surface,
  intentionallyOffline: true,
  monitoringMode: 'expected_offline',
  lifecycleState: 'retired',
  neverAutoRemediate: true,
  note: 'Keep the saved lock',
  setAt: iso(-60_000),
  setBy: 'fixture-admin',
  ...(surface === 'once'
    ? { maintenanceStartAt: iso(30_000), maintenanceEndAt: iso(60_000) }
    : {
        maintenanceWindowActive: surface === 'server',
        ...(surface === 'server' ? { maintenanceActiveEndAt: iso(30_000) } : {}),
        maintenanceScope: 'resource_and_descendants',
        maintenanceRecurrence: {
          timezone: 'UTC',
          weekdays: ['sunday'],
          startMinute: 240,
          endMinute: 241,
        },
      }),
});

async function run(options = {}) {
  const output = options.output ?? root + '/tmp/held-operator-time/browser-complete';
  const surfaces = options.surfaces ?? ['once', 'server', 'recurring', 'updates', 'guest'];
  const binding = JSON.parse(fs.readFileSync(root + '/tmp/held-operator-time/source-binding.json'));
  fs.mkdirSync(output, { recursive: true });
  for (const [path, expected] of Object.entries(binding.content_sha256)) {
    assert.equal(
      crypto
        .createHash('sha256')
        .update(fs.readFileSync('/workspace/' + path))
        .digest('hex'),
      expected,
      path,
    );
  }
  const client = require('playwright/package.json').version;
  assert.equal(
    client,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const result = {
    playwright: client,
    source: binding,
    cases: [],
    captures: [],
    completed: false,
    serverClosed: false,
    browsersClosed: false,
    phase: 'start',
  };
  const { createServer } = await import(root + '/node_modules/vite/dist/node/index.js');
  const server = await createServer({
    root,
    configFile: root + '/vite.config.ts',
    cacheDir: output + '/vite-cache',
    server: { host: '127.0.0.1', port: 5343, strictPort: true, watch: null },
  });
  let browser, context;
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
      for (const surface of surfaces) {
        if (options.skip?.includes(name + ':' + surface)) continue;
        const record = {
          engine: name,
          browserVersion: browser.version(),
          viewport: { width, height: 950 },
          dark,
          surface,
          checks: [],
          requests: [],
          writes: [],
          unexpected: [],
          errors: [],
          offOrigin: [],
          completed: false,
          contextClosed: false,
        };
        result.cases.push(record);
        result.phase = name + ' ' + surface + ' mount';
        context = await browser.newContext({
          viewport: record.viewport,
          locale: 'en-GB',
          timezoneId: 'UTC',
          isMobile: width < 500,
          hasTouch: width < 500,
        });
        const page = await context.newPage();
        page.setDefaultTimeout(8000);
        page.setDefaultNavigationTimeout(35000);
        await page.clock.install({ time: new Date(start - 1000) });
        await page.clock.pauseAt(new Date(start));
        await page.addInitScript((dark) => {
          localStorage.clear();
          sessionStorage.clear();
          document.addEventListener(
            'DOMContentLoaded',
            () => document.documentElement.classList.toggle('dark', dark),
            { once: true },
          );
        }, dark);
        page.on('pageerror', (err) => record.errors.push(err.message));
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
              json: { error: 'Synthetic fixture refuses mutations' },
            });
          }
          let data;
          if (/^\/api\/resources\/[^/]+\/operator-state$/.test(url.pathname)) {
            data = url.pathname.includes('/absent/')
              ? { configured: false }
              : { configured: true, state: operatorState(surface) };
          } else if (url.pathname === '/api/version') data = version;
          else if (url.pathname === '/api/updates/check') data = update;
          else if (url.pathname === '/api/updates/history') data = [];
          else if (url.pathname === '/api/settings/alerts')
            data = { enabled: true, backupDefaults: { freshHours: 24, staleHours: 72 } };
          else if (url.pathname === '/api/security/status')
            data = { hasAuthentication: true, requiresAuth: false, sessionCapabilities: {} };
          else if (url.pathname === '/api/health') data = { status: 'ok' };
          else {
            record.unexpected.push(url.pathname);
            return route.fulfill({ status: 404, json: { error: 'Unspecified synthetic read' } });
          }
          return route.fulfill({ json: data });
        });
        const check = async (label, action) => {
          result.phase = name + ' ' + surface + ' ' + label;
          await action();
          record.checks.push(label);
        };
        const capture = async (label) => {
          const file = output + '/' + name + '-' + surface + '-' + label + '.png';
          await page.screenshot({ path: file, fullPage: true, animations: 'disabled' });
          result.captures.push(file.replace('/workspace/', ''));
        };
        await page.goto(origin + '/browser-tests/held-operator-time.html?surface=' + surface);
        await page.waitForFunction(() => window.__heldOperator?.ready());
        if (surface === 'once') {
          await page.getByText('Maintenance window scheduled.', { exact: true }).waitFor();
          await page.getByLabel('Operator note', { exact: true }).fill('Unsaved policy note');
          await page.getByRole('button', { name: 'Edit window', exact: true }).click();
          await page
            .getByLabel('Reason (optional)', { exact: true })
            .fill('Unsaved schedule reason');
          await capture('scheduled-edits');
          await page.clock.fastForward(30_000);
          await check('held one-off becomes active at its inclusive start', async () => {
            await page.getByText('Maintenance window active.', { exact: true }).waitFor();
            assert.equal(
              await page.getByText('Maintenance window scheduled.', { exact: true }).count(),
              0,
            );
          });
          await page.clock.fastForward(30_000);
          await check(
            'one-off expires at its exclusive end without losing unsaved edits or releasing saved restrictions',
            async () => {
              assert.equal(
                await page.getByText('Maintenance window active.', { exact: true }).count(),
                0,
              );
              assert.equal(
                await page.getByText('Maintenance window scheduled.', { exact: true }).count(),
                0,
              );
              assert.equal(
                await page.getByLabel('Operator note', { exact: true }).inputValue(),
                'Unsaved policy note',
              );
              assert.equal(
                await page.getByLabel('Reason (optional)', { exact: true }).inputValue(),
                'Unsaved schedule reason',
              );
              assert.equal(
                await page
                  .getByLabel('Never auto-remediate this resource', { exact: true })
                  .getAttribute('aria-pressed'),
                'true',
              );
              assert.equal(
                await page.getByLabel('Monitoring', { exact: true }).inputValue(),
                'expected_offline',
              );
              assert.equal(
                await page.getByLabel('Lifecycle', { exact: true }).inputValue(),
                'retired',
              );
              assert.equal(
                record.requests.filter((r) => r.path.includes('/operator-state')).length,
                1,
              );
              await page.getByRole('button', { name: 'Save overrides', exact: true }).waitFor();
            },
          );
          await capture('expired-edits');
          await page.evaluate(() => window.__heldOperator.withdrawWindow());
          await check('source removal stays removed across the next clock tick', async () => {
            await page.waitForFunction(() => document.querySelector('textarea')?.value === '');
            await page.clock.fastForward(30_000);
            assert.equal(
              await page.getByText('Maintenance window active.', { exact: true }).count(),
              0,
            );
            assert.equal(
              await page.getByText('Maintenance window scheduled.', { exact: true }).count(),
              0,
            );
            assert.equal(
              record.requests.filter((r) => r.path.includes('/operator-state')).length,
              2,
            );
          });
        } else if (surface === 'server') {
          await page.getByText('Maintenance window active.', { exact: true }).waitFor();
          await page.clock.fastForward(60_000);
          await check(
            'expired server verdict is last reported, preserving scope without inferring recurrence or resumption',
            async () => {
              assert.match(
                await page
                  .getByText('Last reported maintenance window.', { exact: true })
                  .locator('..')
                  .innerText(),
                /and its descendants/,
              );
              assert.equal(
                await page.getByText('Recurring maintenance configured.', { exact: true }).count(),
                0,
              );
              await page
                .getByText(
                  'The reported end has passed. Current maintenance status has not been refreshed.',
                  { exact: true },
                )
                .waitFor();
              assert.equal(
                await page.getByText('Maintenance window active.', { exact: true }).count(),
                0,
              );
              assert.equal(
                record.requests.filter((r) => r.path.includes('/operator-state')).length,
                1,
              );
            },
          );
          await capture('server-verdict');
        } else if (surface === 'recurring') {
          await page.getByText('Recurring maintenance configured.', { exact: true }).waitFor();
          await page.clock.fastForward(60_000);
          await check(
            'inactive recurring rule remains configured rather than becoming a locally invented active verdict',
            async () => {
              await page.getByText('Recurring maintenance configured.', { exact: true }).waitFor();
              assert.equal(
                await page.getByText('Maintenance window active.', { exact: true }).count(),
                0,
              );
              assert.equal(
                record.requests.filter((r) => r.path.includes('/operator-state')).length,
                1,
              );
            },
          );
          await capture('configured');
        } else if (surface === 'updates') {
          await page.getByText('Checked 0s ago', { exact: true }).waitFor();
          await page.clock.fastForward(60_000);
          await check(
            'held successful check ages without a new request, verdict or operation',
            async () => {
              await page.getByText('Checked 1 min ago', { exact: true }).waitFor();
              await page.getByText('Up to date', { exact: true }).waitFor();
              assert.equal(
                record.requests.filter((r) => r.path === '/api/updates/check').length,
                1,
              );
              assert.deepEqual(await page.evaluate(() => window.__heldOperator.controls()), {
                checks: 0,
                installs: 0,
                dirty: false,
                channel: 'stable',
                auto: false,
              });
            },
          );
          await page.getByRole('button', { name: /^Preview/ }).click();
          await page.clock.fastForward(60_000);
          await check(
            'preview auto-update lock and unsaved preference selection survive the clock',
            async () => {
              await page.getByText('Checked 2 mins ago', { exact: true }).waitFor();
              assert.equal(await page.getByTestId('updates-auto-check-toggle').isDisabled(), true);
              assert.deepEqual(await page.evaluate(() => window.__heldOperator.controls()), {
                checks: 0,
                installs: 0,
                dirty: true,
                channel: 'rc',
                auto: false,
              });
            },
          );
          await capture('aged-preview-lock');
          await page.evaluate(() => window.__heldOperator.sourceBuild());
          await page.clock.fastForward(30_000);
          await check(
            'source build withdraws release check age and keeps check unavailable',
            async () => {
              await page.getByText('Release updates disabled', { exact: true }).waitFor();
              assert.equal(await page.getByText(/Checked .*ago/).count(), 0);
              assert.equal(
                await page.getByRole('button', { name: 'Check Now', exact: true }).isDisabled(),
                true,
              );
            },
          );
        } else if (surface === 'guest') {
          await page.getByText('CPU 91%, above the 80% alert level', { exact: true }).waitFor();
          await check(
            'final drawer simultaneously renders held current alert and visible VM cache-source guidance',
            async () => {
              await page.getByText('About this reading', { exact: true }).waitFor();
              const details = page.getByTestId('guest-technical-details');
              assert.match(await details.innerText(), /Current · Proxmox \(may include cache\)/);
              assert.match(await details.innerText(), /cached memory the VM can reuse/);
              assert.match(await details.innerText(), /QEMU guest agent on a Linux VM/);
            },
          );
          await capture('current-cache-source');
          await page.clock.fastForward(30_000);
          await check(
            'unchanged alert becomes a last reading while its source caveat stays visible',
            async () => {
              await page.getByText('Last reading: CPU 91%, 10 mins ago', { exact: true }).waitFor();
              assert.equal(
                await page.getByText('CPU 91%, above the 80% alert level', { exact: true }).count(),
                0,
              );
              await page.getByText('About this reading', { exact: true }).waitFor();
            },
          );
          await page.clock.fastForward(60_000);
          await page.getByText('Last reading: CPU 91%, 11 mins ago', { exact: true }).waitFor();
          await capture('stale-alert-cache-source');
          await check(
            'four cache-inclusive sources retain the visible note without refreshing the held alert',
            async () => {
              for (const source of [
                'status-mem',
                'status-freemem',
                'cluster-resources',
                'derived-total-minus-used',
              ]) {
                await page.evaluate((source) => window.__heldOperator.memory(source), source);
                await page.getByText('About this reading', { exact: true }).waitFor();
                await page
                  .getByText('Last reading: CPU 91%, 11 mins ago', { exact: true })
                  .waitFor();
              }
            },
          );
          await check(
            'cache-aware QEMU and Pulse agent sources remove the cache note without rewriting alert evidence',
            async () => {
              for (const source of ['guest-agent-meminfo', 'agent']) {
                await page.evaluate((source) => window.__heldOperator.memory(source), source);
                assert.equal(
                  await page.getByText('About this reading', { exact: true }).count(),
                  0,
                );
                assert.match(
                  await page.getByTestId('guest-technical-details').innerText(),
                  /Current · (QEMU guest agent|Pulse Agent)/,
                );
                await page
                  .getByText('Last reading: CPU 91%, 11 mins ago', { exact: true })
                  .waitFor();
              }
            },
          );
          await check(
            'retained fallback stays last known and unavailable source withdraws caveat instead of inventing a number',
            async () => {
              await page.evaluate(() => window.__heldOperator.memory('status-mem', 'last-known'));
              assert.match(
                await page.getByTestId('guest-technical-details').innerText(),
                /Last known · Proxmox \(may include cache\)/,
              );
              await page.getByText('About this reading', { exact: true }).waitFor();
              await page.evaluate(() => window.__heldOperator.memory('status-mem', 'unavailable'));
              assert.match(
                await page.getByTestId('guest-technical-details').innerText(),
                /Unavailable · Proxmox/,
              );
              assert.equal(await page.getByText('About this reading', { exact: true }).count(), 0);
            },
          );
          await page.evaluate(() => window.__heldOperator.withdrawAlert());
          await page.clock.fastForward(30_000);
          await check('withdrawn alert stays withdrawn on the next tick', async () => {
            assert.equal(await page.getByTestId('drawer-attention-section').count(), 0);
          });
        }
        await check('no page error, unspecified API, off-origin traffic or mutation', async () => {
          assert.deepEqual(record.errors, []);
          assert.deepEqual(record.unexpected, []);
          assert.deepEqual(record.offOrigin, []);
          assert.deepEqual(record.writes, []);
          assert.equal(
            await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
            false,
          );
        });
        record.completed = true;
        await context.close();
        context = undefined;
        record.contextClosed = true;
      }
      await browser.close();
      browser = undefined;
    }
    result.completed = true;
  } catch (err) {
    result.error = { name: err.name, message: err.message };
    process.exitCode = 1;
  } finally {
    result.cleanupErrors = [];
    if (context) {
      try {
        await context.close();
        result.failedCaseContextClosed = true;
      } catch (err) {
        result.cleanupErrors.push({ phase: 'context-close', message: err.message });
      }
    }
    try {
      if (browser) await browser.close();
      result.browsersClosed = true;
    } catch (err) {
      result.cleanupErrors.push({ phase: 'browser-close', message: err.message });
    }
    try {
      await server.close();
      result.serverClosed = true;
    } catch (err) {
      result.cleanupErrors.push({ phase: 'server-close', message: err.message });
    }
    if (result.cleanupErrors.length) {
      result.completed = false;
      process.exitCode = 1;
    }
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        completed: result.completed,
        phase: result.phase,
        cases: result.cases.filter((r) => r.completed).length,
        captures: result.captures.length,
        error: result.error,
        output,
      }),
    );
  }
}

module.exports = run;
if (require.main === module) void run();
