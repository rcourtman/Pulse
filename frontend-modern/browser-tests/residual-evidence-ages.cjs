// Runs the same preselected matrix on exact parent bytes and final bytes.
// Synthetic local responses only; no external service or credential is used.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const start = Date.parse('2026-10-11T11:00:00Z');
const origin = 'http://127.0.0.1:5349';
const hash = (p) => crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
const iso = (n) => new Date(n).toISOString();

(async () => {
  const binding = JSON.parse(fs.readFileSync('/workspace/tmp/residual-ages/binding.json'));
  const root = '/workspace/' + binding.frontend_root;
  const output = '/workspace/tmp/residual-ages/' + binding.mode;
  fs.mkdirSync(output, { recursive: true });
  for (const [p, h] of Object.entries(binding.content_sha256))
    assert.equal(hash(root + '/' + p.replace('frontend-modern/', '')), h, p);
  const result = {
    binding,
    playwright: require('playwright/package.json').version,
    cases: [],
    captures: [],
    cleanup: {},
    completed: false,
    limits:
      'Production panels/state/adapters/CSS with synthetic providers and responses, not the full application, native recovery, physical phone, publication or release. Parent observations establish stale ages, not a passing repair.',
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
    server: { host: '127.0.0.1', port: 5349, strictPort: true, watch: null },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  let browser, currentPage;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1680, false],
      ['phone', webkit, 390, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      for (const surface of binding.surfaces ?? [
        'resource',
        'operator',
        'tokens',
        'profiles',
        'drafts',
      ]) {
        const page = await browser.newPage({
          viewport: { width, height: 950 },
          isMobile: width < 500,
          hasTouch: width < 500,
          locale: 'en-GB',
          timezoneId: 'UTC',
        });
        currentPage = page;
        page.setDefaultTimeout(7000);
        const record = {
          name,
          surface,
          browser: browser.version(),
          width,
          dark,
          checks: [],
          reads: [],
          computePosts: [],
          writes: [],
          unexpectedReads: [],
          offOrigin: [],
          errors: [],
        };
        result.cases.push(record);
        let tokenTime = start - 60_000,
          agentTime = start - 60_000,
          draftNumber = 0;
        const drafts = [];
        record.consoleErrors = [];
        page.on('console', (message) => {
          if (message.type() === 'error') record.consoleErrors.push(message.text().slice(0, 600));
        });
        page.on('pageerror', (err) => record.errors.push(err.message));
        await page.clock.install({ time: new Date(start - 1000) });
        await page.clock.pauseAt(new Date(start));
        await page.addInitScript((dark) => {
          localStorage.clear();
          document.addEventListener(
            'DOMContentLoaded',
            () => document.documentElement.classList.toggle('dark', dark),
            { once: true },
          );
        }, dark);
        await page.route('**/*', async (route) => {
          const req = route.request(),
            url = new URL(req.url()),
            p = url.pathname;
          if (url.origin !== origin) {
            record.offOrigin.push(url.origin);
            return route.abort();
          }
          if (!p.startsWith('/api/')) return route.continue();
          if (
            req.method() === 'POST' &&
            ['/api/admin/profiles/suggestions', '/api/admin/profiles/validate'].includes(p)
          ) {
            record.computePosts.push(p);
            if (p.endsWith('/validate'))
              return route.fulfill({ json: { Valid: true, Errors: [], Warnings: [] } });
            draftNumber++;
            const draft = {
              name: draftNumber === 1 ? 'First saved draft' : 'Second saved draft',
              description: 'Synthetic draft',
              config: { interval: draftNumber === 1 ? '30s' : '1m' },
              rationale: [],
            };
            drafts.push(draft);
            return route.fulfill({ json: draft });
          }
          if (req.method() !== 'GET') {
            record.writes.push({ p, method: req.method() });
            return route.fulfill({
              status: 403,
              json: { error: 'Synthetic fixture refuses writes' },
            });
          }
          record.reads.push(p);
          let data;
          if (p === '/api/security/status')
            data = { hasAuthentication: true, requiresAuth: false, sessionCapabilities: {} };
          else if (p === '/api/health') data = { status: 'ok' };
          else if (p === '/api/license/runtime-capabilities')
            data = {
              capabilities: ['agent_profiles'],
              limits: [],
              max_history_days: 7,
              hosted_mode: false,
              blocked_capabilities: [],
              runtime: { build: 'community' },
            };
          else if (p === '/api/resources')
            data = {
              data: [
                {
                  id: 'fixture-agent',
                  type: 'agent',
                  name: 'Held profile host',
                  displayName: 'Held profile host',
                  platformId: 'fixture-agent',
                  platformType: 'agent',
                  sourceType: 'agent',
                  status: 'online',
                  lastSeen: iso(agentTime),
                  identity: { hostname: 'fixture-host' },
                  agent: { agentId: 'fixture-agent' },
                },
              ],
              meta: { totalPages: 1 },
            };
          else if (p === '/api/security/tokens')
            data = {
              tokens: [
                {
                  id: 'fixture-token',
                  name: 'Held token',
                  prefix: 'fixture',
                  suffix: '0000',
                  createdAt: iso(tokenTime),
                  scopes: ['monitoring:read'],
                },
              ],
            };
          else if (p === '/api/agent/capabilities')
            data = {
              version: 'v1',
              requiredScopes: ['monitoring:read'],
              capabilities: [],
              categories: [],
            };
          else if (p === '/api/admin/profiles/')
            data = [
              {
                id: 'profile-a',
                name: 'Profile A',
                config: { interval: '30s' },
                created_at: iso(start),
                updated_at: iso(start),
              },
            ];
          else if (p === '/api/admin/profiles/assignments')
            data = [{ agent_id: 'fixture-agent', profile_id: 'profile-a', updated_at: iso(start) }];
          else if (p === '/api/admin/profiles/schema')
            data = [
              {
                Key: 'interval',
                Type: 'duration',
                Description: 'Polling interval',
                DefaultValue: '30s',
                Required: false,
              },
            ];
          else if (p.endsWith('/operator-state'))
            data = {
              configured: true,
              state: {
                canonicalId: 'held-policy',
                intentionallyOffline: true,
                monitoringMode: 'expected_offline',
                lifecycleState: 'retired',
                neverAutoRemediate: true,
                note: 'Keep saved policy',
                setAt: iso(start - 60_000),
                setBy: 'fixture-admin',
              },
            };
          else if (p.endsWith('/maintenance-verifications')) data = { reports: [] };
          else {
            record.unexpectedReads.push(p);
            return route.fulfill({ status: 404, json: { error: 'Unspecified synthetic read' } });
          }
          return route.fulfill({ json: data });
        });
        const activate = async (el) => {
          await el.focus();
          return width < 500 ? el.tap() : el.press('Enter');
        };
        const capture = async (label) => {
          const file = output + '/' + name + '-' + surface + '-' + label + '.png';
          await page.screenshot({ path: file, fullPage: false, animations: 'disabled' });
          result.captures.push({
            path: file.replace('/workspace/', ''),
            sha256: hash(file),
            bytes: fs.statSync(file).size,
          });
        };
        await page.goto(origin + '/browser-tests/residual-evidence-ages.html?surface=' + surface, {
          waitUntil: 'domcontentloaded',
          timeout: 45000,
        });
        let age,
          compact = false,
          target,
          baseline,
          initialSnapshot;
        if (surface === 'resource') {
          age = page.getByTestId('resource-change-history-section');
          await age.waitFor();
          assert.match(await age.innerText(), /Occurred 2 mins ago/);
          await activate(age.getByRole('button', { name: 'Filter history', exact: true }));
          target = age.getByRole('combobox', { name: 'Change kind' });
          assert.match(await page.getByTestId('held-summary').innerText(), /1m ago/);
        } else if (surface === 'operator') {
          await page.getByText('Set by fixture-admin', { exact: false }).waitFor();
          age = page.getByText('Set by fixture-admin', { exact: false }).locator('..');
          compact = true;
          target = page.getByLabel('Operator note', { exact: true });
          await target.fill('Unsaved operator note');
        } else if (surface === 'tokens') {
          // Both projections are mounted; desktop CSS hides the compact card
          // from the accessibility tree, so select its DOM identity explicitly.
          const card = page.locator('[data-api-token-card="fixture-token"]');
          await card.waitFor({ state: 'attached' });
          age =
            width < 500
              ? card.getByText('Created', { exact: true }).locator('..').locator('dd')
              : page
                  .locator('tr')
                  .filter({ hasText: 'Held token' })
                  .locator('td')
                  .nth(
                    await page
                      .getByRole('columnheader', { name: 'Created', exact: true })
                      .evaluate((el) => el.cellIndex),
                  );
          await activate(page.getByRole('button', { name: 'Generate new', exact: true }));
          target = page.getByLabel('Token name', { exact: true });
          await target.fill('Unsaved token name');
        } else if (surface === 'profiles') {
          const row = page.locator('tr').filter({ hasText: 'Held profile host' });
          await row.waitFor({ state: 'attached' });
          await page.clock.runFor(2000); // Finish the existing background canonical revalidation before the quiet interval.
          age = row.locator('td').last();
          target = row.getByRole('combobox');
          assert.equal(await target.inputValue(), 'profile-a');
          if (width < 500)
            assert.equal(await age.isVisible(), false, 'existing phone column stays hidden');
        } else {
          target = page.getByPlaceholder('Describe the agents and use case for this profile...');
          await target.fill('Original draft request');
          await activate(page.getByRole('button', { name: 'Get Ideas', exact: true }));
          await page.getByRole('heading', { name: 'First saved draft', exact: true }).waitFor();
          await page.clock.fastForward(60_000);
          await target.fill('Second draft request');
          await activate(page.getByRole('button', { name: 'Try Again', exact: true }));
          await page.getByRole('heading', { name: 'Second saved draft', exact: true }).waitFor();
          age = page.getByRole('button', { name: /First saved draft/ });
          target = age;
        }
        const text = async () => (await age.textContent()).trim();
        assert.match(await text(), compact ? /1m ago/ : /1 min ago/);
        await target.focus();
        const element = await target.elementHandle();
        initialSnapshot = await page.evaluate(() => window.__residualAges.snapshot());
        baseline = { reads: record.reads.length, computePosts: record.computePosts.length };
        await (surface === 'profiles' && width < 500 ? target : age).scrollIntoViewIfNeeded();
        await capture('initial');
        await page.clock.fastForward(2 * 3_600_000);
        const held = await text();
        assert.match(
          held,
          binding.mode === 'parent'
            ? compact
              ? /1m ago/
              : /1 min ago/
            : compact
              ? /2h ago/
              : /2 hours ago/,
        );
        assert(
          await element.evaluate((el) => el === document.activeElement),
          'focused control survives tick',
        );
        assert.equal(
          await page.evaluate(() => window.__residualAges.snapshot()),
          initialSnapshot,
          'clock changes no evidence',
        );
        assert.equal(record.reads.length, baseline.reads, 'no age-triggered API read');
        assert.equal(
          record.computePosts.length,
          baseline.computePosts,
          'no age-triggered computation',
        );
        if (surface === 'resource') {
          assert.match(
            held,
            binding.mode === 'parent' ? /Occurred 2 mins ago/ : /Occurred 2 hours ago/,
          );
          assert.match(
            await page.getByTestId('held-summary').innerText(),
            binding.mode === 'parent' ? /1m ago/ : /2h ago/,
          );
        } else if (surface === 'operator') {
          assert.equal(await target.inputValue(), 'Unsaved operator note');
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
        } else if (surface === 'tokens')
          assert.equal(await target.inputValue(), 'Unsaved token name');
        else if (surface === 'profiles') assert.equal(await target.inputValue(), 'profile-a');
        record.checks.push({
          held,
          initialSnapshot,
          baseline,
          unchangedEvidenceFocusAndRequests: true,
        });
        // Last Seen is deliberately hidden on phones. Capture its visible
        // assignment control without pretending the hidden age is on screen.
        await (surface === 'profiles' && width < 500 ? target : age).scrollIntoViewIfNeeded();
        await capture('held-two-hours');
        if (binding.mode === 'final') {
          if (surface === 'resource') {
            await page.evaluate(() => window.__residualAges.replaceChange());
            assert.match(await age.innerText(), /0s ago/);
            assert.doesNotMatch(await age.innerText(), /Occurred/);
            await page.clock.fastForward(60_000);
            assert.match(await age.innerText(), /1 min ago/);
            await page.evaluate(() => window.__residualAges.removeChange());
            await page.clock.fastForward(30_000);
            assert.equal(
              await page.getByText('Retained retention change', { exact: true }).count(),
              0,
            );
          } else if (surface === 'tokens') {
            tokenTime = await page.evaluate(() => Date.now());
            await page.evaluate(() => window.__residualAges.refreshTokens());
            await page.waitForFunction(() =>
              [...document.querySelectorAll('[data-api-token-card] dd')].some(
                (el) => el.textContent === '0s ago',
              ),
            );
            assert.equal(await target.inputValue(), 'Unsaved token name');
            await page.clock.fastForward(60_000);
            assert.match(await text(), /1 min ago/);
          } else if (surface === 'profiles') {
            agentTime = await page.evaluate(() => Date.now());
            await page.evaluate(() => window.__residualAges.replaceAgent());
            await page.waitForFunction(() =>
              [...document.querySelectorAll('td')].some((el) => el.textContent === '0s ago'),
            );
            assert.equal(await target.inputValue(), 'profile-a');
            await page.clock.fastForward(60_000);
            assert.match(await text(), /1 min ago/);
          } else if (surface === 'drafts') {
            await activate(age);
            await page.getByRole('heading', { name: 'First saved draft', exact: true }).waitFor();
            assert.equal(
              await page
                .getByPlaceholder('Describe the agents and use case for this profile...')
                .inputValue(),
              'Original draft request',
            );
            await activate(page.getByRole('button', { name: 'Use This Profile', exact: true }));
            assert.deepEqual(
              await page.evaluate(() => window.__residualAges.accepted()),
              drafts[0],
            );
            assert.equal(draftNumber, 2, 'no extra suggestion');
          }
          record.checks.push({ replacementOrOriginalDraftAcceptance: true });
        }
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          true,
          'no horizontal page overflow',
        );
        assert.deepEqual(record.errors, []);
        assert.deepEqual(record.writes, []);
        assert.deepEqual(record.offOrigin, []);
        assert.deepEqual(record.unexpectedReads, []);
        await page.close();
        record.pageClosed = true;
      }
      await browser.close();
      browser = null;
    }
    result.completed = true;
  } catch (err) {
    if (currentPage && !currentPage.isClosed()) {
      result.failureBody = (await currentPage.locator('body').innerText()).slice(0, 12000);
      await currentPage.screenshot({ path: output + '/failure.png', fullPage: false });
    }
    result.error = String(err.stack || err);
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    await server.close();
    result.cleanup.serverClosed = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        mode: binding.mode,
        completed: result.completed,
        cases: result.cases.length,
        error: result.error,
        cleanup: result.cleanup,
      }),
    );
  }
})();
