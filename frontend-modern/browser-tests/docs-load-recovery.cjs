// #3102: read the actual Docs producer/renderer/router with local HTTP faults.
// No account, service, browser history/reload or external endpoint is exercised.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const preview = path.join(root, 'tmp/doc-command-help/static');
const output = path.join(root, 'tmp/docs-load-recovery');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const fragment = 'what-do-pro-and-cloud-unlock';
const target = `/docs/FAQ?reader=fixture#${fragment}`;
const messages = {
  missing: "That document is not part of this installation's shipped documentation set.",
  access: 'Access to this guide was refused.',
  unavailable: 'This guide could not be loaded.',
};
const privateBody = '<h1>private-upstream-detail</h1><script>window.leaked = true</script>';
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const result = {
  result: 'incomplete',
  source_sha: binding.source_sha,
  binding_sha256: hash(bindingBytes),
  cases: [],
  captures: [],
  requests: [],
  expected_failures: [],
  errors: [],
  cleanup: {},
};
const deferred = () => {
  let resolve;
  const promise = new Promise((done) => {
    resolve = done;
  });
  return { promise, resolve };
};
(async () => {
  fs.mkdirSync(output, { recursive: true });
  let server, browser;
  try {
    result.playwright = require('playwright/package.json').version;
    assert.equal(
      result.playwright,
      JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    for (const [file, expected] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(frontend, file))), expected, file);
    const assets = new Map();
    for (const [file, expected] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(preview, file));
      assert.equal(hash(bytes), expected, file);
      assets.set(file, bytes);
    }
    assert.equal(
      hash(assets.get('docs/FAQ.md')),
      hash(fs.readFileSync(path.join(root, 'docs/FAQ.md'))),
    );
    result.source_inputs = Object.keys(binding.sources).length;
    result.artifacts = assets.size;
    server = http.createServer((req, res) => {
      if (req.method !== 'GET') {
        res.writeHead(405);
        res.end();
        return;
      }
      const route = new URL(req.url, 'http://fixture.invalid').pathname;
      let file = route.slice(1);
      if (route.startsWith('/docs/') && !route.endsWith('.md'))
        file = 'browser-tests/doc-command-help.html';
      if (!assets.has(file)) {
        res.writeHead(404);
        res.end();
        return;
      }
      const type = {
        '.md': 'text/markdown',
        '.css': 'text/css',
        '.js': 'text/javascript',
        '.html': 'text/html',
      }[path.extname(file)];
      res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' });
      res.end(assets.get(file));
    });
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const state of [
      { name: 'desktop', width: 1360, height: 920, engine: 'chromium', touch: false, dark: false },
      { name: 'narrow', width: 320, height: 844, engine: 'webkit', touch: true, dark: true },
    ]) {
      browser = await (state.touch
        ? webkit.launch({ headless: true })
        : chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] }));
      const record = { ...state, browser: browser.version(), classifications: [], checks: [] };
      result.cases.push(record);
      const context = await browser.newContext({
        viewport: { width: state.width, height: state.height },
        isMobile: state.touch,
        hasTouch: state.touch,
        locale: 'en-GB',
        colorScheme: state.dark ? 'dark' : 'light',
      });
      await context.addInitScript((dark) => {
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.toggle('dark', dark),
        );
      }, state.dark);
      let plans = [],
        faqRequests = 0,
        holds = [];
      await context.route('**/*', async (route) => {
        const request = route.request(),
          url = new URL(request.url());
        result.requests.push({
          path: url.pathname,
          method: request.method(),
          local: url.origin === origin,
        });
        if (
          url.origin !== origin ||
          request.method() !== 'GET' ||
          url.pathname.startsWith('/api/')
        ) {
          result.errors.push('Unexpected request rejected');
          return route.abort();
        }
        if (url.pathname === '/docs/FAQ.md') {
          faqRequests++;
          assert.equal(request.headers().accept, 'text/markdown, text/plain');
          const plan = plans.shift();
          if (!plan) {
            result.errors.push('Unplanned FAQ read');
            return route.abort();
          }
          if (plan.hold) {
            holds.push(plan.hold);
            await plan.hold.promise;
          }
          if (plan.kind === 'network') return route.abort('failed');
          if (plan.kind !== 'success')
            return route.fulfill({
              status: plan.kind === 'html' ? 200 : Number(plan.kind),
              contentType: 'text/html',
              body: privateBody,
            });
        }
        return route.continue();
      });
      let page;
      const newPage = async () => {
        if (page) await page.close();
        page = await context.newPage();
        page.setDefaultTimeout(6500);
        page.on('pageerror', (error) => result.errors.push(error.message));
        page.on('requestfailed', (request) =>
          result.expected_failures.push({
            path: new URL(request.url()).pathname,
            failure: request.failure()?.errorText,
          }),
        );
        // HTTP/network controls produce native resource diagnostics, not page exceptions.
        page.on('console', (message) => {
          if (message.type() === 'error')
            result.expected_failures.push({ console: message.text().slice(0, 300) });
        });
      };
      const activate = async (locator) => {
        await locator.scrollIntoViewIfNeeded();
        if (state.touch) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      const capture = async (suffix) => {
        await page.evaluate(
          () =>
            new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
        );
        assert(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          'horizontal overflow',
        );
        const file = `${state.name}-${suffix}.png`;
        await page.screenshot({ path: path.join(output, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))) });
      };
      const recovered = async () => {
        await page.waitForFunction((id) => {
          const heading = document.getElementById(id);
          if (!heading) return false;
          const box = heading.getBoundingClientRect();
          return (
            document.activeElement === heading &&
            Math.round(box.top) >= 0 &&
            box.bottom <= innerHeight
          );
        }, fragment);
        assert.equal(
          new URL(page.url()).pathname + new URL(page.url()).search + new URL(page.url()).hash,
          target,
        );
        assert.equal(await page.getByRole('alert').count(), 0);
        assert.equal(await page.evaluate(() => window.leaked), undefined);
      };
      try {
        for (const [kind, category] of [
          ['401', 'access'],
          ['403', 'access'],
          ['503', 'unavailable'],
          ['network', 'unavailable'],
          ['404', 'missing'],
          ['410', 'missing'],
          ['html', 'missing'],
        ]) {
          const before = faqRequests;
          plans = [{ kind }, { kind: 'success' }];
          await newPage();
          await page.goto(origin + target, { waitUntil: 'domcontentloaded' });
          const alert = page.getByRole('alert');
          await alert.waitFor();
          assert((await alert.innerText()).startsWith(messages[category]));
          assert(!(await page.locator('body').innerText()).includes('private-upstream-detail'));
          assert.equal(await page.locator('article').count(), 0);
          assert.equal(faqRequests - before, 1, 'no automatic retry');
          if (kind === '403' || kind === '503' || kind === '404') await capture(category);
          await page.evaluate(() => (window.__docs_read_shell = 'preserved'));
          const historyLength = await page.evaluate(() => history.length);
          await activate(page.getByRole('button', { name: 'Try again', exact: true }));
          await recovered();
          assert.equal(faqRequests - before, 2);
          assert.equal(await page.evaluate(() => window.__docs_read_shell), 'preserved');
          assert.equal(await page.evaluate(() => history.length), historyLength);
          record.classifications.push({
            kind,
            category,
            reads: faqRequests - before,
            section_restored: true,
          });
        }
        // An initial pending read has a status, not an absence diagnosis.
        const initial = deferred();
        plans = [{ kind: 'success', hold: initial }];
        await newPage();
        await page.goto(origin + target, { waitUntil: 'domcontentloaded' });
        await page.getByRole('status').waitFor();
        assert.equal(await page.getByRole('alert').count(), 0);
        initial.resolve();
        await recovered();
        record.checks.push('initial loading status then actual shipped FAQ/fragment');
        // Retry can itself fail without losing its control or making a new claim.
        const pendingFailure = deferred(),
          pendingSuccess = deferred();
        plans = [
          { kind: '403' },
          { kind: '503', hold: pendingFailure },
          { kind: 'success', hold: pendingSuccess },
        ];
        await newPage();
        await page.goto(origin + target, { waitUntil: 'domcontentloaded' });
        const retry = page.getByRole('button', { name: 'Try again', exact: true });
        await retry.waitFor();
        await activate(retry);
        await page.getByRole('status').waitFor();
        assert(await retry.isDisabled());
        const beforeDuplicate = faqRequests;
        await retry.evaluate((button) => button.click());
        assert.equal(faqRequests, beforeDuplicate, 'duplicate read blocked');
        await capture('retry-pending');
        pendingFailure.resolve();
        await page.waitForFunction(() =>
          document
            .querySelector('[role="alert"]')
            ?.textContent.startsWith('This guide could not be loaded.'),
        );
        assert(await retry.isEnabled());
        record.retry_focus_after_failure = await retry.evaluate(
          (button) => document.activeElement === button,
        );
        assert(record.retry_focus_after_failure, 'retry focus survives failed read');
        await activate(retry);
        await page.getByRole('status').waitFor();
        pendingSuccess.resolve();
        await recovered();
        await capture('recovered-section');
        record.checks.push(
          'explicit pending/failed/successful retry, duplicate suppression, retained focus and exact URL/fragment',
        );
        // Leaving during the held read must neither carry over its diagnosis nor render a late guide.
        const obsolete = deferred();
        plans = [{ kind: '503' }, { kind: 'success', hold: obsolete }];
        await newPage();
        await page.goto(origin + target, { waitUntil: 'domcontentloaded' });
        await activate(page.getByRole('button', { name: 'Try again', exact: true }));
        await page.getByRole('status').waitFor();
        await activate(
          page.getByRole('link', { name: 'Back to the documentation index', exact: true }),
        );
        await page.waitForURL('**/docs/README');
        await page.locator('article').waitFor();
        const index = await page.locator('article').innerText();
        obsolete.resolve();
        await page.waitForLoadState('networkidle');
        assert.equal(await page.locator('article').innerText(), index);
        assert.equal(await page.getByRole('alert').count(), 0);
        assert.equal(await page.getByRole('button', { name: 'Try again', exact: true }).count(), 0);
        record.checks.push('index remains available during retry, late old FAQ cannot replace it');
      } finally {
        for (const hold of holds) hold.resolve();
        await context.close();
      }
      await browser.close();
      browser = undefined;
    }
    assert.deepEqual(result.errors, []);
    result.result = 'passed';
  } catch (error) {
    result.result = 'failed';
    result.error = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browsers_closed = true;
    if (server)
      await new Promise((resolve) => {
        server.close(resolve);
        server.closeAllConnections();
      });
    result.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        source: result.source_sha,
        result: result.result,
        cases: result.cases.length,
        captures: result.captures.length,
        errors: result.errors,
        cleanup: result.cleanup,
        error: result.error,
      }),
    );
  }
})();
