const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/diagnostics-safety-proof';
const origin = 'http://127.0.0.1:5293';
const runtime = [
  'frontend-modern/src/components/Settings/diagnosticsModel.ts',
  'frontend-modern/src/components/Settings/DiagnosticsPanel.tsx',
  'frontend-modern/src/components/Settings/DiagnosticsResultsPanel.tsx',
  'frontend-modern/src/components/Settings/useDiagnosticsPanelState.ts',
  'frontend-modern/src/utils/diagnosticsPresentation.ts',
];
// Only synthetic server-shaped fields, including identities omitted by the old
// export redactor. No credential or actual report is read by this fixture.
const payload = {
  version: '6.4.5',
  runtime: 'go',
  uptime: 300,
  nodes: [],
  pbs: [],
  errors: [],
  system: {
    os: 'linux',
    arch: 'amd64',
    goVersion: 'go1.26.7',
    numCPU: 4,
    numGoroutine: 30,
    memoryMB: 128,
  },
  apiTokens: {
    enabled: true,
    tokenCount: 2,
    recommendTokenSetup: false,
    tokens: [
      {
        id: 'synthetic-private-token-a',
        name: 'synthetic-private-purpose-a',
        hint: 'synthetic-private-hint',
      },
      {
        id: 'synthetic-private-token-b',
        name: 'synthetic-private-purpose-b',
        hint: 'synthetic-private-hint',
      },
    ],
    usage: [
      { tokenId: 'synthetic-private-token-b', agentCount: 1, agents: ['synthetic-private-agent'] },
      { tokenId: 'synthetic-private-token-a', agentCount: 1, agents: ['synthetic-private-agent'] },
    ],
  },
  nodeSnapshots: [
    {
      instance: 'synthetic-private-instance',
      node: 'synthetic-private-node',
      memorySource: 'agent',
      memory: { used: 256 },
      raw: { total: 512 },
    },
  ],
  guestSnapshots: [
    {
      instance: 'synthetic-private-instance',
      node: 'synthetic-private-node',
      name: 'synthetic-private-guest',
      vmid: 9501,
      guestType: 'vm',
      memorySource: 'agent',
      memory: { used: 128 },
      raw: { hostAgentUsed: 128 },
      notes: [],
    },
  ],
  memorySourceBreakdown: [
    {
      instance: 'synthetic-private-instance',
      scope: 'node',
      source: 'agent',
      trust: 'host',
      count: 1,
      fallback: false,
      fallbackReasons: [],
    },
  ],
  metricsStore: {
    enabled: true,
    status: 'buffering',
    bufferSize: 17,
    notes: ['dial tcp 10.20.30.40 failed'],
  },
};
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const playwrightVersion = require('playwright/package.json').version;
  assert.equal(
    playwrightVersion,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const hashes = Object.fromEntries(
    runtime.map((p) => [
      p,
      crypto
        .createHash('sha256')
        .update(fs.readFileSync(path.join('/workspace', p)))
        .digest('hex'),
    ]),
  );
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5293, strictPort: true },
  });
  const results = [],
    screenshots = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1365, false],
      ['phone', webkit, 390, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: width === 390 ? 844 : 900 },
        isMobile: width === 390,
        hasTouch: width === 390,
        locale: 'en-GB',
        timezoneId: 'UTC',
        acceptDownloads: true,
      });
      page.setDefaultTimeout(20000);
      const checks = [],
        requests = [],
        errors = [],
        offOrigin = [],
        writes = [];
      let mode = 'held',
        release;
      page.on('pageerror', (error) => errors.push(error.message));
      await page.addInitScript((d) => {
        if (!d) return;
        const apply = () => document.documentElement.classList.add('dark');
        if (document.documentElement) apply();
        else document.addEventListener('DOMContentLoaded', apply, { once: true });
      }, dark);
      await page.route('**/*', async (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== origin) {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        requests.push({ path: url.pathname, method: request.method(), mode });
        if (request.method() !== 'GET') {
          writes.push(url.pathname);
          return route.abort();
        }
        if (url.pathname === '/api/diagnostics') {
          if (mode === 'held')
            await new Promise((resolve) => {
              release = resolve;
            });
          return route.fulfill(
            mode === 'failed'
              ? { status: 400, json: { error: 'Synthetic diagnostic connection failure' } }
              : { json: payload },
          );
        }
        if (url.pathname === '/api/security/status')
          return route.fulfill({ json: { hasAuthentication: true, requiresAuth: false } });
        return route.fulfill({ json: {} });
      });
      const check = async (label, action) => {
        await action();
        checks.push({ label, passed: true });
      };
      const snapshot = async (state) => {
        const file = `${name}-${state}.png`;
        // Capture settled animations, not an unreadable toast entering the viewport.
        await page.waitForTimeout(800);
        await page.screenshot({ path: path.join(output, file), fullPage: false });
        screenshots.push(file);
      };
      const countRuns = () => requests.filter((r) => r.path === '/api/diagnostics').length;
      const describedBy = async (locator) => {
        return locator.evaluate((el) =>
          (el.getAttribute('aria-describedby') || '')
            .split(/\s+/)
            .map((id) => document.getElementById(id)?.textContent || '')
            .join(' '),
        );
      };
      const verifyLayout = async () => {
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        for (const button of await page
          .getByRole('button', { name: /Run Diagnostics|Full \(private\)|GitHub \(review first\)/ })
          .all()) {
          const box = await button.boundingBox();
          assert(box && box.x >= 0 && box.x + box.width <= width + 1);
          if (width === 390) assert(box.height >= 44);
        }
      };
      try {
        await page.goto(`${origin}/browser-tests/diagnostics-safety.html`);
        await page.getByRole('heading', { name: 'System Diagnostics' }).waitFor();
        const run = page.getByRole('button', { name: 'Run Diagnostics', exact: true }).first();
        await check(
          'No automatic diagnostic request; safety precedes both run controls',
          async () => {
            assert.equal(countRuns(), 0);
            for (const action of await page
              .getByRole('button', { name: 'Run Diagnostics', exact: true })
              .all())
              assert.match(
                await describedBy(action),
                /Do not run it during a backup, freeze\/thaw/,
              );
            assert.equal(
              await page.getByRole('button', { name: 'Full (private)', exact: true }).count(),
              0,
            );
            await verifyLayout();
          },
        );
        await snapshot('empty');
        await check(
          'Keyboard triggers only one explicit request; pending run is disabled',
          async () => {
            await run.focus();
            await page.keyboard.press('Enter');
            await page.getByRole('button', { name: 'Running...', exact: true }).waitFor();
            assert.equal(countRuns(), 1);
            assert(
              await page.getByRole('button', { name: 'Running...', exact: true }).isDisabled(),
            );
          },
        );
        await snapshot('pending');
        mode = 'success';
        release();
        const full = page.getByRole('button', { name: 'Full (private)', exact: true });
        const github = page.getByRole('button', { name: 'GitHub (review first)', exact: true });
        await full.waitFor();
        await check(
          'Result offers truthful local-export labels and accessible review instructions',
          async () => {
            for (const button of [full, github])
              assert.match(
                await describedBy(button),
                /Nothing is uploaded.*review even a sanitised file/i,
              );
            await verifyLayout();
          },
        );
        const download = async (button, kind) => {
          const ready = page.waitForEvent('download');
          await button.click();
          const item = await ready;
          assert.match(
            item.suggestedFilename(),
            new RegExp(`^pulse-diagnostics-${kind}-\\d{4}-\\d{2}-\\d{2}\\.json$`),
          );
          const file = path.join(output, `${name}-${kind}.json`);
          await item.saveAs(file);
          return JSON.parse(fs.readFileSync(file));
        };
        await check(
          'Sanitised download redacts actual identities, preserves joins and measurements; no new request',
          async () => {
            const out = await download(github, 'sanitized');
            assert.equal(countRuns(), 1);
            assert(!JSON.stringify(out).includes('synthetic-private-'));
            assert(!JSON.stringify(out).includes('10.20.30.40'));
            assert.equal(out.apiTokens.usage[0].tokenId, out.apiTokens.tokens[1].id);
            assert.equal(out.apiTokens.usage[0].agents[0], out.apiTokens.usage[1].agents[0]);
            assert.equal(out.guestSnapshots[0].instance, out.nodeSnapshots[0].instance);
            assert.equal(out.memorySourceBreakdown[0].instance, out.nodeSnapshots[0].instance);
            assert.equal(out.guestSnapshots[0].vmid, undefined);
            assert.deepEqual(out.guestSnapshots[0].memory, payload.guestSnapshots[0].memory);
            await page
              .getByRole('heading', {
                name: 'Sanitised diagnostics downloaded — review before sharing',
                exact: true,
              })
              .waitFor();
          },
        );
        await check(
          'Full private download is unchanged; no diagnostic rerun or upload',
          async () => {
            assert.deepEqual(await download(full, 'full'), payload);
            assert.equal(countRuns(), 1);
            await page
              .getByRole('heading', {
                name: 'Full diagnostics downloaded — keep this file private',
                exact: true,
              })
              .waitFor();
          },
        );
        await snapshot('result');
        await check(
          'Refresh disables both downloads; failed request retains the previous result',
          async () => {
            mode = 'held';
            release = undefined;
            await run.click();
            await page.getByRole('button', { name: 'Running...', exact: true }).waitFor();
            assert(await full.isDisabled());
            assert(await github.isDisabled());
            assert.equal(countRuns(), 2);
            mode = 'failed';
            release();
            await page
              .getByRole('heading', {
                name: 'Synthetic diagnostic connection failure',
                exact: true,
              })
              .waitFor();
            assert(!(await full.isDisabled()));
            assert(!(await github.isDisabled()));
            assert.match(
              await page.getByText('Version 6.4.5', { exact: true }).textContent(),
              /6\.4\.5/,
            );
            assert.equal(countRuns(), 2);
            await verifyLayout();
          },
        );
        await snapshot('failed');
        await check('Existing shipped Docs viewer opens the exact safety section', async () => {
          const link = page.getByRole('link', {
            name: 'Safe diagnostics and sharing',
            exact: true,
          });
          assert.equal(
            await link.getAttribute('href'),
            '/docs/TROUBLESHOOTING#collect-diagnostics-safely',
          );
          await link.click();
          await page
            .getByRole('heading', { name: 'Collect diagnostics safely', exact: true })
            .waitFor();
          assert.match(page.url(), /\/docs\/TROUBLESHOOTING#collect-diagnostics-safely$/);
          assert(
            await page
              .locator('article')
              .getByText(/not an upload/)
              .count(),
          );
          assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        });
        await snapshot('docs');
        assert.deepEqual(errors, []);
        assert.deepEqual(offOrigin, []);
        assert.deepEqual(writes, []);
        results.push({
          name,
          browser_version: browser.version(),
          viewport: { width, height: width === 390 ? 844 : 900 },
          dark,
          checks,
          requests,
          errors,
          offOrigin,
          writes,
        });
      } finally {
        if (release) release();
        await browser.close();
        browser = null;
      }
    }
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          passed: true,
          playwright_version: playwrightVersion,
          content_sha256: hashes,
          changed_paths: runtime,
          results,
          screenshots,
        },
        null,
        2,
      ) + '\n',
    );
    console.log(
      JSON.stringify({
        passed: true,
        checks: results.reduce((n, r) => n + r.checks.length, 0),
        screenshots: screenshots.length,
        playwright_version: playwrightVersion,
        browsers: results.map((r) => ({ name: r.name, version: r.browser_version })),
      }),
    );
  } catch (error) {
    fs.writeFileSync(
      path.join(output, 'failed.json'),
      JSON.stringify({ passed: false, error: error.message, results, screenshots }, null, 2) + '\n',
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
