// Browser proof for the production action dialog's non-mutating receipt re-read.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'browser-tests', 'action-receipt-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5202, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    const results = [];
    for (const width of [1365, 390]) {
      let mode = 'pending';
      const page = await browser.newPage({ viewport: { width, height: width === 390 ? 844 : 900 } });
      const pageErrors = [];
      const actionRequests = [];
      page.on('pageerror', (error) => pageErrors.push(error.message));
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5202') return route.abort();
        if (url.pathname === '/api/security/status') {
          return route.fulfill({ json: {
            hasAuthentication: true,
            requiresAuth: true,
            settingsCapabilities: { authenticationWrite: true },
          } });
        }
        if (url.pathname === '/api/actions/action-fixture-1') {
          actionRequests.push({ method: route.request().method(), mode });
          if (mode === 'error') return route.fulfill({ status: 503, json: { error: 'fixture read failed' } });
          if (mode === 'complete') {
            return route.fulfill({ json: {
              audit: {
                id: 'action-fixture-1',
                createdAt: new Date().toISOString(),
                updatedAt: new Date().toISOString(),
                state: 'completed',
                decisionRevision: 1,
                request: {
                  requestId: 'request-fixture-1', resourceId: 'docker:container:edge',
                  capabilityName: 'update', reason: 'Update edge to its latest image.', requestedBy: 'operator',
                },
                plan: {
                  actionId: 'action-fixture-1', requestId: 'request-fixture-1', allowed: true,
                  requiresApproval: false, rollbackAvailable: false,
                  expiresAt: new Date(Date.now() + 30 * 60_000).toISOString(),
                  planHash: 'sha256:fixture-plan', policyDecision: { status: 'resolved', authorities: [] },
                },
                result: {
                  success: true,
                  actionResultV2: {
                    execution: { status: 'succeeded' },
                    verification: { status: 'verified', evidenceClass: 'agent' },
                    compensation: { status: 'not_attempted', support: 'unavailable' },
                  },
                },
              },
              events: [],
              attempt: { id: 'attempt-fixture-1', actionId: 'action-fixture-1', state: 'receipt_recorded',
                createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(), dispatchCount: 1 },
              receipt: { receivedAt: new Date().toISOString() },
            } });
          }
          return route.fulfill({ json: {
            audit: {
              id: 'action-fixture-1', createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(),
              state: 'executing', decisionRevision: 1,
              request: { requestId: 'request-fixture-1', resourceId: 'docker:container:edge',
                capabilityName: 'update', reason: 'Update edge to its latest image.', requestedBy: 'operator' },
              plan: { actionId: 'action-fixture-1', requestId: 'request-fixture-1', allowed: true,
                requiresApproval: false, rollbackAvailable: false, expiresAt: new Date(Date.now() + 30 * 60_000).toISOString(),
                planHash: 'sha256:fixture-plan', policyDecision: { status: 'resolved', authorities: [] } },
            },
            events: [],
            attempt: { id: 'attempt-fixture-1', actionId: 'action-fixture-1', state: 'receipt_pending',
              createdAt: new Date(Date.now() - 5 * 60_000).toISOString(),
              updatedAt: new Date(Date.now() - 5 * 60_000).toISOString(), dispatchCount: 1 },
          } });
        }
        if (url.pathname.startsWith('/api/')) return route.abort();
        return route.continue();
      });
      await page.goto('http://127.0.0.1:5202/browser-tests/action-receipt-wait.html', {
        waitUntil: 'domcontentloaded', timeout: 120_000,
      });
      const dialog = page.getByRole('dialog', { name: 'Update' });
      await dialog.waitFor();
      await page.getByText('Receipt pending', { exact: true }).waitFor();
      assert.match(await page.getByTestId('action-receipt-waiting').innerText(), /Do not start the action again/);
      assert.equal(await page.getByTestId('action-stuck-recovery').count(), 0);
      await page.waitForTimeout(400); // Let the dialog entrance transition finish before capture.
      await page.screenshot({ path: path.join(artifacts, `pending-${width}.png`) });

      const refresh = page.getByRole('button', { name: 'Check for receipt' });
      if (width === 390) {
        await refresh.focus();
        await page.keyboard.press('Enter');
      } else {
        await refresh.click();
      }
      await page.waitForFunction(() => document.querySelector('[data-testid="action-receipt-waiting"]') !== null);
      assert.equal(actionRequests.length, 1);
      mode = 'error';
      await refresh.click();
      await page.getByRole('alert').filter({ hasText: 'No new action was sent' }).waitFor();
      await page.waitForTimeout(400);
      await page.screenshot({ path: path.join(artifacts, `read-error-${width}.png`) });

      mode = 'complete';
      await refresh.click();
      await page.getByText('Completed', { exact: true }).waitFor();
      assert.equal(await page.getByTestId('action-receipt-waiting').count(), 0);
      await page.waitForTimeout(400);
      await page.screenshot({ path: path.join(artifacts, `completed-${width}.png`) });
      assert.equal(actionRequests.length, 3);
      assert.ok(actionRequests.every((request) => request.method === 'GET'), JSON.stringify(actionRequests));
      assert.deepEqual(pageErrors, []);
      const dimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, inner: innerWidth }));
      assert.ok(dimensions.scroll <= dimensions.inner + 1, JSON.stringify(dimensions));
      results.push({ width, actionRequests, pageErrors, dimensions });
      await page.close();
    }
    console.log(JSON.stringify({ result: 'passed', browser: browser.version(), playwright: '1.56.1', results }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
