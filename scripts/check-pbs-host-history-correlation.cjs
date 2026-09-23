// Offline real-browser regression for #1723.
//
// Exercises PBS-only, bare-metal PVE/PBS side-by-side, and a PVE guest plus
// standalone representation of one agent. Each has two datastores. Switches
// rows and applies timer-driven fresh snapshots in stable and reversed order.
// Checks the drawer identity, selected History tab, populated chart paths and
// exact host metrics target at desktop/mobile widths. Synthetic API responses
// prove UI behaviour, not backend persistence or installed release behaviour.
// Run: pulse-worker-browser scripts/check-pbs-host-history-correlation.cjs
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');

const ROOT = path.resolve(
  process.cwd(),
  process.env.PBS_HISTORY_SOURCE_ROOT || '.',
  'frontend-modern',
);
const launchOptions = {
  headless: true,
  channel: 'chromium',
  args: ['--no-sandbox'],
};

(async () => {
  process.chdir(ROOT);
  const { createServer } = await import(
    path.join(ROOT, 'node_modules', 'vite', 'dist', 'node', 'index.js')
  );
  const server = await createServer({
    root: ROOT,
    configFile: path.join(ROOT, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5198, strictPort: true },
  });
  let browser;
  const results = [];
  const pageErrors = [];
  const failures = [];
  const topologies = ['pbs-only', 'side-by-side', 'guest'];
  const wrong = 'agent/pbs-1';
  // Reporter avsdev-cw sees the Identity card's "Discovery" and "Metrics
  // Target" rows flick between a service id and the correlated host id while
  // the table sits idle. Read the rendered rows so a snapshot-driven identity
  // change fails the check instead of only being visible to a human.
  const readIdentityRows = (detail) =>
    detail
      .locator('[data-testid="resource-identity-section"] tr')
      .evaluateAll((rows) =>
        rows
          .map((row) => {
            const cells = row.querySelectorAll('td');
            return cells.length >= 2
              ? [cells[0].textContent.trim(), cells[1].textContent.trim()]
              : null;
          })
          .filter((row) => row && row[0]),
      );
  try {
    await server.listen();
    browser = await chromium.launch(launchOptions);
    for (const topology of topologies) {
      for (const order of ['stable', 'reordered']) {
        const expected =
          topology === 'guest' ? 'vm/proxmox:100' : 'agent/agent-proxback';
        for (const viewport of [
          { width: 1280, height: 900 },
          { width: 390, height: 844 },
        ]) {
          const targets = [];
          const observations = [];
          const caseName = `${topology}-${order}-${viewport.width}`;
          const page = await browser.newPage({ viewport });
          page.on('pageerror', (error) =>
            pageErrors.push(`${viewport.width}px: ${error.message}`),
          );

          await page.route('**/api/license/runtime-capabilities', (route) =>
            route.fulfill({
              json: {
                capabilities: [],
                limits: [],
                hosted_mode: false,
                max_history_days: 7,
                runtime: {
                  build: 'community',
                  label: 'Pulse Community runtime',
                },
                blocked_capabilities: [],
              },
            }),
          );
          await page.route('**/api/settings/ai', (route) =>
            route.fulfill({ json: { enabled: false } }),
          );

          await page.route('**/api/metrics-store/history?**', async (route) => {
            const query = new URL(route.request().url()).searchParams;
            targets.push(
              `${query.get('resourceType')}/${query.get('resourceId')}`,
            );
            await route.fulfill({
              json: {
                resourceType: query.get('resourceType'),
                resourceId: query.get('resourceId'),
                range: query.get('range'),
                start: Date.now() - 3600000,
                end: Date.now(),
                source: 'store',
                metrics: Object.fromEntries(
                  [
                    'cpu',
                    'memory',
                    'disk',
                    'netin',
                    'netout',
                    'diskread',
                    'diskwrite',
                  ].map((metric) => [
                    metric,
                    [30, 20, 10].map((minutes, i) => ({
                      timestamp: Date.now() - minutes * 60000,
                      value: 10 + i * 5,
                      min: 10 + i * 5,
                      max: 10 + i * 5,
                    })),
                  ]),
                ),
              },
            });
          });

          try {
            await page.goto(
              `http://127.0.0.1:5198/browser-tests/pbs-host-history-correlation.html?topology=${topology}&order=${order}`,
              { timeout: 60000 },
            );
            const rows = page.locator('tr').filter({
              has: page.getByRole('button', {
                name: 'Expand details for proxback',
                exact: true,
              }),
            });
            await rows.first().waitFor({ timeout: 20000 });
            const openDatastore = async (name) => {
              const row = page.locator('tr').filter({
                has: page.locator(`td[title="proxback · ${name}"]`),
              });
              await row.getByRole('button').focus();
              await page.keyboard.press('Enter');
              await page
                .getByRole('tab', { name: 'History', exact: true })
                .click();
              await page
                .locator('[data-testid="guest-history-plot"] path')
                .first()
                .waitFor();
              assert.equal(
                await page
                  .getByRole('tab', { name: 'History', exact: true })
                  .getAttribute('aria-selected'),
                'true',
              );
            };
            // Both datastore rows must use the same host series, including revisiting
            // the first row after its history component has been disposed.
            let retainedIdentity = [];
            let retainedPaths = 0;
            const detail = page.locator(
              '[data-inline-platform-resource-detail-for="pbs-1"]',
            );
            for (const [index, datastore] of [
              'tank',
              'archive',
              'tank',
            ].entries()) {
              assert.deepEqual(
                await page
                  .locator('td[title^="proxback · "]')
                  .evaluateAll((nodes) =>
                    nodes.map((node) => node.getAttribute('title')),
                  ),
                ['proxback · archive', 'proxback · tank'],
              );
              await openDatastore(datastore);
              await detail.evaluate((node) => {
                node.dataset.proofIdentity = 'retained';
              });
              const pathsBefore = await detail
                .locator('[data-testid="guest-history-plot"] path')
                .count();
              assert.ok(pathsBefore > 0);
              const identityBefore = await readIdentityRows(detail);
              const identityLabels = identityBefore.map(([label]) => label);
              assert.ok(
                identityLabels.includes('Discovery'),
                'Discovery identity row missing from the drawer',
              );
              assert.ok(
                identityLabels.includes('Metrics Target'),
                'Metrics Target identity row missing from the drawer',
              );
              await page
                .getByRole('button', { name: 'Schedule automatic snapshot' })
                .click();
              // Return focus to History before the timer-driven data replacement.
              await page
                .getByRole('tab', { name: 'History', exact: true })
                .focus();
              await page
                .getByRole('status', { name: 'Snapshot number' })
                .filter({ hasText: String(index + 1) })
                .waitFor();
              // Datastore usage is visible at both widths and proves that the fresh
              // snapshot reached the UI (the CPU column is hidden on narrow screens).
              await page
                .getByText(`${(41 + index).toFixed(1)}%`, { exact: true })
                .first()
                .waitFor();
              assert.equal(
                await detail.getAttribute('data-proof-identity'),
                'retained',
                'drawer remounted',
              );
              assert.equal(
                await page
                  .getByRole('tab', { name: 'History', exact: true })
                  .getAttribute('aria-selected'),
                'true',
                'history reset after snapshot',
              );
              assert.equal(
                await detail
                  .locator('[data-testid="guest-history-plot"] path')
                  .count(),
                pathsBefore,
              );
              const identityAfter = await readIdentityRows(detail);
              assert.deepEqual(
                identityAfter,
                identityBefore,
                'Identity rows changed after a refreshed snapshot',
              );
              retainedIdentity = identityAfter;
              retainedPaths = pathsBefore;
              observations.push({
                datastore,
                paths: pathsBefore,
                identity: identityAfter,
                refreshPreservedHistory: true,
              });
            }

            // A live refresh can briefly omit the correlated host row while the
            // PBS server row stays. That must not flip the drawer's Identity or
            // History target to the PBS service key (#1723).
            await page.getByRole('button', { name: 'Drop correlated host' }).click();
            await page
              .getByRole('status', { name: 'Snapshot number' })
              .filter({ hasText: '4' })
              .waitFor();
            assert.equal(
              await detail.getAttribute('data-proof-identity'),
              'retained',
              'drawer remounted after the host row was omitted',
            );
            assert.equal(
              await page
                .getByRole('tab', { name: 'History', exact: true })
                .getAttribute('aria-selected'),
              'true',
              'history reset after the host row was omitted',
            );
            assert.equal(
              await detail
                .locator('[data-testid="guest-history-plot"] path')
                .count(),
              retainedPaths,
              'history chart lost its series when the host row was omitted',
            );
            assert.deepEqual(
              await readIdentityRows(detail),
              retainedIdentity,
              'Identity rows changed when the host row was transiently omitted',
            );
            observations.push({
              hostRowOmitted: true,
              identity: retainedIdentity,
              refreshPreservedHistory: true,
            });
            await page.getByRole('button', { name: 'Restore correlated host' }).click();
            await page
              .getByRole('status', { name: 'Snapshot number' })
              .filter({ hasText: '5' })
              .waitFor();

            if (!targets.includes(expected)) {
              throw new Error(
                `${viewport.width}px: expected the host history target ${expected}; recorded ${JSON.stringify(targets)}`,
              );
            }
            if (targets.includes(wrong)) {
              throw new Error(
                `${viewport.width}px: PBS service target ${wrong} was requested instead of the host series`,
              );
            }

            results.push({
              topology,
              order,
              viewport,
              expected,
              targets,
              observations,
              result: 'passed',
            });
          } catch (error) {
            failures.push(`${caseName}: ${error.message}`);
            results.push({
              topology,
              order,
              viewport,
              expected,
              targets,
              observations,
              result: 'failed',
              error: error.message,
            });
          } finally {
            await page.screenshot({
              path: path.join(
                ROOT,
                'browser-tests',
                `pbs-host-history-${caseName}.png`,
              ),
            });
            await page.close();
          }
        }
      }
    }

    if (pageErrors.length > 0) {
      throw new Error(`page errors: ${pageErrors.join(' | ')}`);
    }

    console.log(
      JSON.stringify(
        {
          result: failures.length ? 'failed' : 'passed',
          browser: browser.version(),
          results,
          failures,
        },
        null,
        2,
      ),
    );
    assert.equal(failures.length, 0, failures.join(' | '));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error('FAILED:', error && error.stack ? error.stack : error);
  process.exitCode = 1;
});
