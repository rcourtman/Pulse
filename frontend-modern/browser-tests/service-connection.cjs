// Exercise the changed producer through real resource drawers and shipped CSS.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const staticRoot = path.join(root, 'tmp/service-connection/static');
const output = path.join(root, 'tmp/service-connection/browser');
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const bindingPath = path.join(staticRoot, 'binding.json');
  const binding = JSON.parse(fs.readFileSync(bindingPath));
  assert.match(binding.source_sha, /^[a-f0-9]{40}$/);
  for (const [file, expected] of Object.entries(binding.sources))
    assert.equal(hash(path.join(root, 'frontend-modern', file)), expected, file);
  for (const [file, expected] of Object.entries(binding.artifacts))
    assert.equal(hash(path.join(staticRoot, file)), expected, file);
  const clientVersion = require('playwright/package.json').version;
  assert.equal(
    clientVersion,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );

  const requests = [];
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://127.0.0.1');
    if (url.pathname.startsWith('/api/')) {
      requests.push({ method: req.method, path: url.pathname });
      res.setHeader('Content-Type', 'application/json');
      if (req.method !== 'GET') {
        res.statusCode = 405;
        return res.end('{}');
      }
      const data = url.pathname.includes('runtime-capabilities')
        ? {
            capabilities: [],
            limits: [],
            max_history_days: 7,
            hosted_mode: false,
            runtime: { build: 'community' },
            blocked_capabilities: [],
          }
        : { data: [], alerts: [], anomalies: [], enabled: false };
      return res.end(JSON.stringify(data));
    }
    const file = path.resolve(staticRoot, '.' + decodeURIComponent(url.pathname));
    if (
      !file.startsWith(staticRoot + path.sep) ||
      !fs.existsSync(file) ||
      !fs.statSync(file).isFile()
    ) {
      res.statusCode = 404;
      return res.end();
    }
    res.setHeader(
      'Content-Type',
      file.endsWith('.js')
        ? 'text/javascript'
        : file.endsWith('.css')
          ? 'text/css'
          : file.endsWith('.html')
            ? 'text/html'
            : 'application/octet-stream',
    );
    res.end(fs.readFileSync(file));
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${server.address().port}`;
  const report = {
    result: 'failed',
    source_sha: binding.source_sha,
    binding_sha256: hash(bindingPath),
    clientVersion,
    journeys: [],
    captures: [],
    requests,
    cleanup: { browser_closed: false, server_closed: false },
  };
  let browser;
  let failure;
  try {
    for (const [name, engine, width, height] of [
      ['desktop', chromium, 1365, 1000],
      ['narrow', webkit, 320, 1000],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      for (const transport of ['direct', 'canonical']) {
        const context = await browser.newContext({
          viewport: { width, height },
          isMobile: name === 'narrow',
          hasTouch: name === 'narrow',
        });
        const page = await context.newPage();
        const errors = [],
          external = [],
          transitions = [];
        page.on('pageerror', (error) => errors.push(error.message));
        page.on('request', (request) => {
          if (!request.url().startsWith(origin + '/')) external.push(request.url());
        });
        if (name === 'narrow')
          await page.addInitScript(() => {
            const applyDark = () => document.documentElement.classList.add('dark');
            if (document.documentElement) applyDark();
            else document.addEventListener('DOMContentLoaded', applyDark, { once: true });
          });
        await page.goto(`${origin}/browser-tests/service-connection.html?transport=${transport}`, {
          waitUntil: 'networkidle',
        });
        if (name === 'narrow') await expect(page.locator('html')).toHaveClass('dark');
        const activate = async (locator) => {
          if (name === 'narrow') await locator.tap();
          else {
            await locator.focus();
            await page.keyboard.press('Enter');
          }
        };
        const scopes = {},
          states = {},
          controls = {};
        for (const kind of ['pbs', 'pmg']) {
          const scope = page.locator(`[data-service-fixture="${kind}"]`);
          const details = scope.getByTestId('resource-technical-details');
          await activate(details.locator('summary').first());
          await expect(details).toHaveAttribute('open', '');
          await activate(scope.getByRole('button', { name: 'Show service', exact: true }));
          scopes[kind] = scope.getByTestId('resource-service-details-section');
          states[kind] = scopes[kind].getByText('State', { exact: true }).locator('..');
          controls[kind] = scopes[kind].getByRole('button', { name: 'Hide service', exact: true });
          await states[kind].evaluate((node) => (node.dataset.retainedState = 'original'));
        }
        const expectState = async (kind, label, tone, sameOpen = true) => {
          const text = states[kind].getByText(label, { exact: true });
          await expect(text).toBeVisible();
          await expect(text).toHaveClass(new RegExp(`(?:^|\\s)${tone}(?:\\s|$)`));
          if (sameOpen)
            await expect(states[kind]).toHaveAttribute('data-retained-state', 'original');
          await expect(controls[kind]).toBeVisible();
          transitions.push({ kind, label, tone, sameOpen, text: await states[kind].innerText() });
        };
        const update = async (method, kind, ...args) =>
          page.evaluate(
            ({ method, kind, args }) => window.__serviceConnection[method](kind, ...args),
            { method, kind, args },
          );
        const noOverflow = async () => {
          const widths = await page.evaluate(() => ({
            viewport: innerWidth,
            document: document.documentElement.scrollWidth,
          }));
          assert(widths.document <= widths.viewport + 1, JSON.stringify(widths));
          for (const kind of ['pbs', 'pmg']) {
            const box = await states[kind].boundingBox();
            assert(box && box.x >= 0 && box.x + box.width <= width + 1, `${kind} state clipped`);
          }
        };
        const capture = async (suffix) => {
          if (transport !== 'canonical') return;
          await noOverflow();
          const file = path.join(output, `${name}-${suffix}.png`);
          await page.locator('main').screenshot({ path: file, animations: 'disabled' });
          report.captures.push({
            file: path.relative(root, file),
            sha256: hash(file),
            name,
            state: suffix,
          });
        };
        for (const kind of ['pbs', 'pmg']) await expectState(kind, 'Healthy', 'text-green-700');
        // Carrier-only updates must refresh an already open, same-node service
        // card even while its nested service facet remains healthy.
        if (name === 'desktop') await controls.pmg.focus();
        for (const kind of ['pbs', 'pmg']) {
          await update('status', kind, 'offline');
          await expectState(kind, 'Offline', 'text-red-700');
        }
        if (name === 'desktop') await expect(controls.pmg).toBeFocused();
        await capture('offline');
        for (const kind of ['pbs', 'pmg']) {
          await update('replace', kind, 'online', 'unhealthy');
          await expectState(kind, 'Unhealthy', 'text-red-700');
        }
        // A change in one service never changes the other's classification.
        await update('health', 'pmg', 'not healthy');
        await expectState('pmg', 'not healthy', 'text-muted');
        await expectState('pbs', 'Unhealthy', 'text-red-700');
        await capture('adverse-and-unknown');
        for (const kind of ['pbs', 'pmg']) {
          await update('health', kind, '<healthy>');
          await expectState(kind, '<healthy>', 'text-muted');
          assert.equal(await states[kind].locator('healthy').count(), 0);
          await update('health', kind, 'healthy-but-unreachable');
          await expectState(kind, 'healthy-but-unreachable', 'text-muted');
          await update('health', kind, 'degraded');
          await expectState(kind, 'Degraded', 'text-yellow-700');
          await update('health', kind, 'error');
          await expectState(kind, 'Offline', 'text-red-700');
          await update('health', kind, '  ');
          await expectState(kind, 'Healthy', 'text-green-700');
          await update('replace', kind, 'warning', 'healthy');
          await expectState(kind, 'Healthy', 'text-green-700');
          await update('replace', kind, 'offline', 'unhealthy');
          await expectState(kind, 'Offline', 'text-red-700');
          await update('replace', kind, 'online', 'healthy');
          await expectState(kind, 'Healthy', 'text-green-700');
        }
        await capture('recovered');
        // A genuine close/reopen by keyboard/tap sees the same current result.
        for (const kind of ['pbs', 'pmg']) {
          await activate(controls[kind]);
          await activate(scopes[kind].getByRole('button', { name: 'Show service', exact: true }));
          await expectState(kind, 'Healthy', 'text-green-700', false);
        }
        await noOverflow();
        assert.deepEqual(errors, []);
        assert.deepEqual(external, []);
        report.journeys.push({
          name,
          transport,
          viewport: { width, height },
          browserVersion: browser.version(),
          transitions,
          errors,
          external,
          same_open_state_nodes: true,
          keyboard_or_touch: name === 'desktop' ? 'native Enter' : 'tap',
        });
        await context.close();
      }
      await browser.close();
      browser = undefined;
    }
    assert.equal(report.journeys.length, 4);
    assert(requests.every((request) => request.method === 'GET'));
    report.result = 'passed';
  } catch (error) {
    failure = error;
    report.error = String(error.stack || error);
  } finally {
    if (browser) await browser.close();
    report.cleanup.browser_closed = true;
    await new Promise((resolve) => server.close(resolve));
    report.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
  }
  if (failure) throw failure;
  console.log(
    JSON.stringify({
      result: report.result,
      source: report.source_sha,
      journeys: report.journeys.length,
      captures: report.captures.length,
      cleanup: report.cleanup,
    }),
  );
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
