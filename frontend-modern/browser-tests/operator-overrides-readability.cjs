// Real API client and operator controls inside the production workload drawer.
// Only the local synthetic operator-state endpoint can accept writes.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const baseline = false;
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/operator-guidance/' + (baseline ? 'baseline' : 'final');
const origin = 'http://127.0.0.1:5338';
const runtime = 'frontend-modern/src/components/Infrastructure/ResourceOperatorStateSection.tsx';
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const savedState = () => ({
  canonicalId: 'coverage:fixture',
  monitoringMode: 'normal',
  lifecycleState: 'active',
  intentionallyOffline: false,
  neverAutoRemediate: false,
  autoRemediationPolicy: { enabled: false, capabilityNames: [] },
  criticality: '',
  note: 'Keep independent thaw and filesystem checks outside Pulse.',
  setAt: '2026-10-15T20:00:00Z',
  setBy: 'fixture-administrator-with-a-long-unbroken-identity@example.invalid',
});

// Text presence/element visibility cannot detect inherited nowrap or overlapping
// columns. Check the actual glyph line boxes against their own content block.
async function geometry(section) {
  return section.evaluate((element) => {
    const bounds = element.getBoundingClientRect();
    const texts = [];
    const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
    for (let node; (node = walker.nextNode());) {
      const parent = node.parentElement;
      if (!node.textContent.trim() || !parent.getClientRects().length) continue;
      if (parent.closest('select, textarea, button, .sr-only')) continue;
      const block = parent.closest('p, label, header, fieldset') || parent;
      const limit = block.getBoundingClientRect();
      const range = document.createRange();
      range.selectNodeContents(node);
      const rects = [...range.getClientRects()].filter((rect) => rect.width && rect.height);
      texts.push({
        text: node.textContent.trim(),
        lines: rects.length,
        whiteSpace: getComputedStyle(parent).whiteSpace,
        contained: rects.every(
          (rect) =>
            rect.left >= Math.max(bounds.left, limit.left) - 1 &&
            rect.right <= Math.min(bounds.right, limit.right) + 1 &&
            rect.top >= limit.top - 1 &&
            rect.bottom <= limit.bottom + 1,
        ),
      });
    }
    return {
      width: bounds.width,
      viewport: innerWidth,
      documentWidth: document.documentElement.scrollWidth,
      texts,
      violations: texts.filter((text) => !text.contained),
    };
  });
}

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    baseline,
    playwright: require('playwright/package.json').version,
    runtime: { [runtime]: hash('/workspace/' + runtime) },
    cases: [],
    screenshots: [],
    browserClosed: false,
    serverClosed: false,
    limits:
      'Synthetic local metadata only. No monitoring, guest, backup, action or external endpoint is contacted. This is readability and unchanged UI-policy handling, not native thaw/recovery or installed acceptance.',
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: output + '/cache',
    server: { host: '127.0.0.1', port: 5338, strictPort: true },
  });
  let browser;
  let page;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1440, false],
      ['phone', webkit, 390, true],
      ['narrow', webkit, 320, false],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      page = await browser.newPage({
        viewport: { width, height: 1000 },
        isMobile: width < 640,
        hasTouch: width < 640,
        locale: 'en-GB',
        timezoneId: 'UTC',
        colorScheme: dark ? 'dark' : 'light',
      });
      page.setDefaultTimeout(15000);
      page.setDefaultNavigationTimeout(60000);
      await page.addInitScript((dark) => {
        const applyTheme = () => document.documentElement.classList.toggle('dark', dark);
        if (document.documentElement) applyTheme();
        else document.addEventListener('DOMContentLoaded', applyTheme, { once: true });
      }, dark);
      await page
        .context()
        .addCookies([{ name: 'pulse_csrf', value: 'synthetic-csrf', url: origin }]);
      let state = savedState();
      const record = {
        name,
        width,
        dark,
        browser: browser.version(),
        errors: [],
        external: [],
        requests: [],
        states: [],
      };
      result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      await page.route('**/*', async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        if (url.origin !== origin) {
          record.external.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        const method = request.method();
        record.requests.push({ path: url.pathname, method });
        if (url.pathname.endsWith('/operator-state')) {
          if (method === 'GET') return route.fulfill({ json: { configured: !!state, state } });
          assert(!baseline, 'baseline capture is read-only');
          if (method === 'PUT') {
            const input = request.postDataJSON();
            record.requests.at(-1).input = input;
            state = { ...savedState(), ...input };
            return route.fulfill({ json: state });
          }
          assert.equal(method, 'DELETE');
          state = null;
          return route.fulfill({ json: { success: true } });
        }
        assert.equal(method, 'GET');
        if (url.pathname === '/api/metrics-store/history')
          return route.fulfill({
            json: {
              resourceType: url.searchParams.get('resourceType'),
              resourceId: url.searchParams.get('resourceId'),
              range: url.searchParams.get('range'),
              start: Date.now() - 86400000,
              end: Date.now(),
              metrics: {},
              source: 'store',
            },
          });
        return route.fulfill({
          json:
            url.pathname === '/api/license/runtime-capabilities'
              ? {
                  capabilities: [],
                  limits: [],
                  max_history_days: 7,
                  hosted_mode: false,
                  runtime: { build: 'community' },
                  blocked_capabilities: [],
                }
              : { success: true, data: [], alerts: [], anomalies: [], config: null },
        });
      });
      await page.goto(origin + '/browser-tests/backup-coverage.html', {
        waitUntil: 'domcontentloaded',
      });
      await page
        .getByRole('combobox', { name: 'Monitoring integration' })
        .selectOption('pve-deferred');
      const row = page.locator('tbody > tr').first();
      if (width > 640) {
        const expand = row.getByRole('button', {
          name: 'Expand Integration workload',
          exact: true,
        });
        await expand.focus();
        await page.keyboard.press('Enter');
      } else await row.click();
      await page.getByRole('tab', { name: 'Manage', exact: true }).click();
      const section = page.getByRole('region', { name: 'Operator overrides', exact: true });
      await section.getByRole('button', { name: 'Clear all overrides', exact: true }).waitFor();
      async function inspect(stateName, capture = false) {
        const measured = await geometry(section);
        record.states.push({ name: stateName, ...measured });
        if (capture) {
          const file = output + '/' + name + '-' + stateName + '.png';
          await page.screenshot({ path: file, fullPage: true });
          result.screenshots.push({ file, sha256: hash(file) });
        }
        if (!baseline) {
          assert.equal(measured.documentWidth, width, stateName + ': document overflows');
          assert.deepEqual(measured.violations, [], stateName + ': clipped/overlapping text');
        }
      }
      await inspect('default', true);
      if (!baseline) {
        const monitoring = section.getByRole('combobox', { name: 'Monitoring', exact: true });
        const lifecycle = section.getByRole('combobox', { name: 'Lifecycle', exact: true });
        const lock = section.getByRole('button', { name: 'Never auto-remediate this resource' });
        assert.equal(await monitoring.inputValue(), 'normal');
        assert.equal(await lifecycle.inputValue(), 'active');
        await monitoring.selectOption('expected_offline');
        await section.getByRole('button', { name: 'Discard', exact: true }).click();
        assert.equal(await monitoring.inputValue(), 'normal');
        await lock.click();
        await inspect('lock-confirmation', true);
        await section.getByRole('button', { name: 'Cancel', exact: true }).click();
        assert.equal(await lock.getAttribute('aria-pressed'), 'false');
        await section.getByRole('button', { name: 'Schedule window', exact: true }).click();
        await section.getByRole('button', { name: 'Recurring', exact: true }).click();
        await section
          .getByRole('combobox', { name: 'Applies to' })
          .selectOption('resource_and_descendants');
        await inspect('recurring-scope', true);
        await section.getByRole('button', { name: 'Cancel', exact: true }).click();
        assert.equal(record.requests.filter((request) => request.method !== 'GET').length, 0);
        await lifecycle.selectOption('retired');
        assert(await monitoring.isDisabled());
        assert(await lock.isDisabled());
        await inspect('retired', true);
        await section.getByRole('button', { name: 'Save overrides', exact: true }).click();
        await section.getByRole('button', { name: 'Clear all overrides', exact: true }).waitFor();
        assert.equal(state.lifecycleState, 'retired');
        assert.equal(state.monitoringMode, 'normal');
        assert.equal(state.neverAutoRemediate, false);
        assert.equal(state.note, savedState().note);
        await lifecycle.selectOption('active');
        await section.getByRole('button', { name: 'Save overrides', exact: true }).click();
        await section.getByRole('button', { name: 'Clear all overrides', exact: true }).waitFor();
        assert(!(await monitoring.isDisabled()));
        await inspect('restored');
        assert.deepEqual(
          record.requests
            .filter((request) => request.method !== 'GET')
            .map((request) => request.method),
          ['PUT', 'PUT'],
        );
        // The original safety text remains outside the controls and visible on Manage.
        const precaution = page.getByTestId('guest-read-precaution');
        assert.match(await precaution.innerText(), /running VM does not prove thaw/);
      }
      assert.deepEqual(record.errors, []);
      assert.deepEqual(record.external, []);
      await browser.close();
      browser = null;
    }
    result.result = baseline ? 'baseline-observed' : 'passed';
  } catch (error) {
    result.error = { message: error.message, stack: error.stack };
    if (page && !page.isClosed()) {
      const file = output + '/adverse-failure.png';
      await page.screenshot({ path: file, fullPage: true }).catch(() => {});
      if (fs.existsSync(file)) result.screenshots.push({ file, sha256: hash(file) });
    }
    throw error;
  } finally {
    if (browser) await browser.close();
    result.browserClosed = true;
    await server.close();
    result.serverClosed = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
