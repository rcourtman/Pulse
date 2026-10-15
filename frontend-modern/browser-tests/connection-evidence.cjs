const { chromium } = require('playwright');
const { expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/connection-evidence/static');
const output = path.resolve('tmp/connection-evidence/browser');
fs.mkdirSync(output, { recursive: true });
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
for (const [file, digest] of Object.entries(binding.sources))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Source changed: ${file}`);
const assets = new Map(
  Object.entries(binding.artifacts).map(([file, digest]) => {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, `Artifact changed: ${file}`);
    return ['/' + file, bytes];
  }),
);
const result = {
  source: binding.source_sha,
  playwright: require('playwright/package.json').version,
  inputs: Object.keys(binding.sources).length,
  artifacts: assets.size,
  journeys: [],
  captures: [],
  errors: [],
  cleanup: {},
};
const server = http.createServer((request, response) => {
  const pathname = new URL(request.url, 'http://127.0.0.1').pathname;
  const file =
    pathname === '/settings/infrastructure' ? '/browser-tests/connection-evidence.html' : pathname;
  const bytes = assets.get(file);
  if (!bytes) {
    response.writeHead(404);
    response.end();
    return;
  }
  response.setHeader(
    'Content-Type',
    file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html',
  );
  response.setHeader('Cache-Control', 'no-store');
  response.end(bytes);
});
const saved = {
  id: 'saved-alpha',
  name: 'Synthetic appliance',
  host: 'alpha.example.invalid',
  port: 443,
  username: 'fixture',
  password: '********',
  apiKey: '********',
  useHttps: true,
  insecureSkipVerify: false,
  enabled: true,
  monitorVms: true,
  monitorHosts: true,
  monitorDatastores: true,
  monitorDatasets: true,
  monitorPools: true,
  monitorReplication: true,
};
const impact = (name) => ({
  current_count: 1,
  projected_count: 2,
  additional_count: 1,
  effect: 'adds_new',
  current_systems: [],
  projected_systems: [
    {
      name,
      type: 'synthetic-system',
      status: 'online',
      source: 'fixture',
      status_explanation: { summary: '', reasons: [] },
      latest_included_signal: { name: '', type: '', at: '' },
      explanation: { summary: '', reasons: [], surfaces: [] },
    },
  ],
  current_system: null,
  projected_system: null,
});
let browser;
(async () => {
  try {
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    browser = await chromium.launch({
      headless: true,
      channel: 'chromium',
      args: ['--no-sandbox'],
    });
    result.browser = browser.version();
    for (const platform of ['truenas', 'vmware'])
      for (const viewport of [
        { width: 1360, height: 900 },
        { width: 390, height: 900 },
      ]) {
        const narrow = viewport.width < 768;
        const context = await browser.newContext({
          viewport,
          hasTouch: narrow,
          isMobile: narrow,
          deviceScaleFactor: 1,
        });
        await context.addCookies([{ name: 'pulse_csrf', value: 'fixture-csrf', url: origin }]);
        const page = await context.newPage();
        page.on('pageerror', (error) => result.errors.push(error.message));
        const pending = [],
          requests = [];
        await page.route('**/*', async (route) => {
          const request = route.request(),
            url = new URL(request.url());
          assert.equal(url.origin, origin, 'Off-origin request');
          if (!url.pathname.startsWith('/api/')) return route.continue();
          const base = `/api/${platform}/connections`;
          if (url.pathname === base && request.method() === 'GET')
            return route.fulfill({
              contentType: 'application/json',
              body: JSON.stringify([
                saved,
                { ...saved, id: 'saved-beta', host: 'beta.example.invalid' },
              ]),
            });
          assert(['POST', 'PUT'].includes(request.method()), 'Only explicit connection operations');
          assert(
            ['saved-alpha', 'saved-beta'].some((id) =>
              [base + '/' + id + '/test', base + '/' + id + '/preview', base + '/' + id].includes(
                url.pathname,
              ),
            ),
            'Only explicit fixture connection operations',
          );
          const data = request.postDataJSON();
          assert.equal(data.insecureSkipVerify, false, 'Trust default preserved');
          assert.equal(
            platform === 'truenas' ? data.apiKey : data.password,
            '********',
            'No saved secret in browser',
          );
          requests.push({ method: request.method(), path: url.pathname, body: data });
          await new Promise((resolve) =>
            pending.push(async (body, status = 200) => {
              await route.fulfill({
                status,
                contentType: 'application/json',
                body: JSON.stringify(body),
              });
              resolve();
            }),
          );
        });
        const activate = async (locator) => {
          if (narrow) await locator.tap();
          else {
            await locator.focus();
            await page.keyboard.press('Enter');
          }
        };
        const respond = async (body, status = 200) => {
          await expect.poll(() => pending.length).toBe(1);
          await pending.shift()(body, status);
        };
        const host = page.getByRole('textbox', { name: 'Host', exact: true });
        const test = page.getByRole('button', { name: 'Test connection', exact: true });
        const preview = page.getByRole('button', { name: 'Preview impact', exact: true });
        const notifications = page.getByRole('region', { name: 'Notifications' });
        const noFeedback = async () => {
          await expect(notifications.getByRole('status')).toHaveCount(0);
          await expect(notifications.getByRole('alert')).toHaveCount(0);
          await expect(page.getByTestId('vmware-connection-test-feedback')).toHaveCount(0);
          await expect(page.getByTestId('vmware-connection-test-warning')).toHaveCount(0);
          await expect(page.getByText(/Pulse currently counts/)).toHaveCount(0);
          await expect(page.getByText(/Obsolete/)).toHaveCount(0);
        };
        const capture = async (state, anchor) => {
          await anchor.scrollIntoViewIfNeeded();
          await expect
            .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
            .toBe(true);
          const file = path.join(output, `${platform}-${viewport.width}-${state}.png`);
          await page.screenshot({ path: file });
          result.captures.push({
            file: path.relative(process.cwd(), file),
            sha256: hash(fs.readFileSync(file)),
            platform,
            viewport,
            state,
          });
        };
        await page.goto(`${origin}/settings/infrastructure?platform=${platform}`, {
          waitUntil: 'networkidle',
        });
        await expect(host).toHaveValue(saved.host);
        assert.equal(requests.length, 0, 'No automatic read');
        // Edit A → B → A while the check is genuinely pending. No late success,
        // degraded warning or duplicate check can become evidence for the form.
        await activate(test);
        await expect.poll(() => pending.length).toBe(1);
        await host.fill('beta.example.invalid');
        await host.fill(saved.host);
        await expect(page.getByRole('button', { name: 'Testing…', exact: true })).toBeDisabled();
        await expect(preview).toBeDisabled();
        assert.equal(requests.length, 1);
        await respond({
          success: true,
          degraded: true,
          issueCount: 1,
          issues: [{ message: 'Obsolete optional read' }],
        });
        await expect(test).toBeEnabled();
        await noFeedback();
        await capture('obsolete-test-retired', host);
        // A late TLS failure is also obsolete after a trust-relevant edit.
        await activate(test);
        await expect.poll(() => pending.length).toBe(1);
        await host.fill('beta.example.invalid');
        await respond(
          {
            code: `${platform}_connection_failed`,
            message: 'Obsolete failure',
            details: { category: 'tls', error: 'Obsolete certificate failure' },
          },
          400,
        );
        await expect(test).toBeEnabled();
        await noFeedback();
        // A current failure must remain actionable, and then clear on editing.
        await activate(test);
        await respond(
          {
            code: `${platform}_connection_failed`,
            message: 'Current connection failed',
            details: { category: 'tls', error: 'Current certificate chain is untrusted' },
          },
          400,
        );
        await expect(
          notifications.getByText('Current certificate chain is untrusted'),
        ).toBeVisible();
        await capture('current-failure-kept', host);
        await activate(notifications.getByRole('button', { name: 'Dismiss notification' }));
        await expect(notifications.getByRole('alert')).toHaveCount(0);
        await host.fill('gamma.example.invalid');
        await noFeedback();
        // Preview completion must be retired on A → B → A as well, not just by
        // comparing the final endpoint string with the request's string.
        await activate(preview);
        await expect.poll(() => pending.length).toBe(1);
        await expect(test).toBeDisabled();
        await host.fill('delta.example.invalid');
        await host.fill('gamma.example.invalid');
        await respond(impact('Obsolete systems'));
        await expect(preview).toBeEnabled();
        await noFeedback();
        // Cancel is allowed during a read-only preview. A reopened editor must
        // stay busy until the old request settles and discard its late error.
        await activate(preview);
        await expect.poll(() => pending.length).toBe(1);
        await activate(page.getByRole('button', { name: 'Cancel', exact: true }));
        await activate(page.getByRole('button', { name: 'Reopen editor', exact: true }));
        await expect(host).toHaveValue(saved.host);
        await expect(test).toBeDisabled();
        await respond(
          {
            code: 'monitored_system_usage_unavailable',
            details: { reason: 'supplemental_inventory_unsettled' },
            message: 'Obsolete preview unavailable',
          },
          503,
        );
        await expect(preview).toBeEnabled();
        await noFeedback();
        // Settings keeps the hook alive when its consumer departs. Its test
        // must not toast or publish a warning in the later remounted editor.
        await activate(test);
        await expect.poll(() => pending.length).toBe(1);
        await activate(page.getByRole('button', { name: 'Leave editor', exact: true }));
        await respond({ success: true });
        await activate(page.getByRole('button', { name: 'Reopen editor', exact: true }));
        await expect(test).toBeEnabled();
        await noFeedback();
        // A target identity change updates the real slot even while an old
        // connection check remains in flight; the response cannot revive it.
        await activate(test);
        await expect.poll(() => pending.length).toBe(1);
        await activate(page.getByRole('button', { name: 'Switch to beta', exact: true }));
        await expect(host).toHaveValue('beta.example.invalid');
        await expect(page.getByRole('button', { name: 'Testing…', exact: true })).toBeDisabled();
        await respond({ success: true });
        await expect(test).toBeEnabled();
        await noFeedback();
        // An already-dispatched save still completes once. It must not close
        // or erase the subsequently selected connection's newer draft.
        const update = page.getByRole('button', { name: 'Update connection', exact: true });
        await activate(update);
        await expect.poll(() => pending.length).toBe(1);
        assert.equal(requests.at(-1).method, 'PUT');
        assert.equal(requests.at(-1).path, `/api/${platform}/connections/saved-beta`);
        await activate(page.getByRole('button', { name: 'Switch to alpha', exact: true }));
        await expect(host).toHaveValue(saved.host);
        await host.fill('current.example.invalid');
        await respond({ ...saved, id: 'saved-beta', host: 'beta.example.invalid' });
        await expect(update).toBeEnabled();
        await expect(host).toHaveValue('current.example.invalid');
        await expect(
          notifications.getByText(
            `Saved earlier ${platform === 'truenas' ? 'TrueNAS' : 'VMware'} settings for beta.example.invalid.`,
          ),
        ).toBeVisible();
        await activate(notifications.getByRole('button', { name: 'Dismiss notification' }));
        await expect(notifications.getByRole('status')).toHaveCount(0);
        await noFeedback();
        // Fresh explicit work still works, and the serialized endpoint and TLS
        // policy are those displayed in the actual consuming editor.
        await host.fill('current.example.invalid');
        await activate(preview);
        await respond(impact('Current synthetic systems'));
        await expect(page.getByText(/Pulse currently counts 1 monitored system/)).toBeVisible();
        await expect(page.getByText(/Current synthetic systems/)).toBeVisible();
        assert.equal(requests.at(-1).body.host, 'current.example.invalid');
        await capture('current-impact-kept', preview);
        await host.fill('next.example.invalid');
        await expect(page.getByText(/Pulse currently counts/)).toHaveCount(0);
        const count = requests.length;
        assert.equal(count, 9);
        result.journeys.push({
          platform,
          viewport,
          interaction: narrow ? 'touch' : 'keyboard',
          requests,
          assertions: [
            'no automatic request or trust weakening',
            'late success/degraded/error retired',
            'A-B-A cannot revive tests or previews',
            'test/preview mutually busy',
            'cancel/reopen retains busy until settlement',
            'departed consumer retires feedback while Settings lives',
            'target identity changes initialise the actual slot without accepting old work',
            'old save completes once without closing or overwriting the newer draft',
            'fresh current error and impact remain usable',
            'edit clears completed impact',
            'no horizontal overflow',
          ],
          status: 'passed',
        });
        await context.close();
      }
    assert.deepEqual(result.errors, []);
    result.status = 'passed';
  } catch (error) {
    result.status = 'failed';
    result.failure = error.stack || String(error);
    process.exitCode = 1;
  } finally {
    if (browser) {
      await browser.close();
      result.cleanup.browser_closed = true;
    }
    await new Promise((resolve) => server.close(resolve));
    result.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    process.stdout.write(JSON.stringify(result, null, 2) + '\n');
  }
})();
