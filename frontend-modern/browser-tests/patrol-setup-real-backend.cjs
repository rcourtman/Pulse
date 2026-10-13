const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const cp = require('node:child_process');
const { chromium, webkit } = require('playwright');
const {
  startProvider,
} = require('/workspace/frontend-modern/browser-tests/patrol-setup-provider.cjs');

const root = '/workspace';
const output = path.join(root, 'tmp/patrol-setup-proof/browser');
const binary = path.join(root, 'tmp/patrol-setup-proof/patrol-setup-preview');
const sha256 = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const result = {
  playwright: require('playwright/package.json').version,
  scope:
    'Production /patrol app, public router/auth/settings, Ollama streaming readiness evaluator, Watch-only tool executor and persisted run history. Only host inventory and provider protocol replies are synthetic. No page API routes or backend verdicts are stubbed.',
  source: JSON.parse(fs.readFileSync(path.join(root, 'tmp/patrol-setup-proof/binding.json'))),
  binary_sha256: sha256(fs.readFileSync(binary)),
  checks: [],
  requests: [],
  captures: [],
  provider: [],
  browsers: [],
  errors: [],
  cleanup: [],
};
assert.equal(
  result.playwright,
  JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
    'node_modules/@playwright/test'
  ].version,
);
for (const [file, digest] of Object.entries(result.source.paths)) {
  assert.equal(sha256(fs.readFileSync(path.join(root, file))), digest, file);
}
const record = (name, detail) => result.checks.push({ name, detail });
const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function waitUntil(callback, label, timeout = 20000) {
  const end = Date.now() + timeout;
  while (Date.now() < end) {
    if (await callback()) return;
    await pause(150);
  }
  throw new Error(`Timed out: ${label}`);
}

async function stopOwned(child, log) {
  if (child.exitCode !== null) return { exit: child.exitCode, signal: child.signalCode };
  const exited = new Promise((resolve) =>
    child.once('exit', (exit, signal) => resolve({ exit, signal })),
  );
  child.kill('SIGTERM');
  const observed = await Promise.race([exited, pause(8000).then(() => null)]);
  if (observed) {
    const data = /patrol-setup-fixture-data (\/tmp\/pulse-patrol-setup-[a-zA-Z0-9]+)/.exec(log());
    assert(data, 'owned temporary data identity recorded');
    assert.equal(fs.existsSync(data[1]), false, 'owned data removed after shutdown');
    assert.match(log(), /patrol-setup-fixture-clean-shutdown/);
    return observed;
  }
  child.kill('SIGKILL');
  const forced = await exited;
  throw new Error(`Owned preview needed forced cleanup: ${JSON.stringify(forced)}`);
}

async function capture(page, name, target) {
  if (target) await target.scrollIntoViewIfNeeded();
  await page.evaluate(() => document.fonts.ready);
  await pause(250);
  const filename = `${name}.png`;
  await page.screenshot({ path: path.join(output, filename), fullPage: false });
  result.captures.push({
    file: filename,
    sha256: sha256(fs.readFileSync(path.join(output, filename))),
  });
}

(async () => {
  fs.mkdirSync(output, { recursive: true });
  let vite;
  try {
    process.env.PULSE_DEV_API_PORT = '5301';
    const { createServer } = await import(
      path.join(root, 'frontend-modern/node_modules/vite/dist/node/index.js')
    );
    vite = await createServer({
      root: path.join(root, 'frontend-modern'),
      configFile: path.join(root, 'frontend-modern/vite.config.ts'),
      server: { host: '127.0.0.1', port: 5300, strictPort: true },
    });
    await vite.listen();
    for (const spec of [
      {
        name: 'desktop',
        width: 1280,
        height: 900,
        engine: chromium,
        options: { headless: true, channel: 'chromium', args: ['--no-sandbox'] },
      },
      { name: 'phone', width: 375, height: 812, engine: webkit, options: { headless: true } },
    ]) {
      const providerRecords = [];
      const provider = await startProvider(5302, providerRecords);
      let backendLog = '';
      const backend = cp.spawn(binary, ['-port', '5301'], {
        cwd: path.join(root, 'tmp/patrol-setup-proof'),
        env: {
          ...process.env,
          PULSE_MOCK_MODE: 'false',
          PULSE_TELEMETRY: 'false',
          FRONTEND_DEV_SERVER: 'true',
        },
      });
      backend.stdout.on('data', (chunk) => {
        backendLog += chunk;
      });
      backend.stderr.on('data', (chunk) => {
        backendLog += chunk;
      });
      let browser;
      let page;
      try {
        await waitUntil(
          () => backendLog.includes('patrol-setup-fixture-ready'),
          'real backend ready',
        );
        browser = await spec.engine.launch(spec.options);
        result.browsers.push({
          name: spec.name,
          version: browser.version(),
          viewport: { width: spec.width, height: spec.height },
        });
        const context = await browser.newContext({
          viewport: { width: spec.width, height: spec.height },
          colorScheme: 'light',
          ...(spec.name === 'phone'
            ? { hasTouch: true, isMobile: true, deviceScaleFactor: 2 }
            : {}),
        });
        page = await context.newPage();
        page.setDefaultTimeout(18000);
        const activate = async (locator) => {
          if (spec.name === 'phone') await locator.tap();
          else {
            await locator.focus();
            await locator.press('Enter');
          }
        };
        page.on('pageerror', (error) =>
          result.errors.push({ browser: spec.name, error: error.message }),
        );
        page.on('response', async (response) => {
          const url = new URL(response.url());
          if (!url.pathname.startsWith('/api/')) return;
          const entry = {
            browser: spec.name,
            method: response.request().method(),
            path: url.pathname,
            status: response.status(),
          };
          if (/\/api\/(settings\/ai|ai\/patrol\/(readiness|run|runs|status))/.test(url.pathname)) {
            try {
              entry.body = await response.json();
            } catch {
              /* whitespace streaming is parsed on completion */
            }
          }
          result.requests.push(entry);
        });
        await page.goto('http://127.0.0.1:5300/patrol');
        await page.locator('input[name="username"]').fill('synthetic-browser-admin');
        await page.locator('input[name="password"]').fill('synthetic-browser-password');
        await page.getByRole('button', { name: /sign in/i }).click();
        await page.getByTestId('patrol-setup-panel').waitFor();
        await capture(page, `${spec.name}-initial`);
        await activate(page.getByRole('button', { name: /Ollama/ }));
        await page.getByLabel('Ollama server URL').fill('http://127.0.0.1:5302');
        provider.setMode('wrong-context');
        await activate(page.getByRole('button', { name: 'Connect and check', exact: true }));
        await page.getByTestId('patrol-setup-error').waitFor();
        record(
          `${spec.name}: actual evaluator rejects wrong context`,
          await page.getByTestId('patrol-setup-error').innerText(),
        );
        assert.equal(
          await page.getByRole('button', { name: 'Run Patrol now', exact: true }).count(),
          0,
        );
        await capture(page, `${spec.name}-refused-context`);
        provider.setMode('pass');
        await activate(page.getByRole('button', { name: 'Check qwen3:8b again', exact: true }));
        await page.getByTestId('patrol-setup-ready').waitFor();
        const ready = await page.getByTestId('patrol-setup-ready').innerText();
        assert.match(ready, /Patrol is ready/);
        record(`${spec.name}: actual streaming evaluator passes`, ready);
        await capture(page, `${spec.name}-ready`, page.getByTestId('patrol-setup-ready'));
        const settingsResponse = await context.request.get('http://127.0.0.1:5300/api/settings/ai');
        const saved = await settingsResponse.json();
        assert.equal(saved.model, 'ollama:qwen3:8b');
        assert.equal(saved.patrol_model, 'ollama:qwen3:8b');
        assert.equal(saved.patrol_interval_minutes, 360);
        record(`${spec.name}: real persisted models and original schedule`, {
          model: saved.model,
          patrol_model: saved.patrol_model,
          interval_minutes: saved.patrol_interval_minutes,
        });
        const runResponse = page.waitForResponse(
          (response) =>
            new URL(response.url()).pathname === '/api/ai/patrol/run' &&
            response.request().method() === 'POST',
        );
        await activate(page.getByRole('button', { name: 'Run Patrol now', exact: true }));
        const accepted = await runResponse;
        assert.equal(accepted.status(), 200);
        const acceptance = await accepted.json();
        assert.equal(acceptance.accepted, true);
        assert(acceptance.run_id);
        record(`${spec.name}: real manual first run accepted`, acceptance);
        await waitUntil(async () => {
          const response = await context.request.get('http://127.0.0.1:5300/api/ai/patrol/runs');
          const body = await response.json();
          const runs = body.runs || body;
          const run = runs.find((run) => run.id === acceptance.run_id);
          if (run?.completed_at) {
            result.completedRun = { ...result.completedRun, [spec.name]: run };
            return true;
          }
          return false;
        }, 'actual completed first-run record');
        const run = result.completedRun[spec.name];
        assert.equal(run.error_count, 0);
        assert.equal(run.resources_checked, 1);
        assert(run.tool_call_count > 0);
        assert.equal(run.trigger_reason, 'manual');
        record(`${spec.name}: first run executed tools and persisted`, run);
        // Read it through the actual workbench History journey, not only HTTP.
        await activate(page.getByRole('tab', { name: 'Activity', exact: true }));
        await activate(page.getByRole('button', { name: /Finding options and history/ }));
        await activate(page.getByRole('button', { name: 'History', exact: true }));
        await page.getByText('Patrol history', { exact: true }).waitFor();
        const historyEntry = page.getByRole('button').filter({ hasText: 'All clear' }).first();
        await historyEntry.waitFor();
        await capture(page, `${spec.name}-first-run-history`, historyEntry);
        await activate(historyEntry);
        await page.getByText('Check record', { exact: true }).waitFor();
        await capture(
          page,
          `${spec.name}-first-run-record`,
          page.getByText('Check record', { exact: true }),
        );
        record(`${spec.name}: actual History entry opens the completed run`, {
          run_id: acceptance.run_id,
        });
        const overflow = await page.evaluate(() => ({
          width: innerWidth,
          scrollWidth: document.documentElement.scrollWidth,
        }));
        assert(overflow.scrollWidth <= overflow.width + 1, JSON.stringify(overflow));
        record(`${spec.name}: viewport has no horizontal overflow`, overflow);
        assert(
          providerRecords.some(
            (entry) =>
              entry.purpose === 'readiness-continuation' &&
              entry.emittedTool === 'readiness_confirm_result',
          ),
        );
        assert(
          providerRecords.some(
            (entry) => entry.purpose === 'patrol' && entry.emittedTool === 'pulse_query',
          ),
        );
      } catch (error) {
        if (page) {
          await capture(page, `${spec.name}-failure`).catch(() => {});
          result.failureBody = await page
            .locator('body')
            .innerText()
            .catch(() => 'unavailable');
        }
        throw error;
      } finally {
        if (browser) await browser.close();
        const backendExit = await stopOwned(backend, () => backendLog);
        await provider.close();
        result.provider.push({ browser: spec.name, records: providerRecords });
        result.cleanup.push({
          browser: spec.name,
          backend: backendExit,
          browserClosed: true,
          providerClosed: true,
        });
        fs.writeFileSync(path.join(output, `${spec.name}-backend.log`), backendLog);
      }
    }
  } catch (error) {
    result.failure = error.stack;
    process.exitCode = 1;
  } finally {
    if (vite) await vite.close();
    result.viteClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        passed: !result.failure,
        checks: result.checks.length,
        failure: result.failure,
        captures: result.captures,
        cleanup: result.cleanup,
      }),
    );
  }
})();
