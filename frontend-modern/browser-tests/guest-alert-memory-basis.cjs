const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const original = process.argv.includes('--original');
const root = '/workspace/frontend-modern';
const output = `/workspace/tmp/guest-alert-memory-basis/browser-${original ? 'original' : 'final'}`;
const origin = 'http://127.0.0.1:5344';
const basisNote = 'Alert percentages use guest allocation. The table row uses host capacity.';
const runtime = [
  'frontend-modern/src/components/Workloads/guestDrawerModel.ts',
  'frontend-modern/src/components/Workloads/GuestDrawer.tsx',
  'frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx',
  'frontend-modern/src/components/Workloads/GuestRow.tsx',
  'frontend-modern/src/features/alerts/metricAlertPresentation.ts',
  'frontend-modern/src/utils/memoryObservation.ts',
];
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const settle = (page) =>
  page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  });

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    phase: original ? 'original-adverse-observation' : 'candidate-verification',
    playwright: require('playwright/package.json').version,
    runtime_hashes: Object.fromEntries(runtime.map((file) => [file, hash('/workspace/' + file)])),
    cases: [],
    screenshots: [],
    cleanup: { browsers: false, server: false },
    limits:
      'Production guest row/drawer, alert presentation and CSS; synthetic observations/local GET replies. No guest command, installed recovery, notification delivery, reporter or release acceptance.',
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
    optimizeDeps: { entries: ['browser-tests/guest-alert-memory-basis.html'] },
    server: { host: '127.0.0.1', port: 5344, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, height, dark] of [
      ['chromium-desktop-light', chromium, 1280, 900, false],
      ['webkit-phone-dark', webkit, 390, 844, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height },
        isMobile: width < 768,
        hasTouch: width < 768,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(12000);
      const record = {
        name,
        width,
        height,
        dark,
        browser: browser.version(),
        errors: [],
        requests: [],
        checks: [],
        states: [],
      };
      result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      await page.route('**/*', (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== origin) {
          record.requests.push({ path: url.pathname, method: request.method(), blocked: true });
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        assert.equal(request.method(), 'GET');
        record.requests.push({ path: url.pathname, method: request.method() });
        return route.fulfill({
          json: {
            success: true,
            data: [],
            available: false,
            alerts: [],
            config: null,
            capabilities: [],
            relationships: [],
            anomalies: [],
            count: 0,
          },
        });
      });
      await page.addInitScript((isDark) => {
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.toggle('dark', isDark),
        );
      }, dark);
      await page.goto(`${origin}/browser-tests/guest-alert-memory-basis.html`, {
        waitUntil: 'domcontentloaded',
        timeout: 60000,
      });
      const row = page.locator('[data-guest-id]');
      await row.waitFor({ timeout: 60000 });
      // Keyboard on desktop and a real tap on the phone use the production row opener.
      const opener = row.getByRole('button', { name: 'Expand memory-guest', exact: true });
      if (width < 768) await row.tap();
      else {
        await opener.focus();
        await page.keyboard.press('Enter');
      }
      const attention = page.getByTestId('drawer-attention-section');
      await attention.waitFor();
      const snapshot = async (state) => {
        await settle(page);
        const text = await attention.innerText();
        record.states.push({
          state,
          alert: text,
          row_memory: await row.locator('[data-workload-col="memory"]').innerText(),
        });
        return text;
      };
      const capture = async (state) => {
        await settle(page);
        const file = `${output}/${name}-${state}.png`;
        await page.screenshot({ path: file, fullPage: true });
        result.screenshots.push({ path: file, sha256: hash(file), state });
      };
      const verifyBasis = async (state) => {
        const text = await snapshot(state);
        if (original) {
          assert.match(text, /VM memory at 86\.6% \(guest allocation · 13% of host capacity\)/);
          assert.equal(text.includes(basisNote), false);
        } else {
          assert.match(text, /VM memory at 86\.6%/);
          assert.ok(text.includes(basisNote));
          assert.doesNotMatch(text, /\d+% of host capacity/);
        }
        record.checks.push(state);
      };
      await verifyBasis('retained-memory');
      assert.match(
        await row.locator('[data-workload-col="memory"]').innerText(),
        /Last known|Prior/,
      );
      await capture('retained');
      await page.evaluate(() => {
        const fixture = window.__guestAlertMemory;
        fixture.apply({
          memory: {
            ...fixture.guest().memory,
            usageUnavailable: true,
            observation: { state: 'unavailable', source: 'guest-agent-meminfo' },
          },
        });
      });
      await verifyBasis('unavailable-memory');
      assert.match(
        await row.locator('[data-workload-col="memory"]').innerText(),
        /Unavailable|N\/A/,
      );
      await capture('unavailable');
      if (!original) {
        await page.evaluate(() => {
          const fixture = window.__guestAlertMemory;
          fixture.apply({
            memory: {
              ...fixture.guest().memory,
              usageUnavailable: false,
              observation: {
                state: 'current',
                source: 'agent',
                observedAt: new Date().toISOString(),
              },
            },
          });
        });
        await verifyBasis('current-sample-differs-from-alert');
        assert.match(await row.locator('[data-workload-col="memory"]').innerText(), /13%/);
        await page.evaluate(() => {
          const fixture = window.__guestAlertMemory;
          const memoryAlert = {
            ...fixture.alert,
            metricStatus: {
              phase: 'latched',
              value: 72,
              unit: '%',
              observedAt: new Date().toISOString(),
              trigger: 85,
              recovery: 70,
            },
          };
          fixture.alerts([
            memoryAlert,
            {
              ...fixture.alert,
              id: 'cpu-control',
              type: 'cpu',
              message: 'CPU event stays unchanged',
              value: 92,
              threshold: 80,
            },
          ]);
        });
        const live = await snapshot('alert-owned-live-status-and-cpu-control');
        assert.match(live, /Memory 72% now, back under the 85% alert level/);
        assert.match(live, /Stays open until it reaches 70% or lower\./);
        assert.equal(live.split(basisNote).length - 1, 1);
        assert.match(live, /CPU event stays unchanged/);
        assert.doesNotMatch(live, /\d+% of host capacity/);
        record.checks.push('live-alert-reading-clear-rule-and-cpu-unchanged');
        await capture('live-alert');
        // Use the native select, and return using keyboard, not a synthetic change event.
        await page.getByLabel('Memory basis').selectOption('guest');
        await settle(page);
        assert.equal((await attention.innerText()).includes(basisNote), false);
        // The existing AnimatedNumber eases basis changes over several frames.
        // Wait for the observable final value rather than sampling a mid-frame number.
        await page.waitForFunction(
          () =>
            document.querySelector('[data-workload-col="memory"] [data-animated-number]')
              ?.textContent === '25%',
        );
        assert.match(await row.locator('[data-workload-col="memory"]').innerText(), /25%/);
        await snapshot('guest-basis-final-value');
        await capture('guest-basis');
        await page.getByLabel('Memory basis').focus();
        await page.keyboard.press('ArrowDown');
        await page.keyboard.press('Enter');
        await settle(page);
        assert.ok((await attention.innerText()).includes(basisNote));
        assert.equal(await page.getByLabel('Memory basis').inputValue(), 'host');
        await page.waitForFunction(
          () =>
            document.querySelector('[data-workload-col="memory"] [data-animated-number]')
              ?.textContent === '13%',
        );
        await snapshot('host-basis-final-value');
        record.checks.push('basis-select-preserves-guest-and-host-row-calculation');
        await page.evaluate(() => {
          const fixture = window.__guestAlertMemory;
          fixture.apply(
            {
              memory: {
                total: 4 * 1024 ** 3,
                used: 0,
                free: 4 * 1024 ** 3,
                usage: 0,
                observation: {
                  state: 'current',
                  source: 'agent',
                  observedAt: new Date().toISOString(),
                },
              },
            },
            0,
          );
        });
        await settle(page);
        assert.ok((await attention.innerText()).includes(basisNote));
        assert.doesNotMatch(await attention.innerText(), /\d+% of host capacity/);
        record.checks.push('zero-and-missing-host-capacity-do-not-rewrite-alert');
      }
      const geometry = await page.evaluate(() => ({
        width: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      }));
      assert.ok(geometry.scrollWidth <= geometry.width + 1, JSON.stringify(geometry));
      record.checks.push('no-page-overflow-or-errors');
      assert.deepEqual(record.errors, []);
      assert.equal(
        record.requests.some((request) => request.blocked || request.method !== 'GET'),
        false,
      );
      await browser.close();
      browser = null;
    }
    result.cleanup.browsers = true;
    result.result = original ? 'adverse-reproduced' : 'passed';
  } catch (error) {
    result.error = error.message;
    throw error;
  } finally {
    if (browser) {
      await browser.close();
      result.cleanup.browsers = true;
    }
    await server.close();
    result.cleanup.server = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: result.result,
        cases: result.cases.length,
        checks: result.cases.reduce((sum, item) => sum + item.checks.length, 0),
        captures: result.screenshots.length,
        cleanup: result.cleanup,
      }),
    );
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
